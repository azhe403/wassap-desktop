package app

import (
	"context"
	"net/http"

	"github.com/wailsapp/wails/v2/pkg/options"
)

// Hooks bundles the wails lifecycle callbacks so the composition root
// (package main) can wire wails.Run without exposing them as JS-bindable
// methods on App. Lifecycle entries such as Startup or Shutdown must not be
// callable from the frontend, so they are handed out as func values from a
// package-level function rather than as exported methods of the bound struct.
type Hooks struct {
	Startup        func(ctx context.Context)
	DomReady       func(ctx context.Context)
	BeforeClose    func(ctx context.Context) (prevent bool)
	Shutdown       func(ctx context.Context)
	SecondInstance func(data options.SecondInstanceData)
	Handler        http.Handler
}

// WailsHooks returns the wails lifecycle callbacks for a. Handler is
// resolved once here; it must be wired before wails.Run starts.
func WailsHooks(a *App) Hooks {
	return Hooks{
		Startup:        a.startup,
		DomReady:       a.domReady,
		BeforeClose:    a.onBeforeClose,
		Shutdown:       a.shutdown,
		SecondInstance: a.onSecondInstanceLaunch,
		Handler:        a.handler(),
	}
}
