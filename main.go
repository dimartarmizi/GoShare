package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "GoShare",
		Width:            1024,
		Height:           720,
		MinWidth:         320,
		MinHeight:        568,
		WindowStartState: options.Maximised,
		DisableResize:    false,
		Frameless:        false,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 244, G: 248, B: 247, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
