package main

import (
	"context"
	"embed"
	_ "embed"
	"errors"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	application.RegisterEvent[string]("time")
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
	backendService := NewBackendService(app)
	app.RegisterService(application.NewServiceWithOptions(backendService, application.ServiceOptions{
		Route: "/backend",
	}))
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Browser Automate",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
	})
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
}
