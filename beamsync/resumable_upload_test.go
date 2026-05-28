package beamsync

import "testing"

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
