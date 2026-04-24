package tunnel

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
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
	tunTeardownDelay    = 1200 * time.Millisecond
	tunStartRetries     = 3
)

var tunLifecycleMu sync.Mutex

type Manager struct {
	mu        sync.Mutex
	core      *core.Core
	opts      option.Options
	proxyAddr string
	tunMode   bool
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

func NewTun() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		tunMode: true,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (m *Manager) Start(opts option.Options) error {
	m.opts = opts

	if m.tunMode {
		if err := sysproxy.Disable(); err != nil {
			log.Printf("Warning: failed to clear system proxy before TUN start: %v", err)
		}
	}

	if err := m.startCore(); err != nil {
		return err
	}

	if !m.tunMode {
		if err := sysproxy.Enable(m.proxyAddr); err != nil {
			log.Printf("Warning: failed to set system proxy: %v", err)
			logger.Warnf("Failed to set system proxy: %v", err)
		} else {
			log.Printf("System proxy set to %s", m.proxyAddr)
			logger.Infof("System proxy set to %s", m.proxyAddr)
		}
	}

	go m.watchdog()
	return nil
}

func (m *Manager) Stop() {
	m.cancel()

	if m.tunMode {
		tunLifecycleMu.Lock()
		defer tunLifecycleMu.Unlock()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.tunMode {
		if err := sysproxy.Disable(); err != nil {
			log.Printf("Warning: failed to disable system proxy: %v", err)
			logger.Warnf("Failed to disable system proxy: %v", err)
		} else {
			log.Println("System proxy disabled")
			logger.Info("System proxy disabled")
		}
	}

	if m.core != nil {
		m.core.Close()
		m.core = nil
	}

	if m.tunMode {
		cleanupTun()
		time.Sleep(tunTeardownDelay)
	}
}

func (m *Manager) startCore() error {
	if m.tunMode {
		tunLifecycleMu.Lock()
		defer tunLifecycleMu.Unlock()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.core != nil {
		m.core.Close()
		m.core = nil
		if m.tunMode {
			cleanupTun()
			time.Sleep(tunTeardownDelay)
		}
	}

	attempts := 1
	if m.tunMode {
		attempts = tunStartRetries
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if m.tunMode {
			cleanupTun()
		}

		c := core.NewCore(core.Options{})
		if err := c.StartWithOptions(m.opts); err != nil {
			_ = c.Close()
			lastErr = fmt.Errorf("start core: %w", err)
			if m.tunMode && isTunAddressAlreadyExists(err) && attempt < attempts {
				log.Printf("TUN address still exists, retrying startup (%d/%d)...", attempt+1, attempts)
				logger.Warnf("TUN address still exists, retrying startup (%d/%d)", attempt+1, attempts)
				cleanupTun()
				time.Sleep(time.Duration(attempt) * tunTeardownDelay)
				continue
			}
			return lastErr
		}
		m.core = c
		return nil
	}

	return lastErr
}

func (m *Manager) watchdog() {
	if m.tunMode {
		m.watchdogTun()
	} else {
		m.watchdogProxy()
	}
}

func (m *Manager) watchdogProxy() {
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

func (m *Manager) watchdogTun() {
	attempts := 0

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(healthCheckInterval):
			if err := tunHealthCheck(m.ctx); err == nil {
				attempts = 0
				continue
			} else {
				log.Printf("TUN health check failed: %v", err)
				logger.Warnf("TUN health check failed: %v", err)
			}

			if attempts >= maxReconnects {
				msg := "TUN: max reconnect attempts reached, giving up"
				log.Println(msg)
				logger.Error(msg)
				return
			}
			attempts++
			log.Printf("TUN health check failed, reconnecting (attempt %d/%d)...", attempts, maxReconnects)
			logger.Warnf("TUN health check failed, reconnect attempt %d/%d", attempts, maxReconnects)

			time.Sleep(reconnectDelay)
			if err := m.startCore(); err != nil {
				log.Printf("TUN reconnect failed: %v", err)
				logger.Errorf("TUN reconnect failed: %v", err)
			} else {
				log.Println("TUN reconnected successfully")
				logger.Info("TUN reconnected successfully")
			}
		}
	}
}

func tunHealthCheck(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()

	resolver := net.DefaultResolver
	if _, err := resolver.LookupHost(ctx, "www.google.com"); err != nil {
		return fmt.Errorf("dns lookup www.google.com: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.google.com/generate_204", nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: nil},
		Timeout:   8 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http probe: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("http probe status: %s", resp.Status)
	}
	return nil
}

func isPortAlive(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 4*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
