//go:build windows

package deviceid

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const machineGuidKey = `SOFTWARE\Microsoft\Cryptography`

type Info struct {
	ID   string
	Name string
}

func Get() Info {
	name, _ := os.Hostname()
	id := machineGuid()
	if id == "" {
		id = fallbackID(name)
	}
	return Info{ID: id, Name: name}
}

func machineGuid() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, machineGuidKey, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return ""
	}
	defer key.Close()

	value, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func fallbackID(hostname string) string {
	var macs []string
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if len(iface.HardwareAddr) == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		macs = append(macs, strings.ToLower(iface.HardwareAddr.String()))
	}
	sort.Strings(macs)
	sum := sha256.Sum256([]byte(strings.Join(macs, "|") + "|" + strings.ToLower(hostname)))
	return hex.EncodeToString(sum[:])
}
