package main

import (
	"embed"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	app := NewApp()

	// Run the system tray in a separate goroutine
	go systray.Run(func() {
		systray.SetIcon(icon)
		systray.SetTitle("Secret Sauce")
		systray.SetTooltip("Secret Sauce")

		mShow := systray.AddMenuItem("Show Vault", "Show the Secret Sauce UI")
		mQuit := systray.AddMenuItem("Quit", "Quit the whole app")

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
	}, func() {})

	err := wails.Run(&options.App{
		Title:  "Secret Sauce",
		Width:  900,
		Height: 650,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			Icon: icon,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
