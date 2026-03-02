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
)

// playwrightCDNMirrors is copied from playwright-go.
var playwrightCDNMirrors = []string{
	"https://playwright.azureedge.net",
	"https://playwright-akamai.azureedge.net",
	"https://playwright-verizon.azureedge.net",
}

func (backend *Backend) installdriver(w http.ResponseWriter, r *http.Request) {
	responseController := http.NewResponseController(w)
	eventID := r.Form.Get("eventID")
	if eventID == "" {
		backend.App.Event.Emit("backend:update", UpdateEvent{
			EventID:  eventID,
			Category: "error",
			Message:  "missing eventID",
		})
		fmt.Fprintf(w, "error: missing eventID\n")
		responseController.Flush()
		return
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
	err := os.MkdirAll(backend.PlaywrightDriverDirectory, 0755)
	if err != nil {
		backend.App.Event.Emit("backend:update", UpdateEvent{
			EventID:  eventID,
			Category: "error",
			Message:  fmt.Sprintf("creating directory %s: %v", backend.PlaywrightDriverDirectory, err),
		})
		fmt.Fprintf(w, "error: creating directory %s: %v\n", backend.PlaywrightDriverDirectory, err)
		responseController.Flush()
		return
	}
	baseName := fmt.Sprintf("playwright-%s-%s.zip", backend.PlaywrightDriver.Version, platform)
	filePath := filepath.Join(backend.PlaywrightDriverDirectory, baseName)
	fileInfo, err := os.Stat(filePath)
	needDownloadFile := false
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			backend.App.Event.Emit("backend:update", UpdateEvent{
				EventID:  eventID,
				Category: "error",
				Message:  fmt.Sprintf("fetching file info for %s: %v", filePath, err),
			})
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
			backend.App.Event.Emit("backend:update", UpdateEvent{
				EventID:  eventID,
				Category: "info",
				Message:  fmt.Sprintf("attempting to download from %s", downloadURL),
			})
			fmt.Fprintf(w, "info: attempting to download from %s\n", downloadURL)
			responseController.Flush()
			req, err := http.NewRequest("GET", downloadURL, nil)
			if err != nil {
				backend.App.Event.Emit("backend:update", UpdateEvent{
					EventID:  eventID,
					Category: "info",
					Message:  fmt.Sprintf("info: GET %s: %v", downloadURL, err),
				})
				fmt.Fprintf(w, "info: GET %s: %v\n", downloadURL, err)
				responseController.Flush()
				continue
			}
			resp, err := httpClient.Do(req)
			if err != nil {
				backend.App.Event.Emit("backend:update", UpdateEvent{
					EventID:  eventID,
					Category: "info",
					Message:  fmt.Sprintf("info: GET %s: %v", downloadURL, err),
				})
				fmt.Fprintf(w, "info: GET %s: %v\n", downloadURL, err)
				responseController.Flush()
				continue
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				backend.App.Event.Emit("backend:update", UpdateEvent{
					EventID:  eventID,
					Category: "info",
					Message:  fmt.Sprintf("GET %s: non 200 status code %d (%s)", downloadURL, resp.StatusCode, resp.Status),
				})
				fmt.Fprintf(w, "info: GET %s: non 200 status code %d (%s)\n", downloadURL, resp.StatusCode, resp.Status)
				responseController.Flush()
				continue
			}
			successfulResponse = resp
			break
		}
		if successfulResponse == nil {
			backend.App.Event.Emit("backend:update", UpdateEvent{
				EventID:  eventID,
				Category: "error",
				Message:  fmt.Sprintf("error: failed to download %s from all playwright origins", baseName),
			})
			fmt.Fprintf(w, "error: failed to download %s from all playwright origins\n", baseName)
			responseController.Flush()
			return
		}
		backend.App.Event.Emit("backend:update", UpdateEvent{
			EventID:  eventID,
			Category: "info",
			Message:  fmt.Sprintf("downloading from %s", successfulResponse.Request.URL.String()),
		})
		fmt.Fprintf(w, "info: downloading from %s\n", successfulResponse.Request.URL.String())
		responseController.Flush()
		defer successfulResponse.Body.Close()
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			backend.App.Event.Emit("backend:update", UpdateEvent{
				EventID:  eventID,
				Category: "error",
				Message:  fmt.Sprintf("opening file for writing %s: %v", filePath, err),
			})
			fmt.Fprintf(w, "error: opening file for writing %s: %v\n", filePath, err)
			responseController.Flush()
			return
		}
		defer file.Close()
		var buf [32 * 1024]byte
		var written int64
		for {
			time.Sleep(300 * time.Millisecond)
			bytesRead, readErr := successfulResponse.Body.Read(buf[:])
			if bytesRead > 0 {
				bytesWritten, writeErr := file.Write(buf[:bytesRead])
				written += int64(bytesWritten)
				backend.App.Event.Emit("backend:update", UpdateEvent{
					EventID:  eventID,
					Category: "downloading",
					Message:  fmt.Sprintf("%d", written),
				})
				fmt.Fprintf(w, "downloading: %d\n", written)
				responseController.Flush()
				if writeErr != nil {
					backend.App.Event.Emit("backend:update", UpdateEvent{
						EventID:  eventID,
						Category: "error",
						Message:  fmt.Sprintf("downloading to %s: %v", filePath, writeErr),
					})
					fmt.Fprintf(w, "error: downloading to %s: %v\n", filePath, writeErr)
					responseController.Flush()
					return
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					backend.App.Event.Emit("backend:update", UpdateEvent{
						EventID:  eventID,
						Category: "error",
						Message:  fmt.Sprintf("downloading from %s: %v", successfulResponse.Request.URL.String(), readErr),
					})
					fmt.Fprintf(w, "error: downloading from %s: %v\n", successfulResponse.Request.URL.String(), readErr)
					responseController.Flush()
					return
				}
				backend.App.Event.Emit("backend:update", UpdateEvent{
					EventID:  eventID,
					Category: "downloaded",
					Message:  fmt.Sprintf("%d %s", written, filePath),
				})
				fmt.Fprintf(w, "downloaded: %d %s\n", written, filePath)
				responseController.Flush()
				break
			}
		}
		err = file.Close()
		if err != nil {
			backend.App.Event.Emit("backend:update", UpdateEvent{
				EventID:  eventID,
				Category: "error",
				Message:  fmt.Sprintf("saving to %s: %v", filePath, err),
			})
			fmt.Fprintf(w, "error: saving to %s: %v\n", filePath, err)
			responseController.Flush()
			return
		}
	}
	file, err := os.Open(filePath)
	if err != nil {
		backend.App.Event.Emit("backend:update", UpdateEvent{
			EventID:  eventID,
			Category: "error",
			Message:  fmt.Sprintf("opening file for reading %s: %v", filePath, err),
		})
		fmt.Fprintf(w, "error: opening file for reading %s: %v\n", filePath, err)
		responseController.Flush()
		return
	}
	defer file.Close()
	fileInfo, err = file.Stat()
	if err != nil {
		backend.App.Event.Emit("backend:update", UpdateEvent{
			EventID:  eventID,
			Category: "error",
			Message:  fmt.Sprintf("fetching file info for %s: %v", filePath, err),
		})
		fmt.Fprintf(w, "error: fetching file info for %s: %v\n", filePath, err)
		responseController.Flush()
		return
	}
	zipReader, err := zip.NewReader(file, fileInfo.Size())
	if err != nil {
		backend.App.Event.Emit("backend:update", UpdateEvent{
			EventID:  eventID,
			Category: "error",
			Message:  fmt.Sprintf("reading zip file %s: %v", filePath, err),
		})
		fmt.Fprintf(w, "error: reading zip file %s: %v\n", filePath, err)
		responseController.Flush()
		return
	}
	for _, zipFile := range zipReader.File {
		backend.App.Event.Emit("backend:update", UpdateEvent{
			EventID:  eventID,
			Category: "unzipping",
			Message:  fmt.Sprintf("%d %s", zipFile.UncompressedSize64, zipFile.Name),
		})
		fmt.Fprintf(w, "unzipping: %d %s\n", zipFile.UncompressedSize64, zipFile.Name)
		responseController.Flush()
		destFilePath := filepath.Join(backend.PlaywrightDriverDirectory, zipFile.Name)
		if zipFile.FileInfo().IsDir() {
			err := os.MkdirAll(destFilePath, 0755)
			if err != nil {
				backend.App.Event.Emit("backend:update", UpdateEvent{
					EventID:  eventID,
					Category: "error",
					Message:  fmt.Sprintf("creating folder %s: %v", destFilePath, err),
				})
				fmt.Fprintf(w, "error: creating folder %s: %v\n", destFilePath, err)
				responseController.Flush()
				return
			}
			continue
		}
		srcFile, err := zipFile.Open()
		if err != nil {
			backend.App.Event.Emit("backend:update", UpdateEvent{
				EventID:  eventID,
				Category: "error",
				Message:  fmt.Sprintf("opening file for reading %s: %v", zipFile.Name, err),
			})
			fmt.Fprintf(w, "error: opening file for reading %s: %v\n", zipFile.Name, err)
			responseController.Flush()
			return
		}
		destFile, err := os.OpenFile(destFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			backend.App.Event.Emit("backend:update", UpdateEvent{
				EventID:  eventID,
				Category: "error",
				Message:  fmt.Sprintf("opening file for writing %s: %v", destFilePath, err),
			})
			fmt.Fprintf(w, "error: opening file for writing %s: %v\n", destFilePath, err)
			responseController.Flush()
			return
		}
		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			backend.App.Event.Emit("backend:update", UpdateEvent{
				EventID:  eventID,
				Category: "error",
				Message:  fmt.Sprintf("unzipping %s: %v", zipFile.Name, err),
			})
			fmt.Fprintf(w, "error: unzipping %s: %v\n", zipFile.Name, err)
			responseController.Flush()
			return
		}
		err = destFile.Close()
		if err != nil {
			backend.App.Event.Emit("backend:update", UpdateEvent{
				EventID:  eventID,
				Category: "error",
				Message:  fmt.Sprintf("closing %s: %v", destFilePath, err),
			})
			fmt.Fprintf(w, "error: closing %s: %v\n", destFilePath, err)
			responseController.Flush()
			return
		}
		err = srcFile.Close()
		if err != nil {
			backend.App.Event.Emit("backend:update", UpdateEvent{
				EventID:  eventID,
				Category: "error",
				Message:  fmt.Sprintf("closing %s: %v", zipFile.Name, err),
			})
			fmt.Fprintf(w, "error: closing %s: %v\n", zipFile.Name, err)
			responseController.Flush()
			return
		}
		if zipFile.Mode().Perm()&0111 != 0 && runtime.GOOS != "windows" {
			fileInfo, err := os.Stat(destFilePath)
			if err != nil {
				backend.App.Event.Emit("backend:update", UpdateEvent{
					EventID:  eventID,
					Category: "error",
					Message:  fmt.Sprintf("fetching file info for %s: %v", destFilePath, err),
				})
				fmt.Fprintf(w, "error: fetching file info for %s: %v\n", destFilePath, err)
				responseController.Flush()
				return
			}
			err = os.Chmod(destFilePath, fileInfo.Mode()|0111)
			if err != nil {
				backend.App.Event.Emit("backend:update", UpdateEvent{
					EventID:  eventID,
					Category: "error",
					Message:  fmt.Sprintf("making file executable %s: %v", destFilePath, err),
				})
				fmt.Fprintf(w, "error: making file executable %s: %v\n", destFilePath, err)
				responseController.Flush()
				return
			}
		}
	}
	backend.App.Event.Emit("backend:update", UpdateEvent{
		EventID:  eventID,
		Category: "success",
		Message:  fmt.Sprintf("unzipped %s", filePath),
	})
	fmt.Fprintf(w, "success: unzipped %s\n", filePath)
	responseController.Flush()
}
