package beamsync

import (
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
