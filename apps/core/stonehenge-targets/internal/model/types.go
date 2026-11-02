package model

import "time"

type Priority string

const (
	PriorityLow      Priority = "LOW"
	PriorityMedium   Priority = "MEDIUM"
	PriorityHigh     Priority = "HIGH"
	PriorityCritical Priority = "CRITICAL"
)

type TerrainType string

const (
	TerrainLand TerrainType = "LAND"
	TerrainSea  TerrainType = "SEA"
)

type FragmentTrack struct {
	ExternalID          string      `json:"external_id"`
	SourceID            string      `json:"source_id,omitempty"`
	Bearing             float64     `json:"bearing"`
	Elevation           float64     `json:"elevation"`
	MapX                float64     `json:"map_x"`
	MapY                float64     `json:"map_y"`
	Priority            Priority    `json:"priority"`
	TerrainType         TerrainType `json:"terrain_type"`
	StrategicZone       string      `json:"strategic_zone"`
	TimeToImpactSeconds int         `json:"time_to_impact_seconds"`
	ObservedAt          time.Time   `json:"observed_at"`
}

type Status struct {
	Mode                         string     `json:"mode"`
	DestinationEndpoint          string     `json:"destination_endpoint"`
	SourceID                     string     `json:"source_id"`
	GeneratedCount               uint64     `json:"generated_count"`
	AcceptedCount                uint64     `json:"accepted_count"`
	DuplicateCount               uint64     `json:"duplicate_count"`
	RejectedCount                uint64     `json:"rejected_count"`
	RetryCount                   uint64     `json:"retry_count"`
	HTTP503Count                 uint64     `json:"http_503_count"`
	HTTP429Count                 uint64     `json:"http_429_count"`
	ExpiredUnsentCount           uint64     `json:"expired_unsent_count"`
	FailedCount                  uint64     `json:"failed_count"`
	LastGeneratedExternalID      string     `json:"last_generated_external_id"`
	PendingExternalID            string     `json:"pending_external_id,omitempty"`
	LastSuccessfulSubmissionTime *time.Time `json:"last_successful_submission_time,omitempty"`
	LastError                    string     `json:"last_error,omitempty"`
	ServiceStartTime             time.Time  `json:"service_start_time"`
}

type SubmitResult struct {
	Accepted   bool
	Duplicate  bool
	Permanent  bool
	QueueFull  bool
	Stale      bool
	StatusCode int
	Attempts   int
	RetryAfter time.Duration
	Error      string
}

type MiddlewareReadiness struct {
	Ready            bool
	Reason           string
	QueueDepth       int
	ActiveCount      int
	UncertainCount   int
	OutstandingCount int
}
