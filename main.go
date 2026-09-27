package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	openLog()
	app := NewApp()

	// An overlay, not a document window: no frame, always on top, hidden
	// until the shortcut, and closing only hides it. The Dock icon goes away
	// with the accessory activation policy set from hotkey_darwin.go (the
	// plist's LSUIElement alone is overridden by Wails). The window itself is
	// fully transparent: the page paints the rounded panel, so the corners
	// are round. WindowIsTranslucent is off on purpose, its vibrancy view is
	// rectangular and showed as square corners. No fixed appearance either:
	// the page follows the system's light or dark mode.
	assetOptions := &assetserver.Options{Assets: assets, Handler: app.assets()}

	err := wails.Run(&options.App{
		Title:             "hopto",
		Width:             760,
		Height:            520,
		DisableResize:     true,
		Frameless:         true,
		AlwaysOnTop:       true,
		StartHidden:       true,
		HideWindowOnClose: true,
		AssetServer:       assetOptions,
		BackgroundColour:  &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup:         app.startup,
		Bind:              []interface{}{app},

		// A second launch (`open -n`, or a launcher that starts a new
		// copy) quits at once and shows this copy's panel instead.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "com.drolosoft.hopto",
			OnSecondInstanceLaunch: app.secondInstance,
		},
		Mac: &mac.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
