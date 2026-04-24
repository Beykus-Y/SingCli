//go:build windows

package sysproxy

import (
	"fmt"
	"os/exec"

	"golang.org/x/sys/windows/registry"
)

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

func Enable(addr string) error {
	if err := setRegistry(addr); err != nil {
		return err
	}
	// WinHTTP используется Chrome/Edge — без него браузер игнорирует прокси
	setWinHTTP(addr)
	return nil
}

func Disable() error {
	if err := clearRegistry(); err != nil {
		return err
	}
	resetWinHTTP()
	return nil
}

func setRegistry(addr string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry key: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue("ProxyServer", addr); err != nil {
		return fmt.Errorf("set ProxyServer: %w", err)
	}
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("set ProxyEnable: %w", err)
	}
	return nil
}

func clearRegistry() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry key: %w", err)
	}
	defer k.Close()

	if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
		return fmt.Errorf("set ProxyEnable: %w", err)
	}
	return nil
}

func setWinHTTP(addr string) {
	exec.Command("netsh", "winhttp", "set", "proxy", addr).Run()
}

func resetWinHTTP() {
	exec.Command("netsh", "winhttp", "reset", "proxy").Run()
}
