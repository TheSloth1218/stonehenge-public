package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFragmentAndHistoryConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{
  "fragments": {
    "source": "ingest",
    "ingest": {
      "max_time_to_impact_seconds": 900,
      "duplicate_retention_minutes": 30
    },
    "internal_rng": {
      "enabled": false,
      "interval_seconds": 12
    }
  },
  "history": {
    "adapter": "noop",
    "max_events": 25
  },
  "plc": {
    "adapter": "simulated"
  }
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Fragments.Source != "ingest" {
		t.Fatalf("source = %q, want ingest", cfg.Fragments.Source)
	}
	if cfg.Fragments.Ingest.MaxTimeToImpactSeconds != 900 {
		t.Fatalf("max tti = %d, want 900", cfg.Fragments.Ingest.MaxTimeToImpactSeconds)
	}
	if cfg.Fragments.Ingest.DuplicateRetention != 30*time.Minute {
		t.Fatalf("duplicate retention = %s, want 30m", cfg.Fragments.Ingest.DuplicateRetention)
	}
	if cfg.Fragments.InternalRNG.Enabled {
		t.Fatal("internal rng enabled = true, want false")
	}
	if cfg.Fragments.InternalRNG.Interval != 12*time.Second {
		t.Fatalf("internal rng interval = %s, want 12s", cfg.Fragments.InternalRNG.Interval)
	}
	if cfg.History.Adapter != "noop" {
		t.Fatalf("history adapter = %q, want noop", cfg.History.Adapter)
	}
	if cfg.History.MaxEvents != 25 {
		t.Fatalf("history max events = %d, want 25", cfg.History.MaxEvents)
	}
}

func TestLoadPLCDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.PLC.Adapter != "simulated" {
		t.Fatalf("adapter = %q, want simulated", cfg.PLC.Adapter)
	}
	if cfg.PLC.AutoReadinessPollInterval != 500*time.Millisecond {
		t.Fatalf("auto readiness poll interval = %s, want 500ms", cfg.PLC.AutoReadinessPollInterval)
	}
	if cfg.PLC.AutoSafetyFirePermissive {
		t.Fatal("auto safety fire permissive default = true, want false")
	}
	if cfg.PLC.StartupMode != "manual" || cfg.PLC.StartupModeRetryTimeout != 30*time.Second {
		t.Fatalf("unexpected startup mode defaults: %+v", cfg.PLC)
	}
	if !cfg.SafetyPLC.Enabled || cfg.SafetyPLC.Enforcement || cfg.SafetyPLC.Host != "openplc-safety" ||
		cfg.SafetyPLC.Port != 502 || cfg.SafetyPLC.UnitID != 1 ||
		cfg.SafetyPLC.PollInterval != 250*time.Millisecond ||
		cfg.SafetyPLC.StaleAfter != 2*time.Second ||
		cfg.SafetyPLC.StatusRegisterType != "input" ||
		cfg.SafetyPLC.MWRegisterBase != 80 ||
		cfg.SafetyPLC.ResetPulseDuration != 750*time.Millisecond {
		t.Fatalf("unexpected safety plc defaults: %+v", cfg.SafetyPLC)
	}
	if !cfg.PLC.AutoTelemetryModbusEnabled ||
		cfg.PLC.AutoTelemetryModbusAddr != "0.0.0.0:1502" ||
		cfg.PLC.AutoTelemetryUnitID != 1 ||
		cfg.PLC.AutoTelemetryPollInterval != 500*time.Millisecond ||
		cfg.PLC.AutoTelemetryStaleAfter != 3*time.Second {
		t.Fatalf("unexpected auto telemetry defaults: %+v", cfg.PLC)
	}
	if cfg.Fragments.Source != "internal-rng" {
		t.Fatalf("fragment source = %q, want internal-rng", cfg.Fragments.Source)
	}
	if cfg.History.Adapter != "memory" {
		t.Fatalf("history adapter = %q, want memory", cfg.History.Adapter)
	}
	if len(cfg.Guns) != 1 || cfg.Guns[0].ID != 1 || cfg.Guns[0].PLC.Host != cfg.PLC.Host || cfg.Guns[0].SafetyGunID != 1 {
		t.Fatalf("unexpected default gun config: %+v", cfg.Guns)
	}
}

func TestDispatcherLeaseRenewalIntervalEnv(t *testing.T) {
	t.Setenv("DISPATCHER_ASSIGNMENT_LEASE_DURATION", "2m")
	t.Setenv("DISPATCHER_LEASE_RENEWAL_INTERVAL", "45s")
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Dispatcher.AssignmentLeaseDuration != 2*time.Minute {
		t.Fatalf("assignment lease duration = %s, want 2m", cfg.Dispatcher.AssignmentLeaseDuration)
	}
	if cfg.Dispatcher.LeaseRenewalInterval != 45*time.Second {
		t.Fatalf("lease renewal interval = %s, want 45s", cfg.Dispatcher.LeaseRenewalInterval)
	}
}

func TestLoadRejectsInvalidPLCConfig(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "plc adapter", body: `{"plc":{"adapter":"modbus"}}`},
		{name: "fragment source", body: `{"fragments":{"source":"unknown"}}`},
		{name: "ingest max tti", body: `{"fragments":{"ingest":{"max_time_to_impact_seconds":0}}}`},
		{name: "duplicate retention", body: `{"fragments":{"ingest":{"duplicate_retention_minutes":0}}}`},
		{name: "history adapter", body: `{"history":{"adapter":"unknown"}}`},
		{name: "history max events", body: `{"history":{"max_events":0}}`},
		{name: "auto readiness interval", body: `{"plc":{"auto_readiness_poll_interval_ms":0}}`},
		{name: "auto telemetry unit id", body: `{"plc":{"auto_telemetry_unit_id":999}}`},
		{name: "auto telemetry poll interval", body: `{"plc":{"auto_telemetry_poll_interval":"0s"}}`},
		{name: "auto telemetry stale after", body: `{"plc":{"auto_telemetry_stale_after":"0s"}}`},
		{name: "safety plc host", body: `{"safety_plc":{"host":""}}`},
		{name: "safety plc register type", body: `{"safety_plc":{"status_register_type":"coil"}}`},
		{name: "safety plc reset pulse too short", body: `{"safety_plc":{"reset_pulse_duration":"99ms"}}`},
		{name: "safety plc reset pulse too long", body: `{"safety_plc":{"reset_pulse_duration":"6s"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(tt.body), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("expected config load error")
			}
		})
	}
}

func TestLoadGunsConfigJSON(t *testing.T) {
	t.Setenv("GUNS_CONFIG_JSON", `[
		{"id":2,"name":"Test Gun 2","safety_gun_id":2,"auto_enabled":false,"plc":{"host":"gun2","port":1502,"unit_id":2}},
		{"id":1,"name":"Test Gun 1","safety_gun_id":1,"auto_enabled":true,"plc":{"host":"gun1","port":502,"unit_id":1}}
	]`)
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.Guns) != 2 {
		t.Fatalf("guns len = %d, want 2", len(cfg.Guns))
	}
	if cfg.Guns[0].ID != 2 || cfg.Guns[0].PLC.Host != "gun2" || cfg.Guns[0].PLC.Port != 1502 || cfg.Guns[0].SafetyGunID != 2 {
		t.Fatalf("unexpected gun 2 config: %+v", cfg.Guns[0])
	}
	if cfg.Guns[1].ID != 1 || cfg.Guns[1].PLC.Host != "gun1" || cfg.Guns[1].AutoEnabled != true {
		t.Fatalf("unexpected gun 1 config: %+v", cfg.Guns[1])
	}
}

func TestLoadLinuxConfigIncludesEightCommissioningRuntimes(t *testing.T) {
	cfg, err := Load("../../configs/config.linux.example.json")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.Guns) != 8 {
		t.Fatalf("guns len = %d, want 8", len(cfg.Guns))
	}
	seenEndpoints := map[string]struct{}{}
	for i, gun := range cfg.Guns {
		id := i + 1
		wantHost := "openplc-control-gun1"
		if id > 1 {
			wantHost = fmt.Sprintf("openplc-control-gun%d", id)
		}
		if gun.ID != id || gun.Name != fmt.Sprintf("Gun %d", id) || gun.PLC.Host != wantHost ||
			gun.PLC.Port != 502 || gun.PLC.UnitID != 1 || gun.SafetyGunID != id {
			t.Fatalf("unexpected gun %d config: %+v", id, gun)
		}
		wantExecutionEnabled := true
		if gun.AutoEnabled != wantExecutionEnabled {
			t.Fatalf("gun %d auto_enabled = %v, want %v", id, gun.AutoEnabled, wantExecutionEnabled)
		}
		if gun.DispatcherEligible != wantExecutionEnabled {
			t.Fatalf("gun %d dispatcher_eligible = %v, want %v", id, gun.DispatcherEligible, wantExecutionEnabled)
		}
		if gun.AutoExecutorEnabled != wantExecutionEnabled {
			t.Fatalf("gun %d auto_executor_enabled = %v, want %v", id, gun.AutoExecutorEnabled, wantExecutionEnabled)
		}
		endpoint := gun.PLC.Host + ":502/1"
		if _, ok := seenEndpoints[endpoint]; ok {
			t.Fatalf("duplicate endpoint %s", endpoint)
		}
		seenEndpoints[endpoint] = struct{}{}
	}
}

func TestLoadRejectsInvalidGunConfig(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{name: "duplicate ids", json: `[{"id":1,"plc":{"host":"gun1"}},{"id":1,"plc":{"host":"gun2"}}]`},
		{name: "invalid id", json: `[{"id":0,"plc":{"host":"gun1"}}]`},
		{name: "invalid safety gun", json: `[{"id":1,"safety_gun_id":9,"plc":{"host":"gun1"}}]`},
		{name: "invalid port", json: `[{"id":1,"plc":{"host":"gun1","port":0}}]`},
		{name: "duplicate endpoint", json: `[{"id":1,"plc":{"host":"gun","port":502,"unit_id":1}},{"id":2,"plc":{"host":"gun","port":502,"unit_id":1}}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GUNS_CONFIG_JSON", tt.json)
			if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
				t.Fatal("expected config error")
			}
		})
	}
}

func TestLoadSafetyPLCEnvOverrides(t *testing.T) {
	t.Setenv("SAFETY_PLC_ENABLED", "false")
	t.Setenv("SAFETY_PLC_ENFORCEMENT", "true")
	t.Setenv("SAFETY_PLC_HOST", "safety.local")
	t.Setenv("SAFETY_PLC_PORT", "1502")
	t.Setenv("SAFETY_PLC_UNIT_ID", "2")
	t.Setenv("SAFETY_PLC_POLL_INTERVAL", "750ms")
	t.Setenv("SAFETY_PLC_STALE_AFTER", "5s")
	t.Setenv("SAFETY_PLC_STATUS_REGISTER_TYPE", "holding")
	t.Setenv("SAFETY_PLC_MW_REGISTER_BASE", "96")
	t.Setenv("SAFETY_PLC_RESET_PULSE_DURATION", "1s")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.SafetyPLC.Enabled || !cfg.SafetyPLC.Enforcement ||
		cfg.SafetyPLC.Host != "safety.local" || cfg.SafetyPLC.Port != 1502 || cfg.SafetyPLC.UnitID != 2 ||
		cfg.SafetyPLC.PollInterval != 750*time.Millisecond ||
		cfg.SafetyPLC.StaleAfter != 5*time.Second ||
		cfg.SafetyPLC.StatusRegisterType != "holding" ||
		cfg.SafetyPLC.MWRegisterBase != 96 ||
		cfg.SafetyPLC.ResetPulseDuration != time.Second {
		t.Fatalf("unexpected safety plc env config: %+v", cfg.SafetyPLC)
	}
}

func TestLoadPostgresHistoryEnvOverrides(t *testing.T) {
	t.Setenv("HISTORY_ADAPTER", "postgres")
	t.Setenv("POSTGRES_HOST", "db")
	t.Setenv("POSTGRES_PORT", "15432")
	t.Setenv("POSTGRES_DB", "stonehenge_test")
	t.Setenv("POSTGRES_USER", "writer")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_SSLMODE", "disable")
	t.Setenv("POSTGRES_MAX_CONNECTIONS", "7")
	t.Setenv("POSTGRES_WRITE_QUEUE_SIZE", "77")
	t.Setenv("POSTGRES_RETRY_INTERVAL_SECONDS", "3")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.History.Adapter != "postgres" {
		t.Fatalf("adapter = %q, want postgres", cfg.History.Adapter)
	}
	if cfg.History.PostgresHost != "db" || cfg.History.PostgresPort != 15432 || cfg.History.PostgresDB != "stonehenge_test" || cfg.History.PostgresUser != "writer" || cfg.History.PostgresPassword != "secret" {
		t.Fatalf("unexpected postgres config: %+v", cfg.History)
	}
	if cfg.History.PostgresMaxConnections != 7 || cfg.History.WriteQueueSize != 77 || cfg.History.RetryInterval != 3*time.Second {
		t.Fatalf("unexpected postgres sizing config: %+v", cfg.History)
	}
}

func TestLoadPLCEnvOverrides(t *testing.T) {
	t.Setenv("PLC_HOST", "openplc-control-gun1")
	t.Setenv("PLC_PORT", "502")
	t.Setenv("PLC_UNIT_ID", "1")
	t.Setenv("PLC_COMMAND_ACK_TIMEOUT_SECONDS", "7")
	t.Setenv("PLC_POLL_INTERVAL_MS", "250")
	t.Setenv("PLC_LIFECYCLE_POLL_INTERVAL_MS", "300")
	t.Setenv("PLC_LIFECYCLE_COMPLETION_TIMEOUT_SECONDS", "45")
	t.Setenv("PLC_LIFECYCLE_MAX_CONSECUTIVE_READ_ERRORS", "9")
	t.Setenv("PLC_AUTO_READINESS_POLL_INTERVAL_MS", "700")
	t.Setenv("PLC_AUTO_SAFETY_FIRE_PERMISSIVE", "true")
	t.Setenv("PLC_AUTO_TELEMETRY_MODBUS_ENABLED", "false")
	t.Setenv("PLC_AUTO_TELEMETRY_MODBUS_ADDR", "127.0.0.1:1503")
	t.Setenv("PLC_AUTO_TELEMETRY_UNIT_ID", "2")
	t.Setenv("PLC_AUTO_TELEMETRY_POLL_INTERVAL", "750ms")
	t.Setenv("PLC_AUTO_TELEMETRY_STALE_AFTER", "4s")
	t.Setenv("PLC_STARTUP_MODE", "AUTO")
	t.Setenv("PLC_STARTUP_MODE_RETRY_TIMEOUT", "12s")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.PLC.Host != "openplc-control-gun1" || cfg.PLC.Port != 502 || cfg.PLC.UnitID != 1 {
		t.Fatalf("unexpected plc config: %+v", cfg.PLC)
	}
	if cfg.PLC.CommandAckTimeout != 7*time.Second || cfg.PLC.PollInterval != 250*time.Millisecond ||
		cfg.PLC.LifecyclePollInterval != 300*time.Millisecond ||
		cfg.PLC.LifecycleCompletionTimeout != 45*time.Second ||
		cfg.PLC.LifecycleMaxConsecutiveErrors != 9 ||
		cfg.PLC.AutoReadinessPollInterval != 700*time.Millisecond ||
		!cfg.PLC.AutoSafetyFirePermissive ||
		cfg.PLC.AutoTelemetryModbusEnabled ||
		cfg.PLC.AutoTelemetryModbusAddr != "127.0.0.1:1503" ||
		cfg.PLC.AutoTelemetryUnitID != 2 ||
		cfg.PLC.AutoTelemetryPollInterval != 750*time.Millisecond ||
		cfg.PLC.AutoTelemetryStaleAfter != 4*time.Second ||
		cfg.PLC.StartupMode != "auto" ||
		cfg.PLC.StartupModeRetryTimeout != 12*time.Second {
		t.Fatalf("unexpected plc timing config: %+v", cfg.PLC)
	}
}
