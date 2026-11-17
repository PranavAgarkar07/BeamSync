package beamsync

import (
	"fmt"
	"sync"
	"sync/atomic"
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
