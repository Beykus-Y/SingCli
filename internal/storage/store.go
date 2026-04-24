package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"SingCli/internal/config"

	_ "modernc.org/sqlite"
)

const (
	importedSettingKey = "servers_json_imported"
	dbFilename         = "mgb.db"
	schemaVersion      = 2
)

type Store struct {
	db *sql.DB
}

type ServerRecord struct {
	ID             int64              `json:"id"`
	Name           string             `json:"name"`
	Type           string             `json:"type"`
	Address        string             `json:"address"`
	Server         config.ServerEntry `json:"server"`
	SubscriptionID *int64             `json:"subscriptionId,omitempty"`
	CreatedAt      string             `json:"createdAt"`
	UpdatedAt      string             `json:"updatedAt"`
}

type ServerInput struct {
	Server         config.ServerEntry `json:"server"`
	SubscriptionID *int64             `json:"subscriptionId,omitempty"`
}

type Subscription struct {
	ID                           int64   `json:"id"`
	Name                         string  `json:"name"`
	URL                          string  `json:"url"`
	Enabled                      bool    `json:"enabled"`
	AutoUpdateIntervalMinutes    int     `json:"autoUpdateIntervalMinutes"`
	ProfileUpdateIntervalMinutes *int    `json:"profileUpdateIntervalMinutes,omitempty"`
	LastCheckedAt                *string `json:"lastCheckedAt,omitempty"`
	LastUpdatedAt                *string `json:"lastUpdatedAt,omitempty"`
	LastError                    *string `json:"lastError,omitempty"`
	UploadBytes                  *int64  `json:"uploadBytes,omitempty"`
	DownloadBytes                *int64  `json:"downloadBytes,omitempty"`
	UsedBytes                    *int64  `json:"usedBytes,omitempty"`
	TotalBytes                   *int64  `json:"totalBytes,omitempty"`
	ExpireAt                     *string `json:"expireAt,omitempty"`
	ProfileTitle                 *string `json:"profileTitle,omitempty"`
	ProfileWebPageURL            *string `json:"profileWebPageUrl,omitempty"`
	SupportURL                   *string `json:"supportUrl,omitempty"`
	HeadersJSON                  *string `json:"headersJson,omitempty"`
	ETag                         *string `json:"etag,omitempty"`
	LastModified                 *string `json:"lastModified,omitempty"`
	CreatedAt                    string  `json:"createdAt"`
	UpdatedAt                    string  `json:"updatedAt"`
}

type SubscriptionInput struct {
	Name                      string `json:"name"`
	URL                       string `json:"url"`
	Enabled                   bool   `json:"enabled"`
	AutoUpdateIntervalMinutes int    `json:"autoUpdateIntervalMinutes"`
}

type ImportResult struct {
	Imported bool   `json:"imported"`
	Path     string `json:"path"`
	Count    int    `json:"count"`
}

type SubscriptionMetadata struct {
	ProfileUpdateIntervalMinutes *int
	UploadBytes                  *int64
	DownloadBytes                *int64
	UsedBytes                    *int64
	TotalBytes                   *int64
	ExpireAt                     *string
	ProfileTitle                 *string
	ProfileWebPageURL            *string
	SupportURL                   *string
	HeadersJSON                  *string
	ETag                         *string
	LastModified                 *string
}

type SubscriptionRefreshResult struct {
	Subscription Subscription `json:"subscription"`
	ServerCount  int          `json:"serverCount"`
	Updated      bool         `json:"updated"`
}

func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(configDir, "MGB", dbFilename), nil
}

func OpenDefault() (*Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return Open(path)
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if err := store.Migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Migrate() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer rollback(tx)

	var version int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > schemaVersion {
		return fmt.Errorf("unsupported db schema version %d", version)
	}
	if version == 0 {
		for _, stmt := range migrationV1 {
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("apply migration v1: %w", err)
			}
		}
		if _, err := tx.Exec("PRAGMA user_version = 1"); err != nil {
			return fmt.Errorf("set schema version: %w", err)
		}
		version = 1
	}
	if version == 1 {
		for _, stmt := range migrationV2 {
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("apply migration v2: %w", err)
			}
		}
		if _, err := tx.Exec("PRAGMA user_version = 2"); err != nil {
			return fmt.Errorf("set schema version: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) GetSetting(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, true, nil
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
	`, key, value)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

func (s *Store) CountServers() (int, error) {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM servers").Scan(&count); err != nil {
		return 0, fmt.Errorf("count servers: %w", err)
	}
	return count, nil
}

func (s *Store) ListServerEntries() ([]config.ServerEntry, error) {
	records, err := s.ListServers()
	if err != nil {
		return nil, err
	}
	servers := make([]config.ServerEntry, 0, len(records))
	for _, record := range records {
		servers = append(servers, record.Server)
	}
	return servers, nil
}

func (s *Store) ListServers() ([]ServerRecord, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.name, s.type, s.address, s.server_json, ss.subscription_id, s.created_at, s.updated_at
		FROM servers s
		LEFT JOIN subscription_servers ss ON ss.server_id = s.id
		ORDER BY s.id
	`)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	defer rows.Close()

	var records []ServerRecord
	for rows.Next() {
		record, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate servers: %w", err)
	}
	return records, nil
}

func (s *Store) GetServer(id int64) (ServerRecord, error) {
	row := s.db.QueryRow(`
		SELECT s.id, s.name, s.type, s.address, s.server_json, ss.subscription_id, s.created_at, s.updated_at
		FROM servers s
		LEFT JOIN subscription_servers ss ON ss.server_id = s.id
		WHERE s.id = ?
	`, id)
	record, err := scanServer(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ServerRecord{}, fmt.Errorf("server %d not found", id)
		}
		return ServerRecord{}, err
	}
	return record, nil
}

func (s *Store) CreateServer(input ServerInput) (ServerRecord, error) {
	if err := validateServer(input.Server); err != nil {
		return ServerRecord{}, err
	}
	payload, err := json.Marshal(input.Server)
	if err != nil {
		return ServerRecord{}, fmt.Errorf("marshal server: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return ServerRecord{}, fmt.Errorf("begin create server: %w", err)
	}
	defer rollback(tx)

	result, err := tx.Exec(`
		INSERT INTO servers (name, type, address, server_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'), strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	`, input.Server.Name, input.Server.Type, input.Server.Server, string(payload))
	if err != nil {
		return ServerRecord{}, fmt.Errorf("create server: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return ServerRecord{}, fmt.Errorf("read new server id: %w", err)
	}
	if input.SubscriptionID != nil {
		if err := setServerSubscriptionTx(tx, id, input.SubscriptionID); err != nil {
			return ServerRecord{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ServerRecord{}, fmt.Errorf("commit create server: %w", err)
	}
	return s.GetServer(id)
}

func (s *Store) UpdateServer(id int64, input ServerInput) (ServerRecord, error) {
	if err := validateServer(input.Server); err != nil {
		return ServerRecord{}, err
	}
	payload, err := json.Marshal(input.Server)
	if err != nil {
		return ServerRecord{}, fmt.Errorf("marshal server: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return ServerRecord{}, fmt.Errorf("begin update server: %w", err)
	}
	defer rollback(tx)

	result, err := tx.Exec(`
		UPDATE servers
		SET name = ?, type = ?, address = ?, server_json = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
		WHERE id = ?
	`, input.Server.Name, input.Server.Type, input.Server.Server, string(payload), id)
	if err != nil {
		return ServerRecord{}, fmt.Errorf("update server: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ServerRecord{}, fmt.Errorf("read update server rows: %w", err)
	}
	if affected == 0 {
		return ServerRecord{}, fmt.Errorf("server %d not found", id)
	}
	if err := setServerSubscriptionTx(tx, id, input.SubscriptionID); err != nil {
		return ServerRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ServerRecord{}, fmt.Errorf("commit update server: %w", err)
	}
	return s.GetServer(id)
}

func (s *Store) DeleteServer(id int64) error {
	result, err := s.db.Exec("DELETE FROM servers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete server: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete server rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("server %d not found", id)
	}
	return nil
}

func (s *Store) ImportServers(entries []config.ServerEntry) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin import servers: %w", err)
	}
	defer rollback(tx)

	for i, entry := range entries {
		if err := validateServer(entry); err != nil {
			return 0, fmt.Errorf("server %d: %w", i+1, err)
		}
		payload, err := json.Marshal(entry)
		if err != nil {
			return 0, fmt.Errorf("marshal server %d: %w", i+1, err)
		}
		if _, err := tx.Exec(`
			INSERT INTO servers (name, type, address, server_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'), strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		`, entry.Name, entry.Type, entry.Server, string(payload)); err != nil {
			return 0, fmt.Errorf("import server %d: %w", i+1, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit import servers: %w", err)
	}
	return len(entries), nil
}

func (s *Store) ImportServersFromPath(path string) (ImportResult, error) {
	servers, err := config.LoadServers(path)
	if err != nil {
		return ImportResult{}, err
	}
	count, err := s.ImportServers(servers)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Imported: true, Path: path, Count: count}, nil
}

func (s *Store) ImportServersFromCandidatesOnce(paths []string) (ImportResult, error) {
	if value, ok, err := s.GetSetting(importedSettingKey); err != nil {
		return ImportResult{}, err
	} else if ok && value == "true" {
		return ImportResult{}, nil
	}

	count, err := s.CountServers()
	if err != nil {
		return ImportResult{}, err
	}
	if count > 0 {
		if err := s.SetSetting(importedSettingKey, "true"); err != nil {
			return ImportResult{}, err
		}
		return ImportResult{}, nil
	}

	var lastErr error
	for _, path := range paths {
		result, err := s.ImportServersFromPath(path)
		if err != nil {
			lastErr = err
			continue
		}
		if err := s.SetSetting(importedSettingKey, "true"); err != nil {
			return ImportResult{}, err
		}
		return result, nil
	}
	if lastErr != nil {
		return ImportResult{}, fmt.Errorf("import servers.json: %w", lastErr)
	}
	return ImportResult{}, nil
}

func (s *Store) ListSubscriptions() ([]Subscription, error) {
	rows, err := s.db.Query(`
		SELECT id, name, url, enabled, auto_update_interval_minutes, profile_update_interval_minutes,
			last_checked_at, last_updated_at, last_error,
			upload_bytes, download_bytes, used_bytes, total_bytes, expire_at,
			profile_title, profile_web_page_url, support_url, headers_json, etag, last_modified,
			created_at, updated_at
		FROM subscriptions
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []Subscription
	for rows.Next() {
		subscription, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscriptions: %w", err)
	}
	return subscriptions, nil
}

func (s *Store) GetSubscription(id int64) (Subscription, error) {
	row := s.db.QueryRow(`
		SELECT id, name, url, enabled, auto_update_interval_minutes, profile_update_interval_minutes,
			last_checked_at, last_updated_at, last_error,
			upload_bytes, download_bytes, used_bytes, total_bytes, expire_at,
			profile_title, profile_web_page_url, support_url, headers_json, etag, last_modified,
			created_at, updated_at
		FROM subscriptions
		WHERE id = ?
	`, id)
	subscription, err := scanSubscription(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Subscription{}, fmt.Errorf("subscription %d not found", id)
		}
		return Subscription{}, err
	}
	return subscription, nil
}

func (s *Store) CreateSubscription(input SubscriptionInput) (Subscription, error) {
	input = normalizeSubscriptionInput(input)
	if err := validateSubscription(input); err != nil {
		return Subscription{}, err
	}
	result, err := s.db.Exec(`
		INSERT INTO subscriptions (name, url, enabled, auto_update_interval_minutes, created_at, updated_at)
		VALUES (?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'), strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	`, input.Name, input.URL, boolInt(input.Enabled), input.AutoUpdateIntervalMinutes)
	if err != nil {
		return Subscription{}, fmt.Errorf("create subscription: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Subscription{}, fmt.Errorf("read new subscription id: %w", err)
	}
	return s.GetSubscription(id)
}

func (s *Store) UpdateSubscription(id int64, input SubscriptionInput) (Subscription, error) {
	input = normalizeSubscriptionInput(input)
	if err := validateSubscription(input); err != nil {
		return Subscription{}, err
	}
	result, err := s.db.Exec(`
		UPDATE subscriptions
		SET name = ?, url = ?, enabled = ?, auto_update_interval_minutes = ?,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
		WHERE id = ?
	`, input.Name, input.URL, boolInt(input.Enabled), input.AutoUpdateIntervalMinutes, id)
	if err != nil {
		return Subscription{}, fmt.Errorf("update subscription: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Subscription{}, fmt.Errorf("read update subscription rows: %w", err)
	}
	if affected == 0 {
		return Subscription{}, fmt.Errorf("subscription %d not found", id)
	}
	return s.GetSubscription(id)
}

func (s *Store) DeleteSubscription(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete subscription: %w", err)
	}
	defer rollback(tx)

	if _, err := tx.Exec(`
		DELETE FROM servers
		WHERE id IN (
			SELECT server_id FROM subscription_servers WHERE subscription_id = ?
		)
	`, id); err != nil {
		return fmt.Errorf("delete subscription servers: %w", err)
	}

	result, err := tx.Exec("DELETE FROM subscriptions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete subscription rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("subscription %d not found", id)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete subscription: %w", err)
	}
	return nil
}

func (s *Store) ListSubscriptionServers(subscriptionID int64) ([]ServerRecord, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.name, s.type, s.address, s.server_json, ss.subscription_id, s.created_at, s.updated_at
		FROM subscription_servers ss
		JOIN servers s ON s.id = ss.server_id
		WHERE ss.subscription_id = ?
		ORDER BY s.id
	`, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("list subscription servers: %w", err)
	}
	defer rows.Close()

	var records []ServerRecord
	for rows.Next() {
		record, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscription servers: %w", err)
	}
	return records, nil
}

func (s *Store) SetServerSubscription(serverID int64, subscriptionID *int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin set server subscription: %w", err)
	}
	defer rollback(tx)
	if err := setServerSubscriptionTx(tx, serverID, subscriptionID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit set server subscription: %w", err)
	}
	return nil
}

func (s *Store) ReplaceSubscriptionServers(subscriptionID int64, entries []config.ServerEntry, metadata SubscriptionMetadata) (SubscriptionRefreshResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return SubscriptionRefreshResult{}, fmt.Errorf("begin refresh subscription: %w", err)
	}
	defer rollback(tx)

	if err := updateSubscriptionSuccessTx(tx, subscriptionID, metadata, true); err != nil {
		return SubscriptionRefreshResult{}, err
	}

	if _, err := tx.Exec(`
		DELETE FROM servers
		WHERE id IN (
			SELECT server_id FROM subscription_servers WHERE subscription_id = ?
		)
	`, subscriptionID); err != nil {
		return SubscriptionRefreshResult{}, fmt.Errorf("delete old subscription servers: %w", err)
	}

	for i, entry := range entries {
		if err := validateServer(entry); err != nil {
			return SubscriptionRefreshResult{}, fmt.Errorf("subscription server %d: %w", i+1, err)
		}
		payload, err := json.Marshal(entry)
		if err != nil {
			return SubscriptionRefreshResult{}, fmt.Errorf("marshal subscription server %d: %w", i+1, err)
		}
		result, err := tx.Exec(`
			INSERT INTO servers (name, type, address, server_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'), strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		`, entry.Name, entry.Type, entry.Server, string(payload))
		if err != nil {
			return SubscriptionRefreshResult{}, fmt.Errorf("insert subscription server %d: %w", i+1, err)
		}
		serverID, err := result.LastInsertId()
		if err != nil {
			return SubscriptionRefreshResult{}, fmt.Errorf("read subscription server id %d: %w", i+1, err)
		}
		if err := setServerSubscriptionTx(tx, serverID, &subscriptionID); err != nil {
			return SubscriptionRefreshResult{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return SubscriptionRefreshResult{}, fmt.Errorf("commit refresh subscription: %w", err)
	}
	subscription, err := s.GetSubscription(subscriptionID)
	if err != nil {
		return SubscriptionRefreshResult{}, err
	}
	return SubscriptionRefreshResult{
		Subscription: subscription,
		ServerCount:  len(entries),
		Updated:      true,
	}, nil
}

func (s *Store) MarkSubscriptionNotModified(subscriptionID int64, metadata SubscriptionMetadata) (SubscriptionRefreshResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return SubscriptionRefreshResult{}, fmt.Errorf("begin mark subscription checked: %w", err)
	}
	defer rollback(tx)
	if err := markSubscriptionCheckedTx(tx, subscriptionID, metadata); err != nil {
		return SubscriptionRefreshResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return SubscriptionRefreshResult{}, fmt.Errorf("commit mark subscription checked: %w", err)
	}
	subscription, err := s.GetSubscription(subscriptionID)
	if err != nil {
		return SubscriptionRefreshResult{}, err
	}
	count, err := s.CountSubscriptionServers(subscriptionID)
	if err != nil {
		return SubscriptionRefreshResult{}, err
	}
	return SubscriptionRefreshResult{
		Subscription: subscription,
		ServerCount:  count,
		Updated:      false,
	}, nil
}

func (s *Store) RecordSubscriptionError(subscriptionID int64, message string) (Subscription, error) {
	result, err := s.db.Exec(`
		UPDATE subscriptions
		SET last_checked_at = strftime('%Y-%m-%dT%H:%M:%fZ','now'),
			last_error = ?,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
		WHERE id = ?
	`, strings.TrimSpace(message), subscriptionID)
	if err != nil {
		return Subscription{}, fmt.Errorf("record subscription error: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Subscription{}, fmt.Errorf("read subscription error rows: %w", err)
	}
	if affected == 0 {
		return Subscription{}, fmt.Errorf("subscription %d not found", subscriptionID)
	}
	return s.GetSubscription(subscriptionID)
}

func (s *Store) CountSubscriptionServers(subscriptionID int64) (int, error) {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM subscription_servers WHERE subscription_id = ?", subscriptionID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count subscription servers: %w", err)
	}
	return count, nil
}

func updateSubscriptionSuccessTx(tx *sql.Tx, subscriptionID int64, metadata SubscriptionMetadata, contentUpdated bool) error {
	lastUpdatedAtExpr := "last_updated_at"
	if contentUpdated {
		lastUpdatedAtExpr = "strftime('%Y-%m-%dT%H:%M:%fZ','now')"
	}
	result, err := tx.Exec(fmt.Sprintf(`
		UPDATE subscriptions
		SET profile_update_interval_minutes = ?,
			last_checked_at = strftime('%%Y-%%m-%%dT%%H:%%M:%%fZ','now'),
			last_updated_at = %s,
			last_error = NULL,
			upload_bytes = ?,
			download_bytes = ?,
			used_bytes = ?,
			total_bytes = ?,
			expire_at = ?,
			profile_title = ?,
			profile_web_page_url = ?,
			support_url = ?,
			headers_json = ?,
			etag = ?,
			last_modified = ?,
			updated_at = strftime('%%Y-%%m-%%dT%%H:%%M:%%fZ','now')
		WHERE id = ?
	`, lastUpdatedAtExpr),
		nullableInt(metadata.ProfileUpdateIntervalMinutes),
		nullableInt64(metadata.UploadBytes),
		nullableInt64(metadata.DownloadBytes),
		nullableInt64(metadata.UsedBytes),
		nullableInt64(metadata.TotalBytes),
		nullableString(metadata.ExpireAt),
		nullableString(metadata.ProfileTitle),
		nullableString(metadata.ProfileWebPageURL),
		nullableString(metadata.SupportURL),
		nullableString(metadata.HeadersJSON),
		nullableString(metadata.ETag),
		nullableString(metadata.LastModified),
		subscriptionID,
	)
	if err != nil {
		return fmt.Errorf("update subscription metadata: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read subscription metadata rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("subscription %d not found", subscriptionID)
	}
	return nil
}

func markSubscriptionCheckedTx(tx *sql.Tx, subscriptionID int64, metadata SubscriptionMetadata) error {
	result, err := tx.Exec(`
		UPDATE subscriptions
		SET last_checked_at = strftime('%Y-%m-%dT%H:%M:%fZ','now'),
			last_error = NULL,
			headers_json = COALESCE(?, headers_json),
			etag = COALESCE(?, etag),
			last_modified = COALESCE(?, last_modified),
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
		WHERE id = ?
	`,
		nullableString(metadata.HeadersJSON),
		nullableString(metadata.ETag),
		nullableString(metadata.LastModified),
		subscriptionID,
	)
	if err != nil {
		return fmt.Errorf("mark subscription checked: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read subscription checked rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("subscription %d not found", subscriptionID)
	}
	return nil
}

func setServerSubscriptionTx(tx *sql.Tx, serverID int64, subscriptionID *int64) error {
	if _, err := tx.Exec("DELETE FROM subscription_servers WHERE server_id = ?", serverID); err != nil {
		return fmt.Errorf("clear server subscription: %w", err)
	}
	if subscriptionID == nil {
		return nil
	}
	if _, err := tx.Exec(`
		INSERT INTO subscription_servers (subscription_id, server_id, created_at)
		VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	`, *subscriptionID, serverID); err != nil {
		return fmt.Errorf("set server subscription: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanServer(row rowScanner) (ServerRecord, error) {
	var record ServerRecord
	var payload string
	var subscriptionID sql.NullInt64
	if err := row.Scan(
		&record.ID,
		&record.Name,
		&record.Type,
		&record.Address,
		&payload,
		&subscriptionID,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return ServerRecord{}, err
	}
	if subscriptionID.Valid {
		record.SubscriptionID = &subscriptionID.Int64
	}
	if err := json.Unmarshal([]byte(payload), &record.Server); err != nil {
		return ServerRecord{}, fmt.Errorf("unmarshal server %d: %w", record.ID, err)
	}
	return record, nil
}

func scanSubscription(row rowScanner) (Subscription, error) {
	var subscription Subscription
	var enabled int
	var profileUpdateIntervalMinutes sql.NullInt64
	var lastCheckedAt sql.NullString
	var lastUpdatedAt sql.NullString
	var lastError sql.NullString
	var uploadBytes sql.NullInt64
	var downloadBytes sql.NullInt64
	var usedBytes sql.NullInt64
	var totalBytes sql.NullInt64
	var expireAt sql.NullString
	var profileTitle sql.NullString
	var profileWebPageURL sql.NullString
	var supportURL sql.NullString
	var headersJSON sql.NullString
	var etag sql.NullString
	var lastModified sql.NullString
	if err := row.Scan(
		&subscription.ID,
		&subscription.Name,
		&subscription.URL,
		&enabled,
		&subscription.AutoUpdateIntervalMinutes,
		&profileUpdateIntervalMinutes,
		&lastCheckedAt,
		&lastUpdatedAt,
		&lastError,
		&uploadBytes,
		&downloadBytes,
		&usedBytes,
		&totalBytes,
		&expireAt,
		&profileTitle,
		&profileWebPageURL,
		&supportURL,
		&headersJSON,
		&etag,
		&lastModified,
		&subscription.CreatedAt,
		&subscription.UpdatedAt,
	); err != nil {
		return Subscription{}, err
	}
	subscription.Enabled = enabled != 0
	if profileUpdateIntervalMinutes.Valid {
		value := int(profileUpdateIntervalMinutes.Int64)
		subscription.ProfileUpdateIntervalMinutes = &value
	}
	if lastCheckedAt.Valid {
		subscription.LastCheckedAt = &lastCheckedAt.String
	}
	if lastUpdatedAt.Valid {
		subscription.LastUpdatedAt = &lastUpdatedAt.String
	}
	if lastError.Valid {
		subscription.LastError = &lastError.String
	}
	if uploadBytes.Valid {
		subscription.UploadBytes = &uploadBytes.Int64
	}
	if downloadBytes.Valid {
		subscription.DownloadBytes = &downloadBytes.Int64
	}
	if usedBytes.Valid {
		subscription.UsedBytes = &usedBytes.Int64
	}
	if totalBytes.Valid {
		subscription.TotalBytes = &totalBytes.Int64
	}
	if expireAt.Valid {
		subscription.ExpireAt = &expireAt.String
	}
	if profileTitle.Valid {
		subscription.ProfileTitle = &profileTitle.String
	}
	if profileWebPageURL.Valid {
		subscription.ProfileWebPageURL = &profileWebPageURL.String
	}
	if supportURL.Valid {
		subscription.SupportURL = &supportURL.String
	}
	if headersJSON.Valid {
		subscription.HeadersJSON = &headersJSON.String
	}
	if etag.Valid {
		subscription.ETag = &etag.String
	}
	if lastModified.Valid {
		subscription.LastModified = &lastModified.String
	}
	return subscription, nil
}

func validateServer(server config.ServerEntry) error {
	if strings.TrimSpace(server.Name) == "" {
		return fmt.Errorf("server name is required")
	}
	if strings.TrimSpace(server.Type) == "" {
		return fmt.Errorf("server type is required")
	}
	if strings.TrimSpace(server.Server) == "" {
		return fmt.Errorf("server address is required")
	}
	return nil
}

func validateSubscription(input SubscriptionInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("subscription name is required")
	}
	if strings.TrimSpace(input.URL) == "" {
		return fmt.Errorf("subscription url is required")
	}
	if input.AutoUpdateIntervalMinutes < 0 {
		return fmt.Errorf("subscription auto update interval must not be negative")
	}
	return nil
}

func normalizeSubscriptionInput(input SubscriptionInput) SubscriptionInput {
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimSpace(input.URL)
	if input.AutoUpdateIntervalMinutes == 0 {
		input.AutoUpdateIntervalMinutes = 1440
	}
	return input
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}

var migrationV1 = []string{
	`CREATE TABLE settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,
	`CREATE TABLE subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		last_updated_at TEXT,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,
	`CREATE TABLE servers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		address TEXT NOT NULL,
		server_json TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,
	`CREATE TABLE subscription_servers (
		subscription_id INTEGER NOT NULL,
		server_id INTEGER NOT NULL UNIQUE,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		PRIMARY KEY (subscription_id, server_id),
		FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE CASCADE,
		FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX idx_servers_name ON servers(name)`,
	`CREATE INDEX idx_subscription_servers_subscription ON subscription_servers(subscription_id)`,
}

var migrationV2 = []string{
	`ALTER TABLE subscriptions ADD COLUMN auto_update_interval_minutes INTEGER NOT NULL DEFAULT 1440`,
	`ALTER TABLE subscriptions ADD COLUMN profile_update_interval_minutes INTEGER`,
	`ALTER TABLE subscriptions ADD COLUMN last_checked_at TEXT`,
	`ALTER TABLE subscriptions ADD COLUMN last_error TEXT`,
	`ALTER TABLE subscriptions ADD COLUMN upload_bytes INTEGER`,
	`ALTER TABLE subscriptions ADD COLUMN download_bytes INTEGER`,
	`ALTER TABLE subscriptions ADD COLUMN used_bytes INTEGER`,
	`ALTER TABLE subscriptions ADD COLUMN total_bytes INTEGER`,
	`ALTER TABLE subscriptions ADD COLUMN expire_at TEXT`,
	`ALTER TABLE subscriptions ADD COLUMN profile_title TEXT`,
	`ALTER TABLE subscriptions ADD COLUMN profile_web_page_url TEXT`,
	`ALTER TABLE subscriptions ADD COLUMN support_url TEXT`,
	`ALTER TABLE subscriptions ADD COLUMN headers_json TEXT`,
	`ALTER TABLE subscriptions ADD COLUMN etag TEXT`,
	`ALTER TABLE subscriptions ADD COLUMN last_modified TEXT`,
	`CREATE INDEX idx_subscriptions_enabled ON subscriptions(enabled)`,
	`CREATE INDEX idx_subscriptions_url ON subscriptions(url)`,
}
