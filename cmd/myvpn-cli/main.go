package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"SingCli/internal/config"
	"SingCli/internal/logger"
	"SingCli/internal/tunnel"
)

const (
	socksAddr = "127.0.0.1:1080"
	httpAddr  = "127.0.0.1:1081"
)

func main() {
	exePath, _ := os.Executable()
	logger.Init(exePath)

	configPath := "servers.json"
	if len(os.Args) >= 2 {
		configPath = os.Args[1]
	}

	servers, err := config.LoadServers(configPath)
	if err != nil {
		logger.Errorf("Failed to load servers: %v", err)
		log.Fatalf("Failed to load servers: %v", err)
	}
	if len(servers) == 0 {
		log.Fatal("No servers found in config")
	}

	var selected config.ServerEntry

	if len(os.Args) >= 3 {
		name := os.Args[2]
		found := false
		for _, s := range servers {
			if s.Name == name {
				selected = s
				found = true
				break
			}
		}
		if !found {
			log.Fatalf("Server %q not found", name)
		}
	} else {
		selected = promptSelect(servers)
	}

	opts, err := config.BuildOptionsForServer(selected)
	if err != nil {
		logger.Errorf("Failed to build config for %s: %v", selected.Name, err)
		log.Fatalf("Failed to build config: %v", err)
	}

	mgr := tunnel.New(httpAddr)
	if err := mgr.Start(opts); err != nil {
		logger.Errorf("Failed to start tunnel via %s: %v", selected.Name, err)
		log.Fatalf("Failed to start tunnel: %v", err)
	}

	logger.Infof("Connected via [%s] (%s)", selected.Name, selected.Type)
	fmt.Printf("\nConnected via [%s] (%s)\n", selected.Name, selected.Type)
	fmt.Println("SOCKS5 proxy: 127.0.0.1:1080")
	fmt.Println("HTTP  proxy: 127.0.0.1:1081")
	fmt.Println("Press Ctrl+C to disconnect\n")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nDisconnecting...")
	logger.Infof("Disconnected from [%s]", selected.Name)
	mgr.Stop()
}

func promptSelect(servers []config.ServerEntry) config.ServerEntry {
	fmt.Println("Available servers:")
	for i, s := range servers {
		fmt.Printf("  [%d] %s (%s)\n", i+1, s.Name, s.Type)
	}
	fmt.Print("\nSelect server [1]: ")

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "" {
		return servers[0]
	}

	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(servers) {
		fmt.Printf("Invalid choice, using [1] %s\n", servers[0].Name)
		return servers[0]
	}
	return servers[n-1]
}
