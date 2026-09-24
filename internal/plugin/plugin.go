// Package plugin implements the operations of the Kathará netavark plugin.
//
// A collision domain is a VDE switch (created with the Kathará NetworkPlugin library), started when the
// first interface is attached and stopped when the last one is detached: netavark has no "network
// removed" hook. Each container interface is attached through the attach package.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	katnplib "github.com/KatharaFramework/NetworkPluginLib"

	"github.com/oligiochi/katharanp-netavark/internal/attach"
	"github.com/oligiochi/katharanp-netavark/internal/netavark"
)

// SysctlOptionPrefix marks per-interface sysctls in network and per-connection options.
// IFNAME in the key is replaced with the interface name (same convention as Docker endpoint sysctls).
const SysctlOptionPrefix = "sysctl."

// stateDir is where switches and interface handles are kept: per user, cleaned at logout.
func stateDir() string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "katharanp") + "/"
}

func init() {
	// Requires the upstream change making pluginPath configurable (default: /hosttmp/katharanp/).
	katnplib.SetPluginPath(stateDir())
}

// Validate checks the options of a network definition at `create` time.
func Validate(network netavark.Network) error {
	for key := range network.Options {
		if !strings.HasPrefix(key, SysctlOptionPrefix) {
			return fmt.Errorf("unsupported network option %q", key)
		}
	}
	return nil
}

// Setup attaches a container interface to the collision domain switch.
func Setup(netnsPath string, payload netavark.PluginExec) (*netavark.StatusBlock, error) {
	ifname := payload.NetworkOptions.InterfaceName
	if ifname == "" {
		return nil, fmt.Errorf("interface_name is required")
	}

	switchName, err := ensureSwitch(payload.Network.ID)
	if err != nil {
		return nil, fmt.Errorf("cannot start switch: %w", err)
	}

	iface := attach.Interface{
		NetnsPath: netnsPath,
		SwitchCtl: switchCtlPath(switchName),
		Name:      ifname,
		MAC:       payload.NetworkOptions.StaticMAC,
		Sysctls:   sysctls(payload.Network.Options, payload.NetworkOptions.Options, ifname),
		HandleDir: handleDir(switchName),
		HandleID:  payload.ContainerID + "-" + ifname,
	}

	mac, err := attach.Attach(iface)
	if err != nil {
		stopSwitchIfUnused(payload.Network.ID, switchName)
		return nil, err
	}

	return &netavark.StatusBlock{
		DNSSearchDomains: []string{},
		DNSServerIPs:     []string{},
		Interfaces: map[string]netavark.NetInterface{
			ifname: {MACAddress: mac, Subnets: []interface{}{}},
		},
	}, nil
}

// Teardown detaches a container interface and stops the switch if it has no interfaces left.
func Teardown(netnsPath string, payload netavark.PluginExec) error {
	switchName := switchNameFor(payload.Network.ID)
	err := attach.Detach(handleDir(switchName), payload.ContainerID+"-"+payload.NetworkOptions.InterfaceName)
	stopSwitchIfUnused(payload.Network.ID, switchName)
	return err
}

// sysctls merges network-level and per-connection sysctl options (the latter take precedence),
// strips the prefix and replaces the IFNAME placeholder.
func sysctls(networkOpts, connectionOpts map[string]string, ifname string) map[string]string {
	result := map[string]string{}
	for _, source := range []map[string]string{networkOpts, connectionOpts} {
		for key, value := range source {
			if strings.HasPrefix(key, SysctlOptionPrefix) {
				key = strings.ReplaceAll(strings.TrimPrefix(key, SysctlOptionPrefix), "IFNAME", ifname)
				result[key] = value
			}
		}
	}
	return result
}

// ensureSwitch starts the switch of a network if it is not running yet.
func ensureSwitch(networkID string) (string, error) {
	switchName := switchNameFor(networkID)
	if switchRunning(switchName) {
		return switchName, nil
	}
	return katnplib.CreateSwitch(networkID)
}

func stopSwitchIfUnused(networkID string, switchName string) {
	if attach.Count(handleDir(switchName)) == 0 && switchRunning(switchName) {
		_ = katnplib.DeleteSwitch(networkID)
	}
}

// The helpers below mirror the (unexported) naming of katnplib's switch_utils.go.

func switchNameFor(networkID string) string {
	return "kt-" + networkID[:12]
}

func switchCtlPath(switchName string) string {
	return stateDir() + switchName + "/ctl"
}

func handleDir(switchName string) string {
	return stateDir() + switchName + "/ifaces"
}

func switchRunning(switchName string) bool {
	_, err := os.Stat(stateDir() + switchName + "/pid")
	return err == nil
}
