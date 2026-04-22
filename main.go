package main

import (
	"embed"

	"github.com/nlink-jp/slack-personal-agent/internal/config"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

var version = "dev"

func main() {
	app := NewApp()

	// Load config for window dimensions
	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		cfg = config.DefaultConfig()
	}

	width := cfg.Window.Width
	height := cfg.Window.Height
	if width < 400 {
		width = 1280
	}
	if height < 300 {
		height = 800
	}

	err = wails.Run(&options.App{
		Title:            "slack-personal-agent",
		Width:            width,
		Height:           height,
		HideWindowOnClose: false, // Window close = app quit (no hidden background process)
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 13, G: 17, B: 23, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
