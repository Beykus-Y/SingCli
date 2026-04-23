package tunnel

import (
	"context"
	"fmt"
	"log"

	"SingCli/internal/core"
	"SingCli/internal/sysproxy"

	"github.com/sagernet/sing-box/option"
)

// Manager управляет жизненным циклом VPN: sing-box + системный прокси
type Manager struct {
	core       *core.Core
	proxyAddr  string
	ctx        context.Context
	cancel     context.CancelFunc
}

// New создаёт Manager с указанным SOCKS адресом прокси
func New(proxyAddr string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		proxyAddr: proxyAddr,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start запускает sing-box с переданными опциями и включает системный прокси
func (m *Manager) Start(opts option.Options) error {
	m.core = core.NewCore(core.Options{})

	if err := m.core.StartWithOptions(opts); err != nil {
		return fmt.Errorf("start core: %w", err)
	}

	if err := sysproxy.Enable(m.proxyAddr); err != nil {
		// Не фатально — VPN работает, просто прокси не выставлен
		log.Printf("Warning: failed to set system proxy: %v", err)
	} else {
		log.Printf("System proxy set to %s", m.proxyAddr)
	}

	return nil
}

// Stop останавливает sing-box и снимает системный прокси
func (m *Manager) Stop() {
	if err := sysproxy.Disable(); err != nil {
		log.Printf("Warning: failed to disable system proxy: %v", err)
	} else {
		log.Println("System proxy disabled")
	}

	if m.core != nil {
		if err := m.core.Close(); err != nil {
			log.Printf("Warning: failed to close core: %v", err)
		}
	}

	m.cancel()
}
