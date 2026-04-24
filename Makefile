TAGS = with_quic,with_utls,with_xtls,with_gvisor
OUTPUT = bin/singcli.exe
GUI_OUTPUT = bin/mgb-gui.exe

build:
	GOOS=windows GOARCH=amd64 go build -tags "$(TAGS)" -o $(OUTPUT) ./cmd/myvpn-cli

gui:
	cd cmd/myvpn-gui && wails build -clean -platform windows/amd64 -tags "$(TAGS)"
	mkdir -p bin
	cp build/bin/mgb-gui.exe $(GUI_OUTPUT)

.PHONY: build gui
