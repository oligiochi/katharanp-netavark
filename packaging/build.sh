#!/usr/bin/env bash
# Build the relocatable bundle and copy it to ./dist
set -euo pipefail
cd "$(dirname "$0")/.."
git submodule update --init
podman build -t katharanp-bundle -f packaging/Containerfile .
mkdir -p dist
id=$(podman create katharanp-bundle)
podman cp "$id":/out/katharanp-bundle-amd64.tar.gz dist/
podman rm "$id" >/dev/null
ls -la dist/
