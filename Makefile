BINARY   := katharanp_vde
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PLUGDIR  ?= $(HOME)/.local/libexec/netavark-plugins

.PHONY: build install clean

build:
	CGO_ENABLED=1 go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY) ./cmd/katharanp_vde

install: build
	install -D -m 755 bin/$(BINARY) $(PLUGDIR)/$(BINARY)

clean:
	rm -rf bin
