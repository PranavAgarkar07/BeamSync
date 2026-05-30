package beamsync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseContentRange(t *testing.T) {
	start, end, total, err := parseContentRange("bytes 0-1023/4096")
	if err != nil {
		t.Fatalf("parseContentRange returned error: %v", err)
	}
	if start != 0 || end != 1023 || total != 4096 {
		t.Fatalf("range = %d-%d/%d", start, end, total)
	}
}

func TestParseContentRangeRejectsInvalidBounds(t *testing.T) {
	for _, header := range []string{"", "bytes 9-1/10", "bytes 0-10/10", "items 0-1/2"} {
		if _, _, _, err := parseContentRange(header); err == nil {
			t.Fatalf("expected %q to be rejected", header)
		}
	}
}

func TestIsSafeUploadID(t *testing.T) {
	for _, uploadID := range []string{"abc123", "phone-1", "session_2", "retry.3"} {
		if !isSafeUploadID(uploadID) {
			t.Fatalf("expected %q to be safe", uploadID)
		}
	}
	for _, uploadID := range []string{"", "../escape", "with space", "semi;colon"} {
		if isSafeUploadID(uploadID) {
			t.Fatalf("expected %q to be unsafe", uploadID)
		}
	}
}

func TestUpdateResumableManifestTracksChunks(t *testing.T) {
	var manifest resumableManifest

	updateResumableManifest(&manifest, "upload-1", "video.mp4", 0, 1023, 4096, "abc")
	updateResumableManifest(&manifest, "upload-1", "video.mp4", 1024, 2047, 4096, "def")

	if manifest.UploadID != "upload-1" || manifest.Filename != "video.mp4" {
		t.Fatalf("manifest identity = %+v", manifest)
	}
	if manifest.TotalSize != 4096 || manifest.ChunkSize != 1024 {
		t.Fatalf("manifest sizes = %+v", manifest)
	}
	if manifest.CompletedBytes != 2048 || manifest.CompletedRange != "bytes=0-2047" {
		t.Fatalf("manifest completion = %+v", manifest)
	}
	if len(manifest.Chunks) != 2 || manifest.ChunkHashes["1024-2047"] != "def" {
		t.Fatalf("manifest chunks = %+v", manifest)
	}
}

func TestUpdateResumableManifestHandlesNonSequentialChunks(t *testing.T) {
	var manifest resumableManifest

	updateResumableManifest(&manifest, "upload-1", "video.mp4", 2048, 3071, 4096, "late")
	if manifest.CompletedBytes != 0 {
		t.Fatalf("completed bytes = %d, want 0 until first chunk arrives", manifest.CompletedBytes)
	}

	updateResumableManifest(&manifest, "upload-1", "video.mp4", 0, 1023, 4096, "first")
	if manifest.CompletedBytes != 1024 {
		t.Fatalf("completed bytes = %d, want 1024 while middle chunk is missing", manifest.CompletedBytes)
	}

	updateResumableManifest(&manifest, "upload-1", "video.mp4", 1024, 2047, 4096, "middle")
	if manifest.CompletedBytes != 3072 || manifest.CompletedRange != "bytes=0-3071" {
		t.Fatalf("manifest completion = %+v", manifest)
	}
}

func TestResumableManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upload.json")
	manifest := resumableManifest{
		UploadID:       "upload-1",
		Filename:       "file.bin",
		TotalSize:      99,
		CompletedBytes: 50,
		UpdatedAt:      time.Now().Format(time.RFC3339),
	}

	if err := saveResumableManifest(path, manifest); err != nil {
		t.Fatalf("saveResumableManifest returned error: %v", err)
	}
	loaded, err := loadResumableManifest(path)
	if err != nil {
		t.Fatalf("loadResumableManifest returned error: %v", err)
	}
	if loaded.UploadID != manifest.UploadID || loaded.CompletedBytes != manifest.CompletedBytes {
		t.Fatalf("loaded manifest = %+v", loaded)
	}
}

func TestCleanupResumableUploadsRemovesStaleFiles(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "old.part")
	fresh := filepath.Join(dir, "new.part")
	other := filepath.Join(dir, "notes.txt")

	for _, path := range []string{stale, fresh, other} {
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(stale, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes stale file: %v", err)
	}

	cleanupResumableUploads(dir, 24*time.Hour)

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expected stale part file to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("expected fresh part file to remain: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("expected unrelated file to remain: %v", err)
	}
}
