package beamsync

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var contentRangePattern = regexp.MustCompile(`^bytes (\d+)-(\d+)/(\d+)$`)

func handleResumableUpload(uploadDir string, state *serverState, emit func(string, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
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

		tempPath := filepath.Join(resumeDir, uploadID+".part")
		file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			http.Error(w, "Could not open resumable upload", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		if current, err := file.Seek(0, io.SeekEnd); err == nil && start > current {
			http.Error(w, fmt.Sprintf("Chunk starts at %d but upload has %d bytes", start, current), http.StatusConflict)
			return
		}
		if _, err := file.Seek(start, io.SeekStart); err != nil {
			http.Error(w, "Could not seek resumable upload", http.StatusInternalServerError)
			return
		}

		state.beginUpload()
		defer state.endUpload()
		state.markHeartbeat()

		writer := bufio.NewWriterSize(file, 8*1024*1024)
		written, err := copyChunked(writer, r.Body, 8*1024*1024)
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

		received := end + 1
		emit("upload_progress", fmt.Sprintf("%s|%d|%d", filename, received, total))

		if received < total {
			w.Header().Set("Range", fmt.Sprintf("bytes=0-%d", end))
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("chunk accepted"))
			return
		}

		finalPath := autoRenamePath(uploadDir, filename)
		if err := os.Rename(tempPath, finalPath); err != nil {
			http.Error(w, "Could not finalize resumable upload", http.StatusInternalServerError)
			return
		}

		savedName := filepath.Base(finalPath)
		emit("upload_progress", fmt.Sprintf("%s|%d|%d", savedName, total, total))
		go func(fname string) {
			time.Sleep(100 * time.Millisecond)
			emit("file_received", fname)
		}(savedName)

		fmt.Printf("resumable upload completed: %s (%d bytes, active=%d)\n", savedName, total, atomic.LoadInt32(&state.uploadingCount))
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("upload complete"))
	}
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
