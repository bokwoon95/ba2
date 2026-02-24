package main

import "github.com/wailsapp/wails/v3/pkg/application"

type Server struct {
	App *application.App
}

func NewServer(app *application.App) (*Server, error) {
	server := &Server{
		App: app,
	}
	return server, nil
}
