package config

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"SingCli/internal/tunsettings"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
)

type TLSConfig struct {
	Enabled    bool   `json:"enabled"`
	Insecure   bool   `json:"insecure"`
	ServerName string `json:"serverName"`
}

type RealityConfig struct {
	PublicKey   string `json:"publicKey"`
	ShortID     string `json:"shortId"`
	Fingerprint string `json:"fingerprint"`
	ServerName  string `json:"serverName"`
	SpiderX     string `json:"spiderX"`
}

type ServerEntry struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	Server   string        `json:"server"`
	UUID     string        `json:"uuid"`
	Password string        `json:"password"`
	Username string        `json:"username"`
	TLS      TLSConfig     `json:"tls"`
	Reality  RealityConfig `json:"reality"`
	Insecure bool          `json:"insecure"`
	// allowInsecure is used by some imported Hysteria2 configs.
	AllowInsecure bool `json:"allowInsecure"`

	// Hysteria2
	UpMbps   int `json:"upMbps"`
	DownMbps int `json:"downMbps"`

	// VMess/VLESS transport
	Network string `json:"network"`
	Path    string `json:"path"`
	Host    string `json:"host"`

	// VMess
	AlterID int `json:"alterId"`

	// VLESS flow (для Reality: xtls-rprx-vision)
	Flow string `json:"flow"`

	// Shadowsocks
	Method string `json:"method"`
}

type ServersConfig struct {
	Servers []ServerEntry `json:"servers"`
}

const ServersFilename = "servers.json"

func LoadServers(configPath string) ([]ServerEntry, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read servers config: %w", err)
	}
	return LoadServersFromBytes(data)
}

func LoadServersFromBytes(data []byte) ([]ServerEntry, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(string(data), "\uFEFF"))
	if trimmed == "" {
		return nil, fmt.Errorf("no valid server entries found")
	}

	if strings.HasPrefix(trimmed, "{") {
		var cfg ServersConfig
		if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
			return nil, fmt.Errorf("parse servers config: %w", err)
		}
		for i := range cfg.Servers {
			normalizeServer(&cfg.Servers[i])
		}
		return cfg.Servers, nil
	}

	servers := ParseURIList(trimmed)
	if len(servers) == 0 {
		if decoded, ok := decodeBase64Subscription(trimmed); ok {
			servers = ParseURIList(decoded)
		}
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("no valid server entries found")
	}
	return servers, nil
}

func decodeBase64Subscription(data string) (string, bool) {
	compact := strings.NewReplacer("\r", "", "\n", "", "\t", "", " ", "").Replace(data)
	if compact == "" {
		return "", false
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.RawURLEncoding,
	} {
		if decoded, err := enc.DecodeString(compact); err == nil {
			text := strings.TrimSpace(strings.TrimPrefix(string(decoded), "\uFEFF"))
			if text != "" {
				return text, true
			}
		}
	}
	return "", false
}

func FindServersConfigCandidates() []string {
	var rawPaths []string

	if abs, err := filepath.Abs(ServersFilename); err == nil {
		rawPaths = append(rawPaths, abs)
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		rawPaths = append(rawPaths,
			filepath.Join(exeDir, ServersFilename),
			filepath.Join(filepath.Dir(exeDir), ServersFilename),
		)
	}

	if defaultPath, err := DefaultServersPath(); err == nil {
		rawPaths = append(rawPaths, defaultPath)
	}

	seen := make(map[string]bool, len(rawPaths))
	result := make([]string, 0, len(rawPaths))
	for _, path := range rawPaths {
		cleanPath := filepath.Clean(path)
		if seen[cleanPath] {
			continue
		}
		seen[cleanPath] = true
		if _, err := os.Stat(cleanPath); err == nil {
			result = append(result, cleanPath)
		}
	}
	return result
}

func SaveServers(configPath string, servers []ServerEntry) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create servers config directory: %w", err)
	}

	data, err := json.MarshalIndent(ServersConfig{Servers: servers}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal servers config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return fmt.Errorf("write servers config: %w", err)
	}
	return nil
}

func DefaultServersPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(configDir, "MGB", ServersFilename), nil
}

func normalizeServer(s *ServerEntry) {
	if s.Type == "hysteria2" {
		if s.Password == "" && s.Username != "" {
			s.Password = s.Username
		}
		if s.Insecure || s.AllowInsecure {
			s.TLS.Insecure = true
		}
	}
}

func LoadServerByName(configPath string, serverName string) (option.Options, error) {
	servers, err := LoadServers(configPath)
	if err != nil {
		return option.Options{}, err
	}
	for _, s := range servers {
		if s.Name == serverName {
			return buildOptions(s)
		}
	}
	return option.Options{}, fmt.Errorf("server %q not found", serverName)
}

func BuildOptionsForServer(s ServerEntry) (option.Options, error) {
	return buildOptions(s)
}

// BuildOptionsForSpeedTest builds a minimal proxy config that listens on httpPort.
// It omits watchdog, sysproxy, and extra inbounds to keep startup fast.
func BuildOptionsForSpeedTest(s ServerEntry, httpPort int) (option.Options, error) {
	normalizeServer(&s)
	outbound, err := buildOutboundMap(s)
	if err != nil {
		return option.Options{}, err
	}
	cfg := map[string]interface{}{
		"log": map[string]interface{}{"level": "warn"},
		"dns": map[string]interface{}{
			"servers": []interface{}{
				map[string]interface{}{"address": "1.1.1.1", "detour": "direct"},
				map[string]interface{}{"address": "8.8.8.8", "detour": "direct"},
			},
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"type":        "http",
				"tag":         "http-test",
				"listen":      "127.0.0.1",
				"listen_port": httpPort,
			},
		},
		"outbounds": []interface{}{
			outbound,
			map[string]interface{}{"type": "direct", "tag": "direct"},
			map[string]interface{}{"type": "block", "tag": "block"},
		},
	}
	return unmarshalOptions(cfg, "speed test config")
}

func BuildTunOptionsForServer(s ServerEntry) (option.Options, error) {
	return buildTunOptions(s)
}

func buildTunOptions(s ServerEntry) (option.Options, error) {
	cfg, err := buildConfigMap(s, true)
	if err != nil {
		return option.Options{}, err
	}
	return unmarshalOptions(cfg, "tun config")
}

func buildOptions(s ServerEntry) (option.Options, error) {
	cfg, err := buildConfigMap(s, false)
	if err != nil {
		return option.Options{}, err
	}
	return unmarshalOptions(cfg, "config")
}

func BuildConfigJSONForServer(s ServerEntry, tunMode bool, redact bool) ([]byte, error) {
	cfg, err := buildConfigMap(s, tunMode)
	if err != nil {
		return nil, err
	}
	if redact {
		cfg = redactConfigMap(cfg)
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func buildConfigMap(s ServerEntry, tunMode bool) (map[string]interface{}, error) {
	normalizeServer(&s)
	outbound, err := buildOutboundMap(s)
	if err != nil {
		return nil, err
	}

	if !tunMode {
		return map[string]interface{}{
			"log": map[string]interface{}{
				"level": "info",
			},
			"dns": map[string]interface{}{
				"servers": []interface{}{
					map[string]interface{}{"address": "1.1.1.1", "detour": "direct"},
					map[string]interface{}{"address": "8.8.8.8", "detour": "direct"},
				},
			},
			"inbounds": []interface{}{
				map[string]interface{}{
					"type":        "socks",
					"tag":         "socks-in",
					"listen":      "127.0.0.1",
					"listen_port": 1080,
				},
				map[string]interface{}{
					"type":        "http",
					"tag":         "http-in",
					"listen":      "127.0.0.1",
					"listen_port": 1081,
				},
			},
			"outbounds": []interface{}{
				outbound,
				map[string]interface{}{"type": "direct", "tag": "direct"},
				map[string]interface{}{"type": "block", "tag": "block"},
			},
			"route": map[string]interface{}{
				"rules": []interface{}{
					map[string]interface{}{
						"port":     53,
						"network":  "udp",
						"outbound": "direct",
					},
				},
			},
		}, nil
	}

	routeRules := []interface{}{
		// 1. DNS перехватываем первым — до проверки private IP,
		// иначе запросы к 172.19.0.2:53 (TUN-шлюз) уходят direct и теряются
		map[string]interface{}{
			"protocol": "dns",
			"outbound": "dns-out",
		},
		map[string]interface{}{
			"port":     []int{53},
			"network":  []string{"udp", "tcp"},
			"outbound": "dns-out",
		},
		// 2. Блокируем QUIC (UDP 443), чтобы браузеры шли через TCP
		map[string]interface{}{
			"port":     []int{443},
			"network":  []string{"udp"},
			"outbound": "block",
		},
		// 3. Локальный трафик не трогаем
		map[string]interface{}{
			"ip_is_private": true,
			"outbound":      "direct",
		},
	}

	dnsServers, dnsRules, dnsFinal := buildTunDNS(s)

	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"level": "info",
		},
		"dns": map[string]interface{}{
			"servers":           dnsServers,
			"rules":             dnsRules,
			"final":             dnsFinal,
			"strategy":          "ipv4_only", // Запрещаем IPv6, чтобы избежать "черных дыр" в браузере
			"independent_cache": true,
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"type":                       "tun",
				"tag":                        "tun-in",
				"interface_name":             tunsettings.InterfaceName,
				"inet4_address":              tunsettings.IPv4Prefix,
				"mtu":                        tunMTU(s),
				"auto_route":                 true,
				"strict_route":               true,    // ВАЖНО: включает WFP на Windows для обхода петель
				"stack":                      "mixed", // ВАЖНО: включает gVisor для безупречной работы TCP
				"sniff":                      true,
				"sniff_override_destination": false,
				"route_exclude_address":      serverRouteExcludeAddresses(s),
			},
		},
		"outbounds": []interface{}{
			outbound,
			map[string]interface{}{"type": "direct", "tag": "direct"},
			map[string]interface{}{"type": "block", "tag": "block"},
			map[string]interface{}{"type": "dns", "tag": "dns-out"},
		},
		"route": map[string]interface{}{
			"rules":                 routeRules,
			"final":                 s.Name,
			"auto_detect_interface": true,
		},
	}

	return cfg, nil
}

func buildTunDNS(s ServerEntry) ([]interface{}, []interface{}, string) {
	directDNS := map[string]interface{}{
		"tag":      "dns-direct",
		"address":  "tcp://1.1.1.1",
		"detour":   "direct",
		"strategy": "ipv4_only",
	}

	if s.Type == "hysteria2" {
		return []interface{}{directDNS}, []interface{}{}, "dns-direct"
	}

	return []interface{}{
			map[string]interface{}{
				"tag":              "dns-remote",
				"address":          "https://1.1.1.1/dns-query",
				"detour":           s.Name,
				"address_resolver": "dns-direct",
				"address_strategy": "ipv4_only",
			},
			directDNS,
		}, []interface{}{
			map[string]interface{}{
				"outbound": []string{"any"}, // DNS-запросы от самого VPN-клиента (поиск IP сервера)
				"server":   "dns-direct",
			},
		}, "dns-remote"
}

func unmarshalOptions(cfg map[string]interface{}, label string) (option.Options, error) {
	jsonBytes, err := json.Marshal(cfg)
	if err != nil {
		return option.Options{}, fmt.Errorf("marshal %s: %w", label, err)
	}
	var opts option.Options
	ctx := box.Context(context.Background(), include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry())
	if err := opts.UnmarshalJSONContext(ctx, jsonBytes); err != nil {
		return option.Options{}, fmt.Errorf("unmarshal %s options: %w", label, err)
	}
	return opts, nil
}

func tunMTU(s ServerEntry) int {
	if s.Type == "hysteria2" {
		return 1400
	}
	return 1500
}

func redactConfigMap(cfg map[string]interface{}) map[string]interface{} {
	bytes, err := json.Marshal(cfg)
	if err != nil {
		return cfg
	}
	var cloned map[string]interface{}
	if err := json.Unmarshal(bytes, &cloned); err != nil {
		return cfg
	}
	outbounds, _ := cloned["outbounds"].([]interface{})
	for _, outbound := range outbounds {
		out, ok := outbound.(map[string]interface{})
		if !ok {
			continue
		}
		for _, key := range []string{"password", "uuid"} {
			if _, exists := out[key]; exists {
				out[key] = "<redacted>"
			}
		}
		if tls, ok := out["tls"].(map[string]interface{}); ok {
			if reality, ok := tls["reality"].(map[string]interface{}); ok {
				for _, key := range []string{"public_key", "short_id"} {
					if _, exists := reality[key]; exists {
						reality[key] = "<redacted>"
					}
				}
			}
		}
	}
	return cloned
}

func serverRouteExcludeAddresses(s ServerEntry) []string {
	host, _ := parseHostPort(s.Server)
	if host == "" {
		return nil
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return []string{prefixForAddr(addr)}
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return nil
	}
	excludes := make([]string, 0, len(ips))
	seen := make(map[string]bool, len(ips))
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			continue
		}
		addr = addr.Unmap()
		prefix := prefixForAddr(addr)
		if seen[prefix] {
			continue
		}
		seen[prefix] = true
		excludes = append(excludes, prefix)
	}
	return excludes
}

func prefixForAddr(addr netip.Addr) string {
	if addr.Is4() {
		return addr.String() + "/32"
	}
	return addr.String() + "/128"
}

func buildOutboundMap(s ServerEntry) (map[string]interface{}, error) {
	host, port := parseHostPort(s.Server)
	if host == "" {
		return nil, fmt.Errorf("server address is empty")
	}

	out := map[string]interface{}{
		"type":        s.Type,
		"tag":         s.Name,
		"server":      host,
		"server_port": port,
	}

	tlsServerName := s.TLS.ServerName
	if tlsServerName == "" {
		tlsServerName = host
	}

	switch s.Type {
	case "hysteria2":
		if s.Password == "" {
			return nil, fmt.Errorf("hysteria2 requires password")
		}
		out["password"] = s.Password
		out["up_mbps"] = orDefault(s.UpMbps, 50)
		out["down_mbps"] = orDefault(s.DownMbps, 50)
		out["tls"] = map[string]interface{}{
			"enabled":     true,
			"server_name": tlsServerName,
			"insecure":    s.TLS.Insecure,
		}
		out["domain_strategy"] = "ipv4_only"

	case "vless":
		if s.UUID == "" {
			return nil, fmt.Errorf("vless requires uuid")
		}
		out["uuid"] = s.UUID
		if s.Flow == "" {
			out["packet_encoding"] = "xudp"
		}
		if s.Flow != "" {
			out["flow"] = s.Flow
		}
		if s.Reality.PublicKey != "" {
			// VLESS + Reality
			realityServerName := s.Reality.ServerName
			if realityServerName == "" {
				realityServerName = tlsServerName
			}
			fingerprint := s.Reality.Fingerprint
			if fingerprint == "" {
				fingerprint = "chrome"
			}
			out["tls"] = map[string]interface{}{
				"enabled":     true,
				"server_name": realityServerName,
				"utls": map[string]interface{}{
					"enabled":     true,
					"fingerprint": fingerprint,
				},
				"reality": map[string]interface{}{
					"enabled":    true,
					"public_key": s.Reality.PublicKey,
					"short_id":   s.Reality.ShortID,
				},
			}
		} else if s.TLS.Enabled {
			out["tls"] = map[string]interface{}{
				"enabled":     true,
				"server_name": tlsServerName,
				"insecure":    s.TLS.Insecure,
			}
		}
		if s.Network != "" {
			out["transport"] = buildTransport(s)
		}

	case "vmess":
		if s.UUID == "" {
			return nil, fmt.Errorf("vmess requires uuid")
		}
		out["uuid"] = s.UUID
		out["alter_id"] = s.AlterID
		out["security"] = "auto"
		if s.TLS.Enabled {
			out["tls"] = map[string]interface{}{
				"enabled":     true,
				"server_name": tlsServerName,
				"insecure":    s.TLS.Insecure,
			}
		}
		if s.Network != "" {
			out["transport"] = buildTransport(s)
		}

	case "trojan":
		if s.Password == "" {
			return nil, fmt.Errorf("trojan requires password")
		}
		out["password"] = s.Password
		out["tls"] = map[string]interface{}{
			"enabled":     true,
			"server_name": tlsServerName,
			"insecure":    s.TLS.Insecure,
		}
		if s.Network != "" {
			out["transport"] = buildTransport(s)
		}

	case "shadowsocks":
		if s.Password == "" {
			return nil, fmt.Errorf("shadowsocks requires password")
		}
		method := s.Method
		if method == "" {
			method = "aes-128-gcm"
		}
		out["method"] = method
		out["password"] = s.Password

	default:
		return nil, fmt.Errorf("unsupported protocol: %q", s.Type)
	}

	return out, nil
}

func buildTransport(s ServerEntry) map[string]interface{} {
	switch s.Network {
	case "ws":
		t := map[string]interface{}{"type": "ws"}
		if s.Path != "" {
			t["path"] = s.Path
		}
		if s.Host != "" {
			t["headers"] = map[string]interface{}{"Host": s.Host}
		}
		return t
	case "grpc":
		t := map[string]interface{}{"type": "grpc"}
		if s.Path != "" {
			t["service_name"] = s.Path
		}
		return t
	case "h2":
		t := map[string]interface{}{"type": "http"}
		if s.Path != "" {
			t["path"] = s.Path
		}
		if s.Host != "" {
			t["host"] = []string{s.Host}
		}
		return t
	case "xhttp":
		// sing-box does not implement Xray SplitHTTP/XHTTP natively.
		// Map imported XHTTP links to the closest supported V2Ray HTTP transport.
		t := map[string]interface{}{"type": "http"}
		if s.Path != "" {
			t["path"] = s.Path
		}
		if s.Host != "" {
			t["host"] = []string{s.Host}
		}
		return t
	default:
		return map[string]interface{}{"type": s.Network}
	}
}

func parseHostPort(server string) (string, int) {
	if idx := strings.LastIndex(server, ":"); idx != -1 {
		host := server[:idx]
		if p, err := strconv.Atoi(server[idx+1:]); err == nil {
			return host, p
		}
	}
	return server, 0
}

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func isIPAddress(host string) bool {
	_, err := netip.ParseAddr(host)
	return err == nil
}
