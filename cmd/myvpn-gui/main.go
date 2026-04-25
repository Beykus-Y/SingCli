package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"SingCli/internal/config"
	"SingCli/internal/core"
	"SingCli/internal/deviceid"
	"SingCli/internal/logger"
	"SingCli/internal/netstats"
	"SingCli/internal/storage"
	subfetch "SingCli/internal/subscription"
	"SingCli/internal/tunnel"
	"SingCli/internal/tunsettings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	socksAddr = "127.0.0.1:1080"
	httpAddr  = "127.0.0.1:1081"

	modeProxy = "proxy"
	modeTun   = "tun"

	eventState = "state"
	eventLog   = "log"
)

//go:embed all:frontend/dist
var assets embed.FS

type ServerSummary struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Address        string `json:"address"`
	SubscriptionID *int64 `json:"subscriptionId,omitempty"`
}

type AppState struct {
	Servers        []ServerSummary `json:"servers"`
	SelectedServer string          `json:"selectedServer"`
	Mode           string          `json:"mode"`
	Status         string          `json:"status"`
	Running        bool            `json:"running"`
	Connecting     bool            `json:"connecting"`
	CanConnect     bool            `json:"canConnect"`
	CanDisconnect  bool            `json:"canDisconnect"`
	Logs           []string        `json:"logs"`
	ProxySocks     string          `json:"proxySocks"`
	ProxyHTTP      string          `json:"proxyHTTP"`
	ConnectedAt    string          `json:"connectedAt,omitempty"`
	Traffic        TrafficState    `json:"traffic"`
}

type TrafficState struct {
	SessionDownloadBytes uint64 `json:"sessionDownloadBytes"`
	SessionUploadBytes   uint64 `json:"sessionUploadBytes"`
	DownloadBytesPerSec  uint64 `json:"downloadBytesPerSec"`
	UploadBytesPerSec    uint64 `json:"uploadBytesPerSec"`
}

type App struct {
	ctx context.Context

	mu             sync.Mutex
	store          *storage.Store
	servers        []storage.ServerRecord
	selectedServer string
	mode           string
	status         string
	logs           []string
	manager        *tunnel.Manager
	running        bool
	connecting     bool
	connectedAt    time.Time
	traffic        TrafficState
	connectionID   uint64
	refreshMu      sync.Mutex
	httpClient     *http.Client
	autoCancel     context.CancelFunc
	trafficCancel  context.CancelFunc
	deviceInfo     deviceid.Info
	tray           *trayService
	quitting       bool
}

func main() {
	if !isRunningAsAdmin() {
		if err := restartAsAdmin(); err != nil {
			log.Fatal(err)
		}
		return
	}

	exePath, _ := os.Executable()
	logger.Init(exePath)

	app := NewApp()
	if err := wails.Run(&options.App{
		Title:             "MGB VPN",
		Width:             760,
		Height:            560,
		MinWidth:          680,
		MinHeight:         500,
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         false,
		HideWindowOnClose: hideWindowOnClose(),
		Bind:              []interface{}{app},
		OnStartup:         app.Startup,
		OnShutdown:        app.Shutdown,
		BackgroundColour:  &options.RGBA{R: 12, G: 15, B: 23, A: 255},
		AssetServer:       &assetserver.Options{Assets: assets},
		CSSDragProperty:   "--wails-draggable",
		CSSDragValue:      "drag",
	}); err != nil {
		log.Fatal(err)
	}
}

func NewApp() *App {
	return &App{
		mode:       modeProxy,
		status:     "Loading servers...",
		logs:       make([]string, 0, 64),
		httpClient: &http.Client{Timeout: 45 * time.Second},
		deviceInfo: deviceid.Get(),
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.initStorage(); err != nil {
		a.mu.Lock()
		a.status = "Cannot open app database"
		a.appendLogLocked(err.Error())
		state := a.stateLocked()
		a.mu.Unlock()
		a.emitState(state)
		return
	}
	a.tray = newTrayService(a)
	a.tray.Start(ctx)
	a.startAutoRefresh(ctx)
	a.ReloadServers()
}

func (a *App) Shutdown(ctx context.Context) {
	if a.autoCancel != nil {
		a.autoCancel()
	}
	a.stopTrafficMonitor()
	if a.tray != nil {
		a.tray.Stop()
	}
	a.Disconnect()
	if a.store != nil {
		if err := a.store.Close(); err != nil {
			logger.Errorf("Failed to close app database: %v", err)
		}
	}
}

func (a *App) GetState() AppState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stateLocked()
}

func (a *App) GetServers() []ServerSummary {
	a.mu.Lock()
	defer a.mu.Unlock()
	servers := make([]ServerSummary, 0, len(a.servers))
	for _, record := range a.servers {
		servers = append(servers, serverSummary(record))
	}
	return servers
}

func (a *App) ListServers() ([]storage.ServerRecord, error) {
	if err := a.ensureStorage(); err != nil {
		return nil, err
	}
	return a.store.ListServers()
}

func (a *App) GetServer(id int64) (storage.ServerRecord, error) {
	if err := a.ensureStorage(); err != nil {
		return storage.ServerRecord{}, err
	}
	return a.store.GetServer(id)
}

func (a *App) CreateServer(input storage.ServerInput) (storage.ServerRecord, error) {
	if err := a.ensureStorage(); err != nil {
		return storage.ServerRecord{}, err
	}
	record, err := a.store.CreateServer(input)
	if err != nil {
		return storage.ServerRecord{}, err
	}
	a.ReloadServers()
	return record, nil
}

func (a *App) UpdateServer(id int64, input storage.ServerInput) (storage.ServerRecord, error) {
	if err := a.ensureStorage(); err != nil {
		return storage.ServerRecord{}, err
	}
	record, err := a.store.UpdateServer(id, input)
	if err != nil {
		return storage.ServerRecord{}, err
	}
	a.ReloadServers()
	return record, nil
}

func (a *App) DeleteServer(id int64) error {
	if err := a.ensureStorage(); err != nil {
		return err
	}
	if err := a.store.DeleteServer(id); err != nil {
		return err
	}
	a.ReloadServers()
	return nil
}

func (a *App) ListSubscriptions() ([]storage.Subscription, error) {
	if err := a.ensureStorage(); err != nil {
		return nil, err
	}
	return a.store.ListSubscriptions()
}

func (a *App) GetSubscription(id int64) (storage.Subscription, error) {
	if err := a.ensureStorage(); err != nil {
		return storage.Subscription{}, err
	}
	return a.store.GetSubscription(id)
}

func (a *App) CreateSubscription(input storage.SubscriptionInput) (storage.Subscription, error) {
	if err := a.ensureStorage(); err != nil {
		return storage.Subscription{}, err
	}
	return a.store.CreateSubscription(input)
}

func (a *App) AddSubscription(input storage.SubscriptionInput) (storage.SubscriptionRefreshResult, error) {
	if err := a.ensureStorage(); err != nil {
		return storage.SubscriptionRefreshResult{}, err
	}
	subscription, err := a.store.CreateSubscription(input)
	if err != nil {
		return storage.SubscriptionRefreshResult{}, err
	}
	result, err := a.refreshSubscription(context.Background(), subscription.ID)
	if err != nil {
		a.mu.Lock()
		a.appendLogLocked(fmt.Sprintf("Subscription %s refresh failed: %v", subscription.Name, err))
		a.mu.Unlock()
		if refreshed, getErr := a.store.GetSubscription(subscription.ID); getErr == nil {
			count, _ := a.store.CountSubscriptionServers(subscription.ID)
			return storage.SubscriptionRefreshResult{Subscription: refreshed, ServerCount: count}, nil
		}
		return storage.SubscriptionRefreshResult{Subscription: subscription}, nil
	}
	return result, nil
}

func (a *App) UpdateSubscription(id int64, input storage.SubscriptionInput) (storage.Subscription, error) {
	if err := a.ensureStorage(); err != nil {
		return storage.Subscription{}, err
	}
	return a.store.UpdateSubscription(id, input)
}

func (a *App) RefreshSubscription(id int64) (storage.SubscriptionRefreshResult, error) {
	if err := a.ensureStorage(); err != nil {
		return storage.SubscriptionRefreshResult{}, err
	}
	return a.refreshSubscription(context.Background(), id)
}

func (a *App) DeleteSubscription(id int64) error {
	if err := a.ensureStorage(); err != nil {
		return err
	}
	if err := a.store.DeleteSubscription(id); err != nil {
		return err
	}
	a.ReloadServers()
	return nil
}

func (a *App) ListSubscriptionServers(subscriptionID int64) ([]storage.ServerRecord, error) {
	if err := a.ensureStorage(); err != nil {
		return nil, err
	}
	return a.store.ListSubscriptionServers(subscriptionID)
}

func (a *App) SetServerSubscription(serverID int64, subscriptionID *int64) error {
	if err := a.ensureStorage(); err != nil {
		return err
	}
	if err := a.store.SetServerSubscription(serverID, subscriptionID); err != nil {
		return err
	}
	a.ReloadServers()
	return nil
}

func (a *App) ImportServers(source string) (storage.ImportResult, error) {
	if err := a.ensureStorage(); err != nil {
		return storage.ImportResult{}, err
	}

	var (
		result storage.ImportResult
		err    error
	)
	source = strings.TrimSpace(source)
	if source == "" {
		result, err = a.store.ImportServersFromCandidatesOnce(config.FindServersConfigCandidates())
	} else if shouldImportInline(source) {
		result, err = a.store.ImportServersFromBytes("inline", []byte(source))
	} else {
		result, err = a.store.ImportServersFromPath(source)
	}
	if err != nil {
		return storage.ImportResult{}, err
	}
	a.ReloadServers()
	return result, nil
}

func (a *App) ReloadServers() AppState {
	if err := a.ensureStorage(); err != nil {
		a.mu.Lock()
		a.servers = nil
		a.selectedServer = ""
		a.status = "Cannot open app database"
		a.appendLogLocked(err.Error())
		state := a.stateLocked()
		a.mu.Unlock()
		a.emitState(state)
		return state
	}

	servers, err := a.store.ListServers()
	if err != nil {
		a.mu.Lock()
		a.servers = nil
		a.selectedServer = ""
		a.status = "Cannot load servers"
		a.appendLogLocked(fmt.Sprintf("Failed to load servers from database: %v", err))
		state := a.stateLocked()
		a.mu.Unlock()
		a.emitState(state)
		return state
	}

	a.mu.Lock()
	a.servers = servers
	if len(servers) == 0 {
		a.selectedServer = ""
		a.status = "No servers found"
	} else {
		if a.selectedServer == "" || !serverExists(servers, a.selectedServer) {
			a.selectedServer = servers[0].Name
		}
		a.status = "Ready"
	}
	a.appendLogLocked(fmt.Sprintf("Loaded %d server(s) from database", len(servers)))
	state := a.stateLocked()
	a.mu.Unlock()

	a.emitState(state)
	return state
}

func (a *App) Connect(serverName string, mode string) AppState {
	if mode != modeProxy && mode != modeTun {
		mode = modeProxy
	}

	a.mu.Lock()
	if a.running || a.connecting {
		state := a.stateLocked()
		a.mu.Unlock()
		return state
	}

	server, ok := a.serverByNameLocked(serverName)
	if !ok {
		a.status = "Select a server"
		state := a.stateLocked()
		a.mu.Unlock()
		a.emitState(state)
		return state
	}

	if mode == modeTun {
		if err := checkWintunDLL(); err != nil {
			a.mode = mode
			a.selectedServer = server.Name
			a.status = "TUN is not ready"
			a.appendLogLocked(err.Error())
			state := a.stateLocked()
			a.mu.Unlock()
			a.emitState(state)
			return state
		}
	}

	a.mode = mode
	a.selectedServer = server.Name
	a.connecting = true
	a.connectionID++
	connectionID := a.connectionID
	a.status = "Connecting..."
	a.appendLogLocked(fmt.Sprintf("Connecting to %s...", server.Name))
	state := a.stateLocked()
	a.mu.Unlock()
	a.emitState(state)

	go a.connectAsync(connectionID, server, mode)
	return state
}

func (a *App) Disconnect() AppState {
	a.mu.Lock()
	mgr := a.manager
	a.manager = nil
	wasRunning := a.running || a.connecting
	a.running = false
	a.connecting = false
	a.connectedAt = time.Time{}
	a.traffic = TrafficState{}
	a.connectionID++
	a.stopTrafficMonitorLocked()
	a.mu.Unlock()

	if mgr != nil {
		mgr.Stop()
	}

	a.mu.Lock()
	if wasRunning {
		a.appendLogLocked("Disconnected")
	}
	a.status = "Ready"
	state := a.stateLocked()
	a.mu.Unlock()
	a.emitState(state)
	return state
}

func (a *App) connectAsync(connectionID uint64, server config.ServerEntry, mode string) {
	var (
		mgr *tunnel.Manager
		err error
	)

	if mode == modeTun {
		opts, buildErr := config.BuildTunOptionsForServer(server)
		if buildErr != nil {
			err = buildErr
		} else {
			mgr = tunnel.NewTun()
			err = mgr.Start(opts)
		}
	} else {
		opts, buildErr := config.BuildOptionsForServer(server)
		if buildErr != nil {
			err = buildErr
		} else {
			mgr = tunnel.New(httpAddr)
			err = mgr.Start(opts)
		}
	}

	if err != nil {
		if mgr != nil {
			mgr.Stop()
		}
		a.mu.Lock()
		if connectionID != a.connectionID {
			a.mu.Unlock()
			return
		}
		a.manager = nil
		a.running = false
		a.connecting = false
		a.connectedAt = time.Time{}
		a.traffic = TrafficState{}
		a.stopTrafficMonitorLocked()
		a.status = "Connection failed"
		a.appendLogLocked(fmt.Sprintf("Connection failed: %v", err))
		state := a.stateLocked()
		a.mu.Unlock()
		a.emitState(state)
		return
	}

	a.mu.Lock()
	if connectionID != a.connectionID {
		a.mu.Unlock()
		mgr.Stop()
		return
	}
	a.manager = mgr
	a.running = true
	a.connecting = false
	a.connectedAt = time.Now().UTC()
	a.traffic = TrafficState{}
	a.startTrafficMonitorLocked(connectionID, mode)
	a.status = "Connected"
	a.appendLogLocked(fmt.Sprintf("Connected to %s", server.Name))
	state := a.stateLocked()
	a.mu.Unlock()
	a.emitState(state)
}

func (a *App) serverByNameLocked(name string) (config.ServerEntry, bool) {
	if name == "" && len(a.servers) > 0 {
		return a.servers[0].Server, true
	}
	for _, record := range a.servers {
		if record.Name == name {
			return record.Server, true
		}
	}
	return config.ServerEntry{}, false
}

func (a *App) appendLogLocked(text string) {
	if text == "" {
		return
	}
	a.logs = append(a.logs, text)
	if len(a.logs) > 300 {
		a.logs = a.logs[len(a.logs)-300:]
	}
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, eventLog, text)
	}
}

func (a *App) stateLocked() AppState {
	servers := make([]ServerSummary, 0, len(a.servers))
	for _, record := range a.servers {
		servers = append(servers, serverSummary(record))
	}

	logs := append([]string(nil), a.logs...)
	canConnect := len(a.servers) > 0 && !a.running && !a.connecting
	connectedAt := ""
	if !a.connectedAt.IsZero() {
		connectedAt = a.connectedAt.Format(time.RFC3339)
	}
	return AppState{
		Servers:        servers,
		SelectedServer: a.selectedServer,
		Mode:           a.mode,
		Status:         a.status,
		Running:        a.running,
		Connecting:     a.connecting,
		CanConnect:     canConnect,
		CanDisconnect:  a.running || a.connecting,
		Logs:           logs,
		ProxySocks:     socksAddr,
		ProxyHTTP:      httpAddr,
		ConnectedAt:    connectedAt,
		Traffic:        a.traffic,
	}
}

func (a *App) emitState(state AppState) {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, eventState, state)
	}
}

func (a *App) initStorage() error {
	if a.store != nil {
		return nil
	}
	store, err := storage.OpenDefault()
	if err != nil {
		return err
	}
	a.store = store

	result, err := store.ImportServersFromCandidatesOnce(config.FindServersConfigCandidates())
	if err != nil {
		a.mu.Lock()
		a.appendLogLocked(err.Error())
		a.mu.Unlock()
		return nil
	}
	if result.Imported {
		a.mu.Lock()
		a.appendLogLocked(fmt.Sprintf("Imported %d server(s) from %s", result.Count, result.Path))
		a.mu.Unlock()
	}
	return nil
}

func (a *App) refreshSubscription(ctx context.Context, subscriptionID int64) (storage.SubscriptionRefreshResult, error) {
	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()

	subscription, err := a.store.GetSubscription(subscriptionID)
	if err != nil {
		return storage.SubscriptionRefreshResult{}, err
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	options := subfetch.FetchOptions{
		URL:        subscription.URL,
		DeviceID:   a.deviceInfo.ID,
		DeviceName: a.deviceInfo.Name,
	}
	if subscription.ETag != nil {
		options.ETag = *subscription.ETag
	}
	if subscription.LastModified != nil {
		options.LastModified = *subscription.LastModified
	}

	result, err := subfetch.Fetch(refreshCtx, a.httpClient, options)
	if err != nil {
		updated, recordErr := a.store.RecordSubscriptionError(subscriptionID, err.Error())
		if recordErr != nil {
			return storage.SubscriptionRefreshResult{}, recordErr
		}
		count, _ := a.store.CountSubscriptionServers(subscriptionID)
		return storage.SubscriptionRefreshResult{Subscription: updated, ServerCount: count}, err
	}

	var refreshResult storage.SubscriptionRefreshResult
	if result.NotModified {
		refreshResult, err = a.store.MarkSubscriptionNotModified(subscriptionID, result.Metadata)
	} else {
		refreshResult, err = a.store.ReplaceSubscriptionServers(subscriptionID, result.Servers, result.Metadata)
	}
	if err != nil {
		_, _ = a.store.RecordSubscriptionError(subscriptionID, err.Error())
		return storage.SubscriptionRefreshResult{}, err
	}

	a.mu.Lock()
	if result.NotModified {
		a.appendLogLocked(fmt.Sprintf("Subscription %s is unchanged", subscription.Name))
	} else {
		a.appendLogLocked(fmt.Sprintf("Subscription %s refreshed: %d server(s)", subscription.Name, refreshResult.ServerCount))
	}
	a.mu.Unlock()
	a.ReloadServers()
	return refreshResult, nil
}

func shouldImportInline(source string) bool {
	lower := strings.ToLower(source)
	if strings.HasPrefix(lower, "vless://") ||
		strings.HasPrefix(lower, "ss://") ||
		strings.HasPrefix(lower, "hysteria2://") ||
		strings.HasPrefix(source, "{") ||
		strings.ContainsAny(source, "\r\n") {
		return true
	}
	_, err := config.LoadServersFromBytes([]byte(source))
	return err == nil
}

func (a *App) startTrafficMonitorLocked(connectionID uint64, mode string) {
	a.stopTrafficMonitorLocked()
	monitorCtx, cancel := context.WithCancel(context.Background())
	a.trafficCancel = cancel

	go a.runTrafficMonitor(monitorCtx, connectionID, mode)
}

func (a *App) stopTrafficMonitor() {
	a.mu.Lock()
	a.stopTrafficMonitorLocked()
	a.mu.Unlock()
}

func (a *App) stopTrafficMonitorLocked() {
	if a.trafficCancel != nil {
		a.trafficCancel()
		a.trafficCancel = nil
	}
}

func (a *App) runTrafficMonitor(ctx context.Context, connectionID uint64, mode string) {
	statsMode := netstats.ModeProxy
	if mode == modeTun {
		statsMode = netstats.ModeTun
	}

	base, _, err := netstats.Read(statsMode, tunsettings.InterfaceName)
	if err != nil {
		a.mu.Lock()
		a.appendLogLocked(fmt.Sprintf("Traffic counters unavailable: %v", err))
		a.mu.Unlock()
		return
	}
	prev := base
	prevTime := time.Now()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			current, _, err := netstats.Read(statsMode, tunsettings.InterfaceName)
			if err != nil {
				a.mu.Lock()
				a.appendLogLocked(fmt.Sprintf("Traffic counters unavailable: %v", err))
				a.mu.Unlock()
				return
			}

			elapsed := now.Sub(prevTime).Seconds()
			if elapsed <= 0 {
				elapsed = 1
			}
			downloadDelta := deltaCounter(current.DownloadBytes, prev.DownloadBytes)
			uploadDelta := deltaCounter(current.UploadBytes, prev.UploadBytes)
			sessionDownload := deltaCounter(current.DownloadBytes, base.DownloadBytes)
			sessionUpload := deltaCounter(current.UploadBytes, base.UploadBytes)

			a.mu.Lock()
			if connectionID != a.connectionID || !a.running {
				a.mu.Unlock()
				return
			}
			a.traffic = TrafficState{
				SessionDownloadBytes: sessionDownload,
				SessionUploadBytes:   sessionUpload,
				DownloadBytesPerSec:  uint64(float64(downloadDelta) / elapsed),
				UploadBytesPerSec:    uint64(float64(uploadDelta) / elapsed),
			}
			state := a.stateLocked()
			a.mu.Unlock()
			a.emitState(state)

			prev = current
			prevTime = now
		}
	}
}

func deltaCounter(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}
	return current - previous
}

func (a *App) startAutoRefresh(ctx context.Context) {
	if a.autoCancel != nil {
		return
	}
	autoCtx, cancel := context.WithCancel(ctx)
	a.autoCancel = cancel
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		a.refreshDueSubscriptions(autoCtx)
		for {
			select {
			case <-autoCtx.Done():
				return
			case <-ticker.C:
				a.refreshDueSubscriptions(autoCtx)
			}
		}
	}()
}

func (a *App) refreshDueSubscriptions(ctx context.Context) {
	if err := a.ensureStorage(); err != nil {
		return
	}
	subscriptions, err := a.store.ListSubscriptions()
	if err != nil {
		a.mu.Lock()
		a.appendLogLocked(fmt.Sprintf("Failed to list subscriptions for auto refresh: %v", err))
		a.mu.Unlock()
		return
	}
	now := time.Now().UTC()
	for _, subscription := range subscriptions {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !isSubscriptionDue(subscription, now) {
			continue
		}
		if _, err := a.refreshSubscription(ctx, subscription.ID); err != nil {
			a.mu.Lock()
			a.appendLogLocked(fmt.Sprintf("Auto refresh failed for %s: %v", subscription.Name, err))
			a.mu.Unlock()
		}
	}
}

func isSubscriptionDue(subscription storage.Subscription, now time.Time) bool {
	if !subscription.Enabled || subscription.AutoUpdateIntervalMinutes <= 0 {
		return false
	}
	last := subscription.LastCheckedAt
	if last == nil {
		last = subscription.LastUpdatedAt
	}
	if last == nil || *last == "" {
		return true
	}
	checkedAt, err := time.Parse(time.RFC3339Nano, *last)
	if err != nil {
		return true
	}
	return now.Sub(checkedAt) >= time.Duration(subscription.AutoUpdateIntervalMinutes)*time.Minute
}

func (a *App) ensureStorage() error {
	if a.store != nil {
		return nil
	}
	return a.initStorage()
}

func serverSummary(record storage.ServerRecord) ServerSummary {
	return ServerSummary{
		ID:             record.ID,
		Name:           record.Name,
		Type:           record.Type,
		Address:        record.Address,
		SubscriptionID: record.SubscriptionID,
	}
}

func serverExists(servers []storage.ServerRecord, name string) bool {
	for _, server := range servers {
		if server.Name == name {
			return true
		}
	}
	return false
}

// PingResult holds the result of a connectivity test.
type PingResult struct {
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error,omitempty"`
}

func (a *App) PingServer(id int64) PingResult {
	a.mu.Lock()
	var address string
	for _, s := range a.servers {
		if s.ID == id {
			address = s.Address
			break
		}
	}
	a.mu.Unlock()

	if address == "" {
		return PingResult{LatencyMs: -1, Error: "server not found"}
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return PingResult{LatencyMs: -1, Error: err.Error()}
	}
	conn.Close()
	return PingResult{LatencyMs: time.Since(start).Milliseconds()}
}

// SpeedTestServer starts a temporary sing-box proxy on a free port, makes an
// HTTP request through it to a public endpoint, and returns the round-trip latency.
// The temporary instance is fully isolated: no system proxy is changed.
func (a *App) SpeedTestServer(id int64) PingResult {
	a.mu.Lock()
	var serverEntry config.ServerEntry
	found := false
	for _, s := range a.servers {
		if s.ID == id {
			serverEntry = s.Server
			found = true
			break
		}
	}
	a.mu.Unlock()

	if !found {
		return PingResult{LatencyMs: -1, Error: "server not found"}
	}

	httpPort, err := getFreePort()
	if err != nil {
		return PingResult{LatencyMs: -1, Error: "no free port: " + err.Error()}
	}

	opts, err := config.BuildOptionsForSpeedTest(serverEntry, httpPort)
	if err != nil {
		return PingResult{LatencyMs: -1, Error: "build config: " + err.Error()}
	}

	c := core.NewCore(core.Options{})
	if err := c.StartWithOptions(opts); err != nil {
		_ = c.Close()
		return PingResult{LatencyMs: -1, Error: "start proxy: " + err.Error()}
	}
	defer func() { _ = c.Close() }()

	proxyAddr := fmt.Sprintf("127.0.0.1:%d", httpPort)
	if !waitForPort(proxyAddr, 5*time.Second) {
		return PingResult{LatencyMs: -1, Error: "proxy did not start in time"}
	}

	proxyURL, _ := url.Parse("http://" + proxyAddr)
	testClient := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   15 * time.Second,
	}

	start := time.Now()
	resp, err := testClient.Get("http://cp.cloudflare.com/")
	if err != nil {
		return PingResult{LatencyMs: -1, Error: err.Error()}
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	return PingResult{LatencyMs: time.Since(start).Milliseconds()}
}

func getFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

func waitForPort(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}

func checkWintunDLL() error {
	paths := []string{"wintun.dll"}
	if exePath, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exePath), "wintun.dll"))
	}

	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		cleanPath := filepath.Clean(path)
		if seen[cleanPath] {
			continue
		}
		seen[cleanPath] = true
		if _, err := os.Stat(cleanPath); err == nil {
			return nil
		}
	}

	return fmt.Errorf("TUN requires wintun.dll; put it next to mgb-gui.exe or rebuild with scripts\\build-gui.bat")
}

func quoteArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}

	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" || strings.ContainsAny(arg, " \t\"") {
			quoted = append(quoted, `"`+strings.ReplaceAll(arg, `"`, `\"`)+`"`)
		} else {
			quoted = append(quoted, arg)
		}
	}
	return strings.Join(quoted, " ")
}
