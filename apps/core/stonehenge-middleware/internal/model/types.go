package model

import "time"

type GunMode string

const (
	ModeAuto        GunMode = "AUTO"
	ModeManual      GunMode = "MANUAL"
	ModeMaintenance GunMode = "MAINTENANCE"
)

type GunStateName string

const (
	GunIdle             GunStateName = "IDLE"
	GunTargetAssigned   GunStateName = "TARGET_ASSIGNED"
	GunPostFireLockout  GunStateName = "POST_FIRE_LOCKOUT"
	GunSlewing          GunStateName = "SLEWING"
	GunOnTargetSettling GunStateName = "ON_TARGET_SETTLING"
	GunReadyToFire      GunStateName = "READY_TO_FIRE"
	GunFiring           GunStateName = "FIRING"
	GunFaulted          GunStateName = "FAULTED"
	GunManual           GunStateName = "MANUAL"
)

type Priority string

const (
	PriorityLow      Priority = "LOW"
	PriorityMedium   Priority = "MEDIUM"
	PriorityHigh     Priority = "HIGH"
	PriorityCritical Priority = "CRITICAL"
)

type FragmentStatus string

const (
	FragmentActive      FragmentStatus = "ACTIVE"
	FragmentAssigned    FragmentStatus = "ASSIGNED"
	FragmentIntercepted FragmentStatus = "INTERCEPTED"
	FragmentMissed      FragmentStatus = "MISSED"
)

type ShellType string

const (
	ShellStandard             ShellType = "STANDARD"
	ShellSpecialLargeFragment ShellType = "SPECIAL_LARGE_FRAGMENT"
)

type EventType string

const (
	EventFragmentReceived        EventType = "FRAGMENT_RECEIVED"
	EventFragmentRejected        EventType = "FRAGMENT_REJECTED"
	EventTargetGenerated         EventType = "TARGET_GENERATED"
	EventTargetAssigned          EventType = "TARGET_ASSIGNED"
	EventGunSlewStarted          EventType = "GUN_SLEW_STARTED"
	EventGunOnTarget             EventType = "GUN_ON_TARGET"
	EventFireRequested           EventType = "FIRE_REQUESTED"
	EventSafetyPermissiveGranted EventType = "SAFETY_PERMISSIVE_GRANTED"
	EventSafetyPermissiveBlocked EventType = "SAFETY_PERMISSIVE_BLOCKED"
	EventGunFired                EventType = "GUN_FIRED"
	EventInterceptRecorded       EventType = "INTERCEPT_RECORDED"
	EventFragmentMissed          EventType = "FRAGMENT_MISSED"
	EventGunFaulted              EventType = "GUN_FAULTED"
	EventGunModeChanged          EventType = "GUN_MODE_CHANGED"
	EventGunMaintenanceChanged   EventType = "GUN_MAINTENANCE_CHANGED"
	EventAdminOverrideChanged    EventType = "ADMIN_OVERRIDE_CHANGED"
	EventManualTargetRejected    EventType = "MANUAL_TARGET_REJECTED"
	EventManualTargetAccepted    EventType = "MANUAL_TARGET_ACCEPTED"
	EventFaultCleared            EventType = "GUN_FAULT_CLEARED"
	EventSimulationStarted       EventType = "SIMULATION_STARTED"
	EventSimulationStopped       EventType = "SIMULATION_STOPPED"
	EventSimulationReset         EventType = "SIMULATION_RESET"
)

type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
)

type GunState struct {
	ID                      int          `json:"id"`
	Name                    string       `json:"name"`
	Mode                    GunMode      `json:"mode"`
	State                   GunStateName `json:"state"`
	SectorStart             float64      `json:"sector_start"`
	SectorEnd               float64      `json:"sector_end"`
	CurrentAzimuth          float64      `json:"current_azimuth"`
	CurrentElevation        float64      `json:"current_elevation"`
	TargetAzimuth           float64      `json:"target_azimuth"`
	TargetElevation         float64      `json:"target_elevation"`
	AssignedFragmentID      *string      `json:"assigned_fragment_id"`
	SoftMinElevation        float64      `json:"soft_min_elevation"`
	SoftMaxElevation        float64      `json:"soft_max_elevation"`
	HardLimitStatus         string       `json:"hard_limit_status"`
	SafetyPermissive        bool         `json:"safety_permissive"`
	SafetyFaultReason       string       `json:"safety_fault_reason"`
	ShellType               ShellType    `json:"shell_type"`
	SpecialMeasuresEnabled  bool         `json:"special_measures_enabled"`
	AdminOverrideState      string       `json:"admin_override_state"`
	AdminOverrideActive     bool         `json:"admin_override_active"`
	AdminOverrideExpiresAt  *time.Time   `json:"admin_override_expires_at"`
	AdminOverrideApprovedBy string       `json:"admin_override_approved_by"`
	MaintenanceEnabled      bool         `json:"maintenance_enabled"`
	MaintenanceRequestedBy  string       `json:"maintenance_requested_by"`
	ShotSequence            uint64       `json:"shot_sequence"`
	LastFireTime            *time.Time   `json:"last_fire_time"`
	LastIdleTime            time.Time    `json:"last_idle_time"`
	PostFireLockoutUntil    *time.Time   `json:"post_fire_lockout_until"`
	SettlingUntil           *time.Time   `json:"settling_until"`
	SlewCompleteAt          *time.Time   `json:"slew_complete_at"`
	FaultCode               string       `json:"fault_code"`
	EventSummary            string       `json:"event_summary"`
}

type Fragment struct {
	ID                  string         `json:"id"`
	ExternalID          string         `json:"external_id,omitempty"`
	Azimuth             float64        `json:"azimuth"`
	Elevation           float64        `json:"elevation"`
	MapX                float64        `json:"map_x"`
	MapY                float64        `json:"map_y"`
	Priority            Priority       `json:"priority"`
	TerrainType         string         `json:"terrain_type"`
	StrategicZone       string         `json:"strategic_zone"`
	Status              FragmentStatus `json:"status"`
	AssignedGunID       *int           `json:"assigned_gun_id"`
	CreatedAt           time.Time      `json:"created_at"`
	TimeToImpactSeconds int            `json:"time_to_impact_seconds"`
	ImpactAt            time.Time      `json:"impact_at"`
	InterceptedAt       *time.Time     `json:"intercepted_at"`
	MissedAt            *time.Time     `json:"missed_at"`
}

type OperationalEvent struct {
	ID                 int64                  `json:"id"`
	EventUUID          string                 `json:"event_uuid,omitempty"`
	Timestamp          time.Time              `json:"timestamp"`
	EventType          EventType              `json:"event_type"`
	Severity           Severity               `json:"severity"`
	GunID              *int                   `json:"gun_id"`
	FragmentID         *string                `json:"fragment_id"`
	ExternalFragmentID *string                `json:"external_fragment_id,omitempty"`
	Azimuth            *float64               `json:"azimuth,omitempty"`
	Elevation          *float64               `json:"elevation,omitempty"`
	Mode               *GunMode               `json:"mode,omitempty"`
	ShellType          *ShellType             `json:"shell_type,omitempty"`
	ShotSequence       *uint64                `json:"shot_sequence,omitempty"`
	Message            string                 `json:"message"`
	Details            map[string]interface{} `json:"details,omitempty"`
}

type Event = OperationalEvent

type HardLimits struct {
	MinElevation float64 `json:"min_elevation"`
	MaxElevation float64 `json:"max_elevation"`
}

type SpecialMeasuresState struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}

type SafetyDecision struct {
	Allowed                 bool      `json:"allowed"`
	Reason                  string    `json:"reason"`
	HardLimitStatus         string    `json:"hard_limit_status"`
	SpecialMeasuresRequired bool      `json:"special_measures_required"`
	Timestamp               time.Time `json:"timestamp"`
}

type Status struct {
	Running              bool           `json:"running"`
	GunCount             int            `json:"gun_count"`
	ActiveFragments      int            `json:"active_fragments"`
	EventCount           int            `json:"event_count"`
	FragmentSource       string         `json:"fragment_source"`
	FragmentSourceHealth SourceHealth   `json:"fragment_source_health"`
	IngestCounters       IngestCounters `json:"ingest_counters"`
	PLC                  PLCHealth      `json:"plc"`
	Safety               SafetyHealth   `json:"safety"`
	History              HistoryHealth  `json:"history"`
	Guns                 []GunState     `json:"guns"`
	FragmentList         []Fragment     `json:"fragment_list"`
	ActiveFragmentList   []Fragment     `json:"active_fragment_list"`
	RecentEvents         []Event        `json:"recent_events"`
	GeneratedAt          time.Time      `json:"generated_at"`
}

type PLCHealth struct {
	Adapter       string     `json:"adapter"`
	Connected     bool       `json:"connected"`
	LastError     string     `json:"last_error,omitempty"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
	LastOKAt      *time.Time `json:"last_ok_at,omitempty"`
}

type SourceHealth struct {
	Name            string     `json:"name"`
	Running         bool       `json:"running"`
	LastError       string     `json:"last_error,omitempty"`
	LastGeneratedAt *time.Time `json:"last_generated_at,omitempty"`
	LastReceivedAt  *time.Time `json:"last_received_at,omitempty"`
}

type IngestCounters struct {
	Received                uint64 `json:"received"`
	Accepted                uint64 `json:"accepted"`
	Rejected                uint64 `json:"rejected"`
	DuplicateReplayRejected uint64 `json:"duplicate_replay_rejected"`
}

type SafetyHealth struct {
	Name      string `json:"name"`
	Healthy   bool   `json:"healthy"`
	LastError string `json:"last_error,omitempty"`
}

type HistoryHealth struct {
	Name             string     `json:"name"`
	Adapter          string     `json:"adapter,omitempty"`
	Healthy          bool       `json:"healthy"`
	Connected        bool       `json:"connected"`
	EventCount       int        `json:"event_count"`
	QueueDepth       int        `json:"queue_depth"`
	SuccessfulWrites uint64     `json:"successful_writes"`
	Retries          uint64     `json:"retries"`
	FailedWrites     uint64     `json:"failed_writes"`
	DroppedEvents    uint64     `json:"dropped_events"`
	LastWriteAt      *time.Time `json:"last_write_at,omitempty"`
	LastSuccessAt    *time.Time `json:"last_success_at,omitempty"`
	LastError        string     `json:"last_error,omitempty"`
}
