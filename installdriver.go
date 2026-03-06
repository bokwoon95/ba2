package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// playwrightCDNMirrors is copied from playwright-go.
var playwrightCDNMirrors = []string{
	"https://playwright.azureedge.net",
	"https://playwright-akamai.azureedge.net",
	"https://playwright-verizon.azureedge.net",
}

func (backend *Backend) installdriver(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	eventID := r.Form.Get("eventID")
	if eventID == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "MissingEventID")
		return
	}
	responseController := http.NewResponseController(w)
	writeResponse := func(w http.ResponseWriter, category string, message string) {
		fmt.Fprintln(w, category+": "+message)
		responseController.Flush()
		backend.App.Event.EmitEvent(&application.CustomEvent{
			Name: "update_event",
			Data: UpdateEvent{
				EventID:  eventID,
				Category: category,
				Message:  message,
			},
		})
	}
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
	err := os.MkdirAll(backend.PlaywrightRunOptions.DriverDirectory, 0755)
	if err != nil {
		writeResponse(w, "error", fmt.Sprintf("creating directory %s: %v", backend.PlaywrightRunOptions.DriverDirectory, err))
		return
	}
	baseName := fmt.Sprintf("playwright-%s-%s.zip", backend.PlaywrightDriver.Version, platform)
	filePath := filepath.Join(backend.PlaywrightRunOptions.DriverDirectory, baseName)
	fileInfo, err := os.Stat(filePath)
	needDownloadFile := false
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			writeResponse(w, "error", fmt.Sprintf("fetching file info for %s: %v", filePath, err))
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
		if !strings.Contains(backend.PlaywrightDriver.Version, "beta") && !strings.Contains(backend.PlaywrightDriver.Version, "alpha") && !strings.Contains(backend.PlaywrightDriver.Version, "next") {
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
			writeResponse(w, "info", fmt.Sprintf("attempting to download from %s", downloadURL))
			req, err := http.NewRequest("GET", downloadURL, nil)
			if err != nil {
				writeResponse(w, "info", fmt.Sprintf("info: GET %s: %v", downloadURL, err))
				continue
			}
			resp, err := httpClient.Do(req)
			if err != nil {
				writeResponse(w, "info", fmt.Sprintf("info: GET %s: %v", downloadURL, err))
				continue
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				writeResponse(w, "info", fmt.Sprintf("GET %s: non 200 status code %d (%s)", downloadURL, resp.StatusCode, resp.Status))
				continue
			}
			successfulResponse = resp
			break
		}
		if successfulResponse == nil {
			writeResponse(w, "error", fmt.Sprintf("error: failed to download %s from all playwright origins", baseName))
			return
		}
		writeResponse(w, "info", fmt.Sprintf("downloading from %s", successfulResponse.Request.URL.String()))
		defer successfulResponse.Body.Close()
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			writeResponse(w, "error", fmt.Sprintf("opening file for writing %s: %v", filePath, err))
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
				writeResponse(w, "downloading", fmt.Sprintf("%d", written))
				if writeErr != nil {
					writeResponse(w, "error", fmt.Sprintf("downloading to %s: %v", filePath, writeErr))
					return
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					writeResponse(w, "error", fmt.Sprintf("downloading from %s: %v", successfulResponse.Request.URL.String(), readErr))
					return
				}
				writeResponse(w, "downloaded", fmt.Sprintf("%d %s", written, filePath))
				break
			}
		}
		err = file.Close()
		if err != nil {
			writeResponse(w, "error", fmt.Sprintf("saving to %s: %v", filePath, err))
			return
		}
	}
	file, err := os.Open(filePath)
	if err != nil {
		writeResponse(w, "error", fmt.Sprintf("opening file for reading %s: %v", filePath, err))
		return
	}
	defer file.Close()
	fileInfo, err = file.Stat()
	if err != nil {
		writeResponse(w, "error", fmt.Sprintf("fetching file info for %s: %v", filePath, err))
		return
	}
	zipReader, err := zip.NewReader(file, fileInfo.Size())
	if err != nil {
		writeResponse(w, "error", fmt.Sprintf("reading zip file %s: %v", filePath, err))
		return
	}
	for _, zipFile := range zipReader.File {
		writeResponse(w, "unzipping", fmt.Sprintf("%d %s", zipFile.UncompressedSize64, zipFile.Name))
		destFilePath := filepath.Join(backend.PlaywrightRunOptions.DriverDirectory, zipFile.Name)
		if zipFile.FileInfo().IsDir() {
			err := os.MkdirAll(destFilePath, 0755)
			if err != nil {
				writeResponse(w, "error", fmt.Sprintf("creating folder %s: %v", destFilePath, err))
				return
			}
			continue
		}
		srcFile, err := zipFile.Open()
		if err != nil {
			writeResponse(w, "error", fmt.Sprintf("opening file for reading %s: %v", zipFile.Name, err))
			return
		}
		destFile, err := os.OpenFile(destFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			writeResponse(w, "error", fmt.Sprintf("opening file for writing %s: %v", destFilePath, err))
			return
		}
		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			writeResponse(w, "error", fmt.Sprintf("unzipping %s: %v", zipFile.Name, err))
			return
		}
		err = destFile.Close()
		if err != nil {
			writeResponse(w, "error", fmt.Sprintf("closing %s: %v", destFilePath, err))
			return
		}
		err = srcFile.Close()
		if err != nil {
			writeResponse(w, "error", fmt.Sprintf("closing %s: %v", zipFile.Name, err))
			return
		}
		if zipFile.Mode().Perm()&0111 != 0 && runtime.GOOS != "windows" {
			fileInfo, err := os.Stat(destFilePath)
			if err != nil {
				writeResponse(w, "error", fmt.Sprintf("fetching file info for %s: %v", destFilePath, err))
				return
			}
			err = os.Chmod(destFilePath, fileInfo.Mode()|0111)
			if err != nil {
				writeResponse(w, "error", fmt.Sprintf("making file executable %s: %v", destFilePath, err))
				return
			}
		}
	}
	writeResponse(w, "success", fmt.Sprintf("unzipped %s", filePath))
}
