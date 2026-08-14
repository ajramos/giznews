// GizNews Desktop — a thin Wails presentation layer over the giznews service
// layer (pkg/desktop). The heavy lifting (fetch, classify, digest, kb, search)
// lives in the main module and is bound through the ctx-less wrappers in
// app.go. This module never imports internal/ packages.
package main

import (
	"context"
	"embed"
	"log"

	gizdesktop "github.com/ajramos/giznews/pkg/desktop"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	api, err := gizdesktop.OpenApp()
	if err != nil {
		log.Fatalf("giznews: %v", err)
	}
	defer api.Close()

	app := &App{api: api}

	err = wails.Run(&options.App{
		Title:     "GizNews",
		Width:     1280,
		Height:    832,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 16, G: 18, B: 24, A: 1},
		OnStartup:        func(ctx context.Context) {},
		OnShutdown:       func(ctx context.Context) {},
		Bind:             []interface{}{app},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "GizNews",
				Message: "AI-powered news reader and knowledge-graph builder.",
			},
		},
	})
	if err != nil {
		log.Fatalf("wails: %v", err)
	}
}
