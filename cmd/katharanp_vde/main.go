// katharanp_vde is the netavark plugin of Kathará: pure L2 collision domains backed by VDE switches.
//
// netavark runs it once per operation, with the subcommand as first argument:
//
//	info                  -> print plugin and API version
//	create                -> validate a network definition (stdin) and echo it back (stdout)
//	setup    <netns-path> -> attach a container interface (stdin: PluginExec, stdout: StatusBlock)
//	teardown <netns-path> -> detach a container interface (stdin: PluginExec)
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/oligiochi/katharanp-netavark/internal/netavark"
	"github.com/oligiochi/katharanp-netavark/internal/plugin"
)

// version is set at build time: go build -ldflags "-X main.version=..."
var version = "dev"

// init puts the plugin directory first in PATH: the plugin starts vde_switch and vde_plug2tap by name,
// and when it is distributed as a bundle they sit next to it. netavark runs the plugin with its own
// environment, which does not include that directory.

func init() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	os.Setenv("PATH", filepath.Dir(exe)+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("missing subcommand"))
	}

	var err error
	switch cmd := os.Args[1]; cmd {
	case "info":
		err = writeJSON(netavark.Info{Version: version, APIVersion: netavark.APIVersion})
	case "create":
		err = runCreate()
	case "setup", "teardown":
		if len(os.Args) < 3 {
			fail(fmt.Errorf("%s: missing netns path", cmd))
		}
		err = runExec(cmd, os.Args[2])
	default:
		err = fmt.Errorf("unknown subcommand %q", cmd)
	}

	if err != nil {
		fail(err)
	}
}

func runCreate() error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	var network netavark.Network
	if err := json.Unmarshal(raw, &network); err != nil {
		return fmt.Errorf("invalid network definition: %w", err)
	}
	if err := plugin.Validate(network); err != nil {
		return err
	}
	// Echo the definition back unchanged, so that fields this plugin does not model are preserved.
	_, err = os.Stdout.Write(raw)
	return err
}

func runExec(cmd string, netnsPath string) error {
	var payload netavark.PluginExec
	if err := json.NewDecoder(os.Stdin).Decode(&payload); err != nil {
		return fmt.Errorf("invalid %s payload: %w", cmd, err)
	}

	if cmd == "teardown" {
		return plugin.Teardown(netnsPath, payload)
	}

	status, err := plugin.Setup(netnsPath, payload)
	if err != nil {
		return err
	}
	return writeJSON(status)
}

func writeJSON(v interface{}) error {
	return json.NewEncoder(os.Stdout).Encode(v)
}

func fail(err error) {
	_ = writeJSON(netavark.Error{Error: err.Error()})
	os.Exit(1)
}
