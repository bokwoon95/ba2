package main

import "github.com/wailsapp/wails/v3/pkg/application"

type BackendService struct {
	App *application.App
}

func NewBackendService(app *application.App) *BackendService {
	server := &BackendService{
		App: app,
	}
	return server
}

func (service *BackendService) Hello() string {
	return "hello"
}
