// Package attach connects container interfaces to a VDE switch and keeps them alive.
//
// netavark runs the plugin once per operation and the plugin exits right after, so an interface cannot be
// held by a thread as in the Docker plugin: each interface is kept alive by its own long-running process.
// Everything about how that is done stays behind Attach/Detach/Count, so that the mechanism
// (vde_plug2tap today; vde_ext, a per-user daemon or anything else later) can change without touching
// the rest of the plugin.
package attach

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
// and returns the MAC address of the interface. On error, nothing must be left behind.
func Attach(iface Interface) (string, error) {
	// TODO (passo 3):
	//  1. os.MkdirAll(iface.HandleDir, 0o700)
	//  2. start `vde_plug2tap -s <SwitchCtl> -d -P <HandleDir>/<HandleID>.pid <Name>` inside NetnsPath
	//     (nsenter --net=<NetnsPath> ...), then wait for the tap to appear
	//  3. set the MAC (if any), bring the interface up, apply each sysctl (sysctl -w inside NetnsPath)
	//  4. read the actual MAC address and return it
	//  5. on any error after step 2: kill the process through its pidfile (rollback) and return the error
	panic("not implemented")
}

// Detach stops the process keeping an interface alive; the interface disappears with it.
func Detach(handleDir string, handleID string) error {
	// TODO (passo 3): kill the process of <handleDir>/<handleID>.pid and remove the pidfile.
	panic("not implemented")
}

// Count returns how many interfaces are still attached (i.e. have a live process) in handleDir.
func Count(handleDir string) int {
	// TODO (passo 3): count the pidfiles in handleDir whose process is still alive.
	panic("not implemented")
}
