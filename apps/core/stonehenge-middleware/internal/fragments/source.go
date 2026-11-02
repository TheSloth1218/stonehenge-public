package fragments

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/model"
	"stonehenge-middleware/internal/terrain"
)

const maxRetainedExternalIDs = 10_000

var ErrExternalIDCapacity = errors.New("external_id retention capacity reached")

type Submitter interface {
	SubmitFragment(ctx context.Context, input Input) (model.Fragment, error)
}

type Source interface {
	Name() string
	Start(ctx context.Context, submit Submitter) error
	Health() model.SourceHealth
}

type Input struct {
	ExternalID             string
	Bearing                float64
	Elevation              float64
	MapX                   float64
	MapY                   float64
	Priority               model.Priority
	TerrainType            string
	StrategicZone          string
	TimeToImpactSeconds    int
	ObservedAt             time.Time
	GeneratedByInternalRNG bool
}

type InternalRandomSource struct {
	cfg    config.Config
	rand   *rand.Rand
	mu     sync.RWMutex
	health model.SourceHealth
}

func NewInternalRandomSource(cfg config.Config) *InternalRandomSource {
	return &InternalRandomSource{
		cfg:  cfg,
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
		health: model.SourceHealth{
			Name: "internal-rng",
		},
	}
}

func (s *InternalRandomSource) Name() string {
	return "internal-rng"
}

func (s *InternalRandomSource) Start(ctx context.Context, submit Submitter) error {
	s.setRunning(true, "")
	defer s.setRunning(false, "")
	if !s.cfg.Fragments.InternalRNG.Enabled {
		<-ctx.Done()
		return nil
	}
	interval := s.cfg.Fragments.InternalRNG.Interval
	if interval <= 0 {
		interval = s.cfg.Simulation.TargetGenerationInterval
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			input := s.nextInput(time.Now().UTC())
			if _, err := submit.SubmitFragment(ctx, input); err != nil && !errors.Is(err, context.Canceled) {
				s.setRunning(true, err.Error())
			} else {
				s.setGeneratedAt(time.Now().UTC())
			}
			timer.Reset(interval)
		}
	}
}

func (s *InternalRandomSource) Health() model.SourceHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.health
}

func (s *InternalRandomSource) nextInput(now time.Time) Input {
	minTTI := s.cfg.Simulation.TargetTimeToImpactMinSeconds
	maxTTI := s.cfg.Simulation.TargetTimeToImpactMaxSeconds
	if maxTTI < minTTI {
		maxTTI = minTTI
	}
	tti := minTTI
	if maxTTI > minTTI {
		tti += s.rand.Intn(maxTTI - minTTI + 1)
	}
	mapX, mapY, terrainType, strategicZone := terrain.RandomImpact(s.rand)
	return Input{
		Bearing:                float64(s.rand.Intn(360)),
		Elevation:              5 + s.rand.Float64()*80,
		MapX:                   mapX,
		MapY:                   mapY,
		Priority:               terrain.PriorityFor(terrainType, strategicZone, s.rand),
		TerrainType:            terrainType,
		StrategicZone:          strategicZone,
		TimeToImpactSeconds:    tti,
		ObservedAt:             now,
		GeneratedByInternalRNG: true,
	}
}

func (s *InternalRandomSource) setRunning(running bool, err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.health.Name = s.Name()
	s.health.Running = running
	s.health.LastError = err
}

func (s *InternalRandomSource) setGeneratedAt(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.health.LastGeneratedAt = &t
	s.health.LastError = ""
}

type ExternalIngestSource struct {
	cfg       config.IngestConfig
	mu        sync.RWMutex
	submitter Submitter
	seen      map[string]time.Time
	counters  model.IngestCounters
	health    model.SourceHealth
}

func NewExternalIngestSource(cfg config.IngestConfig) *ExternalIngestSource {
	return &ExternalIngestSource{
		cfg:  cfg,
		seen: make(map[string]time.Time),
		health: model.SourceHealth{
			Name: "ingest",
		},
	}
}

func (s *ExternalIngestSource) Name() string {
	return "ingest"
}

func (s *ExternalIngestSource) Start(ctx context.Context, submit Submitter) error {
	s.Bind(submit)
	defer func() {
		s.mu.Lock()
		s.health.Running = false
		s.mu.Unlock()
	}()
	<-ctx.Done()
	return nil
}

func (s *ExternalIngestSource) Bind(submit Submitter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.submitter = submit
	s.health.Name = s.Name()
	s.health.Running = true
}

func (s *ExternalIngestSource) Submit(ctx context.Context, input Input) (model.Fragment, error) {
	now := time.Now().UTC()
	s.mu.Lock()
	s.counters.Received++
	s.pruneLocked(now)
	if input.ExternalID == "" {
		s.counters.Rejected++
		s.mu.Unlock()
		return model.Fragment{}, ValidationError("external_id is required")
	}
	if _, ok := s.seen[input.ExternalID]; ok {
		s.counters.Rejected++
		s.counters.DuplicateReplayRejected++
		s.mu.Unlock()
		return model.Fragment{}, DuplicateError{ExternalID: input.ExternalID}
	}
	if len(s.seen) >= maxRetainedExternalIDs {
		s.counters.Rejected++
		s.mu.Unlock()
		return model.Fragment{}, ErrExternalIDCapacity
	}
	s.seen[input.ExternalID] = now
	submitter := s.submitter
	s.mu.Unlock()
	if submitter == nil {
		s.mu.Lock()
		delete(s.seen, input.ExternalID)
		s.counters.Rejected++
		s.mu.Unlock()
		return model.Fragment{}, errors.New("ingest source is not running")
	}
	fragment, err := submitter.SubmitFragment(ctx, input)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		delete(s.seen, input.ExternalID)
		s.counters.Rejected++
		s.health.LastError = err.Error()
		return model.Fragment{}, err
	}
	s.counters.Accepted++
	s.health.LastError = ""
	receivedAt := time.Now().UTC()
	s.health.LastReceivedAt = &receivedAt
	return fragment, nil
}

func (s *ExternalIngestSource) Health() model.SourceHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.health
}

func (s *ExternalIngestSource) Counters() model.IngestCounters {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.counters
}

func (s *ExternalIngestSource) pruneLocked(now time.Time) {
	retention := s.cfg.DuplicateRetention
	if retention <= 0 {
		retention = 24 * time.Hour
	}
	cutoff := now.Add(-retention)
	for id, seenAt := range s.seen {
		if seenAt.Before(cutoff) {
			delete(s.seen, id)
		}
	}
}

type ValidationError string

func (e ValidationError) Error() string {
	return string(e)
}

type DuplicateError struct {
	ExternalID string
}

func (e DuplicateError) Error() string {
	return fmt.Sprintf("external_id %q was already processed recently", e.ExternalID)
}
