package main

import (
	"embed"

	"github.com/nlink-jp/slack-personal-agent/internal/config"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

	// Application menu
	appMenu := menu.NewMenu()

	// App menu (macOS standard)
	appMenu.Append(menu.AppMenu())

	// File menu
	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Start All Polling", keys.CmdOrCtrl("r"), func(_ *menu.CallbackData) {
		for _, ws := range app.cfg.Workspaces {
			app.StartPolling(ws.Name)
		}
	})
	fileMenu.AddText("Stop All Polling", keys.Combo("r", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *menu.CallbackData) {
		for _, ws := range app.cfg.Workspaces {
			app.StopPolling(ws.Name)
		}
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		wailsRuntime.Quit(app.ctx)
	})

	// Edit menu (standard copy/paste)
	appMenu.Append(menu.EditMenu())

	// View menu
	viewMenu := appMenu.AddSubmenu("View")
	viewMenu.AddText("Reload", keys.CmdOrCtrl("l"), func(_ *menu.CallbackData) {
		wailsRuntime.WindowReloadApp(app.ctx)
	})

	// Window menu
	appMenu.Append(menu.WindowMenu())

	err = wails.Run(&options.App{
		Title:             "slack-personal-agent",
		Width:             width,
		Height:            height,
		HideWindowOnClose: false,
		Menu:              appMenu,
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
