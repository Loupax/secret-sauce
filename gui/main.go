package main

import (
	"context"
	"embed"

	"github.com/getlantern/systray"
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

func main() {
	app := NewApp()

	// Register systray without creating a separate GTK main loop
	systray.Register(func() {
		systray.SetIcon(icon)
		systray.SetTitle("Secret Sauce")
		systray.SetTooltip("Secret Sauce")

		mShow := systray.AddMenuItem("Show Vault", "Show the Secret Sauce UI")
		mQuit := systray.AddMenuItem("Quit", "Quit the application")

		go func() {
			for {
				select {
				case <-mShow.ClickedCh:
					if app.ctx != nil {
						runtime.WindowShow(app.ctx)
					}
				case <-mQuit.ClickedCh:
					if app.ctx != nil {
						runtime.Quit(app.ctx)
					}
					systray.Quit()
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
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup: func(ctx context.Context) {
			// Resets signal handlers so Go can handle panics/signals correctly alongside WebKitGTK
			runtime.ResetSignalHandlers()
			app.startup(ctx)
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
