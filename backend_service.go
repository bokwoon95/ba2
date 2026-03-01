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
	"time"

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

func (service *BackendService) driver(w http.ResponseWriter, r *http.Request) {
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
		response.Error = fmt.Sprintf("could not run driver: %v", err)
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
func (svc *BackendService) installdriver(w http.ResponseWriter, r *http.Request) {
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
	err := os.MkdirAll(svc.PlaywrightDriverDirectory, 0755)
	if err != nil {
		fmt.Fprintf(w, "creating directory %s: %v\n", svc.PlaywrightDriverDirectory, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	baseName := fmt.Sprintf("playwright-%s-%s.zip", svc.PlaywrightDriver.Version, platform)
	filePath := filepath.Join(svc.PlaywrightDriverDirectory, baseName)
	fileInfo, err := os.Stat(filePath)
	if err != nil || fileInfo.Size() == 0 {
		if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(w, "error fetching file info for %s: %v\n", filePath, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var pathName string
		if !strings.Contains(svc.PlaywrightDriver.Version, "beta") && !strings.Contains(svc.PlaywrightDriver.Version, "alpha") && !strings.Contains(svc.PlaywrightDriver.Version, "next") {
			pathName = "/builds/driver/" + baseName
		} else {
			pathName = "/builds/driver/next/" + baseName
		}
		var origins []string
		if s := os.Getenv("PLAYWRIGHT_DOWNLOAD_HOST"); s != "" {
			origins = []string{s}
		} else {
			origins = playwrightCDNMirrors
		}
		var successfulResponse *http.Response
		httpClient := &http.Client{
			Timeout: 5 * time.Minute,
		}
		for _, origin := range origins {
			downloadURL := origin + pathName
			req, err := http.NewRequest("GET", downloadURL, nil)
			if err != nil {
				fmt.Fprintf(w, "GET %s: %v\n", downloadURL, err)
				continue
			}
			resp, err := httpClient.Do(req)
			if err != nil {
				fmt.Fprintf(w, "GET %s: %v\n", downloadURL, err)
				continue
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				fmt.Fprintf(w, "GET %s: non 200 status code %d (%s)\n", downloadURL, resp.StatusCode, resp.Status)
				continue
			}
			successfulResponse = resp
			break
		}
		if successfulResponse == nil {
			fmt.Fprintf(w, "failed to download %s from all playwright origins\n", baseName)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "downloading from %s\n", successfulResponse.Request.URL.String())
		defer successfulResponse.Body.Close()
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			fmt.Fprintf(w, "error opening file %s: %v\n", filePath, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer file.Close()
		var buf [32 * 1024]byte
		var written int64
		for {
			bytesRead, readErr := successfulResponse.Body.Read(buf[:])
			if bytesRead > 0 {
				bytesWritten, writeErr := file.Write(buf[:bytesRead])
				written += int64(bytesWritten)
				fmt.Fprintf(w, "downloading to %s: %s\n", filePath, HumanReadableFileSize(written))
				if writeErr != nil {
					fmt.Fprintf(w, "error downloading to %s: %v\n", filePath, writeErr)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					fmt.Fprintf(w, "error downloading from %s: %v\n", successfulResponse.Request.URL.String(), readErr)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				fmt.Fprintf(w, "download to %s complete\n", filePath)
				break
			}
		}
		err = file.Close()
		if err != nil {
			fmt.Fprintf(w, "error saving to %s: %v\n", filePath, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	// TODO: unzip filePath into the driver directory.
	_ = svc.PlaywrightDriver.DownloadDriver
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
	switch pathHead {
	case "driver":
		if pathTail != "" {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		service.driver(w, r)
		return
	case "installdriver":
		if pathTail != "" {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		service.installdriver(w, r)
		return
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
}

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
