package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
	"time"

	"stonehenge-middleware/internal/model"
)

const maxRetainedFragments = 500

var ErrFragmentCapacity = errors.New("live fragment retention is full")

type MemoryStore struct {
	mu          sync.RWMutex
	guns        map[int]model.GunState
	fragments   map[string]model.Fragment
	events      []model.Event
	maxEvents   int
	nextEventID int64
}

func NewMemoryStore(maxEvents int) *MemoryStore {
	if maxEvents <= 0 {
		maxEvents = 500
	}
	return &MemoryStore{
		guns:      make(map[int]model.GunState),
		fragments: make(map[string]model.Fragment),
		maxEvents: maxEvents,
	}
}

func (s *MemoryStore) Reset(guns []model.GunState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guns = make(map[int]model.GunState, len(guns))
	for _, gun := range guns {
		s.guns[gun.ID] = gun
	}
	s.fragments = make(map[string]model.Fragment)
	s.events = nil
	s.nextEventID = 0
}

func (s *MemoryStore) Guns() []model.GunState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.GunState, 0, len(s.guns))
	for _, gun := range s.guns {
		out = append(out, gun)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *MemoryStore) Gun(id int) (model.GunState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	gun, ok := s.guns[id]
	return gun, ok
}

func (s *MemoryStore) SaveGun(gun model.GunState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guns[gun.ID] = gun
}

func (s *MemoryStore) Fragments() []model.Fragment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Fragment, 0, len(s.fragments))
	for _, fragment := range s.fragments {
		out = append(out, fragment)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *MemoryStore) ActiveFragments() []model.Fragment {
	all := s.Fragments()
	out := make([]model.Fragment, 0, len(all))
	for _, fragment := range all {
		if isActiveFragment(fragment) {
			out = append(out, fragment)
		}
	}
	return out
}

func (s *MemoryStore) ActiveFragmentByExternalID(externalID string) (model.Fragment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, fragment := range s.fragments {
		if fragment.ExternalID == externalID && isActiveFragment(fragment) {
			return fragment, true
		}
	}
	return model.Fragment{}, false
}

func (s *MemoryStore) Fragment(id string) (model.Fragment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fragment, ok := s.fragments[id]
	return fragment, ok
}

func (s *MemoryStore) SaveFragment(fragment model.Fragment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.fragments[fragment.ID]; exists {
		s.fragments[fragment.ID] = fragment
		return nil
	}
	if len(s.fragments) >= maxRetainedFragments {
		var oldestID string
		var oldestAt time.Time
		for id, retained := range s.fragments {
			if isActiveFragment(retained) {
				continue
			}
			if oldestID == "" || retained.CreatedAt.Before(oldestAt) {
				oldestID = id
				oldestAt = retained.CreatedAt
			}
		}
		if oldestID == "" {
			return ErrFragmentCapacity
		}
		delete(s.fragments, oldestID)
	}
	s.fragments[fragment.ID] = fragment
	return nil
}

func isActiveFragment(fragment model.Fragment) bool {
	return fragment.Status == model.FragmentActive || fragment.Status == model.FragmentAssigned
}

func (s *MemoryStore) Events(limit int) []model.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.events) {
		limit = len(s.events)
	}
	start := len(s.events) - limit
	out := append([]model.Event(nil), s.events[start:]...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *MemoryStore) EventCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

func (s *MemoryStore) AddEvent(event model.Event) model.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextEventID++
	event.ID = s.nextEventID
	if event.EventUUID == "" {
		event.EventUUID = randomUUID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	s.events = append(s.events, event)
	if len(s.events) > s.maxEvents {
		s.events = s.events[len(s.events)-s.maxEvents:]
	}
	return event
}

func randomUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		sum := time.Now().UTC().AppendFormat(nil, time.RFC3339Nano)
		copy(b[:], sum)
	}
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	dst := make([]byte, 36)
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])
	return string(dst)
}
