package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/oligiochi/katharanp-netavark/internal/attach"
	"github.com/oligiochi/katharanp-netavark/internal/netavark"
)

const testNetworkID = "963ca160b022aabbccddeeff00112233"

// fakeSwitches replaces the process-starting operations with fakes that only touch the state directory
// (a temporary XDG_RUNTIME_DIR), and makes every attach fail. It returns a counter of deleteSwitch calls.
func fakeSwitches(t *testing.T) *int {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	deleted := 0
	origCreate, origDelete, origAttach := createSwitch, deleteSwitch, attachIface
	t.Cleanup(func() { createSwitch, deleteSwitch, attachIface = origCreate, origDelete, origAttach })

	createSwitch = func(networkID string) (string, error) {
		name := switchNameFor(networkID)
		if err := os.MkdirAll(stateDir()+name, 0o700); err != nil {
			return "", err
		}
		// switchRunning only checks that the pidfile exists.
		return name, os.WriteFile(stateDir()+name+"/pid", []byte("1\n"), 0o600)
	}
	deleteSwitch = func(networkID string) error {
		deleted++
		return os.RemoveAll(stateDir() + switchNameFor(networkID))
	}
	attachIface = func(attach.Interface) (string, error) {
		return "", errors.New("attach failed")
	}
	return &deleted
}

func payload(ifname string) netavark.PluginExec {
	return netavark.PluginExec{
		ContainerID:    "c1",
		Network:        netavark.Network{ID: testNetworkID},
		NetworkOptions: netavark.PerNetworkOptions{InterfaceName: ifname},
	}
}

// A failed attach of the first interface must not leave the switch it started behind.
func TestSetupFirstInterfaceFailureStopsSwitch(t *testing.T) {
	deleted := fakeSwitches(t)

	if _, err := Setup("/proc/self/ns/net", payload("eth0")); err == nil {
		t.Fatal("Setup succeeded, want the attach error")
	}
	if *deleted != 1 {
		t.Errorf("deleteSwitch called %d times, want 1", *deleted)
	}
	if switchRunning(switchNameFor(testNetworkID)) {
		t.Error("switch still running after the rollback")
	}
}

// A failed attach must not stop a switch that still has other interfaces attached.
func TestSetupFailureKeepsSwitchInUse(t *testing.T) {
	deleted := fakeSwitches(t)

	switchName, err := createSwitch(testNetworkID)
	if err != nil {
		t.Fatal(err)
	}
	// Another interface kept alive by a live process: the test process itself.
	if err := os.MkdirAll(handleDir(switchName), 0o700); err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(handleDir(switchName), "c0-eth0.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Setup("/proc/self/ns/net", payload("eth1")); err == nil {
		t.Fatal("Setup succeeded, want the attach error")
	}
	if *deleted != 0 {
		t.Errorf("deleteSwitch called %d times, want 0", *deleted)
	}
	if !switchRunning(switchName) {
		t.Error("switch stopped while another interface is still attached")
	}
}
