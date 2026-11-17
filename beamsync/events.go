package beamsync

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type EventCallback func(eventName string, data string)

const eventDispatcherBufferSize = 1024

type eventDispatchJob struct {
	emit  EventCallback
	event string
	data  string
}

type eventDispatcher struct {
	queue   chan eventDispatchJob
	dropped atomic.Int64

	// stateMu guards closed. emit takes the read lock and close takes the
	// write lock, so closing the dispatcher can never race with an emit
	// (which would panic on a send to a closed channel).
	stateMu sync.RWMutex
	closed  bool
}

var defaultEventDispatcher = newEventDispatcher(eventDispatcherBufferSize)

func newEventDispatcher(bufferSize int) *eventDispatcher {
	if bufferSize <= 0 {
		bufferSize = eventDispatcherBufferSize
	}
	dispatcher := &eventDispatcher{
		queue: make(chan eventDispatchJob, bufferSize),
	}
	go dispatcher.run()
	return dispatcher
}

func (d *eventDispatcher) run() {
	for job := range d.queue {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Event callback panic: %v\n", r)
				}
			}()
			if job.emit != nil {
				job.emit(job.event, job.data)
			}
		}()
	}
}

func (d *eventDispatcher) emit(job eventDispatchJob) bool {
	if job.emit == nil {
		return true
	}
	d.stateMu.RLock()
	defer d.stateMu.RUnlock()
	if d.closed {
		return false
	}
	select {
	case d.queue <- job:
		return true
	default:
		d.dropped.Add(1)
		return false
	}
}

func (d *eventDispatcher) DroppedCount() int64 {
	return d.dropped.Load()
}

// close stops the dispatcher goroutine and makes subsequent emits no-ops.
// It is idempotent and safe to call concurrently with emit — the write lock
// excludes any in-flight emit, and later emits are rejected instead of
// panicking on a send to a closed channel.
//
// It is intended for tests and for shutdown paths: the worker ranges over
// d.queue, so without this the goroutine leaks for the lifetime of the
// process.
func (d *eventDispatcher) close() {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	if d.closed {
		return
	}
	d.closed = true
	close(d.queue)
}

func startWatchdog(ctx context.Context, state *serverState, emit func(string, string)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Watchdog panic: %v\n", r)
			}
		}()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, timedOut := state.checkTimeout()
				if timedOut {
					emit("device_disconnected", "")
				}
			}
		}
	}()
}

func startBootstrapRotator(ctx context.Context, hs *HTTPServer, port string, emit func(string, string)) {
	if hs == nil || hs.tokens == nil {
		return
	}
	interval := bootstrapTokenTTL - 30*time.Second
	if interval <= 0 {
		interval = bootstrapTokenTTL / 2
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Bootstrap rotator panic: %v\n", r)
			}
		}()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if hs.state != nil && hs.state.activeUploads() > 0 {
					// Defer rotation briefly instead of skipping a full interval
					select {
					case <-ctx.Done():
						return
					case <-time.After(30 * time.Second):
					}
					if hs.state.activeUploads() > 0 {
						continue
					}
				}
				newToken, err := hs.RotateBootstrapToken()
				if err != nil {
					fmt.Printf("Failed to rotate bootstrap token: %v\n", err)
					continue
				}
				ip := getLocalIPForURL()
				newURL := fmt.Sprintf("%s://%s:%s/?token=%s", ServerScheme(), ip, port, newToken)
				fmt.Printf("🔄 Bootstrap token rotated, new URL: %s\n", newURL)
				emit("url_changed", newURL)
				emit("token_rotated", newURL)
			}
		}
	}()
}

func safeEmit(emit EventCallback, event, data string) {
	if emit == nil {
		return
	}
	if !defaultEventDispatcher.emit(eventDispatchJob{emit: emit, event: event, data: data}) {
		fmt.Printf("Dropping event because dispatch queue is full: %s\n", event)
	}
}

func logTransfer(history *TransferHistory, emit func(string, string), record TransferRecord) {
	if history == nil {
		return
	}
	saved := history.Add(record)
	data, err := json.Marshal(saved)
	if err != nil {
		fmt.Println("Failed to marshal transfer history record:", err)
		return
	}
	emit("transfer_logged", string(data))
}
