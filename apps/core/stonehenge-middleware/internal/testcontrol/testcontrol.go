package testcontrol

import (
	"context"
	"os"
	"sync"
	"time"
)

const (
	PointBeforeAutoSubmit  = "before_auto_submit"
	PointAfterCommit       = "after_commit"
	PointAfterShotEvidence = "after_shot_evidence"
)

type PauseRequest struct {
	Point    string `json:"point"`
	TargetID uint32 `json:"target_id,omitempty"`
}

type PauseStatus struct {
	Enabled      bool           `json:"enabled"`
	Armed        []PauseRequest `json:"armed"`
	Paused       []PausedPoint  `json:"paused"`
	LastReleased *PausedPoint   `json:"last_released,omitempty"`
}

type PausedPoint struct {
	Point        string    `json:"point"`
	TargetID     uint32    `json:"target_id"`
	AssignmentID int64     `json:"assignment_id"`
	Generation   int       `json:"generation"`
	PausedAt     time.Time `json:"paused_at"`
}

type pauseState struct {
	req      PauseRequest
	release  chan struct{}
	paused   *PausedPoint
	released bool
}

var controls = struct {
	mu           sync.Mutex
	enabled      bool
	token        string
	armed        map[string]*pauseState
	lastReleased *PausedPoint
}{
	enabled: os.Getenv("STONEHENGE_TEST_CONTROLS") == "true" || os.Getenv("STONEHENGE_TEST_CONTROLS") == "1",
	token:   os.Getenv("STONEHENGE_TEST_TOKEN"),
	armed:   make(map[string]*pauseState),
}

func Enabled() bool {
	controls.mu.Lock()
	defer controls.mu.Unlock()
	return controls.enabled
}

func TokenOK(token string) bool {
	controls.mu.Lock()
	defer controls.mu.Unlock()
	return controls.enabled && controls.token != "" && token == controls.token
}

func Arm(req PauseRequest) bool {
	controls.mu.Lock()
	defer controls.mu.Unlock()
	if !controls.enabled || req.Point == "" {
		return false
	}
	controls.armed[req.Point] = &pauseState{req: req, release: make(chan struct{})}
	return true
}

func Release(point string) bool {
	controls.mu.Lock()
	defer controls.mu.Unlock()
	state := controls.armed[point]
	if state == nil {
		return false
	}
	if state.paused != nil {
		copy := *state.paused
		controls.lastReleased = &copy
	}
	if !state.released {
		close(state.release)
		state.released = true
	}
	delete(controls.armed, point)
	return true
}

func Status() PauseStatus {
	controls.mu.Lock()
	defer controls.mu.Unlock()
	out := PauseStatus{Enabled: controls.enabled}
	for _, state := range controls.armed {
		out.Armed = append(out.Armed, state.req)
		if state.paused != nil {
			out.Paused = append(out.Paused, *state.paused)
		}
	}
	if controls.lastReleased != nil {
		copy := *controls.lastReleased
		out.LastReleased = &copy
	}
	return out
}

func Wait(ctx context.Context, point string, targetID uint32, assignmentID int64, generation int) {
	controls.mu.Lock()
	state := controls.armed[point]
	if !controls.enabled || state == nil || (state.req.TargetID != 0 && state.req.TargetID != targetID) {
		controls.mu.Unlock()
		return
	}
	if state.paused == nil {
		state.paused = &PausedPoint{
			Point:        point,
			TargetID:     targetID,
			AssignmentID: assignmentID,
			Generation:   generation,
			PausedAt:     time.Now().UTC(),
		}
	}
	release := state.release
	controls.mu.Unlock()

	select {
	case <-ctx.Done():
	case <-release:
	}
}
