package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"SingCli/internal/config"
	"SingCli/internal/logger"
	"SingCli/internal/tunnel"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const (
	socksAddr = "127.0.0.1:1080"
	httpAddr  = "127.0.0.1:1081"
	swShow    = 5

	modeProxy = "proxy"
	modeTun   = "tun"

	eventState = "state"
	eventLog   = "log"
)

//go:embed all:frontend/dist
var assets embed.FS

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procIsUserAnAdmin = shell32.NewProc("IsUserAnAdmin")
	procShellExecute  = shell32.NewProc("ShellExecuteW")
)

type ServerSummary struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
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
}

type App struct {
	ctx context.Context

	mu             sync.Mutex
	servers        []config.ServerEntry
	selectedServer string
	mode           string
	status         string
	logs           []string
	manager        *tunnel.Manager
	running        bool
	connecting     bool
	connectionID   uint64
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
		Title:            "MGB VPN",
		Width:            760,
		Height:           560,
		MinWidth:         680,
		MinHeight:        500,
		DisableResize:    false,
		Fullscreen:       false,
		Frameless:        false,
		Bind:             []interface{}{app},
		OnStartup:        app.Startup,
		OnShutdown:       app.Shutdown,
		BackgroundColour: &options.RGBA{R: 12, G: 15, B: 23, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		CSSDragProperty:  "--wails-draggable",
		CSSDragValue:     "drag",
	}); err != nil {
		log.Fatal(err)
	}
}

func NewApp() *App {
	return &App{
		mode:   modeProxy,
		status: "Loading servers...",
		logs:   make([]string, 0, 64),
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.ReloadServers()
}

func (a *App) Shutdown(ctx context.Context) {
	a.Disconnect()
}

func (a *App) GetState() AppState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stateLocked()
}

func (a *App) ReloadServers() AppState {
	configPath, err := findServersConfig()
	if err != nil {
		a.mu.Lock()
		a.servers = nil
		a.selectedServer = ""
		a.status = "Cannot load servers.json"
		a.appendLogLocked(err.Error())
		state := a.stateLocked()
		a.mu.Unlock()
		a.emitState(state)
		return state
	}

	servers, err := config.LoadServers(configPath)
	if err != nil {
		a.mu.Lock()
		a.servers = nil
		a.selectedServer = ""
		a.status = "Cannot load servers.json"
		a.appendLogLocked(fmt.Sprintf("Failed to load %s: %v", configPath, err))
		state := a.stateLocked()
		a.mu.Unlock()
		a.emitState(state)
		return state
	}

	a.mu.Lock()
	a.servers = servers
	if len(servers) == 0 {
		a.selectedServer = ""
		a.status = "No servers in servers.json"
	} else {
		if a.selectedServer == "" || !serverExists(servers, a.selectedServer) {
			a.selectedServer = servers[0].Name
		}
		a.status = "Ready"
	}
	a.appendLogLocked(fmt.Sprintf("Loaded %d server(s) from %s", len(servers), configPath))
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
	a.connectionID++
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
	a.status = "Connected"
	a.appendLogLocked(fmt.Sprintf("Connected to %s", server.Name))
	state := a.stateLocked()
	a.mu.Unlock()
	a.emitState(state)
}

func (a *App) serverByNameLocked(name string) (config.ServerEntry, bool) {
	if name == "" && len(a.servers) > 0 {
		return a.servers[0], true
	}
	for _, server := range a.servers {
		if server.Name == name {
			return server, true
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
	for _, server := range a.servers {
		servers = append(servers, ServerSummary{
			Name:    server.Name,
			Type:    server.Type,
			Address: server.Server,
		})
	}

	logs := append([]string(nil), a.logs...)
	canConnect := len(a.servers) > 0 && !a.running && !a.connecting
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
	}
}

func (a *App) emitState(state AppState) {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, eventState, state)
	}
}

func serverExists(servers []config.ServerEntry, name string) bool {
	for _, server := range servers {
		if server.Name == name {
			return true
		}
	}
	return false
}

func findServersConfig() (string, error) {
	paths := []string{"servers.json"}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		paths = append(paths,
			filepath.Join(exeDir, "servers.json"),
			filepath.Join(filepath.Dir(exeDir), "servers.json"),
		)
	}
	if defaultPath, err := config.DefaultServersPath(); err == nil {
		paths = append(paths, defaultPath)
	}

	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		cleanPath := filepath.Clean(path)
		if seen[cleanPath] {
			continue
		}
		seen[cleanPath] = true

		if _, err := os.Stat(cleanPath); err == nil {
			return cleanPath, nil
		}
	}

	return "", fmt.Errorf("servers.json not found; checked: %s", strings.Join(paths, ", "))
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

func isRunningAsAdmin() bool {
	ret, _, _ := procIsUserAnAdmin.Call()
	return ret != 0
}

func restartAsAdmin() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		workDir = filepath.Dir(exePath)
	}

	ret, _, callErr := procShellExecute.Call(
		0,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("runas"))),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(exePath))),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(quoteArgs(os.Args[1:])))),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(workDir))),
		swShow,
	)
	if ret <= 32 {
		return fmt.Errorf("request administrator rights: %w", callErr)
	}
	return nil
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
