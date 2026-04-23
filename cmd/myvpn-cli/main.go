package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"SingCli/internal/config"
	"SingCli/internal/tunnel"
)

const socksAddr = "127.0.0.1:1080"
const httpAddr = "127.0.0.1:1081"

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: singcli servers.json <server-name>")
		os.Exit(1)
	}

	configPath := os.Args[1]
	serverName := os.Args[2]

	opts, err := config.LoadServerByName(configPath, serverName)
	if err != nil {
		log.Fatalf("Failed to load server config: %v", err)
	}

	mgr := tunnel.New(httpAddr)

	if err := mgr.Start(opts); err != nil {
		log.Fatalf("Failed to start tunnel: %v", err)
	}

	log.Printf("VPN started via %s, press Ctrl+C to stop", serverName)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	mgr.Stop()
}
