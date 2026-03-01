package main

import (
	"archive/zip"
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

func (svc *BackendService) installdriver(w http.ResponseWriter, _ *http.Request) {
	responseController := http.NewResponseController(w)
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
		fmt.Fprintf(w, "error: creating directory %s: %v\n", svc.PlaywrightDriverDirectory, err)
		responseController.Flush()
		return
	}
	baseName := fmt.Sprintf("playwright-%s-%s.zip", svc.PlaywrightDriver.Version, platform)
	filePath := filepath.Join(svc.PlaywrightDriverDirectory, baseName)
	fileInfo, err := os.Stat(filePath)
	needDownloadFile := false
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(w, "error: fetching file info for %s: %v\n", filePath, err)
			responseController.Flush()
			return
		}
		needDownloadFile = true
	} else {
		if fileInfo.Size() == 0 {
			needDownloadFile = true
		}
	}
	if needDownloadFile {
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
			fmt.Fprintf(w, "info: attempting to download from %s\n", downloadURL)
			responseController.Flush()
			req, err := http.NewRequest("GET", downloadURL, nil)
			if err != nil {
				fmt.Fprintf(w, "info: GET %s: %v\n", downloadURL, err)
				responseController.Flush()
				continue
			}
			resp, err := httpClient.Do(req)
			if err != nil {
				fmt.Fprintf(w, "info: GET %s: %v\n", downloadURL, err)
				responseController.Flush()
				continue
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				fmt.Fprintf(w, "info: GET %s: non 200 status code %d (%s)\n", downloadURL, resp.StatusCode, resp.Status)
				responseController.Flush()
				continue
			}
			successfulResponse = resp
			break
		}
		if successfulResponse == nil {
			fmt.Fprintf(w, "error: failed to download %s from all playwright origins\n", baseName)
			responseController.Flush()
			return
		}
		fmt.Fprintf(w, "info: downloading from %s\n", successfulResponse.Request.URL.String())
		responseController.Flush()
		defer successfulResponse.Body.Close()
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			fmt.Fprintf(w, "error: opening file for writing %s: %v\n", filePath, err)
			responseController.Flush()
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
				fmt.Fprintf(w, "downloading: %d\n", written)
				responseController.Flush()
				if writeErr != nil {
					fmt.Fprintf(w, "error: downloading to %s: %v\n", filePath, writeErr)
					responseController.Flush()
					return
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					fmt.Fprintf(w, "error: downloading from %s: %v\n", successfulResponse.Request.URL.String(), readErr)
					responseController.Flush()
					return
				}
				fmt.Fprintf(w, "downloaded: %d %s\n", written, filePath)
				responseController.Flush()
				break
			}
		}
		err = file.Close()
		if err != nil {
			fmt.Fprintf(w, "error: saving to %s: %v\n", filePath, err)
			responseController.Flush()
			return
		}
	}
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(w, "error: opening file for reading %s: %v\n", filePath, err)
		responseController.Flush()
		return
	}
	defer file.Close()
	fileInfo, err = file.Stat()
	if err != nil {
		fmt.Fprintf(w, "error: fetching file info for %s: %v\n", filePath, err)
		responseController.Flush()
		return
	}
	zipReader, err := zip.NewReader(file, fileInfo.Size())
	if err != nil {
		fmt.Fprintf(w, "error: reading zip file %s: %v\n", filePath, err)
		responseController.Flush()
		return
	}
	for _, zipFile := range zipReader.File {
		fmt.Fprintf(w, "unzipping: %d %s\n", zipFile.UncompressedSize64, zipFile.Name)
		responseController.Flush()
		destFilePath := filepath.Join(svc.PlaywrightDriverDirectory, zipFile.Name)
		if zipFile.FileInfo().IsDir() {
			err := os.MkdirAll(destFilePath, 0755)
			if err != nil {
				fmt.Fprintf(w, "error: creating folder %s: %v\n", destFilePath, err)
				responseController.Flush()
				return
			}
			continue
		}
		srcFile, err := zipFile.Open()
		if err != nil {
			fmt.Fprintf(w, "error: opening file for reading %s: %v\n", zipFile.Name, err)
			responseController.Flush()
			return
		}
		destFile, err := os.OpenFile(destFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			fmt.Fprintf(w, "error: opening file for writing %s: %v\n", destFilePath, err)
			responseController.Flush()
			return
		}
		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			fmt.Fprintf(w, "error: unzipping %s: %v\n", zipFile.Name, err)
			responseController.Flush()
			return
		}
		err = destFile.Close()
		if err != nil {
			fmt.Fprintf(w, "error: closing %s: %v\n", destFilePath, err)
			responseController.Flush()
			return
		}
		err = srcFile.Close()
		if err != nil {
			fmt.Fprintf(w, "error: closing %s: %v\n", zipFile.Name, err)
			responseController.Flush()
			return
		}
		if zipFile.Mode().Perm()&0111 != 0 && runtime.GOOS != "windows" {
			fileInfo, err := os.Stat(destFilePath)
			if err != nil {
				fmt.Fprintf(w, "error: fetching file info for %s: %v\n", destFilePath, err)
				responseController.Flush()
				return
			}
			err = os.Chmod(destFilePath, fileInfo.Mode()|0111)
			if err != nil {
				fmt.Fprintf(w, "error: making file executable %s: %v\n", destFilePath, err)
				responseController.Flush()
				return
			}
		}
	}
	fmt.Fprintf(w, "unzipped: %s\n", filePath)
	responseController.Flush()
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
