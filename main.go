package main

import (
	"changeme/stacktrace"
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
	var playwrightDriver *playwright.PlaywrightDriver
	var playwrightRunOptions *playwright.RunOptions
	startupErr := func() error {
		userHomeDir, err := os.UserHomeDir()
		if err != nil {
			return stacktrace.New(err)
		}
		var driverDirectory string
		if s := os.Getenv("PLAYWRIGHT_DRIVER_PATH"); s != "" {
			driverDirectory = s
		} else {
			driverDirectory = filepath.Join(userHomeDir, "browserautomate", "playwrightdriver")
		}
		err = os.MkdirAll(driverDirectory, 0755)
		if err != nil {
			return stacktrace.New(err)
		}
		playwrightRunOptions = &playwright.RunOptions{
			DriverDirectory:     driverDirectory,
			SkipInstallBrowsers: true,
			Verbose:             true,
		}
		playwrightDriver, err = playwright.NewDriver(playwrightRunOptions)
		if err != nil {
			return stacktrace.New(err)
		}
		return nil
	}()
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
	backend := &Backend{
		App:                  app,
		PlaywrightDriver:     playwrightDriver,
		PlaywrightRunOptions: playwrightRunOptions,
		Windows:              make(map[string]*application.WebviewWindow),
	}
	defer backend.Close()
	app.RegisterService(application.NewServiceWithOptions(backend, application.ServiceOptions{
		Route: "/backend",
	}))
	go func() {
		http.ListenAndServe("localhost:9246", backend)
	}()
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Browser Automate",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/index.html?foo=bar&foo=baz",
	})
	backend.Mutex.Lock()
	backend.Windows["index"] = window
	backend.Mutex.Unlock()
	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()
	err := app.Run()
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
	if startupErr != nil {
		dialog := app.Dialog.Error()
		dialog.SetTitle("Error")
		dialog.SetMessage(startupErr.Error())
		dialog.Show()
	}
}
