package beamsync

import (
	"testing"
	"time"
)

func TestEventDispatcherDeliversNormalEmit(t *testing.T) {
	t.Run("single emit delivers", func(t *testing.T) {
		dispatcher := newEventDispatcher(4)
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
