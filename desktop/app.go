package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

func handleManifestResponse(w http.ResponseWriter, r *http.Request, filename string) {
	// Sanitize filename
	safeFilename := filepath.Base(filename)

	// Set security header to prevent browser MIME-type sniffing
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Set Content-Disposition to prevent inline rendering of potentially dangerous files
	// Using inline forces the browser to respect X-Content-Type-Options: nosniff
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", safeFilename))

	// Set explicit content type
	w.Header().Set("Content-Type", "application/json")

	file, err := os.Open(filepath.Join("./manifests", safeFilename))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	http.ServeContent(w, r, safeFilename, time.Time{}, file)
}
