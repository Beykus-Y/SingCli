TAGS = with_quic,with_utls,with_gvisor
OUTPUT = bin/singcli.exe
GUI_OUTPUT = bin/mgb-gui.exe
GOCACHE_DIR ?= /tmp/go-build-cache

build: build-windows

build-windows:
	GOCACHE=$(GOCACHE_DIR) GOOS=windows GOARCH=amd64 go build -tags "$(TAGS)" -o $(OUTPUT) ./cmd/myvpn-cli

gui:
	cd cmd/myvpn-gui && wails build -clean -platform windows/amd64 -tags "$(TAGS)"
	mkdir -p bin
	cp build/bin/mgb-gui.exe $(GUI_OUTPUT)

test:
	GOCACHE=$(GOCACHE_DIR) go test ./...
	GOCACHE=$(GOCACHE_DIR) GOOS=windows GOARCH=amd64 go test -exec=true -tags "$(TAGS)" ./...

.PHONY: build build-windows gui test
