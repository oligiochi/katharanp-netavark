module github.com/oligiochi/katharanp-netavark

go 1.21

require github.com/KatharaFramework/NetworkPluginLib v0.0.0-00010101000000-000000000000

// Reuse the VDE switch logic of the Kathará Docker plugin without copying it.
replace github.com/KatharaFramework/NetworkPluginLib => ./third_party/NetworkPlugin/vde/go-src/src/lib
