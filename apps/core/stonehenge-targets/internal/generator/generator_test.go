package generator

import (
	"testing"
	"time"

	"stonehenge-targets/internal/model"
)

func TestDeterministicSeededGeneration(t *testing.T) {
	seed := int64(42)
	a := New("USEA-TRACKING-NET", &seed)
	b := New("USEA-TRACKING-NET", &seed)
	now := time.Date(2026, 6, 26, 14, 30, 0, 0, time.UTC)
	left := a.Next(now)
	right := b.Next(now)
	if left != right {
		t.Fatalf("seeded generation mismatch:\n%+v\n%+v", left, right)
	}
}

func TestGeneratedValuesAreInValidRanges(t *testing.T) {
	seed := int64(7)
	gen := New("SRC", &seed)
	for i := 0; i < 100; i++ {
		track := gen.Next(time.Now().UTC())
		if track.Bearing < 0 || track.Bearing > 359 {
			t.Fatalf("bearing out of range: %f", track.Bearing)
		}
		if track.Elevation < 5 || track.Elevation > 85 {
			t.Fatalf("elevation out of range: %f", track.Elevation)
		}
		if track.MapX < 0 || track.MapX > 100 || track.MapY < 0 || track.MapY > 100 {
			t.Fatalf("map point out of range: %f/%f", track.MapX, track.MapY)
		}
		if track.TimeToImpactSeconds <= 0 || track.TimeToImpactSeconds > 600 {
			t.Fatalf("tti out of range: %d", track.TimeToImpactSeconds)
		}
	}
}

func TestLandSeaPriorityBehavior(t *testing.T) {
	seed := int64(1)
	r := New("SRC", &seed).rand
	for i := 0; i < 20; i++ {
		p := PriorityFor(model.TerrainSea, "OPEN_SEA", r)
		if p != model.PriorityLow && p != model.PriorityMedium {
			t.Fatalf("sea priority = %s", p)
		}
	}
	seenHigh := false
	for i := 0; i < 20; i++ {
		p := PriorityFor(model.TerrainLand, "SAN_SALVACION", r)
		if p == model.PriorityHigh || p == model.PriorityCritical {
			seenHigh = true
		}
	}
	if !seenHigh {
		t.Fatal("expected land impacts to produce high or critical priority")
	}
}

func TestStrategicZoneAssignment(t *testing.T) {
	terrain, zone := Classify(30, 43)
	if terrain != model.TerrainLand || zone != "SAN_SALVACION" {
		t.Fatalf("expected San Salvacion land zone, got %s/%s", terrain, zone)
	}
	terrain, zone = Classify(93, 12)
	if terrain != model.TerrainSea || zone != "OPEN_SEA" {
		t.Fatalf("expected open sea, got %s/%s", terrain, zone)
	}
}

func TestUniqueExternalIDs(t *testing.T) {
	seed := int64(2)
	gen := New("SRC", &seed)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		id := gen.Next(time.Now().UTC()).ExternalID
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
	}
	if !seen["SRC-000001"] || !seen["SRC-000050"] {
		t.Fatalf("missing expected monotonic ids: %+v", seen)
	}
}
