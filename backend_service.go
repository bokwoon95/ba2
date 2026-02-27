package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/playwright-community/playwright-go"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type BackendService struct {
	App                       *application.App
	PlaywrightDriver          *playwright.PlaywrightDriver
	PlaywrightDriverDirectory string
}

var _ http.Handler = (*BackendService)(nil)

func (svc *BackendService) Hello() string { return "hello" }

// TODO: make this a GET handlerfunc. Returns a JS object.
func (service *BackendService) GetDriverVersion() (currentVersion string, requiredVersion string, err error) {
	fileInfo, err := os.Stat(filepath.Join(service.PlaywrightDriverDirectory, "package", "cli.js"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", "", nil
		}
		return "", "", err
	}
	if fileInfo.IsDir() {
		return "", "", nil
	}
	cmd := service.PlaywrightDriver.Command("--version")
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("could not run driver: %w", err)
	}
	return string(output), service.PlaywrightDriver.Version, nil
}

// playwrightCDNMirrors is copied from playwright-go.
var playwrightCDNMirrors = []string{
	"https://playwright.azureedge.net",
	"https://playwright-akamai.azureedge.net",
	"https://playwright-verizon.azureedge.net",
}

// TODO: refactor InstallDriver becomes a http.HandlerFunc, and it writes its progress line by line as the response output. Then on the JS side, we will read the
func (svc *BackendService) installDriver(w http.ResponseWriter, r *http.Request) error {
	platform := ""
	switch runtime.GOOS {
	case "windows":
		platform = "win32_x64"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			platform = "mac-arm64"
		} else {
			platform = "mac"
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			platform = "linux-arm64"
		} else {
			platform = "linux"
		}
	}
	pattern := "%s/builds/driver/playwright-%s-%s.zip"
	if strings.Contains(svc.PlaywrightDriver.Version, "beta") || strings.Contains(svc.PlaywrightDriver.Version, "alpha") || strings.Contains(svc.PlaywrightDriver.Version, "next") {
		pattern = "%s/builds/driver/next/playwright-%s-%s.zip"
	}
	var driverURLs []string
	playwrightDownloadHost := os.Getenv("PLAYWRIGHT_DOWNLOAD_HOST")
	if playwrightDownloadHost != "" {
		driverURLs = []string{
			fmt.Sprintf(pattern, playwrightDownloadHost, svc.PlaywrightDriver.Version, platform),
		}
	} else {
		for _, playwrightCDNMirror := range playwrightCDNMirrors {
			driverURLs = append(driverURLs, fmt.Sprintf(pattern, playwrightCDNMirror, svc.PlaywrightDriver.Version, platform))
		}
	}
	var downloadErr error
	for _, driverURL := range driverURLs {
		httpResponse, httpError := http.Get(driverURL) // TODO: change to custom HTTP client with 5min timeout.
		if httpError != nil {
			downloadErr = errors.Join(downloadErr, fmt.Errorf("could not download driver from %s: %w", driverURL, httpError))
			continue
		}
		defer httpResponse.Body.Close()
		if httpResponse.StatusCode != http.StatusOK {
			downloadErr = errors.Join(downloadErr, fmt.Errorf("error: got non 200 status code: %d (%s) from %s", httpResponse.StatusCode, httpResponse.Status, driverURL))
			continue
		}
		body, httpError := io.ReadAll(httpResponse.Body)
		if httpError != nil {
			downloadErr = errors.Join(downloadErr, fmt.Errorf("could not read response body: %w", httpError))
			continue
		}
		_ = body // TODO: write zip file into driver directory.
		break
	}
	// TODO: unzip the zip file in driver directory.
	_ = svc.PlaywrightDriver.DownloadDriver
	return nil
}

func (service *BackendService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("got here!")
	w.Write([]byte(r.URL.Path))
}
