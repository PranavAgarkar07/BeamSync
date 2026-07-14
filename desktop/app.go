package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
)

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func handleFileTransfer(w http.ResponseWriter, r *http.Request) {
	filepath := r.FormValue("filepath")

	// Compute SHA-256 hash for integrity verification
	hash, err := sha256File(filepath)
	if err != nil {
		http.Error(w, "could not compute file hash", http.StatusInternalServerError)
		return
	}

	// Add SHA-256 header for client-side integrity verification
	w.Header().Set("X-Content-SHA256", hash)
	w.Header().Set("Content-Type", "application/octet-stream")

	file, err := os.Open(filepath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	io.Copy(w, file)
}
