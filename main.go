package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"wassap-desktop/internal/appdirs"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Resolve the application directories before wails.Run. WebView2 aborts
	// startup when its user data directory does not exist, and Wails surfaces
	// that as a message box followed by exit, so the directory must already be
	// on disk by the time the window is created.
	dirs, err := appdirs.Resolve()
	if err != nil {
		log.Fatal(err)
	}
	if err := appdirs.Ensure(dirs); err != nil {
		log.Fatalf("cannot create application directories: %v", err)
	}

	// WhatsApp Web must own the window, so the embedded frontend is never
	// served. Wails always navigates to http://wails.localhost/ on startup; the
	// handler below answers every request with a 302 to WhatsApp Web, so the
	// window ends up on WhatsApp Web without any JavaScript navigation.
	app := NewApp(dirs)

	err = wails.Run(&options.App{
		Title:     "Wassap Desktop",
		Width:     1280,
		Height:    800,
		MinWidth:  480,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Handler: whatsappWebHandler(),
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			// The WhatsApp session (cookies, IndexedDB, localStorage) lives
			// here. This is the only thing that makes the login survive a
			// restart: without it WebView2 defaults to %APPDATA%\<exe name>.
			WebviewUserDataPath: dirs.Data,
		},
	})

	if err != nil {
		log.Fatal("Error:", err.Error())
	}
}
