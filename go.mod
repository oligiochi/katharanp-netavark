module github.com/oligiochi/katharanp-netavark

go 1.21

require github.com/KatharaFramework/NetworkPluginLib v0.0.0-00010101000000-000000000000

require (
	github.com/containernetworking/plugins v1.3.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/vishvananda/netlink v1.2.1-beta.2 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	golang.org/x/sys v0.7.0 // indirect
)

// Reuse the VDE switch logic of the Kathará Docker plugin without copying it.
replace github.com/KatharaFramework/NetworkPluginLib => ./third_party/NetworkPlugin/vde/go-src/src/lib
