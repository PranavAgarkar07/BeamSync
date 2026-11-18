package beamsync

import (
	"fmt"
	"io"
	"time"
)

// progressWriter wraps the destination of a transfer and emits a progress event
// as bytes flow through it.
//
// Single-writer type: Write mutates written/lastEmit and is NOT safe for
// concurrent use. Each transfer path owns exactly one progressWriter and writes
// to it from a single goroutine (receive path and each send path).
type progressWriter struct {
	w           io.Writer
	total       int64
	written     int64
	filename    string
	eventName   string
	emit        func(string, string)
	lastEmit    time.Time
	minInterval time.Duration
	now         func() time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p) //nolint:errcheck,gosec
	if n == 0 {
		// A zero-byte write transfers nothing; emitting progress for it would
		// report "0 bytes written" as a real update.
		return n, err
	}
	pw.written += int64(n)
	clock := time.Now
	if pw.now != nil {
		clock = pw.now
	}
	now := clock()
	if now.Sub(pw.lastEmit) >= pw.minInterval {
		pw.emit(pw.eventName, fmt.Sprintf("%s|%d|%d", pw.filename, pw.written, pw.total))
		pw.lastEmit = now
	}
	return n, err
}
