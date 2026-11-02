package terrain

import (
	"math/rand"
	"testing"

	"stonehenge-middleware/internal/model"
)

func TestClassifyLandAndSea(t *testing.T) {
	terrainType, zone := Classify(31, 51)
	if terrainType != TerrainLand {
		t.Fatalf("expected Stonehenge area to classify as land, got %s", terrainType)
	}
	if zone != "STONEHENGE" {
		t.Fatalf("expected Stonehenge strategic zone, got %s", zone)
	}

	terrainType, zone = Classify(93, 12)
	if terrainType != TerrainSea {
		t.Fatalf("expected open water to classify as sea, got %s", terrainType)
	}
	if zone != "OPEN_SEA" {
		t.Fatalf("expected OPEN_SEA zone, got %s", zone)
	}
}

func TestClassifyProvidedMapAnchors(t *testing.T) {
	cases := []struct {
		name string
		x    float64
		y    float64
		zone string
	}{
		{name: "Farbanti", x: 14, y: 39, zone: "FARBANTI"},
		{name: "San Salvacion", x: 30, y: 43, zone: "SAN_SALVACION"},
		{name: "Stonehenge", x: 29, y: 51, zone: "STONEHENGE"},
		{name: "Los Canas", x: 57, y: 59, zone: "LOS_CANAS"},
		{name: "Rigley Air Base", x: 64, y: 42, zone: "RIGLEY_AIR_BASE"},
		{name: "North Point", x: 83, y: 29, zone: "NORTH_POINT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			terrainType, zone := Classify(tc.x, tc.y)
			if terrainType != TerrainLand {
				t.Fatalf("expected %s to classify as land, got %s", tc.name, terrainType)
			}
			if zone != tc.zone {
				t.Fatalf("expected zone %s, got %s", tc.zone, zone)
			}
		})
	}
}

func TestBroadRangeRingDoesNotDriveStrategicZone(t *testing.T) {
	terrainType, zone := Classify(42, 60)
	if terrainType != TerrainLand {
		t.Fatalf("expected central mainland to classify as land, got %s", terrainType)
	}
	if zone == "STONEHENGE" || zone == "STONEHENGE_DEFENSE_RING" {
		t.Fatalf("expected central mainland outside the Stonehenge site to avoid ring-based zone, got %s", zone)
	}
}

func TestPriorityForLandAndSea(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 20; i++ {
		if p := PriorityFor(TerrainSea, "OPEN_SEA", r); p != model.PriorityLow && p != model.PriorityMedium {
			t.Fatalf("expected sea priority to stay low/medium, got %s", p)
		}
	}

	seenHighOrCritical := false
	for i := 0; i < 20; i++ {
		p := PriorityFor(TerrainLand, "SAN_SALVACION", r)
		if p == model.PriorityHigh || p == model.PriorityCritical {
			seenHighOrCritical = true
			break
		}
	}
	if !seenHighOrCritical {
		t.Fatal("expected land strategic zone to produce high or critical priorities")
	}
}
