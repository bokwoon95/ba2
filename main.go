package main

import (
	"context"
	"embed"
	_ "embed"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	application.RegisterEvent[string]("time")
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	playwrightDriverDirectory := filepath.Join(userHomeDir, "browserautomate", "playwrightdriver")
	playwrightDriver, err := playwright.NewDriver(&playwright.RunOptions{
		DriverDirectory:     playwrightDriverDirectory,
		SkipInstallBrowsers: true,
		Verbose:             true,
	})
	if err != nil {
		log.Fatal(err)
	}
	app := application.New(application.Options{
		Name:        "ba2",
		Description: "A demo of using raw HTML & CSS",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	backendService := &BackendService{
		App:                       app,
		PlaywrightDriver:          playwrightDriver,
		PlaywrightDriverDirectory: playwrightDriverDirectory,
	}
	go func() {
		http.ListenAndServe("localhost:9246", backendService)
	}()
	app.RegisterService(application.NewServiceWithOptions(backendService, application.ServiceOptions{
		Route: "/backend",
	}))
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Browser Automate",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/index.html",
	})
	_ = window
	// window.Show()
	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()
	err = app.Run()
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
