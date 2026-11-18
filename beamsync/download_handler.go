package beamsync

import (
	"bufio"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// handleDownload streams filePath to the client as an attachment. A single
// method serves both the /download route and the indexed /download/N routes:
// the two bodies were behaviorally identical apart from their base-name local
// (CONTEXT D-12), so one implementation keeps them from drifting apart.
func (s *HTTPServer) handleDownload(filePath string, w http.ResponseWriter, r *http.Request) {
	realName := filepath.Base(filePath)
	activeDownloads := s.state.beginUpload()
	s.emit("transfer_stats", transferStatsJSON(s.stats.snapshotDownloads(activeDownloads)))
	defer func() {
		activeDownloads := s.state.endUpload()
		s.emit("transfer_stats", transferStatsJSON(s.stats.snapshotDownloads(activeDownloads)))
	}()
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, realName))
	w.Header().Set("X-Filename", realName)
	w.Header().Set("Access-Control-Expose-Headers", "X-Filename")

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		http.Error(w, "Failed to stat file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	mimeType := mime.TypeByExtension(filepath.Ext(realName))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mimeType)

	startedAt := time.Now()
	bufReader := bufio.NewReaderSize(file, 8*1024*1024)
	pw := &progressWriter{
		w:           w,
		total:       fileInfo.Size(),
		filename:    realName,
		eventName:   "download_progress",
		emit:        s.emit,
		minInterval: 500 * time.Millisecond,
		now:         time.Now,
	}
	written, copyErr := copyChunked(pw, bufReader, 8*1024*1024)
	status := TransferStatusCompleted
	errMsg := ""
	if copyErr != nil {
		status = TransferStatusFailed
		errMsg = copyErr.Error()
	} else {
		// Completion event. Without it a fast transfer (finished inside the
		// 500ms throttle window) emits NO progress event at all, so the
		// desktop send-direction panel would never move.
		s.emit(pw.eventName, fmt.Sprintf("%s|%d|%d", realName, written, written))
		snapshot := s.stats.recordSent(realName, written, s.state.activeUploads())
		s.emit("transfer_stats", transferStatsJSON(snapshot))
	}
	logTransfer(s.history, s.emit, TransferRecord{
		Filename:  realName,
		Direction: TransferDirectionSend,
		Status:    status,
		SizeBytes: written,
		StartedAt: startedAt,
		Error:     errMsg,
	})
}
