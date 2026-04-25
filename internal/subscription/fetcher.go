package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"SingCli/internal/config"
	"SingCli/internal/storage"
)

const maxSubscriptionBodyBytes = 12 * 1024 * 1024

type FetchOptions struct {
	URL          string
	ETag         string
	LastModified string
	DeviceID     string
	DeviceName   string
}

type FetchResult struct {
	Servers     []config.ServerEntry
	Metadata    storage.SubscriptionMetadata
	NotModified bool
}

func Fetch(ctx context.Context, client *http.Client, options FetchOptions) (FetchResult, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(options.URL), nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("build subscription request: %w", err)
	}
	req.Header.Set("User-Agent", "MGB VPN/1.0")
	req.Header.Set("Accept", "text/plain, application/json, */*")
	if options.DeviceID != "" {
		req.Header.Set("X-Device-ID", options.DeviceID)
		req.Header.Set("X-HWID", options.DeviceID)
	}
	if options.DeviceName != "" {
		req.Header.Set("X-Device-Name", options.DeviceName)
	}
	if options.ETag != "" {
		req.Header.Set("If-None-Match", options.ETag)
	}
	if options.LastModified != "" {
		req.Header.Set("If-Modified-Since", options.LastModified)
	}

	resp, err := client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("download subscription: %w", err)
	}
	defer resp.Body.Close()

	metadata := metadataFromHeaders(resp.Header)
	if resp.StatusCode == http.StatusNotModified {
		return FetchResult{Metadata: metadata, NotModified: true}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return FetchResult{}, fmt.Errorf("download subscription: unexpected HTTP status %s", resp.Status)
	}

	body, err := readLimited(resp.Body)
	if err != nil {
		return FetchResult{}, err
	}
	servers, err := config.LoadServersFromBytes(body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("parse subscription servers: %w", err)
	}
	return FetchResult{
		Servers:  servers,
		Metadata: metadata,
	}, nil
}

func readLimited(reader io.Reader) ([]byte, error) {
	limited := io.LimitReader(reader, maxSubscriptionBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read subscription response: %w", err)
	}
	if len(body) > maxSubscriptionBodyBytes {
		return nil, fmt.Errorf("subscription response is larger than %d bytes", maxSubscriptionBodyBytes)
	}
	return body, nil
}

func metadataFromHeaders(headers http.Header) storage.SubscriptionMetadata {
	meta := storage.SubscriptionMetadata{
		ETag:         stringPtrIfNotEmpty(headers.Get("ETag")),
		LastModified: stringPtrIfNotEmpty(headers.Get("Last-Modified")),
	}

	if userInfo := headers.Get("Subscription-Userinfo"); userInfo != "" {
		parts := parseSemicolonParams(userInfo)
		meta.UploadBytes = int64PtrFromParam(parts, "upload")
		meta.DownloadBytes = int64PtrFromParam(parts, "download")
		meta.TotalBytes = int64PtrFromParam(parts, "total")
		if meta.UploadBytes != nil || meta.DownloadBytes != nil {
			used := int64(0)
			if meta.UploadBytes != nil {
				used += *meta.UploadBytes
			}
			if meta.DownloadBytes != nil {
				used += *meta.DownloadBytes
			}
			meta.UsedBytes = &used
		}
		if expireUnix := int64PtrFromParam(parts, "expire"); expireUnix != nil && *expireUnix > 0 {
			expireAt := time.Unix(*expireUnix, 0).UTC().Format(time.RFC3339)
			meta.ExpireAt = &expireAt
		}
	}

	if interval := parseProfileUpdateInterval(headers.Get("Profile-Update-Interval")); interval > 0 {
		meta.ProfileUpdateIntervalMinutes = &interval
	}
	meta.ProfileWebPageURL = stringPtrIfNotEmpty(headers.Get("Profile-Web-Page-URL"))
	meta.SupportURL = stringPtrIfNotEmpty(headers.Get("Support-URL"))
	meta.ProfileTitle = firstStringPtr(
		stringPtrIfNotEmpty(headers.Get("Profile-Title")),
		contentDispositionFilename(headers.Get("Content-Disposition")),
	)

	if raw := selectedHeadersJSON(headers); raw != "" {
		meta.HeadersJSON = &raw
	}
	return meta
}

func parseSemicolonParams(value string) map[string]string {
	result := make(map[string]string)
	for _, part := range strings.Split(value, ";") {
		key, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		result[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(val)
	}
	return result
}

func int64PtrFromParam(values map[string]string, key string) *int64 {
	raw := values[strings.ToLower(key)]
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return nil
	}
	return &value
}

func parseProfileUpdateInterval(value string) int {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return int(parsed * 60)
}

func contentDispositionFilename(value string) *string {
	if value == "" {
		return nil
	}
	_, params, err := mime.ParseMediaType(value)
	if err != nil {
		return nil
	}
	return stringPtrIfNotEmpty(params["filename"])
}

func selectedHeadersJSON(headers http.Header) string {
	selected := map[string][]string{}
	for _, key := range []string{
		"Subscription-Userinfo",
		"Profile-Update-Interval",
		"Profile-Web-Page-URL",
		"Profile-Title",
		"Support-URL",
		"Content-Disposition",
		"ETag",
		"Last-Modified",
	} {
		if values := headers.Values(key); len(values) > 0 {
			selected[key] = values
		}
	}
	if len(selected) == 0 {
		return ""
	}
	data, err := json.Marshal(selected)
	if err != nil {
		return ""
	}
	return string(data)
}

func firstStringPtr(values ...*string) *string {
	for _, value := range values {
		if value != nil && strings.TrimSpace(*value) != "" {
			return value
		}
	}
	return nil
}

func stringPtrIfNotEmpty(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
