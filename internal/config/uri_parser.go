package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParseURI parses a single proxy URI (vless://, ss://, hysteria2://) into a ServerEntry.
func ParseURI(raw string) (ServerEntry, error) {
	raw = strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(raw, "vless://"):
		return parseVLESS(raw)
	case strings.HasPrefix(raw, "ss://"):
		return parseShadowsocks(raw)
	case strings.HasPrefix(raw, "hysteria2://"):
		return parseHysteria2(raw)
	default:
		preview := raw
		if len(preview) > 30 {
			preview = preview[:30]
		}
		return ServerEntry{}, fmt.Errorf("unsupported URI scheme: %q", preview)
	}
}

// ParseURIList parses newline-separated proxy URIs, skipping blanks and comments.
func ParseURIList(data string) []ServerEntry {
	var out []ServerEntry
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\uFEFF"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		s, err := ParseURI(line)
		if err != nil {
			continue
		}
		out = append(out, s)
	}
	return out
}

func parseVLESS(raw string) (ServerEntry, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return ServerEntry{}, fmt.Errorf("parse vless uri: %w", err)
	}

	uuid := u.User.Username()
	host := u.Hostname()
	portStr := u.Port()
	port, _ := strconv.Atoi(portStr)

	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("vless-%s:%d", host, port)
	}

	q := u.Query()
	security := q.Get("security")
	netType := q.Get("type")
	sni := q.Get("sni")
	fp := q.Get("fp")
	pbk := q.Get("pbk")
	sid := q.Get("sid")
	flow := q.Get("flow")
	serviceName := q.Get("serviceName")
	path := q.Get("path")
	hostHeader := q.Get("host")

	s := ServerEntry{
		Name:   name,
		Type:   "vless",
		Server: fmt.Sprintf("%s:%d", host, port),
		UUID:   uuid,
		Flow:   flow,
	}

	switch security {
	case "reality":
		s.Reality = RealityConfig{
			PublicKey:   pbk,
			ShortID:     sid,
			Fingerprint: fp,
			ServerName:  sni,
		}
	case "tls":
		s.TLS = TLSConfig{
			Enabled:    true,
			ServerName: sni,
		}
	}

	switch netType {
	case "grpc":
		s.Network = "grpc"
		s.Path = serviceName
	case "ws":
		s.Network = "ws"
		s.Path = path
		s.Host = hostHeader
	case "xhttp", "splithttp":
		s.Network = "xhttp"
		s.Path = path
		s.Host = hostHeader
	}

	return s, nil
}

func parseHysteria2(raw string) (ServerEntry, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return ServerEntry{}, fmt.Errorf("parse hysteria2 uri: %w", err)
	}

	password := u.User.Username()
	host := u.Hostname()
	portStr := u.Port()
	port, _ := strconv.Atoi(portStr)
	if host == "" {
		return ServerEntry{}, fmt.Errorf("invalid hysteria2 uri: missing host")
	}
	if port == 0 {
		return ServerEntry{}, fmt.Errorf("invalid hysteria2 uri: missing or invalid port")
	}

	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("hysteria2-%s:%d", host, port)
	}

	q := u.Query()
	s := ServerEntry{
		Name:     name,
		Type:     "hysteria2",
		Server:   fmt.Sprintf("%s:%d", host, port),
		Password: password,
		TLS: TLSConfig{
			ServerName: firstNonEmpty(q.Get("sni"), q.Get("peer")),
			Insecure:   queryBool(q, "insecure") || queryBool(q, "allowInsecure"),
		},
		UpMbps:   firstPositiveInt(q.Get("upmbps"), q.Get("upMbps"), q.Get("up")),
		DownMbps: firstPositiveInt(q.Get("downmbps"), q.Get("downMbps"), q.Get("down")),
	}
	normalizeServer(&s)
	return s, nil
}

func parseShadowsocks(raw string) (ServerEntry, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return ServerEntry{}, fmt.Errorf("parse ss uri: %w", err)
	}

	host := u.Hostname()
	portStr := u.Port()
	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("ss-%s:%s", host, portStr)
	}

	encoded := u.User.Username()
	decoded, err := tryBase64Decode(encoded)
	if err != nil {
		return ServerEntry{}, fmt.Errorf("decode ss credentials: %w", err)
	}

	idx := strings.IndexByte(decoded, ':')
	if idx < 0 {
		return ServerEntry{}, fmt.Errorf("invalid ss credentials: missing colon")
	}

	return ServerEntry{
		Name:     name,
		Type:     "shadowsocks",
		Server:   fmt.Sprintf("%s:%s", host, portStr),
		Method:   decoded[:idx],
		Password: decoded[idx+1:],
	}, nil
}

func tryBase64Decode(s string) (string, error) {
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return string(b), nil
		}
	}
	return "", fmt.Errorf("failed to base64-decode %q", s)
}

func queryBool(q url.Values, key string) bool {
	switch strings.ToLower(q.Get(key)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}

func firstPositiveInt(values ...string) int {
	for _, value := range values {
		n, err := strconv.Atoi(value)
		if err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
