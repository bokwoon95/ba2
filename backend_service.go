package main

import (
	"net/http"

	"github.com/playwright-community/playwright-go"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type BackendService struct {
	App             *application.App
	Driver          *playwright.PlaywrightDriver
	DriverDirectory string
}

func (service *BackendService) Hello() string {
	return "hello"
}

func (service *BackendService) DriverIsInstalled() bool {
	_ = playwright.Install
	return false
}

func (service *BackendService) DriverIsUpToDate() bool {
	return false
}

func (service *BackendService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}
