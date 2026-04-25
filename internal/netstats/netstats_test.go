package netstats

import "testing"

func TestAggregateProxySkipsDownAndLoopback(t *testing.T) {
	total, ok := Aggregate([]AdapterSnapshot{
		{Name: "eth", DownloadBytes: 100, UploadBytes: 50, Up: true},
		{Name: "loop", DownloadBytes: 1000, UploadBytes: 1000, Up: true, Loopback: true},
		{Name: "down", DownloadBytes: 1000, UploadBytes: 1000, Up: false},
	}, ModeProxy, "MGB VPN")
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if total.DownloadBytes != 100 || total.UploadBytes != 50 {
		t.Fatalf("total = %#v, want 100/50", total)
	}
}

func TestAggregateTunMatchesNameOrDescription(t *testing.T) {
	total, ok := Aggregate([]AdapterSnapshot{
		{Name: "Wi-Fi", DownloadBytes: 100, UploadBytes: 50, Up: true},
		{Name: "MGB VPN", Description: "Wintun Userspace Tunnel", DownloadBytes: 25, UploadBytes: 10, Up: true},
		{Name: "tun2", Description: "MGB VPN Adapter", DownloadBytes: 5, UploadBytes: 2, Up: true},
	}, ModeTun, "MGB VPN")
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if total.DownloadBytes != 30 || total.UploadBytes != 12 {
		t.Fatalf("total = %#v, want 30/12", total)
	}
}
