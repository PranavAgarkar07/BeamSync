package beamsync

import (
	"bytes"
	"strings"
	"testing"
)

func TestCopyChunkedCopiesDataAndReportsCount(t *testing.T) {
	payload := strings.Repeat("beam-sync", 1024)
	var dst bytes.Buffer

	n, err := copyChunked(&dst, strings.NewReader(payload), 17)
	if err != nil {
		t.Fatalf("copyChunked returned error: %v", err)
	}
	if n != int64(len(payload)) {
		t.Fatalf("copyChunked bytes = %d, want %d", n, len(payload))
	}
	if dst.String() != payload {
		t.Fatal("copyChunked did not preserve payload")
	}

	t.Run("empty", func(t *testing.T) {
		var dst bytes.Buffer

		n, err := copyChunked(&dst, strings.NewReader(""), 17)
		if err != nil {
			t.Fatalf("copyChunked returned error: %v", err)
		}
		if n != 0 {
			t.Fatalf("copyChunked bytes = %d, want 0", n)
		}
		if dst.Len() != 0 {
			t.Fatalf("copyChunked wrote %d bytes to the destination, want 0", dst.Len())
		}
	})

	t.Run("shorter-than-chunk", func(t *testing.T) {
		payload := "beam"
		var dst bytes.Buffer

		n, err := copyChunked(&dst, strings.NewReader(payload), 17)
		if err != nil {
			t.Fatalf("copyChunked returned error: %v", err)
		}
		if n != int64(len(payload)) {
			t.Fatalf("copyChunked bytes = %d, want %d", n, len(payload))
		}
		if dst.String() != payload {
			t.Fatal("copyChunked did not preserve a payload shorter than the chunk")
		}
	})

	t.Run("exactly-one-chunk", func(t *testing.T) {
		payload := strings.Repeat("b", 17)
		var dst bytes.Buffer

		n, err := copyChunked(&dst, strings.NewReader(payload), 17)
		if err != nil {
			t.Fatalf("copyChunked returned error: %v", err)
		}
		if n != int64(len(payload)) {
			t.Fatalf("copyChunked bytes = %d, want %d", n, len(payload))
		}
		if dst.String() != payload {
			t.Fatal("copyChunked did not preserve a payload exactly one chunk long")
		}
	})
}
