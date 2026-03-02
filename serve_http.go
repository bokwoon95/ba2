package main

import (
	"net/http"
	"path"
	"strings"
)

func (backend *Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		backend.driver(w, r)
		return
	case "installdriver":
		if pathTail != "" {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		backend.installdriver(w, r)
		return
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
}
