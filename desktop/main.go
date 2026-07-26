package main

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	windowsoptions "github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/yourusername/restaurant-finance/internal/bootstrap"
	"github.com/yourusername/restaurant-finance/internal/config"
	"github.com/yourusername/restaurant-finance/internal/desktop"
	appweb "github.com/yourusername/restaurant-finance/web"
)

func main() {
	if !checkOGRExpiry() {
		return
	}
	cfg, err := config.LoadDesktopConfig()
	if err != nil {
		log.Fatal(err)
	}
	services, err := bootstrap.NewServices(cfg)
	if err != nil {
		log.Fatal(err)
	}
	assets, err := fs.Sub(appweb.Static, "static")
	if err != nil {
		log.Fatal(err)
	}
	app := desktop.New(services, cfg.SQLitePath)
	windows := &windowsoptions.Options{}
	if path := bundledWebView2Path(); path != "" {
		windows.WebviewBrowserPath = path
	}
	if err := wails.Run(&options.App{
		Title:     "Restaurant Finance",
		Width:     1440,
		Height:    900,
		MinWidth:  1050,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 244, G: 246, B: 248, A: 1},
		OnStartup:        app.Startup,
		OnShutdown:       app.Shutdown,
		Bind:             []interface{}{app},
		Windows:          windows,
	}); err != nil {
		log.Fatal(err)
	}
}

func bundledWebView2Path() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	path := filepath.Join(filepath.Dir(executable), "WebView2Runtime")
	if _, err := os.Stat(filepath.Join(path, "msedgewebview2.exe")); err != nil {
		return ""
	}
	return path
}
