package server

import (
	"sync"
	"time"

	"stonehenge-targets/internal/config"
	"stonehenge-targets/internal/model"
)

type Tracker struct {
	mu     sync.RWMutex
	status model.Status
}

func NewTracker(mode, endpoint, sourceID string, start time.Time) *Tracker {
	return &Tracker{
		status: model.Status{
			Mode:                mode,
			DestinationEndpoint: config.RedactEndpoint(endpoint),
			SourceID:            sourceID,
			ServiceStartTime:    start.UTC(),
		},
	}
}

func (t *Tracker) Generated(externalID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.GeneratedCount++
	t.status.LastGeneratedExternalID = externalID
}

func (t *Tracker) Accepted(at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.AcceptedCount++
	ts := at.UTC()
	t.status.LastSuccessfulSubmissionTime = &ts
	t.status.LastError = ""
	t.status.PendingExternalID = ""
}

func (t *Tracker) Pending(externalID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.PendingExternalID = externalID
}

func (t *Tracker) ClearedPending() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.PendingExternalID = ""
}

func (t *Tracker) Duplicate(err string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.DuplicateCount++
	t.status.PendingExternalID = ""
	t.status.LastError = err
}

func (t *Tracker) Rejected(err string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.RejectedCount++
	t.status.PendingExternalID = ""
	t.status.LastError = err
}

func (t *Tracker) Retried(count int) {
	if count <= 1 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.RetryCount += uint64(count - 1)
}

func (t *Tracker) Failed(err string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.FailedCount++
	t.status.LastError = err
}

func (t *Tracker) HTTP503(err string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.HTTP503Count++
	t.status.LastError = err
}

func (t *Tracker) HTTP429(err string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.HTTP429Count++
	t.status.LastError = err
}

func (t *Tracker) ExpiredUnsent(externalID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.ExpiredUnsentCount++
	t.status.PendingExternalID = ""
	t.status.LastError = "target expired before successful acceptance: " + externalID
}

func (t *Tracker) Snapshot() model.Status {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}
