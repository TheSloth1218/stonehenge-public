package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server     ServerConfig     `json:"server"`
	Simulation SimulationConfig `json:"simulation"`
	Safety     SafetyConfig     `json:"safety"`
	Fragments  FragmentsConfig  `json:"fragments"`
	History    HistoryConfig    `json:"history"`
	PLC        PLCConfig        `json:"plc"`
	Guns       []GunConfig      `json:"guns"`
	SafetyPLC  SafetyPLCConfig  `json:"safety_plc"`
	TargetFeed TargetFeedConfig `json:"target_feed"`
	Dispatcher DispatcherConfig `json:"dispatcher"`
}

type ServerConfig struct {
	Addr string `json:"addr"`
}

type SimulationConfig struct {
	AutoStart                    bool          `json:"auto_start"`
	GunCount                     int           `json:"gun_count"`
	TickInterval                 time.Duration `json:"tick_interval"`
	TargetGenerationInterval     time.Duration `json:"target_generation_interval"`
	TargetTimeToImpactMinSeconds int           `json:"target_time_to_impact_min_seconds"`
	TargetTimeToImpactMaxSeconds int           `json:"target_time_to_impact_max_seconds"`
	SoftMinElevation             float64       `json:"soft_min_elevation"`
	SoftMaxElevation             float64       `json:"soft_max_elevation"`
	PostFireLockout              time.Duration `json:"post_fire_lockout"`
	SettlingDelay                time.Duration `json:"settling_delay"`
	SlewDegreesPerSecond         float64       `json:"slew_degrees_per_second"`
	MinSlewDuration              time.Duration `json:"min_slew_duration"`
	MaxSlewDuration              time.Duration `json:"max_slew_duration"`
	MaxEvents                    int           `json:"max_events"`
	FaultOnSafetyBlocked         bool          `json:"fault_on_safety_blocked"`
}

type SafetyConfig struct {
	HardMinElevation       float64 `json:"hard_min_elevation"`
	HardMaxElevation       float64 `json:"hard_max_elevation"`
	SpecialMeasuresEnabled bool    `json:"special_measures_enabled"`
}

type PLCConfig struct {
	Adapter                       string        `json:"adapter"`
	Host                          string        `json:"host"`
	Port                          int           `json:"port"`
	UnitID                        byte          `json:"unit_id"`
	CommandAckTimeout             time.Duration `json:"command_ack_timeout"`
	PollInterval                  time.Duration `json:"poll_interval"`
	LifecyclePollInterval         time.Duration `json:"lifecycle_poll_interval"`
	LifecycleCompletionTimeout    time.Duration `json:"lifecycle_completion_timeout"`
	LifecycleMaxConsecutiveErrors int           `json:"lifecycle_max_consecutive_errors"`
	AutoReadinessPollInterval     time.Duration `json:"auto_readiness_poll_interval"`
	AutoSafetyFirePermissive      bool          `json:"auto_safety_fire_permissive"`
	AutoTelemetryModbusEnabled    bool          `json:"auto_telemetry_modbus_enabled"`
	AutoTelemetryModbusAddr       string        `json:"auto_telemetry_modbus_addr"`
	AutoTelemetryUnitID           byte          `json:"auto_telemetry_unit_id"`
	AutoTelemetryPollInterval     time.Duration `json:"auto_telemetry_poll_interval"`
	AutoTelemetryStaleAfter       time.Duration `json:"auto_telemetry_stale_after"`
	FaultResetTimeout             time.Duration `json:"fault_reset_timeout"`
	StartupMode                   string        `json:"startup_mode"`
	StartupModeRetryTimeout       time.Duration `json:"startup_mode_retry_timeout"`
}

type GunConfig struct {
	ID                  int       `json:"id"`
	Name                string    `json:"name"`
	PLCEnabled          bool      `json:"plc_enabled"`
	SafetyGunID         int       `json:"safety_gun_id"`
	AutoEnabled         bool      `json:"auto_enabled"`
	DispatcherEligible  bool      `json:"dispatcher_eligible"`
	AutoExecutorEnabled bool      `json:"auto_executor_enabled"`
	MinAzimuth          float64   `json:"min_azimuth"`
	MaxAzimuth          float64   `json:"max_azimuth"`
	MinElevation        float64   `json:"min_elevation"`
	MaxElevation        float64   `json:"max_elevation"`
	PLC                 PLCConfig `json:"plc"`
}

type SafetyPLCConfig struct {
	Enabled            bool          `json:"enabled"`
	Enforcement        bool          `json:"enforcement"`
	Host               string        `json:"host"`
	Port               int           `json:"port"`
	UnitID             byte          `json:"unit_id"`
	PollInterval       time.Duration `json:"poll_interval"`
	StaleAfter         time.Duration `json:"stale_after"`
	StatusRegisterType string        `json:"status_register_type"`
	MWRegisterBase     uint16        `json:"mw_register_base"`
	ResetPulseDuration time.Duration `json:"reset_pulse_duration"`
}

type FragmentsConfig struct {
	Source      string            `json:"source"`
	Ingest      IngestConfig      `json:"ingest"`
	InternalRNG InternalRNGConfig `json:"internal_rng"`
}

type IngestConfig struct {
	MaxTimeToImpactSeconds int           `json:"max_time_to_impact_seconds"`
	DuplicateRetention     time.Duration `json:"duplicate_retention_minutes"`
}

type InternalRNGConfig struct {
	Enabled  bool          `json:"enabled"`
	Interval time.Duration `json:"interval_seconds"`
}

type HistoryConfig struct {
	Adapter                string        `json:"adapter"`
	MaxEvents              int           `json:"max_events"`
	PostgresHost           string        `json:"postgres_host"`
	PostgresPort           int           `json:"postgres_port"`
	PostgresDB             string        `json:"postgres_db"`
	PostgresUser           string        `json:"postgres_user"`
	PostgresPassword       string        `json:"postgres_password"`
	PostgresSSLMode        string        `json:"postgres_sslmode"`
	PostgresMaxConnections int32         `json:"postgres_max_connections"`
	WriteQueueSize         int           `json:"postgres_write_queue_size"`
	RetryInterval          time.Duration `json:"postgres_retry_interval_seconds"`
	MigrationsPath         string        `json:"migrations_path"`
}

type TargetFeedConfig struct {
	Enabled              bool          `json:"enabled"`
	MaxPending           int           `json:"max_pending"`
	TargetTTL            time.Duration `json:"target_ttl"`
	HealthyWindow        time.Duration `json:"healthy_window"`
	DispatchPollInterval time.Duration `json:"dispatch_poll_interval"`
}

type DispatcherMode string

const (
	DispatcherModeDisabled DispatcherMode = "disabled"
	DispatcherModeShadow   DispatcherMode = "shadow"
	DispatcherModeActive   DispatcherMode = "active"
)

type DispatcherConfig struct {
	Enabled                 bool           `json:"enabled"`
	Mode                    DispatcherMode `json:"mode"`
	PollInterval            time.Duration  `json:"poll_interval"`
	AssignmentLeaseDuration time.Duration  `json:"assignment_lease_duration"`
	LeaseRenewalInterval    time.Duration  `json:"lease_renewal_interval"`
	ReconciliationInterval  time.Duration  `json:"reconciliation_interval"`
	ShadowDecisionRetention time.Duration  `json:"shadow_decision_retention"`
	ElevationWeight         float64        `json:"elevation_weight"`
}

type rawConfig struct {
	Server     rawServerConfig     `json:"server"`
	Simulation rawSimulationConfig `json:"simulation"`
	Safety     rawSafetyConfig     `json:"safety"`
	Fragments  rawFragmentsConfig  `json:"fragments"`
	History    rawHistoryConfig    `json:"history"`
	PLC        rawPLCConfig        `json:"plc"`
	Guns       []rawGunConfig      `json:"guns"`
	SafetyPLC  rawSafetyPLCConfig  `json:"safety_plc"`
	TargetFeed rawTargetFeedConfig `json:"target_feed"`
	Dispatcher rawDispatcherConfig `json:"dispatcher"`
}

type rawServerConfig struct {
	Addr *string `json:"addr"`
}

type rawSimulationConfig struct {
	AutoStart                    *bool    `json:"auto_start"`
	GunCount                     *int     `json:"gun_count"`
	TickInterval                 *string  `json:"tick_interval"`
	TargetGenerationInterval     *string  `json:"target_generation_interval"`
	TargetTimeToImpactMinSeconds *int     `json:"target_time_to_impact_min_seconds"`
	TargetTimeToImpactMaxSeconds *int     `json:"target_time_to_impact_max_seconds"`
	SoftMinElevation             *float64 `json:"soft_min_elevation"`
	SoftMaxElevation             *float64 `json:"soft_max_elevation"`
	PostFireLockout              *string  `json:"post_fire_lockout"`
	SettlingDelay                *string  `json:"settling_delay"`
	SlewDegreesPerSecond         *float64 `json:"slew_degrees_per_second"`
	MinSlewDuration              *string  `json:"min_slew_duration"`
	MaxSlewDuration              *string  `json:"max_slew_duration"`
	MaxEvents                    *int     `json:"max_events"`
	FaultOnSafetyBlocked         *bool    `json:"fault_on_safety_blocked"`
}

type rawSafetyConfig struct {
	HardMinElevation       *float64 `json:"hard_min_elevation"`
	HardMaxElevation       *float64 `json:"hard_max_elevation"`
	SpecialMeasuresEnabled *bool    `json:"special_measures_enabled"`
}

type rawPLCConfig struct {
	Adapter                           *string `json:"adapter"`
	Host                              *string `json:"host"`
	Port                              *int    `json:"port"`
	UnitID                            *int    `json:"unit_id"`
	CommandAckTimeoutSeconds          *int    `json:"command_ack_timeout_seconds"`
	PollIntervalMS                    *int    `json:"poll_interval_ms"`
	LifecyclePollIntervalMS           *int    `json:"lifecycle_poll_interval_ms"`
	LifecycleCompletionTimeoutSeconds *int    `json:"lifecycle_completion_timeout_seconds"`
	LifecycleMaxConsecutiveReadErrors *int    `json:"lifecycle_max_consecutive_read_errors"`
	AutoReadinessPollIntervalMS       *int    `json:"auto_readiness_poll_interval_ms"`
	AutoSafetyFirePermissive          *bool   `json:"auto_safety_fire_permissive"`
	AutoTelemetryModbusEnabled        *bool   `json:"auto_telemetry_modbus_enabled"`
	AutoTelemetryModbusAddr           *string `json:"auto_telemetry_modbus_addr"`
	AutoTelemetryUnitID               *int    `json:"auto_telemetry_unit_id"`
	AutoTelemetryPollInterval         *string `json:"auto_telemetry_poll_interval"`
	AutoTelemetryStaleAfter           *string `json:"auto_telemetry_stale_after"`
	FaultResetTimeout                 *string `json:"fault_reset_timeout"`
	StartupMode                       *string `json:"startup_mode"`
	StartupModeRetryTimeout           *string `json:"startup_mode_retry_timeout"`
}

type rawGunConfig struct {
	ID                  *int         `json:"id"`
	Name                *string      `json:"name"`
	PLCEnabled          *bool        `json:"plc_enabled"`
	SafetyGunID         *int         `json:"safety_gun_id"`
	AutoEnabled         *bool        `json:"auto_enabled"`
	DispatcherEligible  *bool        `json:"dispatcher_eligible"`
	AutoExecutorEnabled *bool        `json:"auto_executor_enabled"`
	MinAzimuth          *float64     `json:"min_azimuth"`
	MaxAzimuth          *float64     `json:"max_azimuth"`
	MinElevation        *float64     `json:"min_elevation"`
	MaxElevation        *float64     `json:"max_elevation"`
	PLC                 rawPLCConfig `json:"plc"`
}

type rawSafetyPLCConfig struct {
	Enabled            *bool   `json:"enabled"`
	Enforcement        *bool   `json:"enforcement"`
	Host               *string `json:"host"`
	Port               *int    `json:"port"`
	UnitID             *int    `json:"unit_id"`
	PollInterval       *string `json:"poll_interval"`
	StaleAfter         *string `json:"stale_after"`
	StatusRegisterType *string `json:"status_register_type"`
	MWRegisterBase     *int    `json:"mw_register_base"`
	ResetPulseDuration *string `json:"reset_pulse_duration"`
}

type rawFragmentsConfig struct {
	Source      *string              `json:"source"`
	Ingest      rawIngestConfig      `json:"ingest"`
	InternalRNG rawInternalRNGConfig `json:"internal_rng"`
}

type rawIngestConfig struct {
	MaxTimeToImpactSeconds    *int `json:"max_time_to_impact_seconds"`
	DuplicateRetentionMinutes *int `json:"duplicate_retention_minutes"`
}

type rawInternalRNGConfig struct {
	Enabled         *bool `json:"enabled"`
	IntervalSeconds *int  `json:"interval_seconds"`
}

type rawHistoryConfig struct {
	Adapter                *string `json:"adapter"`
	MaxEvents              *int    `json:"max_events"`
	PostgresHost           *string `json:"postgres_host"`
	PostgresPort           *int    `json:"postgres_port"`
	PostgresDB             *string `json:"postgres_db"`
	PostgresUser           *string `json:"postgres_user"`
	PostgresPassword       *string `json:"postgres_password"`
	PostgresSSLMode        *string `json:"postgres_sslmode"`
	PostgresMaxConnections *int32  `json:"postgres_max_connections"`
	WriteQueueSize         *int    `json:"postgres_write_queue_size"`
	RetryIntervalSeconds   *int    `json:"postgres_retry_interval_seconds"`
	MigrationsPath         *string `json:"migrations_path"`
}

type rawTargetFeedConfig struct {
	Enabled                *bool   `json:"enabled"`
	MaxPending             *int    `json:"max_pending"`
	TargetTTL              *string `json:"target_ttl"`
	HealthyWindow          *string `json:"healthy_window"`
	DispatchPollIntervalMS *int    `json:"dispatch_poll_interval_ms"`
}

type rawDispatcherConfig struct {
	Enabled                 *bool    `json:"enabled"`
	Mode                    *string  `json:"mode"`
	PollIntervalMS          *int     `json:"poll_interval_ms"`
	AssignmentLeaseDuration *string  `json:"assignment_lease_duration"`
	LeaseRenewalInterval    *string  `json:"lease_renewal_interval"`
	ReconciliationInterval  *string  `json:"reconciliation_interval"`
	ShadowDecisionRetention *string  `json:"shadow_decision_retention"`
	ElevationWeight         *float64 `json:"elevation_weight"`
}

func Defaults() Config {
	return Config{
		Server: ServerConfig{Addr: ":8080"},
		Simulation: SimulationConfig{
			AutoStart:                    true,
			GunCount:                     8,
			TickInterval:                 250 * time.Millisecond,
			TargetGenerationInterval:     4 * time.Second,
			TargetTimeToImpactMinSeconds: 45,
			TargetTimeToImpactMaxSeconds: 120,
			SoftMinElevation:             5,
			SoftMaxElevation:             85,
			PostFireLockout:              10 * time.Second,
			SettlingDelay:                5 * time.Second,
			SlewDegreesPerSecond:         18,
			MinSlewDuration:              1 * time.Second,
			MaxSlewDuration:              12 * time.Second,
			MaxEvents:                    500,
			FaultOnSafetyBlocked:         false,
		},
		Safety: SafetyConfig{
			HardMinElevation:       0,
			HardMaxElevation:       90,
			SpecialMeasuresEnabled: false,
		},
		Fragments: FragmentsConfig{
			Source: "internal-rng",
			Ingest: IngestConfig{
				MaxTimeToImpactSeconds: 600,
				DuplicateRetention:     1440 * time.Minute,
			},
			InternalRNG: InternalRNGConfig{
				Enabled:  true,
				Interval: 4 * time.Second,
			},
		},
		History: HistoryConfig{
			Adapter:                "memory",
			MaxEvents:              500,
			PostgresHost:           "postgres",
			PostgresPort:           5432,
			PostgresDB:             "stonehenge",
			PostgresUser:           "stonehenge_writer",
			PostgresSSLMode:        "disable",
			PostgresMaxConnections: 10,
			WriteQueueSize:         1000,
			RetryInterval:          5 * time.Second,
			MigrationsPath:         "migrations",
		},
		PLC: PLCConfig{
			Adapter:                       "simulated",
			Host:                          "openplc-control-gun1",
			Port:                          502,
			UnitID:                        1,
			CommandAckTimeout:             5 * time.Second,
			PollInterval:                  100 * time.Millisecond,
			LifecyclePollInterval:         200 * time.Millisecond,
			LifecycleCompletionTimeout:    30 * time.Second,
			LifecycleMaxConsecutiveErrors: 5,
			AutoReadinessPollInterval:     500 * time.Millisecond,
			AutoSafetyFirePermissive:      false,
			AutoTelemetryModbusEnabled:    true,
			AutoTelemetryModbusAddr:       "0.0.0.0:1502",
			AutoTelemetryUnitID:           1,
			AutoTelemetryPollInterval:     500 * time.Millisecond,
			AutoTelemetryStaleAfter:       3 * time.Second,
			FaultResetTimeout:             30 * time.Second,
			StartupMode:                   "manual",
			StartupModeRetryTimeout:       30 * time.Second,
		},
		Guns: []GunConfig{{
			ID:          1,
			Name:        "Gun 1",
			PLCEnabled:  true,
			SafetyGunID: 1,
			AutoEnabled: true,
		}},
		SafetyPLC: SafetyPLCConfig{
			Enabled:            true,
			Enforcement:        false,
			Host:               "openplc-safety",
			Port:               502,
			UnitID:             1,
			PollInterval:       250 * time.Millisecond,
			StaleAfter:         2 * time.Second,
			StatusRegisterType: "input",
			MWRegisterBase:     80,
			ResetPulseDuration: 750 * time.Millisecond,
		},
		TargetFeed: TargetFeedConfig{
			Enabled:              true,
			MaxPending:           25,
			TargetTTL:            2 * time.Minute,
			HealthyWindow:        30 * time.Second,
			DispatchPollInterval: 500 * time.Millisecond,
		},
		Dispatcher: DispatcherConfig{
			Enabled:                 true,
			Mode:                    DispatcherModeActive,
			PollInterval:            500 * time.Millisecond,
			AssignmentLeaseDuration: 90 * time.Second,
			LeaseRenewalInterval:    30 * time.Second,
			ReconciliationInterval:  5 * time.Second,
			ShadowDecisionRetention: 10 * time.Minute,
			ElevationWeight:         1,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	if path == "" {
		path = "configs/config.example.json"
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return applyEnv(cfg)
	}
	if err != nil {
		return cfg, err
	}
	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return cfg, err
	}
	cfg, err = merge(cfg, raw)
	if err != nil {
		return cfg, err
	}
	return applyEnv(cfg)
}

func merge(cfg Config, raw rawConfig) (Config, error) {
	if raw.Server.Addr != nil {
		cfg.Server.Addr = *raw.Server.Addr
	}
	if raw.Simulation.AutoStart != nil {
		cfg.Simulation.AutoStart = *raw.Simulation.AutoStart
	}
	if raw.Simulation.GunCount != nil {
		cfg.Simulation.GunCount = *raw.Simulation.GunCount
	}
	if raw.Simulation.TickInterval != nil {
		d, err := time.ParseDuration(*raw.Simulation.TickInterval)
		if err != nil {
			return cfg, err
		}
		cfg.Simulation.TickInterval = d
	}
	if raw.Simulation.TargetGenerationInterval != nil {
		d, err := time.ParseDuration(*raw.Simulation.TargetGenerationInterval)
		if err != nil {
			return cfg, err
		}
		cfg.Simulation.TargetGenerationInterval = d
		cfg.Fragments.InternalRNG.Interval = d
	}
	if raw.Simulation.TargetTimeToImpactMinSeconds != nil {
		cfg.Simulation.TargetTimeToImpactMinSeconds = *raw.Simulation.TargetTimeToImpactMinSeconds
	}
	if raw.Simulation.TargetTimeToImpactMaxSeconds != nil {
		cfg.Simulation.TargetTimeToImpactMaxSeconds = *raw.Simulation.TargetTimeToImpactMaxSeconds
	}
	if raw.Simulation.SoftMinElevation != nil {
		cfg.Simulation.SoftMinElevation = *raw.Simulation.SoftMinElevation
	}
	if raw.Simulation.SoftMaxElevation != nil {
		cfg.Simulation.SoftMaxElevation = *raw.Simulation.SoftMaxElevation
	}
	if raw.Simulation.PostFireLockout != nil {
		d, err := time.ParseDuration(*raw.Simulation.PostFireLockout)
		if err != nil {
			return cfg, err
		}
		cfg.Simulation.PostFireLockout = d
	}
	if raw.Simulation.SettlingDelay != nil {
		d, err := time.ParseDuration(*raw.Simulation.SettlingDelay)
		if err != nil {
			return cfg, err
		}
		cfg.Simulation.SettlingDelay = d
	}
	if raw.Simulation.SlewDegreesPerSecond != nil {
		cfg.Simulation.SlewDegreesPerSecond = *raw.Simulation.SlewDegreesPerSecond
	}
	if raw.Simulation.MinSlewDuration != nil {
		d, err := time.ParseDuration(*raw.Simulation.MinSlewDuration)
		if err != nil {
			return cfg, err
		}
		cfg.Simulation.MinSlewDuration = d
	}
	if raw.Simulation.MaxSlewDuration != nil {
		d, err := time.ParseDuration(*raw.Simulation.MaxSlewDuration)
		if err != nil {
			return cfg, err
		}
		cfg.Simulation.MaxSlewDuration = d
	}
	if raw.Simulation.MaxEvents != nil {
		cfg.Simulation.MaxEvents = *raw.Simulation.MaxEvents
	}
	if raw.Simulation.FaultOnSafetyBlocked != nil {
		cfg.Simulation.FaultOnSafetyBlocked = *raw.Simulation.FaultOnSafetyBlocked
	}
	if raw.Safety.HardMinElevation != nil {
		cfg.Safety.HardMinElevation = *raw.Safety.HardMinElevation
	}
	if raw.Safety.HardMaxElevation != nil {
		cfg.Safety.HardMaxElevation = *raw.Safety.HardMaxElevation
	}
	if raw.Safety.SpecialMeasuresEnabled != nil {
		cfg.Safety.SpecialMeasuresEnabled = *raw.Safety.SpecialMeasuresEnabled
	}
	if raw.PLC.Adapter != nil {
		cfg.PLC.Adapter = *raw.PLC.Adapter
	}
	if raw.PLC.Host != nil {
		cfg.PLC.Host = *raw.PLC.Host
	}
	if raw.PLC.Port != nil {
		cfg.PLC.Port = *raw.PLC.Port
	}
	if raw.PLC.UnitID != nil {
		if *raw.PLC.UnitID < 0 || *raw.PLC.UnitID > 255 {
			return cfg, errors.New("plc.unit_id must be between 0 and 255")
		}
		cfg.PLC.UnitID = byte(*raw.PLC.UnitID)
	}
	if raw.PLC.CommandAckTimeoutSeconds != nil {
		if *raw.PLC.CommandAckTimeoutSeconds <= 0 {
			return cfg, errors.New("plc.command_ack_timeout_seconds must be positive")
		}
		cfg.PLC.CommandAckTimeout = time.Duration(*raw.PLC.CommandAckTimeoutSeconds) * time.Second
	}
	if raw.PLC.PollIntervalMS != nil {
		if *raw.PLC.PollIntervalMS <= 0 {
			return cfg, errors.New("plc.poll_interval_ms must be positive")
		}
		cfg.PLC.PollInterval = time.Duration(*raw.PLC.PollIntervalMS) * time.Millisecond
	}
	if raw.PLC.LifecyclePollIntervalMS != nil {
		if *raw.PLC.LifecyclePollIntervalMS <= 0 {
			return cfg, errors.New("plc.lifecycle_poll_interval_ms must be positive")
		}
		cfg.PLC.LifecyclePollInterval = time.Duration(*raw.PLC.LifecyclePollIntervalMS) * time.Millisecond
	}
	if raw.PLC.LifecycleCompletionTimeoutSeconds != nil {
		if *raw.PLC.LifecycleCompletionTimeoutSeconds <= 0 {
			return cfg, errors.New("plc.lifecycle_completion_timeout_seconds must be positive")
		}
		cfg.PLC.LifecycleCompletionTimeout = time.Duration(*raw.PLC.LifecycleCompletionTimeoutSeconds) * time.Second
	}
	if raw.PLC.LifecycleMaxConsecutiveReadErrors != nil {
		if *raw.PLC.LifecycleMaxConsecutiveReadErrors <= 0 {
			return cfg, errors.New("plc.lifecycle_max_consecutive_read_errors must be positive")
		}
		cfg.PLC.LifecycleMaxConsecutiveErrors = *raw.PLC.LifecycleMaxConsecutiveReadErrors
	}
	if raw.PLC.AutoReadinessPollIntervalMS != nil {
		if *raw.PLC.AutoReadinessPollIntervalMS <= 0 {
			return cfg, errors.New("plc.auto_readiness_poll_interval_ms must be positive")
		}
		cfg.PLC.AutoReadinessPollInterval = time.Duration(*raw.PLC.AutoReadinessPollIntervalMS) * time.Millisecond
	}
	if raw.PLC.AutoSafetyFirePermissive != nil {
		cfg.PLC.AutoSafetyFirePermissive = *raw.PLC.AutoSafetyFirePermissive
	}
	if raw.PLC.AutoTelemetryModbusEnabled != nil {
		cfg.PLC.AutoTelemetryModbusEnabled = *raw.PLC.AutoTelemetryModbusEnabled
	}
	if raw.PLC.AutoTelemetryModbusAddr != nil {
		cfg.PLC.AutoTelemetryModbusAddr = *raw.PLC.AutoTelemetryModbusAddr
	}
	if raw.PLC.AutoTelemetryUnitID != nil {
		if *raw.PLC.AutoTelemetryUnitID < 0 || *raw.PLC.AutoTelemetryUnitID > 255 {
			return cfg, errors.New("plc.auto_telemetry_unit_id must be between 0 and 255")
		}
		cfg.PLC.AutoTelemetryUnitID = byte(*raw.PLC.AutoTelemetryUnitID)
	}
	if raw.PLC.AutoTelemetryPollInterval != nil {
		d, err := time.ParseDuration(*raw.PLC.AutoTelemetryPollInterval)
		if err != nil {
			return cfg, fmt.Errorf("plc.auto_telemetry_poll_interval: %w", err)
		}
		if d <= 0 {
			return cfg, errors.New("plc.auto_telemetry_poll_interval must be positive")
		}
		cfg.PLC.AutoTelemetryPollInterval = d
	}
	if raw.PLC.AutoTelemetryStaleAfter != nil {
		d, err := time.ParseDuration(*raw.PLC.AutoTelemetryStaleAfter)
		if err != nil {
			return cfg, fmt.Errorf("plc.auto_telemetry_stale_after: %w", err)
		}
		if d <= 0 {
			return cfg, errors.New("plc.auto_telemetry_stale_after must be positive")
		}
		cfg.PLC.AutoTelemetryStaleAfter = d
	}
	if raw.PLC.FaultResetTimeout != nil {
		d, err := time.ParseDuration(*raw.PLC.FaultResetTimeout)
		if err != nil {
			return cfg, fmt.Errorf("plc.fault_reset_timeout: %w", err)
		}
		if d <= 0 {
			return cfg, errors.New("plc.fault_reset_timeout must be positive")
		}
		cfg.PLC.FaultResetTimeout = d
	}
	if raw.PLC.StartupMode != nil {
		cfg.PLC.StartupMode = strings.ToLower(*raw.PLC.StartupMode)
	}
	if raw.PLC.StartupModeRetryTimeout != nil {
		d, err := time.ParseDuration(*raw.PLC.StartupModeRetryTimeout)
		if err != nil {
			return cfg, fmt.Errorf("plc.startup_mode_retry_timeout: %w", err)
		}
		if d < 0 {
			return cfg, errors.New("plc.startup_mode_retry_timeout must be non-negative")
		}
		cfg.PLC.StartupModeRetryTimeout = d
	}
	if raw.Guns != nil {
		guns, err := mergeGunConfigs(cfg.PLC, raw.Guns)
		if err != nil {
			return cfg, err
		}
		cfg.Guns = guns
	}
	var safetyErr error
	cfg, safetyErr = mergeSafetyPLC(cfg, raw.SafetyPLC)
	if safetyErr != nil {
		return cfg, safetyErr
	}
	switch cfg.PLC.Adapter {
	case "simulated":
	default:
		return cfg, errors.New("plc.adapter must be simulated")
	}
	if cfg.PLC.Host == "" {
		return cfg, errors.New("plc.host must not be empty")
	}
	if cfg.PLC.Port <= 0 || cfg.PLC.Port > 65535 {
		return cfg, errors.New("plc.port must be between 1 and 65535")
	}
	if cfg.PLC.CommandAckTimeout <= 0 {
		return cfg, errors.New("plc.command_ack_timeout must be positive")
	}
	if cfg.PLC.PollInterval <= 0 {
		return cfg, errors.New("plc.poll_interval must be positive")
	}
	if cfg.PLC.LifecyclePollInterval <= 0 {
		return cfg, errors.New("plc.lifecycle_poll_interval must be positive")
	}
	if cfg.PLC.LifecycleCompletionTimeout <= 0 {
		return cfg, errors.New("plc.lifecycle_completion_timeout must be positive")
	}
	if cfg.PLC.LifecycleMaxConsecutiveErrors <= 0 {
		return cfg, errors.New("plc.lifecycle_max_consecutive_read_errors must be positive")
	}
	if cfg.PLC.AutoReadinessPollInterval <= 0 {
		return cfg, errors.New("plc.auto_readiness_poll_interval must be positive")
	}
	if cfg.PLC.AutoTelemetryModbusAddr == "" {
		return cfg, errors.New("plc.auto_telemetry_modbus_addr must not be empty")
	}
	if cfg.PLC.AutoTelemetryPollInterval <= 0 {
		return cfg, errors.New("plc.auto_telemetry_poll_interval must be positive")
	}
	if cfg.PLC.AutoTelemetryStaleAfter <= 0 {
		return cfg, errors.New("plc.auto_telemetry_stale_after must be positive")
	}
	if cfg.PLC.FaultResetTimeout <= 0 {
		return cfg, errors.New("plc.fault_reset_timeout must be positive")
	}
	switch cfg.PLC.StartupMode {
	case "manual", "auto", "maintenance":
	default:
		return cfg, errors.New("plc.startup_mode must be manual, auto, or maintenance")
	}
	if cfg.PLC.StartupModeRetryTimeout < 0 {
		return cfg, errors.New("plc.startup_mode_retry_timeout must be non-negative")
	}
	cfg = syncGunDefaults(cfg)
	if err := validateGunConfigs(cfg.Guns); err != nil {
		return cfg, err
	}
	if raw.Fragments.Source != nil {
		cfg.Fragments.Source = *raw.Fragments.Source
	}
	switch cfg.Fragments.Source {
	case "internal-rng", "ingest":
	default:
		return cfg, errors.New("fragments.source must be internal-rng or ingest")
	}
	if raw.Fragments.Ingest.MaxTimeToImpactSeconds != nil {
		cfg.Fragments.Ingest.MaxTimeToImpactSeconds = *raw.Fragments.Ingest.MaxTimeToImpactSeconds
	}
	if cfg.Fragments.Ingest.MaxTimeToImpactSeconds <= 0 {
		return cfg, errors.New("fragments.ingest.max_time_to_impact_seconds must be positive")
	}
	if raw.Fragments.Ingest.DuplicateRetentionMinutes != nil {
		if *raw.Fragments.Ingest.DuplicateRetentionMinutes <= 0 {
			return cfg, errors.New("fragments.ingest.duplicate_retention_minutes must be positive")
		}
		cfg.Fragments.Ingest.DuplicateRetention = time.Duration(*raw.Fragments.Ingest.DuplicateRetentionMinutes) * time.Minute
	}
	if raw.Fragments.InternalRNG.Enabled != nil {
		cfg.Fragments.InternalRNG.Enabled = *raw.Fragments.InternalRNG.Enabled
	}
	if raw.Fragments.InternalRNG.IntervalSeconds != nil {
		if *raw.Fragments.InternalRNG.IntervalSeconds <= 0 {
			return cfg, errors.New("fragments.internal_rng.interval_seconds must be positive")
		}
		cfg.Fragments.InternalRNG.Interval = time.Duration(*raw.Fragments.InternalRNG.IntervalSeconds) * time.Second
		cfg.Simulation.TargetGenerationInterval = cfg.Fragments.InternalRNG.Interval
	}
	if raw.History.Adapter != nil {
		cfg.History.Adapter = *raw.History.Adapter
	}
	switch cfg.History.Adapter {
	case "memory", "noop", "postgres":
	default:
		return cfg, errors.New("history.adapter must be memory, noop, or postgres")
	}
	if raw.History.MaxEvents != nil {
		cfg.History.MaxEvents = *raw.History.MaxEvents
		cfg.Simulation.MaxEvents = *raw.History.MaxEvents
	}
	if raw.History.PostgresHost != nil {
		cfg.History.PostgresHost = *raw.History.PostgresHost
	}
	if raw.History.PostgresPort != nil {
		cfg.History.PostgresPort = *raw.History.PostgresPort
	}
	if raw.History.PostgresDB != nil {
		cfg.History.PostgresDB = *raw.History.PostgresDB
	}
	if raw.History.PostgresUser != nil {
		cfg.History.PostgresUser = *raw.History.PostgresUser
	}
	if raw.History.PostgresPassword != nil {
		cfg.History.PostgresPassword = *raw.History.PostgresPassword
	}
	if raw.History.PostgresSSLMode != nil {
		cfg.History.PostgresSSLMode = *raw.History.PostgresSSLMode
	}
	if raw.History.PostgresMaxConnections != nil {
		cfg.History.PostgresMaxConnections = *raw.History.PostgresMaxConnections
	}
	if raw.History.WriteQueueSize != nil {
		cfg.History.WriteQueueSize = *raw.History.WriteQueueSize
	}
	if raw.History.RetryIntervalSeconds != nil {
		cfg.History.RetryInterval = time.Duration(*raw.History.RetryIntervalSeconds) * time.Second
	}
	if raw.History.MigrationsPath != nil {
		cfg.History.MigrationsPath = *raw.History.MigrationsPath
	}
	if cfg.History.MaxEvents <= 0 {
		return cfg, errors.New("history.max_events must be positive")
	}
	if cfg.History.PostgresPort <= 0 {
		return cfg, errors.New("history.postgres_port must be positive")
	}
	if cfg.History.PostgresMaxConnections <= 0 {
		return cfg, errors.New("history.postgres_max_connections must be positive")
	}
	if cfg.History.WriteQueueSize <= 0 {
		return cfg, errors.New("history.postgres_write_queue_size must be positive")
	}
	if cfg.History.RetryInterval <= 0 {
		return cfg, errors.New("history.postgres_retry_interval_seconds must be positive")
	}
	if raw.TargetFeed.Enabled != nil {
		cfg.TargetFeed.Enabled = *raw.TargetFeed.Enabled
	}
	if raw.TargetFeed.MaxPending != nil {
		cfg.TargetFeed.MaxPending = *raw.TargetFeed.MaxPending
	}
	if raw.TargetFeed.TargetTTL != nil {
		d, err := time.ParseDuration(*raw.TargetFeed.TargetTTL)
		if err != nil {
			return cfg, fmt.Errorf("target_feed.target_ttl: %w", err)
		}
		cfg.TargetFeed.TargetTTL = d
	}
	if raw.TargetFeed.HealthyWindow != nil {
		d, err := time.ParseDuration(*raw.TargetFeed.HealthyWindow)
		if err != nil {
			return cfg, fmt.Errorf("target_feed.healthy_window: %w", err)
		}
		cfg.TargetFeed.HealthyWindow = d
	}
	if raw.TargetFeed.DispatchPollIntervalMS != nil {
		cfg.TargetFeed.DispatchPollInterval = time.Duration(*raw.TargetFeed.DispatchPollIntervalMS) * time.Millisecond
	}
	if cfg.TargetFeed.MaxPending <= 0 {
		return cfg, errors.New("target_feed.max_pending must be positive")
	}
	if cfg.TargetFeed.TargetTTL <= 0 {
		return cfg, errors.New("target_feed.target_ttl must be positive")
	}
	if cfg.TargetFeed.HealthyWindow <= 0 {
		return cfg, errors.New("target_feed.healthy_window must be positive")
	}
	if cfg.TargetFeed.DispatchPollInterval <= 0 {
		return cfg, errors.New("target_feed.dispatch_poll_interval must be positive")
	}
	var dispatcherErr error
	cfg, dispatcherErr = mergeDispatcher(cfg, raw.Dispatcher)
	if dispatcherErr != nil {
		return cfg, dispatcherErr
	}
	return cfg, nil
}

func applyEnv(cfg Config) (Config, error) {
	if v := os.Getenv("HISTORY_ADAPTER"); v != "" {
		cfg.History.Adapter = v
	}
	if v := os.Getenv("POSTGRES_HOST"); v != "" {
		cfg.History.PostgresHost = v
	}
	if v := os.Getenv("POSTGRES_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("POSTGRES_PORT: %w", err)
		}
		cfg.History.PostgresPort = n
	}
	if v := os.Getenv("POSTGRES_DB"); v != "" {
		cfg.History.PostgresDB = v
	}
	if v := os.Getenv("POSTGRES_USER"); v != "" {
		cfg.History.PostgresUser = v
	}
	if v, ok := os.LookupEnv("POSTGRES_PASSWORD"); ok {
		cfg.History.PostgresPassword = v
	}
	if v := os.Getenv("POSTGRES_SSLMODE"); v != "" {
		cfg.History.PostgresSSLMode = v
	}
	if v := os.Getenv("POSTGRES_MAX_CONNECTIONS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("POSTGRES_MAX_CONNECTIONS: %w", err)
		}
		cfg.History.PostgresMaxConnections = int32(n)
	}
	if v := os.Getenv("POSTGRES_WRITE_QUEUE_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("POSTGRES_WRITE_QUEUE_SIZE: %w", err)
		}
		cfg.History.WriteQueueSize = n
	}
	if v := os.Getenv("POSTGRES_RETRY_INTERVAL_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("POSTGRES_RETRY_INTERVAL_SECONDS: %w", err)
		}
		cfg.History.RetryInterval = time.Duration(n) * time.Second
	}
	if v := os.Getenv("PLC_HOST"); v != "" {
		cfg.PLC.Host = v
	}
	if v := os.Getenv("PLC_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_PORT: %w", err)
		}
		cfg.PLC.Port = n
	}
	if v := os.Getenv("PLC_UNIT_ID"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_UNIT_ID: %w", err)
		}
		if n < 0 || n > 255 {
			return cfg, errors.New("PLC_UNIT_ID must be between 0 and 255")
		}
		cfg.PLC.UnitID = byte(n)
	}
	if v := os.Getenv("PLC_COMMAND_ACK_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_COMMAND_ACK_TIMEOUT_SECONDS: %w", err)
		}
		cfg.PLC.CommandAckTimeout = time.Duration(n) * time.Second
	}
	if v := os.Getenv("PLC_POLL_INTERVAL_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_POLL_INTERVAL_MS: %w", err)
		}
		cfg.PLC.PollInterval = time.Duration(n) * time.Millisecond
	}
	if v := os.Getenv("PLC_LIFECYCLE_POLL_INTERVAL_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_LIFECYCLE_POLL_INTERVAL_MS: %w", err)
		}
		cfg.PLC.LifecyclePollInterval = time.Duration(n) * time.Millisecond
	}
	if v := os.Getenv("PLC_LIFECYCLE_COMPLETION_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_LIFECYCLE_COMPLETION_TIMEOUT_SECONDS: %w", err)
		}
		cfg.PLC.LifecycleCompletionTimeout = time.Duration(n) * time.Second
	}
	if v := os.Getenv("PLC_LIFECYCLE_MAX_CONSECUTIVE_READ_ERRORS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_LIFECYCLE_MAX_CONSECUTIVE_READ_ERRORS: %w", err)
		}
		cfg.PLC.LifecycleMaxConsecutiveErrors = n
	}
	if v := os.Getenv("PLC_AUTO_READINESS_POLL_INTERVAL_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_AUTO_READINESS_POLL_INTERVAL_MS: %w", err)
		}
		cfg.PLC.AutoReadinessPollInterval = time.Duration(n) * time.Millisecond
	}
	if v := os.Getenv("PLC_AUTO_SAFETY_FIRE_PERMISSIVE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_AUTO_SAFETY_FIRE_PERMISSIVE: %w", err)
		}
		cfg.PLC.AutoSafetyFirePermissive = b
	}
	if v := os.Getenv("PLC_AUTO_TELEMETRY_MODBUS_ENABLED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_AUTO_TELEMETRY_MODBUS_ENABLED: %w", err)
		}
		cfg.PLC.AutoTelemetryModbusEnabled = b
	}
	if v := os.Getenv("PLC_AUTO_TELEMETRY_MODBUS_ADDR"); v != "" {
		cfg.PLC.AutoTelemetryModbusAddr = v
	}
	if v := os.Getenv("PLC_AUTO_TELEMETRY_UNIT_ID"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_AUTO_TELEMETRY_UNIT_ID: %w", err)
		}
		if n < 0 || n > 255 {
			return cfg, errors.New("PLC_AUTO_TELEMETRY_UNIT_ID must be between 0 and 255")
		}
		cfg.PLC.AutoTelemetryUnitID = byte(n)
	}
	if v := os.Getenv("PLC_AUTO_TELEMETRY_POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_AUTO_TELEMETRY_POLL_INTERVAL: %w", err)
		}
		cfg.PLC.AutoTelemetryPollInterval = d
	}
	if v := os.Getenv("PLC_AUTO_TELEMETRY_STALE_AFTER"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_AUTO_TELEMETRY_STALE_AFTER: %w", err)
		}
		cfg.PLC.AutoTelemetryStaleAfter = d
	}
	if v := os.Getenv("PLC_FAULT_RESET_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_FAULT_RESET_TIMEOUT: %w", err)
		}
		cfg.PLC.FaultResetTimeout = d
	}
	if v := os.Getenv("PLC_STARTUP_MODE"); v != "" {
		cfg.PLC.StartupMode = strings.ToLower(v)
	}
	if v := os.Getenv("PLC_STARTUP_MODE_RETRY_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("PLC_STARTUP_MODE_RETRY_TIMEOUT: %w", err)
		}
		cfg.PLC.StartupModeRetryTimeout = d
	}
	if v := os.Getenv("GUNS_CONFIG_JSON"); v != "" {
		var rawGuns []rawGunConfig
		if err := json.Unmarshal([]byte(v), &rawGuns); err != nil {
			return cfg, fmt.Errorf("GUNS_CONFIG_JSON: %w", err)
		}
		guns, err := mergeGunConfigs(cfg.PLC, rawGuns)
		if err != nil {
			return cfg, fmt.Errorf("GUNS_CONFIG_JSON: %w", err)
		}
		cfg.Guns = guns
	}
	if v := os.Getenv("SAFETY_PLC_ENABLED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("SAFETY_PLC_ENABLED: %w", err)
		}
		cfg.SafetyPLC.Enabled = b
	}
	if v := os.Getenv("SAFETY_PLC_ENFORCEMENT"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("SAFETY_PLC_ENFORCEMENT: %w", err)
		}
		cfg.SafetyPLC.Enforcement = b
	}
	if v := os.Getenv("SAFETY_PLC_HOST"); v != "" {
		cfg.SafetyPLC.Host = v
	}
	if v := os.Getenv("SAFETY_PLC_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("SAFETY_PLC_PORT: %w", err)
		}
		cfg.SafetyPLC.Port = n
	}
	if v := os.Getenv("SAFETY_PLC_UNIT_ID"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("SAFETY_PLC_UNIT_ID: %w", err)
		}
		if n < 0 || n > 255 {
			return cfg, errors.New("SAFETY_PLC_UNIT_ID must be between 0 and 255")
		}
		cfg.SafetyPLC.UnitID = byte(n)
	}
	if v := os.Getenv("SAFETY_PLC_POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("SAFETY_PLC_POLL_INTERVAL: %w", err)
		}
		cfg.SafetyPLC.PollInterval = d
	}
	if v := os.Getenv("SAFETY_PLC_STALE_AFTER"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("SAFETY_PLC_STALE_AFTER: %w", err)
		}
		cfg.SafetyPLC.StaleAfter = d
	}
	if v := os.Getenv("SAFETY_PLC_STATUS_REGISTER_TYPE"); v != "" {
		cfg.SafetyPLC.StatusRegisterType = strings.ToLower(v)
	}
	if v := os.Getenv("SAFETY_PLC_MW_REGISTER_BASE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("SAFETY_PLC_MW_REGISTER_BASE: %w", err)
		}
		if n < 0 || n > 65535 {
			return cfg, errors.New("SAFETY_PLC_MW_REGISTER_BASE must be between 0 and 65535")
		}
		cfg.SafetyPLC.MWRegisterBase = uint16(n)
	}
	if v := os.Getenv("SAFETY_PLC_RESET_PULSE_DURATION"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("SAFETY_PLC_RESET_PULSE_DURATION: %w", err)
		}
		cfg.SafetyPLC.ResetPulseDuration = d
	}
	if v := os.Getenv("TARGET_FEED_ENABLED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("TARGET_FEED_ENABLED: %w", err)
		}
		cfg.TargetFeed.Enabled = b
	}
	if v := os.Getenv("TARGET_FEED_MAX_PENDING"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("TARGET_FEED_MAX_PENDING: %w", err)
		}
		cfg.TargetFeed.MaxPending = n
	}
	if v := os.Getenv("TARGET_FEED_TARGET_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("TARGET_FEED_TARGET_TTL: %w", err)
		}
		cfg.TargetFeed.TargetTTL = d
	}
	if v := os.Getenv("TARGET_FEED_HEALTHY_WINDOW"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("TARGET_FEED_HEALTHY_WINDOW: %w", err)
		}
		cfg.TargetFeed.HealthyWindow = d
	}
	if v := os.Getenv("TARGET_FEED_DISPATCH_POLL_INTERVAL_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("TARGET_FEED_DISPATCH_POLL_INTERVAL_MS: %w", err)
		}
		cfg.TargetFeed.DispatchPollInterval = time.Duration(n) * time.Millisecond
	}
	if v := os.Getenv("DISPATCHER_ENABLED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("DISPATCHER_ENABLED: %w", err)
		}
		cfg.Dispatcher.Enabled = b
	}
	if v := os.Getenv("DISPATCHER_MODE"); v != "" {
		cfg.Dispatcher.Mode = DispatcherMode(strings.ToLower(v))
	}
	if v := os.Getenv("DISPATCHER_POLL_INTERVAL_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("DISPATCHER_POLL_INTERVAL_MS: %w", err)
		}
		cfg.Dispatcher.PollInterval = time.Duration(n) * time.Millisecond
	}
	if v := os.Getenv("DISPATCHER_ASSIGNMENT_LEASE_DURATION"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("DISPATCHER_ASSIGNMENT_LEASE_DURATION: %w", err)
		}
		cfg.Dispatcher.AssignmentLeaseDuration = d
	}
	if v := os.Getenv("DISPATCHER_LEASE_RENEWAL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("DISPATCHER_LEASE_RENEWAL_INTERVAL: %w", err)
		}
		cfg.Dispatcher.LeaseRenewalInterval = d
	}
	if v := os.Getenv("DISPATCHER_RECONCILIATION_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("DISPATCHER_RECONCILIATION_INTERVAL: %w", err)
		}
		cfg.Dispatcher.ReconciliationInterval = d
	}
	if v := os.Getenv("DISPATCHER_SHADOW_DECISION_RETENTION"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("DISPATCHER_SHADOW_DECISION_RETENTION: %w", err)
		}
		cfg.Dispatcher.ShadowDecisionRetention = d
	}
	if v := os.Getenv("DISPATCHER_ELEVATION_WEIGHT"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return cfg, fmt.Errorf("DISPATCHER_ELEVATION_WEIGHT: %w", err)
		}
		cfg.Dispatcher.ElevationWeight = f
	}
	switch cfg.History.Adapter {
	case "memory", "noop", "postgres":
	default:
		return cfg, errors.New("history.adapter must be memory, noop, or postgres")
	}
	if cfg.PLC.Host == "" {
		return cfg, errors.New("plc.host must not be empty")
	}
	if cfg.PLC.Port <= 0 || cfg.PLC.Port > 65535 {
		return cfg, errors.New("plc.port must be between 1 and 65535")
	}
	if cfg.PLC.CommandAckTimeout <= 0 {
		return cfg, errors.New("plc.command_ack_timeout must be positive")
	}
	if cfg.PLC.PollInterval <= 0 {
		return cfg, errors.New("plc.poll_interval must be positive")
	}
	if cfg.PLC.LifecyclePollInterval <= 0 {
		return cfg, errors.New("plc.lifecycle_poll_interval must be positive")
	}
	if cfg.PLC.LifecycleCompletionTimeout <= 0 {
		return cfg, errors.New("plc.lifecycle_completion_timeout must be positive")
	}
	if cfg.PLC.LifecycleMaxConsecutiveErrors <= 0 {
		return cfg, errors.New("PLC_LIFECYCLE_MAX_CONSECUTIVE_READ_ERRORS must be positive")
	}
	if cfg.PLC.AutoReadinessPollInterval <= 0 {
		return cfg, errors.New("PLC_AUTO_READINESS_POLL_INTERVAL_MS must be positive")
	}
	if cfg.PLC.AutoTelemetryModbusAddr == "" {
		return cfg, errors.New("PLC_AUTO_TELEMETRY_MODBUS_ADDR must not be empty")
	}
	if cfg.PLC.AutoTelemetryPollInterval <= 0 {
		return cfg, errors.New("PLC_AUTO_TELEMETRY_POLL_INTERVAL must be positive")
	}
	if cfg.PLC.AutoTelemetryStaleAfter <= 0 {
		return cfg, errors.New("PLC_AUTO_TELEMETRY_STALE_AFTER must be positive")
	}
	if cfg.PLC.FaultResetTimeout <= 0 {
		return cfg, errors.New("PLC_FAULT_RESET_TIMEOUT must be positive")
	}
	switch cfg.PLC.StartupMode {
	case "manual", "auto", "maintenance":
	default:
		return cfg, errors.New("PLC_STARTUP_MODE must be manual, auto, or maintenance")
	}
	if cfg.PLC.StartupModeRetryTimeout < 0 {
		return cfg, errors.New("PLC_STARTUP_MODE_RETRY_TIMEOUT must be non-negative")
	}
	cfg = syncGunDefaults(cfg)
	if err := validateGunConfigs(cfg.Guns); err != nil {
		return cfg, err
	}
	var err error
	cfg, err = validateSafetyPLC(cfg)
	if err != nil {
		return cfg, err
	}
	if cfg.TargetFeed.MaxPending <= 0 {
		return cfg, errors.New("TARGET_FEED_MAX_PENDING must be positive")
	}
	if cfg.TargetFeed.TargetTTL <= 0 {
		return cfg, errors.New("TARGET_FEED_TARGET_TTL must be positive")
	}
	if cfg.TargetFeed.HealthyWindow <= 0 {
		return cfg, errors.New("TARGET_FEED_HEALTHY_WINDOW must be positive")
	}
	if cfg.TargetFeed.DispatchPollInterval <= 0 {
		return cfg, errors.New("TARGET_FEED_DISPATCH_POLL_INTERVAL_MS must be positive")
	}
	if err := validateDispatcher(cfg.Dispatcher); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func mergeDispatcher(cfg Config, raw rawDispatcherConfig) (Config, error) {
	if raw.Enabled != nil {
		cfg.Dispatcher.Enabled = *raw.Enabled
	}
	if raw.Mode != nil {
		cfg.Dispatcher.Mode = DispatcherMode(strings.ToLower(*raw.Mode))
	}
	if raw.PollIntervalMS != nil {
		cfg.Dispatcher.PollInterval = time.Duration(*raw.PollIntervalMS) * time.Millisecond
	}
	if raw.AssignmentLeaseDuration != nil {
		d, err := time.ParseDuration(*raw.AssignmentLeaseDuration)
		if err != nil {
			return cfg, fmt.Errorf("dispatcher.assignment_lease_duration: %w", err)
		}
		cfg.Dispatcher.AssignmentLeaseDuration = d
	}
	if raw.LeaseRenewalInterval != nil {
		d, err := time.ParseDuration(*raw.LeaseRenewalInterval)
		if err != nil {
			return cfg, fmt.Errorf("dispatcher.lease_renewal_interval: %w", err)
		}
		cfg.Dispatcher.LeaseRenewalInterval = d
	}
	if raw.ReconciliationInterval != nil {
		d, err := time.ParseDuration(*raw.ReconciliationInterval)
		if err != nil {
			return cfg, fmt.Errorf("dispatcher.reconciliation_interval: %w", err)
		}
		cfg.Dispatcher.ReconciliationInterval = d
	}
	if raw.ShadowDecisionRetention != nil {
		d, err := time.ParseDuration(*raw.ShadowDecisionRetention)
		if err != nil {
			return cfg, fmt.Errorf("dispatcher.shadow_decision_retention: %w", err)
		}
		cfg.Dispatcher.ShadowDecisionRetention = d
	}
	if raw.ElevationWeight != nil {
		cfg.Dispatcher.ElevationWeight = *raw.ElevationWeight
	}
	return cfg, validateDispatcher(cfg.Dispatcher)
}

func validateDispatcher(cfg DispatcherConfig) error {
	switch cfg.Mode {
	case DispatcherModeDisabled, DispatcherModeShadow, DispatcherModeActive:
	default:
		return errors.New("dispatcher.mode must be disabled, shadow, or active")
	}
	if cfg.PollInterval <= 0 {
		return errors.New("dispatcher.poll_interval_ms must be positive")
	}
	if cfg.AssignmentLeaseDuration <= 0 {
		return errors.New("dispatcher.assignment_lease_duration must be positive")
	}
	if cfg.LeaseRenewalInterval <= 0 {
		return errors.New("dispatcher.lease_renewal_interval must be positive")
	}
	if cfg.AssignmentLeaseDuration <= cfg.LeaseRenewalInterval {
		return errors.New("dispatcher.assignment_lease_duration must be greater than dispatcher.lease_renewal_interval")
	}
	if cfg.AssignmentLeaseDuration-cfg.LeaseRenewalInterval < 5*time.Second {
		return errors.New("dispatcher assignment lease duration must leave at least 5s renewal margin")
	}
	if cfg.ReconciliationInterval <= 0 {
		return errors.New("dispatcher.reconciliation_interval must be positive")
	}
	if cfg.ShadowDecisionRetention <= 0 {
		return errors.New("dispatcher.shadow_decision_retention must be positive")
	}
	if cfg.ElevationWeight < 0 {
		return errors.New("dispatcher.elevation_weight must be non-negative")
	}
	return nil
}

func mergeSafetyPLC(cfg Config, raw rawSafetyPLCConfig) (Config, error) {
	if raw.Enabled != nil {
		cfg.SafetyPLC.Enabled = *raw.Enabled
	}
	if raw.Enforcement != nil {
		cfg.SafetyPLC.Enforcement = *raw.Enforcement
	}
	if raw.Host != nil {
		cfg.SafetyPLC.Host = *raw.Host
	}
	if raw.Port != nil {
		cfg.SafetyPLC.Port = *raw.Port
	}
	if raw.UnitID != nil {
		if *raw.UnitID < 0 || *raw.UnitID > 255 {
			return cfg, errors.New("safety_plc.unit_id must be between 0 and 255")
		}
		cfg.SafetyPLC.UnitID = byte(*raw.UnitID)
	}
	if raw.PollInterval != nil {
		d, err := time.ParseDuration(*raw.PollInterval)
		if err != nil {
			return cfg, fmt.Errorf("safety_plc.poll_interval: %w", err)
		}
		cfg.SafetyPLC.PollInterval = d
	}
	if raw.StaleAfter != nil {
		d, err := time.ParseDuration(*raw.StaleAfter)
		if err != nil {
			return cfg, fmt.Errorf("safety_plc.stale_after: %w", err)
		}
		cfg.SafetyPLC.StaleAfter = d
	}
	if raw.StatusRegisterType != nil {
		cfg.SafetyPLC.StatusRegisterType = strings.ToLower(*raw.StatusRegisterType)
	}
	if raw.MWRegisterBase != nil {
		if *raw.MWRegisterBase < 0 || *raw.MWRegisterBase > 65535 {
			return cfg, errors.New("safety_plc.mw_register_base must be between 0 and 65535")
		}
		cfg.SafetyPLC.MWRegisterBase = uint16(*raw.MWRegisterBase)
	}
	if raw.ResetPulseDuration != nil {
		d, err := time.ParseDuration(*raw.ResetPulseDuration)
		if err != nil {
			return cfg, fmt.Errorf("safety_plc.reset_pulse_duration: %w", err)
		}
		cfg.SafetyPLC.ResetPulseDuration = d
	}
	return validateSafetyPLC(cfg)
}

func mergeGunConfigs(base PLCConfig, raws []rawGunConfig) ([]GunConfig, error) {
	if len(raws) == 0 {
		return nil, errors.New("guns must contain at least one gun")
	}
	guns := make([]GunConfig, 0, len(raws))
	for i, raw := range raws {
		gun := GunConfig{
			ID:                  i + 1,
			Name:                fmt.Sprintf("Gun %d", i+1),
			PLCEnabled:          true,
			SafetyGunID:         i + 1,
			AutoEnabled:         i == 0,
			DispatcherEligible:  i == 0,
			AutoExecutorEnabled: i == 0,
			MinAzimuth:          0,
			MaxAzimuth:          360,
			MinElevation:        5,
			MaxElevation:        85,
			PLC:                 base,
		}
		if raw.ID != nil {
			gun.ID = *raw.ID
			gun.Name = fmt.Sprintf("Gun %d", gun.ID)
			gun.SafetyGunID = gun.ID
		}
		if raw.Name != nil {
			gun.Name = *raw.Name
		}
		if raw.PLCEnabled != nil {
			gun.PLCEnabled = *raw.PLCEnabled
		}
		if raw.SafetyGunID != nil {
			gun.SafetyGunID = *raw.SafetyGunID
		}
		if raw.AutoEnabled != nil {
			gun.AutoEnabled = *raw.AutoEnabled
			if raw.AutoExecutorEnabled == nil {
				gun.AutoExecutorEnabled = *raw.AutoEnabled
			}
		}
		if raw.DispatcherEligible != nil {
			gun.DispatcherEligible = *raw.DispatcherEligible
		}
		if raw.AutoExecutorEnabled != nil {
			gun.AutoExecutorEnabled = *raw.AutoExecutorEnabled
		}
		if raw.MinAzimuth != nil {
			gun.MinAzimuth = *raw.MinAzimuth
		}
		if raw.MaxAzimuth != nil {
			gun.MaxAzimuth = *raw.MaxAzimuth
		}
		if raw.MinElevation != nil {
			gun.MinElevation = *raw.MinElevation
		}
		if raw.MaxElevation != nil {
			gun.MaxElevation = *raw.MaxElevation
		}
		merged := Config{PLC: gun.PLC}
		var err error
		merged, err = mergePLCOnly(merged, raw.PLC)
		if err != nil {
			return nil, fmt.Errorf("guns[%d].plc: %w", i, err)
		}
		gun.PLC = merged.PLC
		guns = append(guns, gun)
	}
	if err := validateGunConfigs(guns); err != nil {
		return nil, err
	}
	return guns, nil
}

func mergePLCOnly(cfg Config, raw rawPLCConfig) (Config, error) {
	if raw.Host != nil {
		cfg.PLC.Host = *raw.Host
	}
	if raw.Port != nil {
		cfg.PLC.Port = *raw.Port
	}
	if raw.UnitID != nil {
		if *raw.UnitID < 0 || *raw.UnitID > 255 {
			return cfg, errors.New("unit_id must be between 0 and 255")
		}
		cfg.PLC.UnitID = byte(*raw.UnitID)
	}
	return cfg, nil
}

func syncGunDefaults(cfg Config) Config {
	if len(cfg.Guns) == 0 {
		cfg.Guns = []GunConfig{{ID: 1, Name: "Gun 1", PLCEnabled: true, SafetyGunID: 1, AutoEnabled: true, DispatcherEligible: true, AutoExecutorEnabled: true, MinAzimuth: 0, MaxAzimuth: 360, MinElevation: 5, MaxElevation: 85, PLC: cfg.PLC}}
		return cfg
	}
	if len(cfg.Guns) == 1 && cfg.Guns[0].ID == 1 {
		if cfg.Guns[0].PLC.Host == "" {
			cfg.Guns[0].PLC = cfg.PLC
		}
		if cfg.Guns[0].Name == "" {
			cfg.Guns[0].Name = "Gun 1"
		}
		if cfg.Guns[0].SafetyGunID == 0 {
			cfg.Guns[0].SafetyGunID = 1
		}
		cfg.Guns[0].PLCEnabled = true
		cfg.Guns[0].AutoEnabled = true
		cfg.Guns[0].DispatcherEligible = true
		cfg.Guns[0].AutoExecutorEnabled = true
	}
	for i := range cfg.Guns {
		if cfg.Guns[i].MaxAzimuth == 0 {
			cfg.Guns[i].MaxAzimuth = 360
		}
		if cfg.Guns[i].MinElevation == 0 && cfg.Guns[i].MaxElevation == 0 {
			cfg.Guns[i].MinElevation = 5
			cfg.Guns[i].MaxElevation = 85
		}
	}
	return cfg
}

func validateGunConfigs(guns []GunConfig) error {
	if len(guns) == 0 {
		return errors.New("at least one gun must be configured")
	}
	ids := make(map[int]struct{}, len(guns))
	endpoints := make(map[string]int, len(guns))
	for _, gun := range guns {
		if gun.ID <= 0 {
			return fmt.Errorf("gun id must be positive: %d", gun.ID)
		}
		if _, exists := ids[gun.ID]; exists {
			return fmt.Errorf("duplicate gun id %d", gun.ID)
		}
		ids[gun.ID] = struct{}{}
		if gun.SafetyGunID <= 0 || gun.SafetyGunID > 8 {
			return fmt.Errorf("gun %d safety_gun_id must be between 1 and 8", gun.ID)
		}
		if gun.PLC.Host == "" {
			return fmt.Errorf("gun %d plc.host must not be empty", gun.ID)
		}
		if gun.PLC.Port <= 0 || gun.PLC.Port > 65535 {
			return fmt.Errorf("gun %d plc.port must be between 1 and 65535", gun.ID)
		}
		if gun.MinAzimuth < 0 || gun.MinAzimuth > 360 || gun.MaxAzimuth < 0 || gun.MaxAzimuth > 360 {
			return fmt.Errorf("gun %d azimuth envelope must be within 0..360 degrees", gun.ID)
		}
		if gun.MinAzimuth > gun.MaxAzimuth {
			return fmt.Errorf("gun %d min_azimuth must be <= max_azimuth; wrap-around envelopes are not enabled in this phase", gun.ID)
		}
		if gun.MinElevation > gun.MaxElevation {
			return fmt.Errorf("gun %d min_elevation must be <= max_elevation", gun.ID)
		}
		endpoint := fmt.Sprintf("%s:%d/%d", gun.PLC.Host, gun.PLC.Port, gun.PLC.UnitID)
		if other, exists := endpoints[endpoint]; exists {
			return fmt.Errorf("guns %d and %d share duplicate plc endpoint %s", other, gun.ID, endpoint)
		}
		endpoints[endpoint] = gun.ID
	}
	return nil
}

func validateSafetyPLC(cfg Config) (Config, error) {
	if cfg.SafetyPLC.Host == "" {
		return cfg, errors.New("safety_plc.host must not be empty")
	}
	if cfg.SafetyPLC.Port <= 0 || cfg.SafetyPLC.Port > 65535 {
		return cfg, errors.New("safety_plc.port must be between 1 and 65535")
	}
	if cfg.SafetyPLC.PollInterval <= 0 {
		return cfg, errors.New("safety_plc.poll_interval must be positive")
	}
	if cfg.SafetyPLC.StaleAfter <= 0 {
		return cfg, errors.New("safety_plc.stale_after must be positive")
	}
	switch cfg.SafetyPLC.StatusRegisterType {
	case "input", "holding":
	default:
		return cfg, errors.New("safety_plc.status_register_type must be input or holding")
	}
	if cfg.SafetyPLC.ResetPulseDuration < 100*time.Millisecond || cfg.SafetyPLC.ResetPulseDuration > 5*time.Second {
		return cfg, errors.New("safety_plc.reset_pulse_duration must be between 100ms and 5s")
	}
	return cfg, nil
}
