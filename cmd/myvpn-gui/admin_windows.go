//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const swShow = 5

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procIsUserAnAdmin = shell32.NewProc("IsUserAnAdmin")
	procShellExecute  = shell32.NewProc("ShellExecuteW")
)

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
