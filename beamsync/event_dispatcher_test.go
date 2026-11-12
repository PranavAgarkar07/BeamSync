package beamsync

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventDispatcherDeliversNormalEmit(t *testing.T) {
	t.Run("single emit delivers", func(t *testing.T) {
		dispatcher := newEventDispatcher(4)
		t.Cleanup(dispatcher.close)
		delivered := make(chan string, 1)
		dispatcher.emit(eventDispatchJob{
			emit:  func(eventName string, data string) { delivered <- eventName + "|" + data },
			event: "device_connected",
			data:  "peer-a",
		})

		select {
		case got := <-delivered:
			if got != "device_connected|peer-a" {
				t.Fatalf("delivered event = %q, want %q", got, "device_connected|peer-a")
			}
		case <-time.After(time.Second):
			t.Fatal("eventDispatcher did not deliver the emitted event within 1s")
		}
	})

	t.Run("emits delivered in FIFO order", func(t *testing.T) {
		dispatcher := newEventDispatcher(4)
		t.Cleanup(dispatcher.close)
		const deliveries = 3
		delivered := make(chan string, deliveries)
		emit := func(eventName string, data string) { delivered <- eventName + "|" + data }
		dispatcher.emit(eventDispatchJob{emit: emit, event: "evt-alpha", data: "one"})
		dispatcher.emit(eventDispatchJob{emit: emit, event: "evt-beta", data: "two"})
		dispatcher.emit(eventDispatchJob{emit: emit, event: "evt-gamma", data: "three"})

		want := []string{"evt-alpha|one", "evt-beta|two", "evt-gamma|three"}
		for i := 0; i < deliveries; i++ {
			select {
			case got := <-delivered:
				if got != want[i] {
					t.Fatalf("delivery %d = %q, want %q", i, got, want[i])
				}
			case <-time.After(time.Second):
				t.Fatalf("timed out waiting for delivery %d", i)
			}
		}
	})

	t.Run("emits duplicates separately", func(t *testing.T) {
		dispatcher := newEventDispatcher(4)
		t.Cleanup(dispatcher.close)
		delivered := make(chan string, 2)
		emit := func(eventName string, data string) { delivered <- eventName + "|" + data }
		dispatcher.emit(eventDispatchJob{emit: emit, event: "ping", data: "x"})
		dispatcher.emit(eventDispatchJob{emit: emit, event: "ping", data: "x"})

		for i := 0; i < 2; i++ {
			select {
			case got := <-delivered:
				if got != "ping|x" {
					t.Fatalf("delivery %d = %q, want %q", i, got, "ping|x")
				}
			case <-time.After(time.Second):
				t.Fatalf("timed out waiting for duplicate delivery %d", i)
			}
		}
	})
}

// startBlockingEventDispatcher hands back a dispatcher whose worker goroutine
// is provably blocked inside a callback: the callback closes `entered` (signalled
// by the dispatcher itself once it dequeues the primer) and then blocks on
// `release`. The queue is therefore empty and undrainable while the caller holds
// it — deterministic drop accounting without scheduler luck.
func startBlockingEventDispatcher(t *testing.T, bufferSize int) (*eventDispatcher, chan struct{}) {
	t.Helper()
	dispatcher := newEventDispatcher(bufferSize)
	t.Cleanup(dispatcher.close)
	entered := make(chan struct{})
	release := make(chan struct{})
	dispatcher.emit(eventDispatchJob{
		emit: func(eventName string, data string) {
			close(entered)
			<-release
		},
		event: "primer",
		data:  "block",
	})
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatalf("eventDispatcher worker did not start processing (bufferSize=%d)", bufferSize)
	}
	return dispatcher, release
}

func TestEventDispatcherEmptyBufferFallsBack(t *testing.T) {
	for _, bufferSize := range []int{0, -1} {
		t.Run(fmt.Sprintf("buffer size %d falls back to eventDispatcherBufferSize", bufferSize), func(t *testing.T) {
			dispatcher, release := startBlockingEventDispatcher(t, bufferSize)

			for i := 0; i < eventDispatcherBufferSize; i++ {
				if !dispatcher.emit(eventDispatchJob{
					emit:  func(eventName string, data string) {},
					event: "evt",
					data:  "payload",
				}) {
					t.Fatalf("emit %d of %d through buffer size %d = false, want true", i, eventDispatcherBufferSize, bufferSize)
				}
			}
			if dispatcher.emit(eventDispatchJob{
				emit:  func(eventName string, data string) {},
				event: "evt",
				data:  "payload",
			}) {
				t.Fatalf("emit %d of %d through buffer size %d = true, want false", eventDispatcherBufferSize, eventDispatcherBufferSize, bufferSize)
			}
			if got := dispatcher.DroppedCount(); got != 1 {
				t.Fatalf("DroppedCount() = %d, want 1", got)
			}
			close(release)
		})
	}
}

func TestEventDispatcherDropsFromFullBuffer(t *testing.T) {
	for _, bufferSize := range []int{1, 2} {
		t.Run(fmt.Sprintf("capacity %d drops from full buffer", bufferSize), func(t *testing.T) {
			dispatcher, release := startBlockingEventDispatcher(t, bufferSize)

			for i := 0; i < bufferSize; i++ {
				if !dispatcher.emit(eventDispatchJob{
					emit:  func(eventName string, data string) {},
					event: "evt",
					data:  "payload",
				}) {
					t.Fatalf("emit %d of %d with capacity %d = false, want true", i, bufferSize, bufferSize)
				}
			}
			if dispatcher.emit(eventDispatchJob{
				emit:  func(eventName string, data string) {},
				event: "evt",
				data:  "payload",
			}) {
				t.Fatalf("emit capacity+1 (%d) with capacity %d = true, want false", bufferSize+1, bufferSize)
			}
			if got := dispatcher.DroppedCount(); got != 1 {
				t.Fatalf("DroppedCount() = %d, want 1", got)
			}
			close(release)
		})
	}
}

func TestEventDispatcherSurvivesCallbackPanic(t *testing.T) {
	dispatcher := newEventDispatcher(8)
	t.Cleanup(dispatcher.close)
	dispatcher.emit(eventDispatchJob{
		emit:  func(eventName string, data string) { panic("boom") },
		event: "boom",
		data:  "payload",
	})

	delivered := make(chan string, 1)
	dispatcher.emit(eventDispatchJob{
		emit:  func(eventName string, data string) { delivered <- eventName + "|" + data },
		event: "later",
		data:  "ok",
	})

	select {
	case got := <-delivered:
		if got != "later|ok" {
			t.Fatalf("delivered event = %q, want %q", got, "later|ok")
		}
	case <-time.After(time.Second):
		t.Fatal("eventDispatcher did not deliver the event after the panicking callback")
	}
}

func TestEventDispatcherConcurrentEmitsExactDropAccounting(t *testing.T) {
	const (
		emitters    = 4
		emitPerEmit = 25
	)
	const total = emitters * emitPerEmit

	dispatcher := newEventDispatcher(8)
	t.Cleanup(dispatcher.close)
	delivered := make(chan string, total)
	var failed atomic.Int64

	var wg sync.WaitGroup
	for g := 0; g < emitters; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < emitPerEmit; i++ {
				if dispatcher.emit(eventDispatchJob{
					emit:  func(eventName string, data string) { delivered <- eventName + "|" + data },
					event: "evt",
					data:  "payload",
				}) {
					continue
				}
				failed.Add(1)
			}
		}()
	}
	wg.Wait()

	received := 0
	deadline := time.Now().Add(5 * time.Second)
	for int64(received)+failed.Load() < int64(total) {
		select {
		case <-delivered:
			received++
		case <-time.After(50 * time.Millisecond):
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for deliveries: received %d, failed %d, want %d", received, failed.Load(), total)
			}
		}
	}

	if got := dispatcher.DroppedCount(); got != failed.Load() {
		t.Fatalf("DroppedCount() = %d, want %d (failed enqueues)", got, failed.Load())
	}
}
