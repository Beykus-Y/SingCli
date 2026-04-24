package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"

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
	TLS      TLSConfig     `json:"tls"`
	Reality  RealityConfig `json:"reality"`

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

func LoadServers(configPath string) ([]ServerEntry, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read servers config: %w", err)
	}
	var cfg ServersConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse servers config: %w", err)
	}
	return cfg.Servers, nil
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

func BuildTunOptionsForServer(s ServerEntry) (option.Options, error) {
	return buildTunOptions(s)
}

func buildTunOptions(s ServerEntry) (option.Options, error) {
	outbound, err := buildOutboundMap(s)
	if err != nil {
		return option.Options{}, err
	}

	routeRules := []interface{}{
		// 1. DNS перехватываем первым — до проверки private IP,
		// иначе запросы к 172.19.0.2:53 (TUN-шлюз) уходят direct и теряются
		map[string]interface{}{
			"protocol": "dns",
			"outbound": "dns-out",
		},
		map[string]interface{}{
			"port":    []int{53},
			"network": []string{"udp", "tcp"},
			"outbound": "dns-out",
		},
		// 2. Блокируем QUIC (UDP 443), чтобы браузеры шли через TCP
		map[string]interface{}{
			"port":    []int{443},
			"network": []string{"udp"},
			"outbound": "block",
		},
		// 3. Локальный трафик не трогаем
		map[string]interface{}{
			"ip_is_private": true,
			"outbound":      "direct",
		},
	}

	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"level": "info",
		},
		"dns": map[string]interface{}{
			"servers": []interface{}{
				map[string]interface{}{
					"tag":     "dns-remote",
					// Используем DNS-over-HTTPS: он на 100% стабильно работает через любой VLESS
					"address": "https://1.1.1.1/dns-query", 
					"detour":  s.Name, // Жестко направляем в туннель
				},
				map[string]interface{}{
					"tag":     "dns-direct",
					"address": "local", // Системный DNS для самого sing-box
					"detour":  "direct",
				},
			},
			"rules":[]interface{}{
				map[string]interface{}{
					"outbound":[]string{"any"}, // DNS-запросы от самого VPN-клиента (поиск IP сервера)
					"server":   "dns-direct",
				},
			},
			"final":             "dns-remote",
			"strategy":          "ipv4_only", // Запрещаем IPv6, чтобы избежать "черных дыр" в браузере
			"independent_cache": true,
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"type":                       "tun",
				"tag":                        "tun-in",
				"inet4_address":              "172.19.0.1/30",
				"mtu":                        1500,
				"auto_route":                 true,
				"strict_route":               true, // ВАЖНО: включает WFP на Windows для обхода петель
				"stack":                      "mixed", // ВАЖНО: включает gVisor для безупречной работы TCP
				"sniff":                      true,
				"sniff_override_destination": false,
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

	jsonBytes, err := json.Marshal(cfg)
	if err != nil {
		return option.Options{}, fmt.Errorf("marshal tun config: %w", err)
	}

	var opts option.Options
	ctx := box.Context(context.Background(), include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry())
	if err := opts.UnmarshalJSONContext(ctx, jsonBytes); err != nil {
		return option.Options{}, fmt.Errorf("unmarshal tun options: %w", err)
	}
	return opts, nil
}

func buildOptions(s ServerEntry) (option.Options, error) {
	outbound, err := buildOutboundMap(s)
	if err != nil {
		return option.Options{}, err
	}

	cfg := map[string]interface{}{
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
	}

	jsonBytes, err := json.Marshal(cfg)
	if err != nil {
		return option.Options{}, fmt.Errorf("marshal config: %w", err)
	}

	var opts option.Options
	ctx := box.Context(context.Background(), include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry())
	if err := opts.UnmarshalJSONContext(ctx, jsonBytes); err != nil {
		return option.Options{}, fmt.Errorf("unmarshal to options: %w", err)
	}
	return opts, nil
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
