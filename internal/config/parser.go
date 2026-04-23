package config

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strconv"
    "strings"

    box "github.com/sagernet/sing-box"
    "github.com/sagernet/sing-box/include"
    "github.com/sagernet/sing-box/option"
)

// ServerEntry – одна запись в вашем серверном JSON
type ServerEntry struct {
    Name          string `json:"name"`
    Type          string `json:"type"`
    Server        string `json:"server"`
    Username      string `json:"username"`
    Password      string `json:"password"`
    Insecure      bool   `json:"insecure"`
    AllowInsecure bool   `json:"allowInsecure"`
}

// ServersConfig – корневая структура файла профилей
type ServersConfig struct {
    Servers []ServerEntry `json:"servers"`
}

// LoadServerByName читает файл профилей, ищет сервер по имени и возвращает option.Options
func LoadServerByName(configPath string, serverName string) (option.Options, error) {
    data, err := os.ReadFile(configPath)
    if err != nil {
        return option.Options{}, fmt.Errorf("read servers config: %w", err)
    }

    var serversCfg ServersConfig
    if err := json.Unmarshal(data, &serversCfg); err != nil {
        return option.Options{}, fmt.Errorf("parse servers config: %w", err)
    }

    var target *ServerEntry
    for _, s := range serversCfg.Servers {
        if s.Name == serverName {
            target = &s
            break
        }
    }
    if target == nil {
        return option.Options{}, fmt.Errorf("server '%s' not found", serverName)
    }

    return buildOptions(*target)
}

// buildOptions создаёт стандартный JSON-конфиг Sing-Box и парсит в option.Options
func buildOptions(s ServerEntry) (option.Options, error) {
    // Базовый конфиг: SOCKS-вход + outbound выбранного типа
    cfg := map[string]interface{}{
        "log": map[string]interface{}{
            "level": "debug",
        },
        "dns": map[string]interface{}{
            "servers": []interface{}{
                map[string]interface{}{
                    "address":  "1.1.1.1",
                    "detour":   "direct",
                },
                map[string]interface{}{
                    "address":  "8.8.8.8",
                    "detour":   "direct",
                },
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
            buildOutboundMap(s),
            map[string]interface{}{
                "type": "direct",
                "tag":  "direct",
            },
        },
        "route": map[string]interface{}{
            "rules": []interface{}{
                map[string]interface{}{
                    "port":       53,
                    "network":    "udp",
                    "outbound":   "direct",
                },
            },
        },
    }

    // Маршалим в JSON
    jsonBytes, err := json.Marshal(cfg)
    if err != nil {
        return option.Options{}, fmt.Errorf("marshal config: %w", err)
    }

    // Выводим конфиг для отладки
    fmt.Fprintf(os.Stderr, "Generated config: %s\n", string(jsonBytes))

    // Парсим в option.Options с контекстом sing-box, чтобы сработали регистры (outbound options registry и т.д.)
    var opts option.Options
    ctx := box.Context(context.Background(), include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry())
    if err := opts.UnmarshalJSONContext(ctx, jsonBytes); err != nil {
        return option.Options{}, fmt.Errorf("unmarshal to options: %w", err)
    }
    return opts, nil
}

// buildOutboundMap возвращает map с полями outbound в зависимости от типа
func buildOutboundMap(s ServerEntry) map[string]interface{} {
    out := map[string]interface{}{
        "type":    s.Type,
        "tag":     s.Name,
        "server":  s.Server,
    }

    switch s.Type {
    case "hysteria2":
        // Hysteria2 требует пароль, TLS конфигурацию и bandwidth параметры
        host, port := parseHostPort(s.Server)
        out["server"] = host
        if port > 0 {
            out["server_port"] = port
        }
        // Не указываем network — по умолчанию поддерживаются TCP и UDP
        password := s.Username
        if s.Password != "" {
            password = s.Password
        }
        out["password"] = password
        out["up_mbps"] = 50
        out["down_mbps"] = 50
        out["tls"] = map[string]interface{}{
            "enabled":     true,
            "server_name": host,
            "insecure":    true,
        }
    default:
        // fallback на direct
        out["type"] = "direct"
    }
    return out
}

// extractServerName извлекает хост из строки "host:port"
func extractServerName(server string) string {
    host, _ := parseHostPort(server)
    return host
}

// parseHostPort возвращает host и порт (0 если не указан)
func parseHostPort(server string) (string, int) {
    if idx := strings.LastIndex(server, ":"); idx != -1 {
        host := server[:idx]
        portStr := server[idx+1:]
        if p, err := strconv.Atoi(portStr); err == nil {
            return host, p
        }
    }
    return server, 0
}
