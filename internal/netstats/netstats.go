package netstats

import "strings"

type Mode string

const (
	ModeProxy Mode = "proxy"
	ModeTun   Mode = "tun"
)

type Counters struct {
	DownloadBytes uint64
	UploadBytes   uint64
}

type AdapterSnapshot struct {
	Name          string
	Description   string
	DownloadBytes uint64
	UploadBytes   uint64
	Up            bool
	Loopback      bool
}

func Aggregate(adapters []AdapterSnapshot, mode Mode, tunName string) (Counters, bool) {
	var total Counters
	matched := false
	tunName = strings.ToLower(strings.TrimSpace(tunName))

	for _, adapter := range adapters {
		if !adapter.Up || adapter.Loopback {
			continue
		}
		if mode == ModeTun {
			name := strings.ToLower(adapter.Name)
			description := strings.ToLower(adapter.Description)
			if tunName == "" || (name != tunName && !strings.Contains(description, tunName)) {
				continue
			}
		}
		total.DownloadBytes += adapter.DownloadBytes
		total.UploadBytes += adapter.UploadBytes
		matched = true
	}
	return total, matched
}
