package main

import "context"

// App holds the application state and provides Wails bindings.
type App struct {
	ctx context.Context
}

// NewApp creates a new App instance.
func NewApp() *App { return &App{} }

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called when the app is closing.
func (a *App) shutdown(_ context.Context) {}

// Version returns the application version.
func (a *App) Version() string {
	return version
}
