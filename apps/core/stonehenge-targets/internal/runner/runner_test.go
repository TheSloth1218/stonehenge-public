package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stonehenge-targets/internal/client"
	"stonehenge-targets/internal/config"
	"stonehenge-targets/internal/generator"
	"stonehenge-targets/internal/model"
	"stonehenge-targets/internal/server"
)

func TestContinuousModeCancellation(t *testing.T) {
	hits := make(chan struct{}, 4)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits <- struct{}{}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer target.Close()
	cfg := config.Config{
		TargetEndpoint:       target.URL,
		TargetInterval:       10 * time.Millisecond,
		TargetRequestTimeout: time.Second,
		TargetSourceID:       "SRC",
		TargetMaxRetries:     0,
		TargetMode:           "continuous",
		TargetBurstCount:     1,
	}
	tracker := server.NewTracker(cfg.TargetMode, cfg.TargetEndpoint, cfg.TargetSourceID, time.Now().UTC())
	seed := int64(1)
	run := New(cfg, generator.New(cfg.TargetSourceID, &seed), client.New(cfg.TargetEndpoint, "", time.Second, 0, 0, nil), tracker, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run.Run(ctx) }()
	select {
	case <-hits:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first generated target")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runner returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runner did not stop after cancellation")
	}
}

func TestBurstModeSubmitsConfiguredCount(t *testing.T) {
	count := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer target.Close()
	cfg := config.Config{
		TargetEndpoint:       target.URL,
		TargetInterval:       time.Second,
		TargetRequestTimeout: time.Second,
		TargetSourceID:       "SRC",
		TargetMaxRetries:     0,
		TargetMode:           "burst",
		TargetBurstCount:     3,
	}
	tracker := server.NewTracker(cfg.TargetMode, cfg.TargetEndpoint, cfg.TargetSourceID, time.Now().UTC())
	seed := int64(1)
	run := New(cfg, generator.New(cfg.TargetSourceID, &seed), client.New(cfg.TargetEndpoint, "", time.Second, 0, 0, nil), tracker, nil)
	if err := run.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if count != 3 {
		t.Fatalf("submissions = %d, want 3", count)
	}
}

func TestContinuousModePreservesUnsentTargetUntilMiddlewareRecovers(t *testing.T) {
	var externalIDs []string
	attempts := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var track model.FragmentTrack
		if err := json.NewDecoder(r.Body).Decode(&track); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		externalIDs = append(externalIDs, track.ExternalID)
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer target.Close()
	cfg := config.Config{
		TargetEndpoint:       target.URL,
		TargetInterval:       10 * time.Millisecond,
		TargetRequestTimeout: time.Second,
		TargetSourceID:       "SRC",
		TargetMaxRetries:     0,
		TargetRetryDelay:     time.Millisecond,
		TargetRetryMaxDelay:  5 * time.Millisecond,
		TargetMode:           "continuous",
		TargetBurstCount:     1,
	}
	tracker := server.NewTracker(cfg.TargetMode, cfg.TargetEndpoint, cfg.TargetSourceID, time.Now().UTC())
	seed := int64(1)
	run := New(cfg, generator.New(cfg.TargetSourceID, &seed), client.NewWithBackoff(cfg.TargetEndpoint, "", time.Second, 0, 0, 0, false, nil), tracker, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run.Run(ctx) }()
	deadline := time.After(time.Second)
	for {
		if attempts >= 3 {
			cancel()
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for recovered delivery")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runner did not stop")
	}
	if len(externalIDs) != 3 {
		t.Fatalf("external IDs = %v", externalIDs)
	}
	for _, id := range externalIDs[1:] {
		if id != externalIDs[0] {
			t.Fatalf("unsent target was replaced across retries: %v", externalIDs)
		}
	}
}

func TestContinuousMode503RetryAfterRetainsTarget(t *testing.T) {
	var externalIDs []string
	attempts := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var track model.FragmentTrack
		if err := json.NewDecoder(r.Body).Decode(&track); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		externalIDs = append(externalIDs, track.ExternalID)
		if attempts == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer target.Close()
	cfg := continuousTestConfig(target.URL)
	cfg.TargetTTL = 5 * time.Second
	run, tracker := continuousTestRunner(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- run.Run(ctx) }()
	waitUntil(t, time.Second+1500*time.Millisecond, func() bool { return attempts >= 2 })
	cancel()
	if time.Since(start) < 900*time.Millisecond {
		t.Fatalf("Retry-After delay was not applied")
	}
	waitDone(t, done)
	if len(externalIDs) != 2 || externalIDs[0] != externalIDs[1] {
		t.Fatalf("target changed across Retry-After retry: %v", externalIDs)
	}
	if tracker.Snapshot().HTTP503Count != 1 {
		t.Fatalf("503 count = %d, want 1", tracker.Snapshot().HTTP503Count)
	}
}

func TestContinuousMode429QueueFullRetainsTarget(t *testing.T) {
	var externalIDs []string
	attempts := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var track model.FragmentTrack
		if err := json.NewDecoder(r.Body).Decode(&track); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		externalIDs = append(externalIDs, track.ExternalID)
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"target_queue_full","state":{"queued_target_count":25}}`))
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer target.Close()
	cfg := continuousTestConfig(target.URL)
	cfg.QueueFullBackoff = 5 * time.Millisecond
	run, tracker := continuousTestRunner(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run.Run(ctx) }()
	waitUntil(t, time.Second, func() bool { return tracker.Snapshot().AcceptedCount == 1 })
	cancel()
	waitDone(t, done)
	if len(externalIDs) != 2 || externalIDs[0] != externalIDs[1] {
		t.Fatalf("target changed across queue-full retry: %v", externalIDs)
	}
	if tracker.Snapshot().HTTP429Count != 1 {
		t.Fatalf("429 count = %d, want 1", tracker.Snapshot().HTTP429Count)
	}
}

func TestContinuousModeTransientConnectionFailureRetainsTarget(t *testing.T) {
	var externalIDs []string
	attempts := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var track model.FragmentTrack
		if err := json.NewDecoder(r.Body).Decode(&track); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		externalIDs = append(externalIDs, track.ExternalID)
		if attempts == 1 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("response writer cannot hijack")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("hijack: %v", err)
			}
			_ = conn.Close()
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer target.Close()
	cfg := continuousTestConfig(target.URL)
	cfg.TransientErrorBackoff = 5 * time.Millisecond
	run, tracker := continuousTestRunner(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run.Run(ctx) }()
	waitUntil(t, time.Second, func() bool { return tracker.Snapshot().AcceptedCount == 1 })
	cancel()
	waitDone(t, done)
	if len(externalIDs) != 2 || externalIDs[0] != externalIDs[1] {
		t.Fatalf("target changed across transient retry: %v", externalIDs)
	}
	status := tracker.Snapshot()
	if status.AcceptedCount != 1 || status.RetryCount == 0 {
		t.Fatalf("unexpected status after transient retry: %+v", status)
	}
}

func TestContinuousModeDuplicateIsTerminal(t *testing.T) {
	var externalIDs []string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var track model.FragmentTrack
		if err := json.NewDecoder(r.Body).Decode(&track); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		externalIDs = append(externalIDs, track.ExternalID)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"duplicate":true,"feed":{"state":"completed","duplicate":true}}`))
	}))
	defer target.Close()
	cfg := continuousTestConfig(target.URL)
	cfg.TargetInterval = 20 * time.Millisecond
	run, tracker := continuousTestRunner(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run.Run(ctx) }()
	waitUntil(t, time.Second, func() bool { return len(externalIDs) >= 2 })
	cancel()
	waitDone(t, done)
	if externalIDs[0] == externalIDs[1] {
		t.Fatalf("duplicate external ID was reused: %v", externalIDs)
	}
	if tracker.Snapshot().DuplicateCount == 0 {
		t.Fatalf("duplicate was not recorded: %+v", tracker.Snapshot())
	}
}

func TestContinuousModeExpiresUnsentTarget(t *testing.T) {
	var externalIDs []string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var track model.FragmentTrack
		if err := json.NewDecoder(r.Body).Decode(&track); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		externalIDs = append(externalIDs, track.ExternalID)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer target.Close()
	cfg := continuousTestConfig(target.URL)
	cfg.TargetTTL = time.Millisecond
	cfg.TargetRetryDelay = 5 * time.Millisecond
	cfg.TargetRetryMaxDelay = 5 * time.Millisecond
	run, tracker := continuousTestRunner(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run.Run(ctx) }()
	waitUntil(t, time.Second, func() bool { return tracker.Snapshot().ExpiredUnsentCount >= 1 })
	cancel()
	waitDone(t, done)
	if len(externalIDs) != 1 {
		t.Fatalf("expired target attempts = %d, want 1", len(externalIDs))
	}
}

func TestContinuousModeGracefulShutdownRecordsPendingTarget(t *testing.T) {
	attempts := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer target.Close()
	cfg := continuousTestConfig(target.URL)
	cfg.TargetRetryDelay = time.Second
	run, tracker := continuousTestRunner(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run.Run(ctx) }()
	waitUntil(t, time.Second, func() bool { return attempts >= 1 })
	cancel()
	waitDone(t, done)
	if tracker.Snapshot().PendingExternalID == "" {
		t.Fatalf("pending target was not recorded on shutdown: %+v", tracker.Snapshot())
	}
}

func continuousTestConfig(endpoint string) config.Config {
	return config.Config{
		TargetEndpoint:        endpoint,
		TargetInterval:        5 * time.Millisecond,
		TargetRequestTimeout:  time.Second,
		TargetSourceID:        "SRC",
		TargetMaxRetries:      0,
		TargetRetryDelay:      5 * time.Millisecond,
		TargetRetryMaxDelay:   5 * time.Millisecond,
		TargetMode:            "continuous",
		TargetBurstCount:      1,
		TargetTTL:             time.Second,
		QueueFullBackoff:      5 * time.Millisecond,
		TransientErrorBackoff: 5 * time.Millisecond,
	}
}

func continuousTestRunner(cfg config.Config) (*Runner, *server.Tracker) {
	tracker := server.NewTracker(cfg.TargetMode, cfg.TargetEndpoint, cfg.TargetSourceID, time.Now().UTC())
	seed := int64(1)
	run := New(cfg, generator.New(cfg.TargetSourceID, &seed), client.NewWithBackoff(cfg.TargetEndpoint, "", time.Second, 0, 0, 0, false, nil), tracker, nil)
	return run, tracker
}

func waitUntil(t *testing.T, timeout time.Duration, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func waitDone(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runner did not stop")
	}
}
