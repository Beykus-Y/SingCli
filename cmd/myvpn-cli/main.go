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

	// ----------------------------------------------------
	// РЕЖИМ ТЕСТИРОВАНИЯ
	// ----------------------------------------------------
	if *testMode {
		fmt.Printf("\n[Test] Testing connection to [%s] (%s)...\n", selected.Name, selected.Type)

		// 1. Собираем конфиг как для обычного прокси (без TUN)
		opts, err := config.BuildOptionsForServer(selected)
		if err != nil {
			log.Fatalf("[Test] Failed to build config: %v", err)
		}

		// 2. Запускаем ядро НАПРЯМУЮ, минуя tunnel.Manager,
		// чтобы sysproxy не изменял настройки Windows
		c := core.NewCore(core.Options{})
		if err := c.StartWithOptions(opts); err != nil {
			log.Fatalf("[Test] Core failed to start: %v", err)
		}
		defer c.Close()

		// Даем ядру секунду на инициализацию портов
		time.Sleep(1 * time.Second)

		// 3. Создаем HTTP-клиент, который работает через наш локальный HTTP-прокси Sing-box
		proxyURL, _ := url.Parse("http://" + httpAddr)
		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
			Timeout: 10 * time.Second, // Таймаут для проверки
		}

		// 4. Делаем тестовый запрос
		start := time.Now()
		resp, err := client.Get("https://www.google.com")

		if err != nil {
			fmt.Printf("❌ [Test] FAILED! Could not load page: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		elapsed := time.Since(start)
		if resp.StatusCode == http.StatusOK {
			fmt.Printf("✅[Test] SUCCESS! Connection is fully working. (Ping/Delay: %v)\n", elapsed)
		} else {
			fmt.Printf("⚠️ [Test] SUCCESS but got unexpected status: %s\n", resp.Status)
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
		fmt.Println()
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
		fmt.Println()
	}

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
