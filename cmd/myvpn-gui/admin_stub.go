//go:build !windows

package main

func isRunningAsAdmin() bool {
	return true
}

func restartAsAdmin() error {
	return nil
}
