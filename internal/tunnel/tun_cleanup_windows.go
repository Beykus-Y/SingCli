//go:build windows

package tunnel

import (
	"os/exec"
	"strings"

	"SingCli/internal/tunsettings"
)

func cleanupTun() {
	cmd := exec.Command("powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Command",
		"Get-NetIPAddress -IPAddress '"+tunsettings.IPv4Address+"' -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue",
	)
	_ = cmd.Run()
}

func isTunAddressAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "object already exists") ||
		strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "уже существует")
}
