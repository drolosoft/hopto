package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// main opens the log, builds the App and hands both to Wails, which
// owns the main thread from here on.
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
		Bind:              []any{app},

		// Wails ends with a WM_QUIT that never reaches the tray window, so
		// the Windows tray icon is taken away here or it would stay in the
		// notification area as a ghost.
		OnShutdown: func(ctx context.Context) { shutdownNative() },

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

		// Windows: the same transparent window the page paints its
		// rounded panel on, without a backdrop effect (it would be a
		// rectangle behind the panel), without the window icon (there is
		// no title bar) and with a class name of our own, so the native
		// side of hotkey_windows.go can find this window by name.
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.None,
			DisableWindowIcon:    true,
			WindowClassName:      windowClassName,
			Theme:                windows.SystemDefault,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
