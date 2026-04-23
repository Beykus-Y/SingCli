TAGS = with_quic,with_utls,with_xtls
OUTPUT = bin/singcli.exe

build:
	GOOS=windows GOARCH=amd64 go build -tags "$(TAGS)" -o $(OUTPUT) ./cmd/myvpn-cli

.PHONY: build
