package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

func (backend *Backend) driver(w http.ResponseWriter, r *http.Request) {
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
	response.RequiredVersion = backend.PlaywrightDriver.Version
	fileInfo, err := os.Stat(filepath.Join(backend.PlaywrightDriverDirectory, "package", "cli.js"))
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
	cmd := backend.PlaywrightDriver.Command("--version")
	output, err := cmd.Output()
	if err != nil {
		response.Error = fmt.Sprintf("could not run driver: %v", err)
		writeResponse(w, r, response)
		return
	}
	response.CurrentVersion = string(output)
	writeResponse(w, r, response)
}
