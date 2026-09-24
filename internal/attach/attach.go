// Package attach connects container interfaces to a VDE switch and keeps them alive.
//
// netavark runs the plugin once per operation and the plugin exits right after, so an interface cannot be
// held by a thread as in the Docker plugin: each interface is kept alive by its own long-running process.
// Everything about how that is done stays behind Attach/Detach/Count, so that the mechanism
// (vde_plug2tap today; vde_ext, a per-user daemon or anything else later) can change without touching
// the rest of the plugin.
package attach

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// tapWaitTimeout is how long Attach waits for vde_plug2tap to create the tap.
	tapWaitTimeout = 2 * time.Second
	// tapWaitInterval is the pause between two checks.
	tapWaitInterval = 50 * time.Millisecond
)

// Interface is a container interface to attach to a switch.
type Interface struct {
	NetnsPath string            // network namespace of the container (argument of setup)
	SwitchCtl string            // control directory of the VDE switch
	Name      string            // interface name inside the container, e.g. eth1
	MAC       string            // static MAC address, empty for a random one
	Sysctls   map[string]string // per-interface sysctls, already resolved (no prefix, no IFNAME)
	HandleDir string            // where the handle (e.g. a pidfile) of the interface is kept
	HandleID  string            // unique id of the interface within HandleDir
}

// Attach creates the interface inside the container, connects it to the switch, applies MAC and sysctls,
// and returns the MAC address of the interface. On error, nothing is left behind.
//
// The interface is a tap created by vde_plug2tap, started inside the container network namespace so that
// the tap is created there, and kept alive by that process (see Detach).
func Attach(iface Interface) (mac string, err error) {
	if err = os.MkdirAll(iface.HandleDir, 0o700); err != nil {
		return "", fmt.Errorf("cannot create %s: %w", iface.HandleDir, err)
	}

	pidFile := pidFilePath(iface.HandleDir, iface.HandleID)
	if _, err = runInNetns(iface.NetnsPath,
		"vde_plug2tap", "-s", iface.SwitchCtl, "-d", "-P", pidFile, iface.Name); err != nil {
		return "", fmt.Errorf("cannot attach %s to the switch: %w", iface.Name, err)
	}

	// From here on, any error must stop vde_plug2tap: the tap disappears with it.
	defer func() {
		if err != nil {
			_ = Detach(iface.HandleDir, iface.HandleID)
		}
	}()

	if err = waitForInterface(iface.NetnsPath, iface.Name); err != nil {
		return "", err
	}

	if iface.MAC != "" {
		if _, err = runInNetns(iface.NetnsPath, "ip", "link", "set", iface.Name, "address", iface.MAC); err != nil {
			return "", fmt.Errorf("cannot set MAC address of %s: %w", iface.Name, err)
		}
	}

	if _, err = runInNetns(iface.NetnsPath, "ip", "link", "set", iface.Name, "up"); err != nil {
		return "", fmt.Errorf("cannot bring %s up: %w", iface.Name, err)
	}

	for key, value := range iface.Sysctls {
		if _, err = runInNetns(iface.NetnsPath, "sysctl", "-w", key+"="+value); err != nil {
			return "", fmt.Errorf("cannot apply sysctl %s=%s: %w", key, value, err)
		}
	}

	out, err := runInNetns(iface.NetnsPath, "cat", "/sys/class/net/"+iface.Name+"/address")
	if err != nil {
		return "", fmt.Errorf("cannot read MAC address of %s: %w", iface.Name, err)
	}
	return strings.TrimSpace(out), nil
}

// waitForInterface waits until an interface exists in a network namespace.
func waitForInterface(netnsPath string, name string) error {
	deadline := time.Now().Add(tapWaitTimeout)
	for {
		if _, err := runInNetns(netnsPath, "ip", "link", "show", name); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("interface %s did not appear within %s", name, tapWaitTimeout)
		}
		time.Sleep(tapWaitInterval)
	}
}

// runInNetns runs a command inside a network namespace and returns its stdout.
// On failure, the error includes the command stderr, which is what the user ends up seeing.
func runInNetns(netnsPath string, args ...string) (string, error) {
	cmd := exec.Command("nsenter", append([]string{"--net=" + netnsPath}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%s: %s", strings.Join(args, " "), msg)
		}
		return "", fmt.Errorf("%s: %w", strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}

// pidFilePath returns the pidfile of the process keeping an interface alive.
func pidFilePath(handleDir string, handleID string) string {
	return filepath.Join(handleDir, handleID+".pid")
}

// Detach stops the process keeping an interface alive; the interface disappears with it.
// A missing pidfile or an already dead process are not errors.
func Detach(handleDir string, handleID string) error {
	pidFile := pidFilePath(handleDir, handleID)

	pid, err := readPID(pidFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		// Corrupted pidfile: remove it anyway, otherwise every later teardown would fail the same way.
		_ = os.Remove(pidFile)
		return fmt.Errorf("invalid pidfile %s: %w", pidFile, err)
	}

	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("cannot stop interface process %d: %w", pid, err)
	}

	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove pidfile %s: %w", pidFile, err)
	}
	return nil
}

// Count returns how many interfaces are still attached (i.e. have a live process) in handleDir.
func Count(handleDir string) int {
	entries, err := os.ReadDir(handleDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".pid") {
			continue
		}
		if pid, err := readPID(filepath.Join(handleDir, entry.Name())); err == nil && alive(pid) {
			count++
		}
	}
	return count
}

// readPID returns the pid stored in a pidfile.
func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// alive reports whether a process exists (EPERM means it exists but belongs to someone else).
func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}