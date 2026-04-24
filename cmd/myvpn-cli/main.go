package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"SingCli/internal/buildinfo"
	"SingCli/internal/config"
	"SingCli/internal/core"
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

	tunMode := flag.Bool("tun", false, "Use TUN mode (requires administrator privileges)")
	configPath := flag.String("config", "servers.json", "Path to servers config file")
	serverName := flag.String("server", "", "Server name to connect (skip interactive selection)")
	testMode := flag.Bool("test", false, "Run connection test without modifying system routes and exit")
	diagnoseMode := flag.Bool("diagnose", false, "Run backend diagnostics without changing system proxy or routes")
	dumpConfig := flag.Bool("dump-config-redacted", false, "Print generated sing-box config with secrets redacted and exit")
	flag.Parse()

	servers, err := config.LoadServers(*configPath)
	if err != nil {
		logger.Errorf("Failed to load servers: %v", err)
		log.Fatalf("Failed to load servers: %v", err)
	}
	if len(servers) == 0 {
		log.Fatal("No servers found in config")
	}

	var selected config.ServerEntry

	if *serverName != "" {
		found := false
		for _, s := range servers {
			if s.Name == *serverName {
				selected = s
				found = true
				break
			}
		}
		if !found {
			log.Fatalf("Server %q not found", *serverName)
		}
	} else {
		selected = promptSelect(servers)
	}

	if *dumpConfig {
		data, err := config.BuildConfigJSONForServer(selected, *tunMode, true)
		if err != nil {
			log.Fatalf("Failed to build config: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	if *diagnoseMode {
		if err := runDiagnose(selected, *tunMode); err != nil {
			log.Fatalf("[Diagnose] FAILED: %v", err)
		}
		return
	}

	if err := ensureProtocolSupport(selected); err != nil {
		log.Fatal(err)
	}

	// ----------------------------------------------------
	// РЕЖИМ ТЕСТИРОВАНИЯ
	// ----------------------------------------------------
	if *testMode {
		if err := runConnectionTest(selected); err != nil {
			fmt.Printf("[Test] FAILED: %v\n", err)
			os.Exit(1)
		}
		return // Завершаем программу, тест пройден
	}

	// ----------------------------------------------------
	// ОБЫЧНЫЙ РЕЖИМ (TUN или Прокси с изменением настроек ОС)
	// ----------------------------------------------------
	var mgr *tunnel.Manager
	if *tunMode {
		o, err := config.BuildTunOptionsForServer(selected)
		if err != nil {
			logger.Errorf("Failed to build TUN config for %s: %v", selected.Name, err)
			log.Fatalf("Failed to build TUN config: %v", err)
		}
		mgr = tunnel.NewTun()
		if err := mgr.Start(o); err != nil {
			logger.Errorf("Failed to start TUN tunnel via %s: %v", selected.Name, err)
			log.Fatalf("Failed to start TUN tunnel: %v", err)
		}
		logger.Infof("Connected (TUN) via [%s] (%s)", selected.Name, selected.Type)
		fmt.Printf("\nConnected (TUN mode) via [%s] (%s)\n", selected.Name, selected.Type)
		fmt.Println("All traffic routed through VPN")
		fmt.Println("Press Ctrl+C to disconnect")
	} else {
		o, err := config.BuildOptionsForServer(selected)
		if err != nil {
			logger.Errorf("Failed to build config for %s: %v", selected.Name, err)
			log.Fatalf("Failed to build config: %v", err)
		}
		mgr = tunnel.New(httpAddr)
		if err := mgr.Start(o); err != nil {
			logger.Errorf("Failed to start tunnel via %s: %v", selected.Name, err)
			log.Fatalf("Failed to start tunnel: %v", err)
		}
		logger.Infof("Connected via [%s] (%s)", selected.Name, selected.Type)
		fmt.Printf("\nConnected via [%s] (%s)\n", selected.Name, selected.Type)
		fmt.Println("SOCKS5 proxy: 127.0.0.1:1080")
		fmt.Println("HTTP  proxy: 127.0.0.1:1081")
		fmt.Println("Press Ctrl+C to disconnect")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nDisconnecting...")
	logger.Infof("Disconnected from [%s]", selected.Name)
	mgr.Stop()
}

func ensureProtocolSupport(s config.ServerEntry) error {
	if s.Type == "hysteria2" && !buildinfo.QUICEnabled {
		return fmt.Errorf("hysteria2 requires QUIC support, but this binary was built without -tags \"with_quic,with_utls,with_gvisor\"; rebuild with `make build-windows`")
	}
	return nil
}

func runDiagnose(selected config.ServerEntry, tunMode bool) error {
	fmt.Printf("[Diagnose] Server: %s (%s)\n", selected.Name, selected.Type)
	fmt.Printf("[Diagnose] QUIC enabled: %t\n", buildinfo.QUICEnabled)

	if _, err := config.BuildOptionsForServer(selected); err != nil {
		return fmt.Errorf("proxy config validation: %w", err)
	}
	fmt.Println("[Diagnose] Proxy config validation: OK")

	if _, err := config.BuildTunOptionsForServer(selected); err != nil {
		return fmt.Errorf("TUN config validation: %w", err)
	}
	fmt.Println("[Diagnose] TUN config validation: OK")

	if tunMode && selected.Type == "hysteria2" {
		fmt.Println("[Diagnose] Hysteria2 TUN profile: MTU 1400, IPv4-only outbound resolution, direct bootstrap DNS")
	}

	if err := ensureProtocolSupport(selected); err != nil {
		return err
	}

	return runConnectionTest(selected)
}

func runConnectionTest(selected config.ServerEntry) error {
	fmt.Printf("\n[Test] Testing connection to [%s] (%s)...\n", selected.Name, selected.Type)

	opts, err := config.BuildOptionsForServer(selected)
	if err != nil {
		return fmt.Errorf("build config: %w", err)
	}

	c := core.NewCore(core.Options{})
	if err := c.StartWithOptions(opts); err != nil {
		return fmt.Errorf("core start: %w", err)
	}
	defer c.Close()

	time.Sleep(1 * time.Second)

	start := time.Now()
	resp, err := testHTTPViaProxy("https://www.google.com")
	if err != nil {
		return fmt.Errorf("could not load page: %w", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	if resp.StatusCode == http.StatusOK {
		fmt.Printf("[Test] SUCCESS! Connection is fully working. (Ping/Delay: %v)\n", elapsed)
	} else {
		fmt.Printf("[Test] SUCCESS but got unexpected status: %s\n", resp.Status)
	}
	return nil
}

func testHTTPViaProxy(target string) (*http.Response, error) {
	proxyURL, _ := url.Parse("http://" + httpAddr)
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 10 * time.Second,
	}
	return client.Get(target)
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
