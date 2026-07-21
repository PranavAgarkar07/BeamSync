package beamsync

import (
	"fmt"
	"sync"
	"time"
)

const defaultTransferHistoryLimit = 100

const (
	TransferDirectionReceive = "receive"
	TransferDirectionSend    = "send"

	TransferStatusCompleted = "completed"
	TransferStatusFailed    = "failed"
)

// TransferRecord captures one file transfer attempt for the current app session.
type TransferRecord struct {
	ID             string    `json:"id"`
	Filename       string    `json:"filename"`
	Direction      string    `json:"direction"`
	Status         string    `json:"status"`
	SizeBytes      int64     `json:"sizeBytes"`
	StartedAt      time.Time `json:"startedAt"`
	CompletedAt    time.Time `json:"completedAt"`
	DurationMillis int64     `json:"durationMillis"`
	Error          string    `json:"error,omitempty"`
}

// TransferHistory stores the most recent transfer records for one server session.
type TransferHistory struct {
	mu         sync.Mutex
	maxEntries int
	nextID     uint64
	entries    []TransferRecord
	start      int
	count      int
}

func NewTransferHistory(maxEntries int) *TransferHistory {
	if maxEntries <= 0 {
		maxEntries = defaultTransferHistoryLimit
	}
	return &TransferHistory{
		maxEntries: maxEntries,
		entries:    make([]TransferRecord, maxEntries),
	}
}

func (h *TransferHistory) Add(record TransferRecord) TransferRecord {
	if h == nil {
		return record
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.maxEntries <= 0 {
		h.maxEntries = defaultTransferHistoryLimit
	}

	h.nextID++
	if record.ID == "" {
		record.ID = fmt.Sprintf("transfer-%d-%d", time.Now().UnixNano(), h.nextID)
	}
	if record.CompletedAt.IsZero() {
		record.CompletedAt = time.Now()
	}
	if record.StartedAt.IsZero() {
		record.StartedAt = record.CompletedAt
	}
	if record.DurationMillis == 0 {
		record.DurationMillis = record.CompletedAt.Sub(record.StartedAt).Milliseconds()
	}

	if len(h.entries) == 0 {
		h.entries = make([]TransferRecord, h.maxEntries)
	}

	h.start = (h.start - 1 + h.maxEntries) % h.maxEntries
	h.entries[h.start] = record
	if h.count < h.maxEntries {
		h.count++
	}
	return record
}

func (h *TransferHistory) List() []TransferRecord {
	if h == nil {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	records := make([]TransferRecord, h.count)
	for i := 0; i < h.count; i++ {
		records[i] = h.entries[(h.start+i)%h.maxEntries]
	}
	return records
}
