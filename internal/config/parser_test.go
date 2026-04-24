package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadServersFromBytes(t *testing.T) {
	data := []byte(`{
  "servers": [
    {
      "name": "demo-ss",
      "type": "shadowsocks",
      "server": "127.0.0.1:8388",
      "password": "secret",
      "method": "aes-128-gcm"
    },
    {
      "name": "demo-hy",
      "type": "hysteria2",
      "server": "hy.example.com:443",
      "password": "hy-pass",
      "tls": {
        "serverName": "hy.example.com"
      }
    }
  ]
}`)

	servers, err := LoadServersFromBytes(data)
	if err != nil {
		t.Fatalf("LoadServersFromBytes returned error: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
	if servers[0].Type != "shadowsocks" {
		t.Fatalf("expected first type shadowsocks, got %q", servers[0].Type)
	}
	if servers[1].Type != "hysteria2" {
		t.Fatalf("expected second type hysteria2, got %q", servers[1].Type)
	}
}

func TestLoadServersFromBytesInvalidJSON(t *testing.T) {
	_, err := LoadServersFromBytes([]byte(`{"servers":`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSaveServers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "servers.json")
	servers := []ServerEntry{
		{
			Name:     "demo",
			Type:     "trojan",
			Server:   "demo.example.com:443",
			Password: "pwd",
			TLS: TLSConfig{
				Enabled:    true,
				ServerName: "demo.example.com",
			},
		},
	}

	if err := SaveServers(path, servers); err != nil {
		t.Fatalf("SaveServers returned error: %v", err)
	}

	loaded, err := LoadServers(path)
	if err != nil {
		t.Fatalf("LoadServers returned error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 server, got %d", len(loaded))
	}
	if loaded[0].Name != "demo" {
		t.Fatalf("expected server name demo, got %q", loaded[0].Name)
	}
}

func TestDefaultServersPath(t *testing.T) {
	path, err := DefaultServersPath()
	if err != nil {
		t.Fatalf("DefaultServersPath returned error: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("MGB", "servers.json")) {
		t.Fatalf("unexpected default path: %q", path)
	}
}
