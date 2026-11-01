package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"wassap-desktop/internal/appdirs"
)

//go:embed all:frontend/dist
var assets embed.FS

// shellUserDataPath returns a unique user data folder for the Wails shell.
// WebView2 refuses to create a second controller for a user data folder that is
// already in use, so a leftover engine process from a previous run would break
// start-up. The folder name carries the process id, which keeps concurrent
// runs apart and leaves no stale lock behind.
func shellUserDataPath(dirs appdirs.Dirs) string {
	return filepath.Join(dirs.Data, "wails-shell", fmt.Sprintf("run-%d", os.Getpid()))
}

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

	title := "Wassap Desktop"
	width := 1020
	height := 720
	if legacyMode {
		title = "Wassap Desktop (Pecut 1 Legacy)"
		width = 1280
		height = 800
	}

	err = wails.Run(&options.App{
		Title:            title,
		Width:            width,
		Height:           height,
		MinWidth:         480,
		MinHeight:        400,
		BackgroundColour: &options.RGBA{R: 11, G: 20, B: 26, A: 255},
		AssetServer:      assetOpts,
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewUserDataPath: shellUserDataPath(dirs),
		},
	})

	if err != nil {
		log.Fatal("Error:", err.Error())
	}
}
