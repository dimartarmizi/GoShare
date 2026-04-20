package main

import (
	"embed"
	"log"

	"goshare/app"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
	cfg := app.Config{
		ListenAddr:    ":9000",
		SaveDir:       "./received",
		DeviceName:    "",
		DeviceTCPPort: 9000,
		DiscoveryPort: 9999,
		ChunkSize:     1024 * 1024,
	}

	ui := NewUIAPI(cfg)

	err := wails.Run(&options.App{
		Title:            "GoShare",
		Width:            1100,
		Height:           760,
		MinWidth:         900,
		MinHeight:        620,
		Frameless:        false,
		DisableResize:    false,
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        ui.Startup,
		OnBeforeClose:    ui.BeforeClose,
		OnShutdown:       ui.Shutdown,
		Bind:             []interface{}{ui},
		BackgroundColour: &options.RGBA{R: 245, G: 247, B: 241, A: 1},
	})

	if err != nil {
		log.Fatal(err)
	}
}
