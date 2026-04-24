package subscription

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestFetchParsesMetadataAndServers(t *testing.T) {
	expire := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC).Unix()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return response(http.StatusOK, http.Header{
			"Subscription-Userinfo":   []string{"upload=10; download=25; total=100; expire=" + stringInt(expire)},
			"Profile-Update-Interval": []string{"12"},
			"Profile-Title":           []string{"Main"},
			"Etag":                    []string{`"abc"`},
		}, "hysteria2://secret@example.com:443#hy2"), nil
	})}

	result, err := Fetch(context.Background(), client, FetchOptions{URL: "https://example.com/sub"})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(result.Servers) != 1 || result.Servers[0].Name != "hy2" {
		t.Fatalf("servers = %#v, want one hy2 server", result.Servers)
	}
	if result.Metadata.UploadBytes == nil || *result.Metadata.UploadBytes != 10 {
		t.Fatalf("UploadBytes = %#v, want 10", result.Metadata.UploadBytes)
	}
	if result.Metadata.DownloadBytes == nil || *result.Metadata.DownloadBytes != 25 {
		t.Fatalf("DownloadBytes = %#v, want 25", result.Metadata.DownloadBytes)
	}
	if result.Metadata.UsedBytes == nil || *result.Metadata.UsedBytes != 35 {
		t.Fatalf("UsedBytes = %#v, want 35", result.Metadata.UsedBytes)
	}
	if result.Metadata.TotalBytes == nil || *result.Metadata.TotalBytes != 100 {
		t.Fatalf("TotalBytes = %#v, want 100", result.Metadata.TotalBytes)
	}
	if result.Metadata.ProfileUpdateIntervalMinutes == nil || *result.Metadata.ProfileUpdateIntervalMinutes != 720 {
		t.Fatalf("ProfileUpdateIntervalMinutes = %#v, want 720", result.Metadata.ProfileUpdateIntervalMinutes)
	}
	if result.Metadata.ETag == nil || *result.Metadata.ETag != `"abc"` {
		t.Fatalf("ETag = %#v, want abc", result.Metadata.ETag)
	}
}

func TestFetchNotModified(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("If-None-Match") != `"abc"` {
			t.Fatalf("If-None-Match = %q, want abc", req.Header.Get("If-None-Match"))
		}
		return response(http.StatusNotModified, nil, ""), nil
	})}

	result, err := Fetch(context.Background(), client, FetchOptions{URL: "https://example.com/sub", ETag: `"abc"`})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if !result.NotModified {
		t.Fatal("NotModified = false, want true")
	}
}

func stringInt(value int64) string {
	return strconv.FormatInt(value, 10)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func response(status int, header http.Header, body string) *http.Response {
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		StatusCode: status,
		Status:     strconv.Itoa(status) + " " + http.StatusText(status),
		Header:     header,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}
