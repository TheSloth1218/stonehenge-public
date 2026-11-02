package model

import "testing"

func TestSectorCoversWraparound(t *testing.T) {
	if !SectorCovers(315, 45, 5) {
		t.Fatal("expected wraparound sector to cover bearing 5")
	}
	if !SectorCovers(315, 45, 330) {
		t.Fatal("expected wraparound sector to cover bearing 330")
	}
	if SectorCovers(315, 45, 180) {
		t.Fatal("did not expect wraparound sector to cover bearing 180")
	}
}
