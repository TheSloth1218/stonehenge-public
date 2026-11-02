package history

import (
	"context"
	"testing"
	"time"

	"stonehenge-middleware/internal/model"
)

func TestNoOpHistorySinkWorks(t *testing.T) {
	sink := NewNoOpSink()
	if sink.Name() != "noop" {
		t.Fatalf("name = %q, want noop", sink.Name())
	}
	if err := sink.WriteEvent(context.Background(), model.OperationalEvent{EventType: model.EventTargetAssigned}); err != nil {
		t.Fatalf("WriteEvent returned error: %v", err)
	}
	if health := sink.Health(); !health.Healthy {
		t.Fatalf("expected healthy noop sink: %+v", health)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

func TestOperationalEventUUIDSeparatesIdlessEvents(t *testing.T) {
	timestamp := time.Date(2026, time.September, 19, 4, 0, 0, 0, time.UTC)
	fired := model.OperationalEvent{Timestamp: timestamp, EventType: model.EventGunFired}
	intercepted := model.OperationalEvent{Timestamp: timestamp, EventType: model.EventInterceptRecorded}
	if operationalEventUUID(fired) == operationalEventUUID(intercepted) {
		t.Fatal("distinct idless operational events received the same UUID")
	}
	if operationalEventUUID(fired) != operationalEventUUID(fired) {
		t.Fatal("the same idless operational event received different UUIDs")
	}
}

func TestInMemoryHistorySinkRecordsEvents(t *testing.T) {
	sink := NewInMemorySink(10)
	event := model.OperationalEvent{ID: 1, EventType: model.EventTargetAssigned}
	if err := sink.WriteEvent(context.Background(), event); err != nil {
		t.Fatalf("WriteEvent returned error: %v", err)
	}
	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if events[0].EventType != model.EventTargetAssigned {
		t.Fatalf("event type = %s", events[0].EventType)
	}
	if health := sink.Health(); health.EventCount != 1 || !health.Healthy {
		t.Fatalf("unexpected health: %+v", health)
	}
}

func TestNullableUint32BigintPreservesZero(t *testing.T) {
	zero := uint32(0)
	if got := nullableUint32Bigint(&zero); got != int64(0) {
		t.Fatalf("zero shot sequence converted to %#v, want int64(0)", got)
	}
	nonzero := uint32(4294967295)
	if got := nullableUint32Bigint(&nonzero); got != int64(4294967295) {
		t.Fatalf("max uint32 shot sequence converted to %#v, want int64(4294967295)", got)
	}
	if got := nullableUint32Bigint(nil); got != nil {
		t.Fatalf("nil shot sequence converted to %#v, want nil", got)
	}
}
