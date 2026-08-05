// Command Ldapper is a desktop explorer for LDAP and Active Directory
// directories.
package main

import (
	"embed"
	"log"

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
		// The window chrome is part of the design, so it matches the canvas
		// rather than sitting on the system's own grey.
		BackgroundColour: &options.RGBA{R: 24, G: 24, B: 36, A: 1},
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
