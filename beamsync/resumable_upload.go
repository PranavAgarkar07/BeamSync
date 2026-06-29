package beamsync

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var contentRangePattern = regexp.MustCompile(`^bytes (\d+)-(\d+)/(\d+)$`)
var resumableUploadTTL = 24 * time.Hour

type resumableChunk struct {
	Start  int64  `json:"start"`
	End    int64  `json:"end"`
	SHA256 string `json:"sha256,omitempty"`
}

type resumableManifest struct {
	UploadID       string            `json:"uploadId"`
	Filename       string            `json:"filename"`
	TotalSize      int64             `json:"totalSize"`
	ChunkSize      int64             `json:"chunkSize"`
	CompletedBytes int64             `json:"completedBytes"`
	CompletedRange string            `json:"completedRange,omitempty"`
	Chunks         []resumableChunk  `json:"chunks"`
	ChunkHashes    map[string]string `json:"chunkHashes,omitempty"`
	UpdatedAt      string            `json:"updatedAt"`
}

func handleResumableUpload(uploadDir string, state *serverState, emit func(string, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		if r.Method != http.MethodPut && r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		uploadID := strings.TrimSpace(r.Header.Get("X-BeamSync-Upload-ID"))
		filename := filepath.Base(strings.TrimSpace(r.Header.Get("X-BeamSync-Filename")))
		if !isSafeUploadID(uploadID) || filename == "" || filename == "." {
			http.Error(w, "Missing or invalid resumable upload headers", http.StatusBadRequest)
			return
		}

		start, end, total, err := parseContentRange(r.Header.Get("Content-Range"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resumeDir := filepath.Join(uploadDir, ".beamsync-resume")
		if err := os.MkdirAll(resumeDir, 0755); err != nil {
			http.Error(w, "Could not create resume directory", http.StatusInternalServerError)
			return
		}
		cleanupResumableUploads(resumeDir, resumableUploadTTL)

		tempPath := filepath.Join(resumeDir, uploadID+".part")
		manifestPath := filepath.Join(resumeDir, uploadID+".json")
		file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			http.Error(w, "Could not open resumable upload", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		if _, err := file.Seek(start, io.SeekStart); err != nil {
			http.Error(w, "Could not seek resumable upload", http.StatusInternalServerError)
			return
		}

		state.beginUpload()
		defer state.endUpload()
		state.markHeartbeat()

		hash := sha256.New()
		writer := bufio.NewWriterSize(file, 8*1024*1024)
		written, err := copyChunked(writer, io.TeeReader(r.Body, hash), 8*1024*1024)
		if flushErr := writer.Flush(); flushErr != nil && err == nil {
			err = flushErr
		}
		if err != nil {
			http.Error(w, "Could not write resumable chunk", http.StatusInternalServerError)
			return
		}
		if written != end-start+1 {
			http.Error(w, "Chunk size does not match Content-Range", http.StatusBadRequest)
			return
		}
		chunkHash := hex.EncodeToString(hash.Sum(nil))
		expectedHash := strings.TrimSpace(r.Header.Get("X-BeamSync-Chunk-SHA256"))
		if expectedHash != "" && !strings.EqualFold(expectedHash, chunkHash) {
			http.Error(w, "Chunk SHA-256 mismatch", http.StatusBadRequest)
			return
		}

		received := end + 1
		manifest, err := loadResumableManifest(manifestPath)
		if err != nil {
			http.Error(w, "Could not read resumable manifest", http.StatusInternalServerError)
			return
		}
		updateResumableManifest(&manifest, uploadID, filename, start, end, total, chunkHash)
		if err := saveResumableManifest(manifestPath, manifest); err != nil {
			http.Error(w, "Could not write resumable manifest", http.StatusInternalServerError)
			return
		}
		emit("upload_progress", fmt.Sprintf("%s|%d|%d", filename, received, total))

		if manifest.CompletedBytes < total {
			if manifest.CompletedBytes > 0 {
				w.Header().Set("Range", fmt.Sprintf("bytes=0-%d", manifest.CompletedBytes-1))
			}
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("chunk accepted"))
			return
		}

		finalPath := autoRenamePath(uploadDir, filename)
		if err := os.Rename(tempPath, finalPath); err != nil {
			http.Error(w, "Could not finalize resumable upload", http.StatusInternalServerError)
			return
		}
		_ = os.Remove(manifestPath)

		savedName := filepath.Base(finalPath)
		emit("upload_progress", fmt.Sprintf("%s|%d|%d", savedName, total, total))
		go func(fname string) {
			time.Sleep(100 * time.Millisecond)
			emit("file_received", fname)
		}(savedName)

		fmt.Printf("resumable upload completed: %s (%d bytes, active=%d)\n", savedName, total, state.activeUploads())
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("upload complete"))
	}
}

func handleResumableUploadStatus(uploadDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		uploadID := strings.TrimPrefix(r.URL.Path, "/upload-status/")
		uploadID = strings.TrimSpace(uploadID)
		if !isSafeUploadID(uploadID) {
			http.Error(w, "Missing or invalid upload id", http.StatusBadRequest)
			return
		}

		resumeDir := filepath.Join(uploadDir, ".beamsync-resume")
		cleanupResumableUploads(resumeDir, resumableUploadTTL)
		manifest, err := loadResumableManifest(filepath.Join(resumeDir, uploadID+".json"))
		if err != nil {
			http.Error(w, "Could not read resumable manifest", http.StatusInternalServerError)
			return
		}
		if manifest.UploadID == "" {
			http.Error(w, "Upload not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if manifest.CompletedBytes > 0 {
			w.Header().Set("Range", fmt.Sprintf("bytes=0-%d", manifest.CompletedBytes-1))
		}
		json.NewEncoder(w).Encode(manifest)
	}
}

func loadResumableManifest(path string) (resumableManifest, error) {
	var manifest resumableManifest
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return manifest, nil
		}
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func saveResumableManifest(path string, manifest resumableManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func updateResumableManifest(manifest *resumableManifest, uploadID string, filename string, start int64, end int64, total int64, chunkHash string) {
	chunkSize := end - start + 1
	if manifest.UploadID == "" {
		manifest.UploadID = uploadID
		manifest.Filename = filename
		manifest.TotalSize = total
		manifest.ChunkSize = chunkSize
		manifest.ChunkHashes = map[string]string{}
	}
	manifest.Filename = filename
	manifest.TotalSize = total
	manifest.Chunks = upsertResumableChunk(manifest.Chunks, resumableChunk{Start: start, End: end, SHA256: chunkHash})
	manifest.CompletedBytes = completedContiguousBytes(manifest.Chunks)
	if manifest.CompletedBytes > 0 {
		manifest.CompletedRange = fmt.Sprintf("bytes=0-%d", manifest.CompletedBytes-1)
	} else {
		manifest.CompletedRange = ""
	}
	if manifest.ChunkHashes == nil {
		manifest.ChunkHashes = map[string]string{}
	}
	manifest.ChunkHashes[fmt.Sprintf("%d-%d", start, end)] = chunkHash
	manifest.UpdatedAt = time.Now().Format(time.RFC3339)
}

func cleanupResumableUploads(resumeDir string, maxAge time.Duration) {
	entries, err := os.ReadDir(resumeDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".part") && !strings.HasSuffix(name, ".json") {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(resumeDir, name))
		}
	}
}

func upsertResumableChunk(chunks []resumableChunk, next resumableChunk) []resumableChunk {
	for i, chunk := range chunks {
		if chunk.Start == next.Start && chunk.End == next.End {
			chunks[i] = next
			return chunks
		}
	}
	return append(chunks, next)
}

func completedContiguousBytes(chunks []resumableChunk) int64 {
	if len(chunks) == 0 {
		return 0
	}
	ordered := append([]resumableChunk(nil), chunks...)
	sort.Slice(ordered, func(i int, j int) bool {
		if ordered[i].Start == ordered[j].Start {
			return ordered[i].End < ordered[j].End
		}
		return ordered[i].Start < ordered[j].Start
	})

	var nextByte int64
	for _, chunk := range ordered {
		if chunk.Start > nextByte {
			break
		}
		if chunk.End >= nextByte {
			nextByte = chunk.End + 1
		}
	}
	return nextByte
}

func parseContentRange(header string) (start int64, end int64, total int64, err error) {
	matches := contentRangePattern.FindStringSubmatch(strings.TrimSpace(header))
	if matches == nil {
		return 0, 0, 0, fmt.Errorf("Invalid Content-Range; expected bytes start-end/total")
	}
	start, _ = strconv.ParseInt(matches[1], 10, 64)
	end, _ = strconv.ParseInt(matches[2], 10, 64)
	total, _ = strconv.ParseInt(matches[3], 10, 64)
	if start < 0 || end < start || total <= end {
		return 0, 0, 0, fmt.Errorf("Invalid Content-Range bounds")
	}
	return start, end, total, nil
}

func isSafeUploadID(uploadID string) bool {
	if uploadID == "" || len(uploadID) > 128 {
		return false
	}
	for _, ch := range uploadID {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' {
			continue
		}
		return false
	}
	return true
}
