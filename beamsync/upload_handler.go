package beamsync

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"
)

func (s *HTTPServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in upload handler: %v\n%s\n", r, debug.Stack())
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Accept-Ranges", "bytes")
	s.state.markHeartbeat()
	r.Body = http.MaxBytesReader(w, r.Body, 100*1024*1024*1024)

	contentType := r.Header.Get("Content-Type")
	mediaType, params, ctErr := mime.ParseMediaType(contentType)
	if ctErr != nil || !strings.HasPrefix(mediaType, "multipart/") {
		http.Error(w, "Expected multipart/form-data", http.StatusBadRequest)
		return
	}
	boundary := params["boundary"]

	netReader := bufio.NewReaderSize(r.Body, 8*1024*1024)
	mr := multipart.NewReader(netReader, boundary)

	// The manifest is matched to parts in ORDER, not by name alone: two
	// parts may share a base name (a/report.txt and b/report.txt), and
	// keying by base name alone would report one file's size for the
	// other. Each entry is consumed by the first part whose base name
	// matches.
	type manifestEntry struct {
		name string
		size int64
		used bool
	}
	var manifestEntries []manifestEntry
	fileCount := 0
	var parseErr error

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			parseErr = err
			break
		}

		formName := part.FormName()
		filename := part.FileName()

		if formName == "beam_manifest" && filename == "" {
			var manifest []struct {
				Name string `json:"name"`
				Size int64  `json:"size"`
			}
			if err := json.NewDecoder(part).Decode(&manifest); err == nil {
				for _, f := range manifest {
					manifestEntries = append(manifestEntries, manifestEntry{name: f.Name, size: f.Size})
				}
			}
			part.Close()
			continue
		}

		if filename == "" {
			part.Close()
			continue
		}

		fileCount++
		rawName := filepath.Base(filename)
		if rawName == "" || rawName == "." {
			rawName = fmt.Sprintf("upload_%d.bin", time.Now().Unix())
		}

		dstPath := autoRenamePath(s.uploadDir, rawName)
		savedName := filepath.Base(dstPath)

		dst, createErr := os.Create(dstPath)
		if createErr != nil {
			_, _ = io.Copy(io.Discard, part) //nolint:errcheck,gosec
			part.Close()
			continue
		}

		diskBuf := bufio.NewWriterSize(dst, 8*1024*1024)
		// The manifest carries each file's declared size. It was parsed
		// above and then discarded, which left progressWriter.total at 0
		// and made the desktop's upload progress bar unable to compute a
		// percentage. Match the entry by base name, consuming it so a
		// later part with the same base name gets its OWN size.
		var declaredSize int64
		for i := range manifestEntries {
			if manifestEntries[i].used {
				continue
			}
			if filepath.Base(manifestEntries[i].name) != rawName {
				continue
			}
			declaredSize = manifestEntries[i].size
			manifestEntries[i].used = true
			break
		}
		pw := &progressWriter{
			w:           diskBuf,
			total:       declaredSize,
			filename:    savedName,
			eventName:   "upload_progress",
			emit:        s.emit,
			minInterval: 500 * time.Millisecond,
			now:         time.Now,
		}

		startedAt := time.Now()
		s.state.beginUpload()

		// ponytail: no integrity hashing on upload path - adds complexity for LAN transfer where corruption is near-zero. Add if transfer integrity issues reported.
		written, copyErr := copyChunked(pw, part, 8*1024*1024)

		flushErr := diskBuf.Flush()
		dst.Close()
		part.Close()
		s.state.endUpload()

		var transferErr error
		switch {
		case copyErr != nil:
			transferErr = copyErr
		case flushErr != nil:
			transferErr = flushErr
		}

		if transferErr != nil {
			logTransfer(s.history, s.emit, TransferRecord{
				Filename:  savedName,
				Direction: TransferDirectionReceive,
				Status:    TransferStatusFailed,
				SizeBytes: written,
				StartedAt: startedAt,
				Error:     transferErr.Error(),
			})
			continue
		}

		s.emit(pw.eventName, fmt.Sprintf("%s|%d|%d", savedName, written, written))
		snapshot := s.stats.recordReceived(savedName, written, s.state.activeUploads())
		s.emit("transfer_stats", transferStatsJSON(snapshot))

		logTransfer(s.history, s.emit, TransferRecord{
			Filename:  savedName,
			Direction: TransferDirectionReceive,
			Status:    TransferStatusCompleted,
			SizeBytes: written,
			StartedAt: startedAt,
		})

		go func(fname string) {
			time.Sleep(100 * time.Millisecond)
			s.emit("file_received", fname)
		}(savedName)
	}

	if parseErr != nil {
		http.Error(w, "Multipart read error", http.StatusBadRequest)
		return
	}

	if fileCount == 0 {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Upload Complete")) //nolint:errcheck,gosec
}
