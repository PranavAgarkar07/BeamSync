package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

func handleManifestDownload(w http.ResponseWriter, r *http.Request) {
	// Get requested filename and sanitize it
	requestedFile := r.FormValue("filename")

	// Use filepath.Base to prevent directory traversal
	// This strips any directory components and returns only the filename
	safeFilename := filepath.Base(requestedFile)

	// Verify the sanitized filename isn't empty (prevents traversal attempts)
	if safeFilename == "" || safeFilename == "." || safeFilename == ".." {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	// Construct safe path within manifest directory
	manifestDir := "./manifests"
	filePath := filepath.Join(manifestDir, safeFilename)

	// Open and serve the file
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "manifest not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeFilename))
	http.ServeContent(w, r, safeFilename, time.Time{}, file)
}
