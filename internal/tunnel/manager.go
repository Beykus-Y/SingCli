package tunnel

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"SingCli/internal/core"
	"SingCli/internal/logger"
	"SingCli/internal/sysproxy"

	"github.com/sagernet/sing-box/option"
)

const (
	healthCheckInterval = 5 * time.Second
	reconnectDelay      = 3 * time.Second
	maxReconnects       = 5
)

type Manager struct {
	mu        sync.Mutex
	core      *core.Core
	opts      option.Options
	proxyAddr string
	ctx       context.Context
	cancel    context.CancelFunc
}

func New(proxyAddr string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		proxyAddr: proxyAddr,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (m *Manager) Start(opts option.Options) error {
	m.opts = opts

	if err := m.startCore(); err != nil {
		return err
	}

	if err := sysproxy.Enable(m.proxyAddr); err != nil {
		log.Printf("Warning: failed to set system proxy: %v", err)
		logger.Warnf("Failed to set system proxy: %v", err)
	} else {
		log.Printf("System proxy set to %s", m.proxyAddr)
		logger.Infof("System proxy set to %s", m.proxyAddr)
	}

	go m.watchdog()
	return nil
}

func (m *Manager) Stop() {
	m.cancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := sysproxy.Disable(); err != nil {
		log.Printf("Warning: failed to disable system proxy: %v", err)
		logger.Warnf("Failed to disable system proxy: %v", err)
	} else {
		log.Println("System proxy disabled")
		logger.Info("System proxy disabled")
	}

	if m.core != nil {
		m.core.Close()
		m.core = nil
	}
}

func (m *Manager) startCore() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.core != nil {
		m.core.Close()
		m.core = nil
	}

	c := core.NewCore(core.Options{})
	if err := c.StartWithOptions(m.opts); err != nil {
		return fmt.Errorf("start core: %w", err)
	}
	m.core = c
	return nil
}

// watchdog периодически проверяет что SOCKS порт отвечает и перезапускает core если нет
func (m *Manager) watchdog() {
	socksAddr := "127.0.0.1:1080"
	attempts := 0

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(healthCheckInterval):
			if isPortAlive(socksAddr) {
				attempts = 0
				continue
			}

			if attempts >= maxReconnects {
				msg := "Max reconnect attempts reached, disabling proxy"
				log.Println(msg)
				logger.Error(msg)
				sysproxy.Disable()
				return
			}
			attempts++
			log.Printf("Health check failed, reconnecting (attempt %d/%d)...", attempts, maxReconnects)
			logger.Warnf("Health check failed, reconnect attempt %d/%d", attempts, maxReconnects)

			time.Sleep(reconnectDelay)
			if err := m.startCore(); err != nil {
				log.Printf("Reconnect failed: %v", err)
				logger.Errorf("Reconnect failed: %v", err)
			} else {
				log.Println("Reconnected successfully")
				logger.Info("Reconnected successfully")
			}
		}
	}
}

func isPortAlive(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
