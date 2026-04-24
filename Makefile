TAGS = with_quic,with_utls,with_xtls,with_gvisor
OUTPUT = bin/singcli.exe
GUI_OUTPUT = bin/mgb-gui.exe
GOCACHE_DIR ?= /tmp/go-build-cache
GOARCH ?= amd64
WINTUN_ARCH = $(if $(filter 386,$(GOARCH)),x86,$(GOARCH))
WAILS_VERSION ?= v2.12.0
GOPATH_DIR := $(shell go env GOPATH)
WAILS_BIN := $(shell if command -v wails >/dev/null 2>&1; then command -v wails; else printf '%s/bin/wails' '$(GOPATH_DIR)'; fi)
FRONTEND_FLAG := $(shell if command -v npm >/dev/null 2>&1; then printf ''; else printf '-s'; fi)

build: build-windows gui

build-windows:
	GOCACHE=$(GOCACHE_DIR) GOOS=windows GOARCH=$(GOARCH) go build -tags "$(TAGS)" -o $(OUTPUT) ./cmd/myvpn-cli

$(WAILS_BIN):
	go install github.com/wailsapp/wails/v2/cmd/wails@$(WAILS_VERSION)

gui: $(WAILS_BIN)
	cd cmd/myvpn-gui && "$(WAILS_BIN)" build -clean -platform windows/$(GOARCH) -tags "$(TAGS)" $(FRONTEND_FLAG)
	mkdir -p bin build/bin
	chmod -f u+w $(GUI_OUTPUT) bin/wintun.dll build/bin/wintun.dll 2>/dev/null || true
	cp -f build/bin/mgb-gui.exe $(GUI_OUTPUT) || { echo "Cannot overwrite $(GUI_OUTPUT). Close MGB VPN or kill the running mgb-gui.exe process, then rerun make build."; exit 1; }
	cp -f "$$(go list -m -f '{{.Dir}}' github.com/sagernet/sing-tun)/internal/wintun/$(WINTUN_ARCH)/wintun.dll" bin/wintun.dll
	cp bin/wintun.dll build/bin/wintun.dll

test:
	GOCACHE=$(GOCACHE_DIR) go test ./...
	GOCACHE=$(GOCACHE_DIR) GOOS=windows GOARCH=$(GOARCH) go test -exec=true -tags "$(TAGS)" ./...

.PHONY: build build-windows gui test
