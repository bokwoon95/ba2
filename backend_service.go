package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
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

type ContextData struct {
	PathTail string `json:"-"`
}

var _ http.Handler = (*BackendService)(nil)

func (svc *BackendService) Hello() string { return "hello" }

func (service *BackendService) driver(w http.ResponseWriter, r *http.Request, contextData ContextData) {
	if contextData.PathTail != "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	type Response struct {
		CurrentVersion  string `json:"currentVersion"`
		RequiredVersion string `json:"requiredVersion"`
		Error           string `json:"error"`
	}
	if r.Method != "GET" {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	writeResponse := func(w http.ResponseWriter, r *http.Request, response Response) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusOK)
			return
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		err := encoder.Encode(&response)
		if err != nil {
			slog.Error(err.Error())
		}
	}
	var response Response
	response.RequiredVersion = service.PlaywrightDriver.Version
	fileInfo, err := os.Stat(filepath.Join(service.PlaywrightDriverDirectory, "package", "cli.js"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			response.Error = "ErrNotExist"
			writeResponse(w, r, response)
			return
		}
		response.Error = err.Error()
		writeResponse(w, r, response)
		return
	}
	if fileInfo.IsDir() {
		response.Error = "ErrNotExist"
		writeResponse(w, r, response)
		return
	}
	cmd := service.PlaywrightDriver.Command("--version")
	output, err := cmd.Output()
	if err != nil {
		response.Error = fmt.Sprintf("could not run driver: %w", err)
		writeResponse(w, r, response)
		return
	}
	response.CurrentVersion = string(output)
	writeResponse(w, r, response)
}

// playwrightCDNMirrors is copied from playwright-go.
var playwrightCDNMirrors = []string{
	"https://playwright.azureedge.net",
	"https://playwright-akamai.azureedge.net",
	"https://playwright-verizon.azureedge.net",
}

// TODO: refactor InstallDriver becomes a http.HandlerFunc, and it writes its progress line by line as the response output. Then on the JS side, we will read the
func (svc *BackendService) installdriver(w http.ResponseWriter, r *http.Request, contextData ContextData) error {
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
	// Redirect unclean paths to the clean path equivalent.
	urlPath := path.Clean(r.URL.Path)
	if urlPath != "/" {
		urlPath += "/"
	}
	if urlPath != r.URL.Path {
		if r.Method == "GET" || r.Method == "HEAD" {
			uri := *r.URL
			uri.Path = urlPath
			http.Redirect(w, r, uri.String(), http.StatusMovedPermanently)
			return
		}
	}
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	pathHead, pathTail, _ := strings.Cut(strings.Trim(urlPath, "/"), "/")
	contextData := ContextData{
		PathTail: pathTail,
	}
	switch pathHead {
	case "driver":
		service.driver(w, r, contextData)
		return
	case "installdriver":
		service.installdriver(w, r, contextData)
		return
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
}
