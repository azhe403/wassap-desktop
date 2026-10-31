package main

import (
	"context"
	"net/http"

	"wassap-desktop/internal/appdirs"
)

// whatsappURL is the only location this app is meant to show.
const whatsappURL = "https://web.whatsapp.com"

// App holds the runtime state for the desktop shell.
type App struct {
	ctx  context.Context
	dirs appdirs.Dirs
}

// NewApp creates a new App application struct.
func NewApp(dirs appdirs.Dirs) *App {
	return &App{dirs: dirs}
}

// startup is called when the app starts. The context is saved so we can call
// the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// whatsappWebHandler answers the placeholder origin Wails navigates to at
// startup. No assets are registered, so every request reaches this handler.
//
// A plain 302 is used: the WebView follows it as a normal top-level navigation,
// so WhatsApp Web loads once and is not re-navigated afterwards. Do not add any
// script-based navigation that runs on every page load; that causes a reload
// loop.
func whatsappWebHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, whatsappURL, http.StatusFound)
	})
}
