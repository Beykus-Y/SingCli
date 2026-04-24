package config

import (
	"encoding/base64"
	"encoding/json"
	"os"
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

func TestLoadServersFromBytesJSONWithBOM(t *testing.T) {
	servers, err := LoadServersFromBytes([]byte("\uFEFF{\"servers\":[{\"name\":\"demo\",\"type\":\"shadowsocks\",\"server\":\"127.0.0.1:8388\",\"password\":\"secret\"}]}"))
	if err != nil {
		t.Fatalf("LoadServersFromBytes() error = %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("len(servers) = %d, want 1", len(servers))
	}
}

func TestLoadServersFromBytesURIList(t *testing.T) {
	data := []byte(strings.Join([]string{
		"vless://26aa11f0-35e5-4a51-94f6-60ac63c96a35@example.com:443?security=reality&type=grpc&serviceName=grpc-main&sni=www.microsoft.com&fp=chrome&pbk=public-key&sid=short-id#VLESS%20Reality",
		"ss://YWVzLTEyOC1nY206c2VjcmV0@example.net:8388#SS",
		"hysteria2://hy-secret@hy.example.org:8443?sni=hy.example.org&insecure=1#HY2",
	}, "\n"))

	servers, err := LoadServersFromBytes(data)
	if err != nil {
		t.Fatalf("LoadServersFromBytes() error = %v", err)
	}
	if len(servers) != 3 {
		t.Fatalf("len(servers) = %d, want 3", len(servers))
	}
	if servers[0].Name != "VLESS Reality" || servers[0].Type != "vless" {
		t.Fatalf("first server = %#v, want VLESS Reality vless", servers[0])
	}
	if servers[1].Method != "aes-128-gcm" || servers[1].Password != "secret" {
		t.Fatalf("shadowsocks credentials = %q/%q, want aes-128-gcm/secret", servers[1].Method, servers[1].Password)
	}
	if !servers[2].TLS.Insecure {
		t.Fatal("hysteria2 TLS.Insecure = false, want true")
	}
}

func TestLoadServersFromBytesURIListWithBOM(t *testing.T) {
	servers, err := LoadServersFromBytes([]byte("\uFEFFhysteria2://secret@example.com:443#hy2"))
	if err != nil {
		t.Fatalf("LoadServersFromBytes() error = %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("len(servers) = %d, want 1", len(servers))
	}
	if servers[0].Type != "hysteria2" {
		t.Fatalf("Type = %q, want hysteria2", servers[0].Type)
	}
}

func TestLoadServersFromBytesBase64URIList(t *testing.T) {
	payload := "hysteria2://secret@example.com:443#hy2\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	servers, err := LoadServersFromBytes([]byte(encoded))
	if err != nil {
		t.Fatalf("LoadServersFromBytes() error = %v", err)
	}
	if len(servers) != 1 || servers[0].Name != "hy2" {
		t.Fatalf("servers = %#v, want one decoded hysteria2 server", servers)
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

func TestHysteria2PasswordAndTunProfile(t *testing.T) {
	server := ServerEntry{
		Name:     "hy2",
		Type:     "hysteria2",
		Server:   "example.com:8443",
		Password: "secret",
		TLS:      TLSConfig{Insecure: true},
		UpMbps:   25,
		DownMbps: 75,
	}

	if _, err := BuildOptionsForServer(server); err != nil {
		t.Fatalf("BuildOptionsForServer() error = %v", err)
	}
	if _, err := BuildTunOptionsForServer(server); err != nil {
		t.Fatalf("BuildTunOptionsForServer() error = %v", err)
	}

	cfg := mustConfigMap(t, server, true, false)
	outbound := firstOutbound(t, cfg)
	if got := outbound["password"]; got != "secret" {
		t.Fatalf("password = %v, want secret", got)
	}
	if got := outbound["domain_strategy"]; got != "ipv4_only" {
		t.Fatalf("domain_strategy = %v, want ipv4_only", got)
	}

	inbound := firstInbound(t, cfg)
	if got := inbound["mtu"]; got != float64(1400) {
		t.Fatalf("tun mtu = %v, want 1400", got)
	}
	dns := cfg["dns"].(map[string]interface{})
	if got := dns["final"]; got != "dns-direct" {
		t.Fatalf("hysteria2 tun dns final = %v, want dns-direct", got)
	}
	dnsServer := dns["servers"].([]interface{})[0].(map[string]interface{})
	if got := dnsServer["address"]; got != "tcp://1.1.1.1" {
		t.Fatalf("hysteria2 tun dns address = %v, want tcp://1.1.1.1", got)
	}
}

func TestLoadServersHysteria2LegacyAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	data := []byte(`{
		"servers": [{
			"name": "legacy hy2",
			"type": "hysteria2",
			"server": "example.com:8443",
			"username": "legacy-secret",
			"allowInsecure": true
		}]
	}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	servers, err := LoadServers(path)
	if err != nil {
		t.Fatalf("LoadServers() error = %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("len(servers) = %d, want 1", len(servers))
	}
	if got := servers[0].Password; got != "legacy-secret" {
		t.Fatalf("password = %q, want legacy-secret", got)
	}
	if !servers[0].TLS.Insecure {
		t.Fatal("TLS.Insecure = false, want true")
	}

	cfg := mustConfigMap(t, servers[0], false, false)
	tls := firstOutbound(t, cfg)["tls"].(map[string]interface{})
	if got := tls["insecure"]; got != true {
		t.Fatalf("tls.insecure = %v, want true", got)
	}
}

func TestParseHysteria2URI(t *testing.T) {
	server, err := ParseURI("hysteria2://estonia@mgboosthy2.duckdns.org:8443#🇪🇪 Estonia - Hysteria2")
	if err != nil {
		t.Fatalf("ParseURI() error = %v", err)
	}
	if server.Type != "hysteria2" {
		t.Fatalf("Type = %q, want hysteria2", server.Type)
	}
	if server.Name != "🇪🇪 Estonia - Hysteria2" {
		t.Fatalf("Name = %q, want emoji fragment name", server.Name)
	}
	if server.Server != "mgboosthy2.duckdns.org:8443" {
		t.Fatalf("Server = %q, want mgboosthy2.duckdns.org:8443", server.Server)
	}
	if server.Password != "estonia" {
		t.Fatalf("Password = %q, want estonia", server.Password)
	}
	if _, err := BuildTunOptionsForServer(server); err != nil {
		t.Fatalf("BuildTunOptionsForServer() error = %v", err)
	}
}

func TestParseHysteria2URIQueryOptions(t *testing.T) {
	server, err := ParseURI("hysteria2://secret@example.com:443?sni=cdn.example.com&insecure=1&upmbps=25&downmbps=75#hy2")
	if err != nil {
		t.Fatalf("ParseURI() error = %v", err)
	}
	if server.TLS.ServerName != "cdn.example.com" {
		t.Fatalf("TLS.ServerName = %q, want cdn.example.com", server.TLS.ServerName)
	}
	if !server.TLS.Insecure {
		t.Fatal("TLS.Insecure = false, want true")
	}
	if server.UpMbps != 25 {
		t.Fatalf("UpMbps = %d, want 25", server.UpMbps)
	}
	if server.DownMbps != 75 {
		t.Fatalf("DownMbps = %d, want 75", server.DownMbps)
	}
}

func TestBuildOptionsRejectsEmptyHysteria2Password(t *testing.T) {
	_, err := BuildOptionsForServer(ServerEntry{
		Name:   "hy2",
		Type:   "hysteria2",
		Server: "example.com:8443",
	})
	if err == nil {
		t.Fatal("BuildOptionsForServer() error = nil, want error")
	}
}

func TestVLESSRealityGRPCConfigUnchanged(t *testing.T) {
	server := ServerEntry{
		Name:    "vless",
		Type:    "vless",
		Server:  "203.0.113.1:443",
		UUID:    "26aa11f0-35e5-4a51-94f6-60ac63c96a35",
		Network: "grpc",
		Path:    "grpc-main",
		Reality: RealityConfig{
			PublicKey:   "public-key",
			ShortID:     "short-id",
			Fingerprint: "chrome",
			ServerName:  "www.microsoft.com",
		},
	}

	if _, err := BuildTunOptionsForServer(server); err != nil {
		t.Fatalf("BuildTunOptionsForServer() error = %v", err)
	}

	cfg := mustConfigMap(t, server, true, false)
	outbound := firstOutbound(t, cfg)
	if got := outbound["type"]; got != "vless" {
		t.Fatalf("type = %v, want vless", got)
	}
	transport := outbound["transport"].(map[string]interface{})
	if got := transport["type"]; got != "grpc" {
		t.Fatalf("transport.type = %v, want grpc", got)
	}
	if got := transport["service_name"]; got != "grpc-main" {
		t.Fatalf("transport.service_name = %v, want grpc-main", got)
	}
	inbound := firstInbound(t, cfg)
	if got := inbound["mtu"]; got != float64(1500) {
		t.Fatalf("vless tun mtu = %v, want 1500", got)
	}
	dns := cfg["dns"].(map[string]interface{})
	if got := dns["final"]; got != "dns-remote" {
		t.Fatalf("vless tun dns final = %v, want dns-remote", got)
	}
}

func TestParseVLESSXHTTPURIUsesSupportedHTTPTransport(t *testing.T) {
	server, err := ParseURI("vless://26aa11f0-35e5-4a51-94f6-60ac63c96a35@example.com:458?security=reality&type=xhttp&path=%2Fxhttp&host=front.example.com&sni=www.microsoft.com&fp=chrome&pbk=public-key&sid=short-id#xhttp")
	if err != nil {
		t.Fatalf("ParseURI() error = %v", err)
	}
	if server.Network != "xhttp" {
		t.Fatalf("Network = %q, want xhttp", server.Network)
	}
	if _, err := BuildTunOptionsForServer(server); err != nil {
		t.Fatalf("BuildTunOptionsForServer() error = %v", err)
	}

	cfg := mustConfigMap(t, server, true, false)
	transport := firstOutbound(t, cfg)["transport"].(map[string]interface{})
	if got := transport["type"]; got != "http" {
		t.Fatalf("transport.type = %v, want http", got)
	}
	if got := transport["path"]; got != "/xhttp" {
		t.Fatalf("transport.path = %v, want /xhttp", got)
	}
}

func TestRedactedConfigDoesNotPrintSecrets(t *testing.T) {
	server := ServerEntry{
		Name:     "hy2",
		Type:     "hysteria2",
		Server:   "example.com:8443",
		Password: "secret",
	}
	cfg := mustConfigMap(t, server, false, true)
	if got := firstOutbound(t, cfg)["password"]; got != "<redacted>" {
		t.Fatalf("redacted password = %v, want <redacted>", got)
	}
}

func mustConfigMap(t *testing.T, server ServerEntry, tunMode bool, redact bool) map[string]interface{} {
	t.Helper()
	data, err := BuildConfigJSONForServer(server, tunMode, redact)
	if err != nil {
		t.Fatalf("BuildConfigJSONForServer() error = %v", err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return cfg
}

func firstOutbound(t *testing.T, cfg map[string]interface{}) map[string]interface{} {
	t.Helper()
	outbounds := cfg["outbounds"].([]interface{})
	return outbounds[0].(map[string]interface{})
}

func firstInbound(t *testing.T, cfg map[string]interface{}) map[string]interface{} {
	t.Helper()
	inbounds := cfg["inbounds"].([]interface{})
	return inbounds[0].(map[string]interface{})
}
