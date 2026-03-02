package main

import (
	"fmt"
	"net/http"

	"github.com/playwright-community/playwright-go"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func init() {
	application.RegisterEvent[UpdateEvent]("backend:update")
}

type Backend struct {
	App                       *application.App
	PlaywrightDriver          *playwright.PlaywrightDriver
	PlaywrightDriverDirectory string
}

type UpdateEvent struct {
	EventID string `json:"eventID"`
	Category string `json:"category"`
	Message string `json:"message"`
}

var _ http.Handler = (*Backend)(nil)

func (backend *Backend) Hello() string { return "hello" }

// HumanReadableFileSize returns a human readable file size of an int64 size in
// bytes.
func HumanReadableFileSize(size int64) string {
	// https://yourbasic.org/golang/formatting-byte-size-to-human-readable-format/
	if size < 0 {
		return ""
	}
	const unit = 1000
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "kMGTPE"[exp])
}
