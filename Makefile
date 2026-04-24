TAGS = with_quic,with_utls,with_gvisor
OUTPUT = bin/singcli.exe
GOCACHE_DIR ?= /tmp/go-build-cache

build: build-windows

build-windows:
	GOCACHE=$(GOCACHE_DIR) GOOS=windows GOARCH=amd64 go build -tags "$(TAGS)" -o $(OUTPUT) ./cmd/myvpn-cli

test:
	GOCACHE=$(GOCACHE_DIR) go test ./...
	GOCACHE=$(GOCACHE_DIR) GOOS=windows GOARCH=amd64 go test -exec=true -tags "$(TAGS)" ./...

.PHONY: build build-windows test
