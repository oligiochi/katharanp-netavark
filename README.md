# katharanp-netavark

Netavark plugin of [Kathará](https://www.kathara.org): pure L2 collision domains backed by VDE switches,
for the Podman backend. Counterpart of the Docker plugin `kathara/katharanp_vde`
([NetworkPlugin](https://github.com/KatharaFramework/NetworkPlugin)), whose switch library is reused as a
git submodule.

## Build

Requires Go, and `vdeplug4` (headers and `libvdeplug`) for the reused library.

```bash
git submodule update --init
make install    # installs into ~/.local/libexec/netavark-plugins
```

Runtime dependencies: `vde_switch`, `vde_plug2tap`, `nsenter`, `ip`, `sysctl`.
