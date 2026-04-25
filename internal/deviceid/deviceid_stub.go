//go:build !windows

package deviceid

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"sort"
	"strings"
)

type Info struct {
	ID   string
	Name string
}

func Get() Info {
	name, _ := os.Hostname()
	var macs []string
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if len(iface.HardwareAddr) == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		macs = append(macs, strings.ToLower(iface.HardwareAddr.String()))
	}
	sort.Strings(macs)
	sum := sha256.Sum256([]byte(strings.Join(macs, "|") + "|" + strings.ToLower(name)))
	return Info{ID: hex.EncodeToString(sum[:]), Name: name}
}
