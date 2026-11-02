package store

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"stonehenge-middleware/internal/model"
)

func TestFragmentRetentionEvictsOnlyTerminalEntries(t *testing.T) {
	store := NewMemoryStore(10)
	now := time.Now().UTC()
	for i := 0; i < maxRetainedFragments; i++ {
		fragment := model.Fragment{ID: fmt.Sprintf("old-%d", i), Status: model.FragmentMissed, CreatedAt: now.Add(time.Duration(i) * time.Second)}
		if err := store.SaveFragment(fragment); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveFragment(model.Fragment{ID: "new", Status: model.FragmentActive, CreatedAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if len(store.Fragments()) != maxRetainedFragments {
		t.Fatalf("fragment count = %d", len(store.Fragments()))
	}
	if _, found := store.Fragment("old-0"); found {
		t.Fatal("oldest terminal fragment was retained")
	}
	if _, found := store.Fragment("new"); !found {
		t.Fatal("new fragment was not retained")
	}
}

func TestFragmentRetentionRejectsWhenEveryEntryIsActive(t *testing.T) {
	store := NewMemoryStore(10)
	for i := 0; i < maxRetainedFragments; i++ {
		if err := store.SaveFragment(model.Fragment{ID: fmt.Sprintf("active-%d", i), Status: model.FragmentActive}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveFragment(model.Fragment{ID: "overflow", Status: model.FragmentActive}); !errors.Is(err, ErrFragmentCapacity) {
		t.Fatalf("error = %v", err)
	}
	if _, found := store.Fragment("overflow"); found {
		t.Fatal("overflow fragment mutated the store")
	}
}

func TestActiveFragmentByExternalIDIgnoresTerminalEntries(t *testing.T) {
	store := NewMemoryStore(10)
	for _, fragment := range []model.Fragment{
		{ID: "active", ExternalID: "active-external", Status: model.FragmentActive},
		{ID: "assigned", ExternalID: "assigned-external", Status: model.FragmentAssigned},
		{ID: "terminal", ExternalID: "terminal-external", Status: model.FragmentMissed},
	} {
		if err := store.SaveFragment(fragment); err != nil {
			t.Fatal(err)
		}
	}
	for _, externalID := range []string{"active-external", "assigned-external"} {
		if _, found := store.ActiveFragmentByExternalID(externalID); !found {
			t.Fatalf("active fragment %q not found", externalID)
		}
	}
	if _, found := store.ActiveFragmentByExternalID("terminal-external"); found {
		t.Fatal("terminal fragment reported as active")
	}
}
