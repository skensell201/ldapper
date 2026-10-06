// Command Ldapper is a desktop explorer for LDAP and Active Directory
// directories.
package main

import (
	"embed"
	"log"
	goruntime "runtime"

	"github.com/skensell201/ldapper/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("Ldapper cannot start: %v", err)
	}

	err = wails.Run(&options.App{
		Title:       "Ldapper",
		Width:       1280,
		Height:      800,
		MinWidth:    960,
		MinHeight:   600,
		AssetServer: &assetserver.Options{Assets: assets},
		// Windows draws a title bar that has nothing to do with the rest of
		// the window: a light strip above a dark application. Going frameless
		// there lets the interface draw its own, with the window controls at
		// the end of the same bar that carries the connection.
		//
		// macOS is left alone. Its window controls belong to the system, and
		// TitleBarHiddenInset already puts them inside our chrome where they
		// look native.
		Frameless: goruntime.GOOS == "windows",
		// The window chrome is part of the design, so it matches the canvas
		// rather than sitting on the system's own grey. The interface follows
		// the system theme; this is the light canvas, which is all that shows
		// for the moment before the page paints.
		BackgroundColour: &options.RGBA{R: 246, G: 247, B: 249, A: 1},
		OnStartup:        a.Startup,
		OnShutdown:       a.Shutdown,
		Bind:             []any{a},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
	})
	if err != nil {
		log.Fatalf("Ldapper stopped: %v", err)
	}
}
