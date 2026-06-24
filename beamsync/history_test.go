package beamsync

import (
	"fmt"
	"testing"
	"time"
)

func TestTransferHistoryStoresNewestFirstAndCapsEntries(t *testing.T) {
	history := NewTransferHistory(2)

	first := history.Add(TransferRecord{Filename: "one.txt", Direction: TransferDirectionReceive, Status: TransferStatusCompleted})
	if first.ID == "" {
		t.Fatal("expected generated record ID")
	}

	history.Add(TransferRecord{Filename: "two.txt", Direction: TransferDirectionSend, Status: TransferStatusCompleted})
	history.Add(TransferRecord{Filename: "three.txt", Direction: TransferDirectionReceive, Status: TransferStatusFailed})

	records := history.List()
	if len(records) != 2 {
		t.Fatalf("expected capped history length 2, got %d", len(records))
	}
	if records[0].Filename != "three.txt" || records[1].Filename != "two.txt" {
		t.Fatalf("expected newest-first records, got %#v", records)
	}
}

func TestTransferHistoryCopiesEntries(t *testing.T) {
	history := NewTransferHistory(10)
	history.Add(TransferRecord{Filename: "file.txt", Direction: TransferDirectionReceive, Status: TransferStatusCompleted})

	records := history.List()
	records[0].Filename = "mutated.txt"

	freshRecords := history.List()
	if freshRecords[0].Filename != "file.txt" {
		t.Fatalf("expected history to return a defensive copy, got %q", freshRecords[0].Filename)
	}
}

func TestTransferHistoryRingBufferWrapsNewestFirst(t *testing.T) {
	history := NewTransferHistory(3)

	for i := 1; i <= 6; i++ {
		history.Add(TransferRecord{
			Filename:  fmt.Sprintf("file-%d.txt", i),
			Direction: TransferDirectionReceive,
			Status:    TransferStatusCompleted,
		})
	}

	records := history.List()
	if len(records) != 3 {
		t.Fatalf("expected capped history length 3, got %d", len(records))
	}

	want := []string{"file-6.txt", "file-5.txt", "file-4.txt"}
	for i, filename := range want {
		if records[i].Filename != filename {
			t.Fatalf("record %d filename = %q, want %q; records=%#v", i, records[i].Filename, filename, records)
		}
	}
}

func TestTransferHistoryFillsTimingMetadata(t *testing.T) {
	history := NewTransferHistory(10)
	startedAt := time.Now().Add(-2 * time.Second)

	record := history.Add(TransferRecord{
		Filename:  "timed.txt",
		StartedAt: startedAt,
		Status:    TransferStatusCompleted,
	})

	if record.CompletedAt.IsZero() {
		t.Fatal("expected completed timestamp")
	}
	if record.DurationMillis <= 0 {
		t.Fatalf("expected positive duration, got %d", record.DurationMillis)
	}
}

func TestTransferHistoryExactCapacity(t *testing.T) {
	history := NewTransferHistory(3)
	for i := 1; i <= 3; i++ {
		history.Add(TransferRecord{Filename: fmt.Sprintf("file-%d.txt", i)})
	}
	records := history.List()
	if len(records) != 3 {
		t.Fatalf("expected length 3, got %d", len(records))
	}
}

func TestTransferHistoryWrapAround(t *testing.T) {
	history := NewTransferHistory(3)
	for i := 1; i <= 4; i++ {
		history.Add(TransferRecord{Filename: fmt.Sprintf("file-%d.txt", i)})
	}
	records := history.List()
	if len(records) != 3 {
		t.Fatalf("expected length 3, got %d", len(records))
	}
	if records[0].Filename != "file-4.txt" || records[1].Filename != "file-3.txt" || records[2].Filename != "file-2.txt" {
		t.Fatalf("unexpected wrap-around behavior: %v", records)
	}
}

func TestTransferHistorySingleEntry(t *testing.T) {
	history := NewTransferHistory(3)
	history.Add(TransferRecord{Filename: "single.txt"})
	records := history.List()
	if len(records) != 1 {
		t.Fatalf("expected length 1, got %d", len(records))
	}
	if records[0].Filename != "single.txt" {
		t.Fatalf("expected single.txt, got %s", records[0].Filename)
	}
}

func TestTransferHistoryEmptyState(t *testing.T) {
	history := NewTransferHistory(3)
	records := history.List()
	if records == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(records) != 0 {
		t.Fatalf("expected length 0, got %d", len(records))
	}
}

func TestTransferHistoryNilReceiver(t *testing.T) {
	var history *TransferHistory
	// Should not panic
	record := history.Add(TransferRecord{Filename: "test.txt"})
	if record.Filename != "test.txt" {
		t.Fatalf("expected returned record to match input, got %v", record)
	}
	records := history.List()
	if records != nil {
		t.Fatalf("expected nil from nil receiver List(), got %v", records)
	}
}

func TestTransferHistoryCustomMaxEntries(t *testing.T) {
	history := NewTransferHistory(1)
	history.Add(TransferRecord{Filename: "1.txt"})
	history.Add(TransferRecord{Filename: "2.txt"})
	history.Add(TransferRecord{Filename: "3.txt"})
	records := history.List()
	if len(records) != 1 {
		t.Fatalf("expected length 1, got %d", len(records))
	}
	if records[0].Filename != "3.txt" {
		t.Fatalf("expected 3.txt, got %v", records[0].Filename)
	}
}

func TestTransferHistoryZeroMaxEntries(t *testing.T) {
	history := NewTransferHistory(0)
	for i := 0; i < defaultTransferHistoryLimit+5; i++ {
		history.Add(TransferRecord{Filename: "test.txt"})
	}
	records := history.List()
	if len(records) != defaultTransferHistoryLimit {
		t.Fatalf("expected default limit %d, got %d", defaultTransferHistoryLimit, len(records))
	}
}

func TestTransferHistoryConcurrentAccess(t *testing.T) {
	history := NewTransferHistory(100)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			history.Add(TransferRecord{Filename: "concurrent.txt"})
		}
		close(done)
	}()

	for {
		select {
		case <-done:
			return
		default:
			history.List()
		}
	}
}

func TestTransferHistoryRecordIDGeneration(t *testing.T) {
	history := NewTransferHistory(10)
	rec1 := history.Add(TransferRecord{})
	if rec1.ID == "" {
		t.Fatal("expected auto-generated ID")
	}

	rec2 := history.Add(TransferRecord{ID: "custom-id"})
	if rec2.ID != "custom-id" {
		t.Fatalf("expected preserved custom-id, got %v", rec2.ID)
	}
}

func TestTransferHistoryTimestampFallback(t *testing.T) {
	history := NewTransferHistory(10)
	rec := history.Add(TransferRecord{Filename: "test.txt"})
	if rec.StartedAt.IsZero() {
		t.Fatal("expected StartedAt to be populated")
	}
	if rec.StartedAt != rec.CompletedAt {
		t.Fatalf("expected StartedAt == CompletedAt, got StartedAt=%v, CompletedAt=%v", rec.StartedAt, rec.CompletedAt)
	}
}

func BenchmarkTransferHistoryAddAtCapacity(b *testing.B) {
	history := NewTransferHistory(100)
	for i := 0; i < 100; i++ {
		history.Add(TransferRecord{
			Filename:  fmt.Sprintf("warmup-%d.txt", i),
			Direction: TransferDirectionReceive,
			Status:    TransferStatusCompleted,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		history.Add(TransferRecord{
			Filename:  "file.txt",
			Direction: TransferDirectionReceive,
			Status:    TransferStatusCompleted,
		})
	}
}
