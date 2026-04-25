//go:build windows

package main

import (
	"context"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const (
	wmDestroy      = 0x0002
	wmCommand      = 0x0111
	wmApp          = 0x8000
	wmTray         = wmApp + 1
	wmTrayQuit     = wmApp + 2
	wmLButtonUp    = 0x0202
	wmRButtonUp    = 0x0205
	nimAdd         = 0x00000000
	nimDelete      = 0x00000002
	nifMessage     = 0x00000001
	nifIcon        = 0x00000002
	nifTip         = 0x00000004
	imageIcon      = 1
	lrShared       = 0x00008000
	tpmRightButton = 0x0002
	tpmBottomAlign = 0x0020
	tpmLeftAlign   = 0x0000
	mfString       = 0x0000
	mfSeparator    = 0x0800
	wsOverlapped   = 0x00000000
	cwUseDefault   = 0x80000000

	trayID         = 1
	trayShowID     = 1001
	trayDisconnect = 1002
	trayExitID     = 1003
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	trayShell32          = windows.NewLazySystemDLL("shell32.dll")
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostMessageW     = user32.NewProc("PostMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenuW      = user32.NewProc("AppendMenuW")
	procTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	procDestroyMenu      = user32.NewProc("DestroyMenu")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procSetForegroundWnd = user32.NewProc("SetForegroundWindow")
	procLoadImageW       = user32.NewProc("LoadImageW")
	procShellNotifyIconW = trayShell32.NewProc("Shell_NotifyIconW")
)

type trayService struct {
	app  *App
	mu   sync.Mutex
	hwnd uintptr
	done chan struct{}
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type point struct {
	X int32
	Y int32
}

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type notifyIconData struct {
	Size            uint32
	HWnd            uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GuidItem        windows.GUID
	BalloonIcon     uintptr
}

var activeTray *trayService

func newTrayService(app *App) *trayService {
	return &trayService{app: app, done: make(chan struct{})}
}

func hideWindowOnClose() bool {
	return true
}

func (t *trayService) Start(ctx context.Context) {
	activeTray = t
	go t.run()
}

func (t *trayService) Stop() {
	t.mu.Lock()
	hwnd := t.hwnd
	t.mu.Unlock()
	if hwnd != 0 {
		procPostMessageW.Call(hwnd, wmTrayQuit, 0, 0)
	}
	select {
	case <-t.done:
	default:
	}
}

func (t *trayService) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(t.done)

	className := windows.StringToUTF16Ptr("MGBVPNTrayWindow")
	wndProc := syscall.NewCallback(trayWndProc)
	var instance windows.Handle
	_ = windows.GetModuleHandleEx(0, nil, &instance)
	wc := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:   wndProc,
		Instance:  instance,
		ClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("MGB VPN Tray"))),
		wsOverlapped,
		uintptr(cwUseDefault),
		uintptr(cwUseDefault),
		uintptr(cwUseDefault),
		uintptr(cwUseDefault),
		0,
		0,
		uintptr(instance),
		0,
	)
	if hwnd == 0 {
		return
	}

	t.mu.Lock()
	t.hwnd = hwnd
	t.mu.Unlock()
	t.addIcon(hwnd)

	var message msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(ret) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func (t *trayService) addIcon(hwnd uintptr) {
	nid := notifyIconData{
		Size:            uint32(unsafe.Sizeof(notifyIconData{})),
		HWnd:            hwnd,
		ID:              trayID,
		Flags:           nifMessage | nifIcon | nifTip,
		CallbackMessage: wmTray,
		Icon:            loadDefaultIcon(),
	}
	copy(nid.Tip[:], windows.StringToUTF16("MGB VPN"))
	procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
}

func (t *trayService) deleteIcon(hwnd uintptr) {
	nid := notifyIconData{
		Size: uint32(unsafe.Sizeof(notifyIconData{})),
		HWnd: hwnd,
		ID:   trayID,
	}
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
}

func (t *trayService) showMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	appendMenu(menu, mfString, trayShowID, "Show")
	appendMenu(menu, mfString, trayDisconnect, "Disconnect")
	appendMenu(menu, mfSeparator, 0, "")
	appendMenu(menu, mfString, trayExitID, "Exit")

	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	procSetForegroundWnd.Call(hwnd)
	procTrackPopupMenu.Call(menu, tpmLeftAlign|tpmBottomAlign|tpmRightButton, uintptr(p.X), uintptr(p.Y), 0, hwnd, 0)
}

func (t *trayService) showWindow() {
	if t.app.ctx != nil {
		wailsruntime.WindowShow(t.app.ctx)
	}
}

func (t *trayService) disconnect() {
	t.app.Disconnect()
}

func (t *trayService) exit() {
	t.app.mu.Lock()
	t.app.quitting = true
	t.app.mu.Unlock()
	if t.app.ctx != nil {
		wailsruntime.Quit(t.app.ctx)
	}
}

func trayWndProc(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
	t := activeTray
	switch message {
	case wmTray:
		if t == nil {
			break
		}
		switch lParam {
		case wmLButtonUp:
			t.showWindow()
			return 0
		case wmRButtonUp:
			t.showMenu(hwnd)
			return 0
		}
	case wmTrayQuit:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmCommand:
		if t == nil {
			break
		}
		switch uint32(wParam & 0xffff) {
		case trayShowID:
			t.showWindow()
		case trayDisconnect:
			t.disconnect()
		case trayExitID:
			t.exit()
		}
		return 0
	case wmDestroy:
		if t != nil {
			t.deleteIcon(hwnd)
		}
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return ret
}

func appendMenu(menu uintptr, flags uint32, id uint32, label string) {
	var labelPtr uintptr
	if label != "" {
		labelPtr = uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(label)))
	}
	procAppendMenuW.Call(menu, uintptr(flags), uintptr(id), labelPtr)
}

func loadDefaultIcon() uintptr {
	icon, _, _ := procLoadImageW.Call(0, uintptr(32512), imageIcon, 0, 0, lrShared)
	return icon
}
