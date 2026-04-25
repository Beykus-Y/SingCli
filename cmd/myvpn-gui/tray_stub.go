//go:build !windows

package main

import "context"

type trayService struct{}

func newTrayService(app *App) *trayService {
	return &trayService{}
}

func hideWindowOnClose() bool {
	return false
}

func (t *trayService) Start(ctx context.Context) {}

func (t *trayService) Stop() {}
