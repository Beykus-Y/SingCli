package storage

import (
	"path/filepath"
	"reflect"
	"testing"

	"SingCli/internal/config"
)

func TestMigrateFreshAndReopenKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mgb.db")

	store := openTestStore(t, path)
	if err := store.SetSetting("mode", "tun"); err != nil {
		t.Fatalf("SetSetting() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	store = openTestStore(t, path)
	defer store.Close()
	value, ok, err := store.GetSetting("mode")
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if !ok || value != "tun" {
		t.Fatalf("setting mode = %q/%t, want tun/true", value, ok)
	}
}

func TestServerCRUDRoundTrip(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "mgb.db"))
	defer store.Close()

	server := sampleServer()
	created, err := store.CreateServer(ServerInput{Server: server})
	if err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	if created.ID == 0 {
		t.Fatal("CreateServer() returned zero id")
	}
	if !reflect.DeepEqual(created.Server, server) {
		t.Fatalf("created server = %#v, want %#v", created.Server, server)
	}

	server.Name = "updated"
	server.Server = "example.org:8443"
	updated, err := store.UpdateServer(created.ID, ServerInput{Server: server})
	if err != nil {
		t.Fatalf("UpdateServer() error = %v", err)
	}
	if !reflect.DeepEqual(updated.Server, server) {
		t.Fatalf("updated server = %#v, want %#v", updated.Server, server)
	}

	records, err := store.ListServers()
	if err != nil {
		t.Fatalf("ListServers() error = %v", err)
	}
	if len(records) != 1 || records[0].Name != "updated" || records[0].Address != "example.org:8443" {
		t.Fatalf("records = %#v, want one updated server", records)
	}

	if err := store.DeleteServer(created.ID); err != nil {
		t.Fatalf("DeleteServer() error = %v", err)
	}
	count, err := store.CountServers()
	if err != nil {
		t.Fatalf("CountServers() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("CountServers() = %d, want 0", count)
	}
}

func TestSubscriptionCRUDAndOwnership(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "mgb.db"))
	defer store.Close()

	server, err := store.CreateServer(ServerInput{Server: sampleServer()})
	if err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	subscription, err := store.CreateSubscription(SubscriptionInput{
		Name:    "main",
		URL:     "https://example.com/sub",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateSubscription() error = %v", err)
	}
	if subscription.AutoUpdateIntervalMinutes != 1440 {
		t.Fatalf("AutoUpdateIntervalMinutes = %d, want 1440", subscription.AutoUpdateIntervalMinutes)
	}

	if err := store.SetServerSubscription(server.ID, &subscription.ID); err != nil {
		t.Fatalf("SetServerSubscription() error = %v", err)
	}
	servers, err := store.ListSubscriptionServers(subscription.ID)
	if err != nil {
		t.Fatalf("ListSubscriptionServers() error = %v", err)
	}
	if len(servers) != 1 || servers[0].ID != server.ID || servers[0].SubscriptionID == nil || *servers[0].SubscriptionID != subscription.ID {
		t.Fatalf("subscription servers = %#v, want linked server", servers)
	}

	updated, err := store.UpdateSubscription(subscription.ID, SubscriptionInput{
		Name:    "renamed",
		URL:     "https://example.com/renamed",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("UpdateSubscription() error = %v", err)
	}
	if updated.Name != "renamed" || updated.Enabled {
		t.Fatalf("updated subscription = %#v, want renamed disabled", updated)
	}

	if err := store.SetServerSubscription(server.ID, nil); err != nil {
		t.Fatalf("clear SetServerSubscription() error = %v", err)
	}
	servers, err = store.ListSubscriptionServers(subscription.ID)
	if err != nil {
		t.Fatalf("ListSubscriptionServers() after clear error = %v", err)
	}
	if len(servers) != 0 {
		t.Fatalf("subscription servers after clear = %#v, want none", servers)
	}
}

func TestReplaceSubscriptionServersStoresMetadata(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "mgb.db"))
	defer store.Close()

	subscription, err := store.CreateSubscription(SubscriptionInput{
		Name:                      "main",
		URL:                       "https://example.com/sub",
		Enabled:                   true,
		AutoUpdateIntervalMinutes: 60,
	})
	if err != nil {
		t.Fatalf("CreateSubscription() error = %v", err)
	}
	upload := int64(10)
	download := int64(20)
	used := int64(30)
	total := int64(100)
	expireAt := "2026-05-01T00:00:00Z"
	etag := `"abc"`
	result, err := store.ReplaceSubscriptionServers(subscription.ID, []config.ServerEntry{sampleServer()}, SubscriptionMetadata{
		UploadBytes:   &upload,
		DownloadBytes: &download,
		UsedBytes:     &used,
		TotalBytes:    &total,
		ExpireAt:      &expireAt,
		ETag:          &etag,
	})
	if err != nil {
		t.Fatalf("ReplaceSubscriptionServers() error = %v", err)
	}
	if result.ServerCount != 1 || !result.Updated {
		t.Fatalf("refresh result = %#v, want one updated server", result)
	}
	if result.Subscription.UploadBytes == nil || *result.Subscription.UploadBytes != upload {
		t.Fatalf("UploadBytes = %#v, want %d", result.Subscription.UploadBytes, upload)
	}
	if result.Subscription.DownloadBytes == nil || *result.Subscription.DownloadBytes != download {
		t.Fatalf("DownloadBytes = %#v, want %d", result.Subscription.DownloadBytes, download)
	}
	if result.Subscription.UsedBytes == nil || *result.Subscription.UsedBytes != used {
		t.Fatalf("UsedBytes = %#v, want %d", result.Subscription.UsedBytes, used)
	}
	if result.Subscription.TotalBytes == nil || *result.Subscription.TotalBytes != total {
		t.Fatalf("TotalBytes = %#v, want %d", result.Subscription.TotalBytes, total)
	}
	if result.Subscription.ExpireAt == nil || *result.Subscription.ExpireAt != expireAt {
		t.Fatalf("ExpireAt = %#v, want %q", result.Subscription.ExpireAt, expireAt)
	}
	if result.Subscription.ETag == nil || *result.Subscription.ETag != etag {
		t.Fatalf("ETag = %#v, want %q", result.Subscription.ETag, etag)
	}
	servers, err := store.ListSubscriptionServers(subscription.ID)
	if err != nil {
		t.Fatalf("ListSubscriptionServers() error = %v", err)
	}
	if len(servers) != 1 || servers[0].SubscriptionID == nil || *servers[0].SubscriptionID != subscription.ID {
		t.Fatalf("subscription servers = %#v, want linked server", servers)
	}
	if err := store.DeleteSubscription(subscription.ID); err != nil {
		t.Fatalf("DeleteSubscription() error = %v", err)
	}
	count, err := store.CountServers()
	if err != nil {
		t.Fatalf("CountServers() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("CountServers() after subscription delete = %d, want 0", count)
	}
}

func TestImportServersFromCandidatesOnce(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "mgb.db")
	configPath := filepath.Join(dir, "servers.json")
	if err := config.SaveServers(configPath, []config.ServerEntry{sampleServer()}); err != nil {
		t.Fatalf("SaveServers() error = %v", err)
	}

	store := openTestStore(t, dbPath)
	result, err := store.ImportServersFromCandidatesOnce([]string{configPath})
	if err != nil {
		t.Fatalf("ImportServersFromCandidatesOnce() error = %v", err)
	}
	if !result.Imported || result.Count != 1 || result.Path != configPath {
		t.Fatalf("import result = %#v, want one imported server", result)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	changed := sampleServer()
	changed.Name = "changed-json"
	if err := config.SaveServers(configPath, []config.ServerEntry{changed}); err != nil {
		t.Fatalf("SaveServers changed() error = %v", err)
	}

	store = openTestStore(t, dbPath)
	defer store.Close()
	result, err = store.ImportServersFromCandidatesOnce([]string{configPath})
	if err != nil {
		t.Fatalf("second ImportServersFromCandidatesOnce() error = %v", err)
	}
	if result.Imported {
		t.Fatalf("second import result = %#v, want no import", result)
	}
	records, err := store.ListServers()
	if err != nil {
		t.Fatalf("ListServers() error = %v", err)
	}
	if len(records) != 1 || records[0].Name != "demo" {
		t.Fatalf("records after second import = %#v, want original DB data", records)
	}
}

func openTestStore(t *testing.T, path string) *Store {
	t.Helper()
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return store
}

func sampleServer() config.ServerEntry {
	return config.ServerEntry{
		Name:   "demo",
		Type:   "vless",
		Server: "example.com:443",
		UUID:   "26aa11f0-35e5-4a51-94f6-60ac63c96a35",
		TLS: config.TLSConfig{
			Enabled:    true,
			ServerName: "example.com",
		},
		Network: "ws",
		Path:    "/ws",
		Host:    "front.example.com",
		Flow:    "xtls-rprx-vision",
	}
}
