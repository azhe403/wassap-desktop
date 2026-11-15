package main

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"wassap-desktop/internal/appdirs"
	"wassap-desktop/internal/config"
)

//go:embed all:frontend/dist
var assets embed.FS

// shellUserDataPath returns a fresh user data folder for the Wails shell.
// WebView2 refuses to create a controller for a user data folder that is
// already in use, so a leftover engine process from a crashed run would break
// start-up. The folder name carries the process id so stale locks never block
// a restart. With the single-instance lock, concurrent runs are impossible;
// old run-* folders are harmless leftovers.
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

	// Mirror logs to a file as well as stderr so close-path diagnostics are
	// available even when the app is launched from a double-click. Append;
	// the file is regenerable cache content, mirroring what Ensure skips.
	if err := os.MkdirAll(dirs.Cache, 0o700); err != nil {
		log.Printf("log: cannot create cache dir %s: %v", dirs.Cache, err)
	}
	logPath := filepath.Join(dirs.Cache, "wassap-desktop.log")
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err != nil {
		log.Printf("log: cannot open %s, stderr only: %v", logPath, err)
	} else {
		log.SetOutput(io.MultiWriter(os.Stderr, f))
		log.Printf("log: started, writing to %s", logPath)
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
	// Restore the saved window geometry: size (in Wails' logical 96-DPI
	// pixels) and the maximised state go through the window options before
	// creation; the position is applied from the geometry polling loop
	// (windowgeom_*.go) once the window is observable. App builds its own
	// config store; reading here must happen before wails.Run, and reads
	// are cheap.
	startState := options.Normal
	cfgStore := config.NewStore(filepath.Join(dirs.Config, "config.json"))
	if winState, wsErr := cfgStore.GetWindowState(); wsErr != nil {
		log.Printf("config: cannot load window state, using defaults: %v", wsErr)
	} else if winState != nil {
		if winState.Width >= 480 && winState.Height >= 400 {
			width, height = winState.Width, winState.Height
		}
		if winState.Maximised {
			startState = options.Maximised
		}
	}

	err = wails.Run(&options.App{
		Title:            title,
		Width:            width,
		Height:           height,
		MinWidth:         480,
		MinHeight:        400,
		WindowStartState: startState,
		BackgroundColour: &options.RGBA{R: 11, G: 20, B: 26, A: 255},
		AssetServer:      assetOpts,
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnBeforeClose:    app.onBeforeClose,
		OnShutdown:       app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "3f7f3db3-008c-4e3d-a134-0d66e741a2f1",
			OnSecondInstanceLaunch: app.onSecondInstanceLaunch,
		},
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
