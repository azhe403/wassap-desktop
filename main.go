package main

import (
	"embed"
	"flag"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"wassap-desktop/internal/appdirs"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	legacyFlag := flag.Bool("pecut1", false, "run in Pecut 1 legacy mode (single direct WhatsApp session)")
	flag.Parse()

	// Check environment variable or argument for legacy mode
	legacyMode := *legacyFlag
	for _, arg := range os.Args[1:] {
		if arg == "--pecut1" || arg == "-pecut1" || arg == "--legacy" {
			legacyMode = true
		}
	}

	// Resolve the application directories before wails.Run. WebView2 aborts
	// startup when its user data directory does not exist.
	dirs, err := appdirs.Resolve()
	if err != nil {
		log.Fatal(err)
	}
	if err := appdirs.Ensure(dirs); err != nil {
		log.Fatalf("cannot create application directories: %v", err)
	}

	app := NewApp(dirs, legacyMode)

	// Configure asset server:
	// In legacy mode, handler redirects 302 to web.whatsapp.com directly.
	// In Pecut 2 mode, assets are served so the test interface is displayed.
	assetOpts := &assetserver.Options{
		Assets:  assets,
		Handler: app.handler(),
	}

	title := "Wassap Desktop — Pecut 2 Profile Isolation Test"
	width := 900
	height := 650
	if legacyMode {
		title = "Wassap Desktop (Pecut 1 Legacy)"
		width = 1280
		height = 800
	}

	err = wails.Run(&options.App{
		Title:     title,
		Width:     width,
		Height:    height,
		MinWidth:  480,
		MinHeight: 400,
		AssetServer: assetOpts,
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewUserDataPath: dirs.Data,
		},
	})

	if err != nil {
		log.Fatal("Error:", err.Error())
	}
}
