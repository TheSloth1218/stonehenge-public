package targetfeed

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"stonehenge-middleware/internal/autocontrol"
)

func TestDuplicateIngestDoesNotRequeue(t *testing.T) {
	mgr := New(testConfig(), nil, nil)
	target := testTarget(10, 50, time.Now().UTC())
	if _, err := mgr.Ingest(context.Background(), target); err != nil {
		t.Fatalf("Ingest first: %v", err)
	}
	result, err := mgr.Ingest(context.Background(), target)
	if err != nil {
		t.Fatalf("Ingest duplicate: %v", err)
	}
	if !result.Duplicate || mgr.Status().PendingCount != 1 || mgr.Status().DuplicateCount != 1 {
		t.Fatalf("duplicate result=%+v status=%+v", result, mgr.Status())
	}
}

func TestMaterialUpdateUpdatesPendingTarget(t *testing.T) {
	mgr := New(testConfig(), nil, nil)
	target := testTarget(11, 50, time.Now().UTC())
	if _, err := mgr.Ingest(context.Background(), target); err != nil {
		t.Fatalf("Ingest first: %v", err)
	}
	target.Priority = 90
	result, err := mgr.Ingest(context.Background(), target)
	if err != nil {
		t.Fatalf("Ingest update: %v", err)
	}
	if !result.Updated {
		t.Fatalf("expected update result: %+v", result)
	}
	records := mgr.Targets()
	if len(records) != 1 || records[0].Target.Priority != 90 {
		t.Fatalf("records = %+v", records)
	}
}

func TestPendingQueueBounded(t *testing.T) {
	cfg := testConfig()
	cfg.MaxPending = 1
	mgr := New(cfg, nil, nil)
	if _, err := mgr.Ingest(context.Background(), testTarget(12, 10, time.Now().UTC())); err != nil {
		t.Fatalf("Ingest first: %v", err)
	}
	_, err := mgr.Ingest(context.Background(), testTarget(13, 10, time.Now().UTC()))
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("err = %v, want ErrQueueFull", err)
	}
}

func TestRetainedTargetsEvictOnlyTerminalRecords(t *testing.T) {
	cfg := testConfig()
	mgr := New(cfg, nil, nil)
	now := time.Now().UTC()
	for i := 0; i < maxRetainedRecords; i++ {
		target := testTarget(uint32(i+1), 10, now)
		if _, err := mgr.Ingest(context.Background(), target); err != nil {
			t.Fatal(err)
		}
		if !mgr.MarkSubmitted(target, now) || !mgr.MarkCompleted(target, "completed", "", now) {
			t.Fatal("failed to make retained target terminal")
		}
	}
	extra := testTarget(999_999, 10, now)
	if _, err := mgr.Ingest(context.Background(), extra); err != nil {
		t.Fatal(err)
	}
	records := mgr.Targets()
	if len(records) != maxRetainedRecords {
		t.Fatalf("record count = %d", len(records))
	}
	for _, record := range records {
		if record.Target.TargetID == 1 {
			t.Fatal("oldest terminal target was retained")
		}
	}
}

func TestOversizedTargetMetadataIsRejectedBeforeRetention(t *testing.T) {
	mgr := New(testConfig(), nil, nil)
	target := testTarget(60, 10, time.Now().UTC())
	target.Metadata = map[string]interface{}{"value": strings.Repeat("x", maxRetainedTargetBytes)}
	if _, err := mgr.Ingest(context.Background(), target); !errors.Is(err, ErrValidation) {
		t.Fatalf("error = %v", err)
	}
	if len(mgr.Targets()) != 0 {
		t.Fatal("rejected target mutated retention")
	}
}

func TestDispatchSelectsHighestPriorityThenOldest(t *testing.T) {
	auto := &fakeAuto{status: autocontrol.Status{Enabled: true, State: autocontrol.StateIdle}}
	mgr := New(testConfig(), auto, nil)
	now := time.Now().UTC()
	_, _ = mgr.Ingest(context.Background(), testTarget(20, 40, now.Add(2*time.Second)))
	_, _ = mgr.Ingest(context.Background(), testTarget(21, 90, now.Add(3*time.Second)))
	_, _ = mgr.Ingest(context.Background(), testTarget(22, 90, now))
	mgr.dispatch(context.Background())
	if got := auto.submittedIDs(); len(got) != 1 || got[0] != 22 {
		t.Fatalf("submitted IDs = %v, want [22]", got)
	}
}

func TestActiveEngagementPreventsPreemptionAndCompletionAdvances(t *testing.T) {
	auto := &fakeAuto{status: autocontrol.Status{Enabled: true, State: autocontrol.StateIdle}}
	mgr := New(testConfig(), auto, nil)
	now := time.Now().UTC()
	_, _ = mgr.Ingest(context.Background(), testTarget(30, 10, now))
	_, _ = mgr.Ingest(context.Background(), testTarget(31, 100, now))
	mgr.dispatch(context.Background())
	if got := auto.submittedIDs(); len(got) != 1 || got[0] != 31 {
		t.Fatalf("first submitted IDs = %v, want [31]", got)
	}
	auto.setStatus(autocontrol.Status{Enabled: true, State: autocontrol.StateWaitingForCompletion, ActiveTarget: &autocontrol.Target{TargetID: 31}})
	mgr.dispatch(context.Background())
	if got := auto.submittedIDs(); len(got) != 1 {
		t.Fatalf("active engagement should block preemption, got %v", got)
	}
	auto.setStatus(autocontrol.Status{Enabled: true, State: autocontrol.StateIdle, LastResult: "completed_fired"})
	mgr.dispatch(context.Background())
	mgr.dispatch(context.Background())
	if got := auto.submittedIDs(); len(got) != 2 || got[1] != 30 {
		t.Fatalf("after completion submitted IDs = %v, want [31 30]", got)
	}
}

func TestSuspensionStopsQueueAdvancementAndRecoveryResumes(t *testing.T) {
	auto := &fakeAuto{status: autocontrol.Status{Enabled: true, State: autocontrol.StateSuspendedFault}}
	mgr := New(testConfig(), auto, nil)
	_, _ = mgr.Ingest(context.Background(), testTarget(40, 80, time.Now().UTC()))
	mgr.dispatch(context.Background())
	if got := auto.submittedIDs(); len(got) != 0 {
		t.Fatalf("suspended auto submitted IDs = %v", got)
	}
	auto.setStatus(autocontrol.Status{Enabled: true, State: autocontrol.StateIdle})
	mgr.dispatch(context.Background())
	if got := auto.submittedIDs(); len(got) != 1 || got[0] != 40 {
		t.Fatalf("recovered auto submitted IDs = %v, want [40]", got)
	}
}

func TestFeedHealthBecomesStaleAndRecovers(t *testing.T) {
	cfg := testConfig()
	cfg.HealthyWindow = 10 * time.Millisecond
	mgr := New(cfg, nil, nil)
	_, _ = mgr.Ingest(context.Background(), testTarget(50, 10, time.Now().UTC()))
	if !mgr.Status().Healthy {
		t.Fatalf("feed should be healthy immediately: %+v", mgr.Status())
	}
	time.Sleep(20 * time.Millisecond)
	if mgr.Status().Healthy {
		t.Fatalf("feed should become stale: %+v", mgr.Status())
	}
	_, _ = mgr.Ingest(context.Background(), testTarget(51, 10, time.Now().UTC()))
	if !mgr.Status().Healthy {
		t.Fatalf("feed should recover: %+v", mgr.Status())
	}
}

func testConfig() Config {
	return Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, HealthyWindow: time.Second, PollInterval: time.Millisecond}
}

func testTarget(id uint32, priority int, detected time.Time) Target {
	return Target{Source: SourceExternalGenerator, TargetID: id, Azimuth: 120, Elevation: 30, Priority: priority, TargetClass: "test", DetectedAt: detected}
}

type fakeAuto struct {
	mu      sync.Mutex
	status  autocontrol.Status
	targets []autocontrol.Target
}

func (f *fakeAuto) SubmitTarget(ctx context.Context, target autocontrol.Target) (autocontrol.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.targets = append(f.targets, target)
	f.status.QueuedTarget = &target
	return f.status, nil
}

func (f *fakeAuto) Status(ctx context.Context) autocontrol.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

func (f *fakeAuto) setStatus(status autocontrol.Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = status
}

func (f *fakeAuto) submittedIDs() []uint32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]uint32, 0, len(f.targets))
	for _, target := range f.targets {
		out = append(out, target.TargetID)
	}
	return out
}
