package main

import (
	"context"
	"embed"

	"fyne.io/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func showAndFocus(ctx context.Context) {
	runtime.WindowShow(ctx)
	runtime.WindowSetAlwaysOnTop(ctx, true)
	runtime.WindowSetAlwaysOnTop(ctx, false)
}

func main() {
	app := NewApp()
	startupDone := make(chan struct{})

	// RunWithExternalLoop registers callbacks and returns nativeStart so we can
	// drive the systray from OnStartup instead of blocking main.
	startSystray, _ := systray.RunWithExternalLoop(func() {
		systray.SetIcon(icon)
		systray.SetTitle("Secret Sauce")
		systray.SetTooltip("Secret Sauce")

		// Left-click on the tray icon shows the window (macOS/Windows).
		// On Linux with StatusNotifier any click opens the menu, so mShow acts as fallback.
		systray.SetOnTapped(func() {
			showAndFocus(app.ctx)
		})

		mShow := systray.AddMenuItem("Show Vault", "Show the Secret Sauce UI")
		mQuit := systray.AddMenuItem("Quit", "Quit the application")

		go func() {
			<-startupDone
			for {
				select {
				case <-mShow.ClickedCh:
					showAndFocus(app.ctx)
				case <-mQuit.ClickedCh:
					systray.Quit()
					runtime.Quit(app.ctx)
					return
				}
			}
		}()
	}, func() {})

	err := wails.Run(&options.App{
		Title:  "Secret Sauce",
		Width:  900,
		Height: 650,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:  &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		StartHidden:       true,
		HideWindowOnClose: true,
		OnStartup: func(ctx context.Context) {
			// Resets signal handlers so Go can handle panics/signals correctly alongside WebKitGTK
			runtime.ResetSignalHandlers()
			app.startup(ctx)
			close(startupDone)
			go startSystray()
		},
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			Icon: icon,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "Secret Sauce",
				Message: "A local encrypted secret vault.",
				Icon:    icon,
			},
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
