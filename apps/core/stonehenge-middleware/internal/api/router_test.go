package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"stonehenge-middleware/internal/arbitration"
	"stonehenge-middleware/internal/autocontrol"
	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/fragments"
	"stonehenge-middleware/internal/guns"
	"stonehenge-middleware/internal/history"
	"stonehenge-middleware/internal/model"
	"stonehenge-middleware/internal/plc"
	"stonehenge-middleware/internal/safety"
	"stonehenge-middleware/internal/sim"
	"stonehenge-middleware/internal/store"
	"stonehenge-middleware/internal/targetfeed"
)

const (
	testControlToken = "test-control-token"
	testIngestToken  = "test-ingest-token"
)

type authorizedTestRouter struct {
	http.Handler
}

func newAuthorizedRouter(deps Dependencies) http.Handler {
	deps.ControlToken = testControlToken
	deps.IngestToken = testIngestToken
	return authorizedTestRouter{Handler: NewRouter(deps)}
}

func (router authorizedTestRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/v1/ingest/fragments" {
		r.Header.Set("Authorization", "Bearer "+testIngestToken)
	} else if r.Method == http.MethodPost || r.Method == http.MethodPut {
		r.Header.Set("X-Stonehenge-Control-Token", testControlToken)
	}
	router.Handler.ServeHTTP(w, r)
}

func TestMutationRoutesRequireScopedServiceTokens(t *testing.T) {
	transport := newAPITransport()
	router := NewRouter(Dependencies{
		Config:       config.Defaults(),
		PLCCommands:  plc.NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default()),
		ControlToken: testControlToken,
		IngestToken:  testIngestToken,
		Logger:       slog.Default(),
	})

	controlRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/guns/1/commands/manual"},
		{http.MethodPut, "/api/v1/guns/1/mode"},
		{http.MethodPost, "/api/v1/targets"},
		{http.MethodPut, "/api/v1/safety/global"},
		{http.MethodPut, "/api/v1/safety/guns/1"},
		{http.MethodPost, "/api/v1/safety/reset"},
		{http.MethodPost, "/api/v1/sim/start"},
		{http.MethodPost, "/api/v1/sim/stop"},
		{http.MethodPost, "/api/v1/sim/reset"},
	}
	for _, route := range controlRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			req.Header.Set("Authorization", "Bearer "+testIngestToken)
			req.Header.Set("X-Stonehenge-Control-Token", "wrong-token")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
		})
	}
	if transport.writeCount() != 0 {
		t.Fatalf("unauthorized requests wrote %d PLC registers", transport.writeCount())
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/fragments", bytes.NewReader([]byte("{")))
	req.Header.Set("Authorization", "Bearer "+testControlToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("cross-scope ingest status = %d, body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/ingest/fragments", bytes.NewReader([]byte("{")))
	req.Header.Set("Authorization", "Bearer "+testIngestToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("authorized ingest status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestJSONRequestBodiesAreBounded(t *testing.T) {
	for _, tt := range []struct {
		name   string
		strict bool
	}{{"standard", false}, {"strict", true}} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"value":"`+strings.Repeat("x", maxRequestBodyBytes)+`"}`))
			rec := httptest.NewRecorder()
			var dst struct {
				Value string `json:"value"`
			}
			if tt.strict {
				decodeStrictJSON(rec, req, &dst)
			} else {
				decodeJSON(rec, req, &dst)
			}
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestDuplicateFragmentRepairRequiresLiveFragment(t *testing.T) {
	for _, tt := range []struct {
		name       string
		status     model.FragmentStatus
		wantStatus int
	}{
		{name: "active fragment", status: model.FragmentActive, wantStatus: http.StatusAccepted},
		{name: "terminal fragment", status: model.FragmentMissed, wantStatus: http.StatusConflict},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Defaults()
			cfg.Simulation.AutoStart = false
			cfg.Fragments.Source = "ingest"
			cfg.Fragments.InternalRNG.Enabled = false
			state := store.NewMemoryStore(cfg.Simulation.MaxEvents)
			source := fragments.NewExternalIngestSource(cfg.Fragments.Ingest)
			engine := sim.NewEngine(cfg, state, plc.NewSimulatedAdapter(), safety.NewSimulatedController(cfg.Safety), source, history.NewNoOpSink(), slog.Default())
			engine.Start()
			defer engine.Stop()
			feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 1, TargetTTL: time.Hour, HealthyWindow: time.Hour}, nil, slog.Default())
			router := newAuthorizedRouter(Dependencies{Config: cfg, Store: state, Engine: engine, TargetFeed: feed})
			payload := `{"external_id":"DUP-900001","bearing":100,"elevation":45,"map_x":50,"map_y":50,"priority":"HIGH","terrain_type":"LAND","strategic_zone":"TEST","time_to_impact_seconds":30}`
			post := func() *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/fragments", strings.NewReader(payload))
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				return rec
			}

			if rec := post(); rec.Code != http.StatusAccepted {
				t.Fatalf("first ingest status = %d, body=%s", rec.Code, rec.Body.String())
			}
			original := feed.Targets()[0].Target
			now := time.Now().UTC()
			if !feed.MarkSubmitted(original, now) || !feed.MarkCompleted(original, "completed", "", now) {
				t.Fatal("failed to complete original target")
			}
			for id := uint32(1); id <= 500; id++ {
				target := targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: id, Azimuth: 100, Elevation: 45, Priority: 50, TargetClass: "test", DetectedAt: now}
				if _, err := feed.Ingest(context.Background(), target); err != nil {
					t.Fatalf("fill target %d: %v", id, err)
				}
				if !feed.MarkSubmitted(target, now) || !feed.MarkCompleted(target, "completed", "", now) {
					t.Fatalf("failed to complete fill target %d", id)
				}
			}
			for _, record := range feed.Targets() {
				if record.Target.TargetID == original.TargetID {
					t.Fatal("original target was not evicted from feed retention")
				}
			}
			fragment := state.Fragments()[0]
			fragment.Status = tt.status
			if err := state.SaveFragment(fragment); err != nil {
				t.Fatal(err)
			}
			if rec := post(); rec.Code != tt.wantStatus {
				t.Fatalf("duplicate ingest status = %d, want %d, body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestEmptyFaultBodyUsesDefaultFaultCode(t *testing.T) {
	cfg := config.Defaults()
	state := store.NewMemoryStore(cfg.Simulation.MaxEvents)
	engine := sim.NewEngine(
		cfg,
		state,
		plc.NewSimulatedAdapter(),
		safety.NewSimulatedController(cfg.Safety),
		nil,
		history.NewNoOpSink(),
		slog.Default(),
	)
	router := newAuthorizedRouter(Dependencies{Config: cfg, Store: state, Engine: engine})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guns/1/fault", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var gun model.GunState
	if err := json.NewDecoder(rec.Body).Decode(&gun); err != nil {
		t.Fatal(err)
	}
	if gun.FaultCode != "HMI_FAULT_REQUEST" {
		t.Fatalf("fault code = %q", gun.FaultCode)
	}
}

func TestHistoryQueryResourceLimitsAreBounded(t *testing.T) {
	router := NewRouter(Dependencies{History: history.NewInMemorySink(10)})
	for _, path := range []string{
		"/api/v1/dispatcher/decisions?limit=%2B501&offset=010001",
		"/api/v1/dispatcher/assignments?limit=501&offset=10001",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
			var body struct {
				Limit  int `json:"limit"`
				Offset int `json:"offset"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Limit != maxHistoryPageSize || body.Offset != maxHistoryOffset {
				t.Fatalf("limit/offset = %d/%d", body.Limit, body.Offset)
			}
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/?limit=501&offset=10001", nil)
	rec := httptest.NewRecorder()
	filter, ok := parseBaseHistoryFilter(rec, req)
	if !ok || filter.limit != maxHistoryPageSize || filter.offset != maxHistoryOffset {
		t.Fatalf("ordinary history limit/offset = %d/%d, ok=%t", filter.limit, filter.offset, ok)
	}
}

func TestManualPLCCommandValidation(t *testing.T) {
	router := newAuthorizedRouter(Dependencies{
		Config:      config.Defaults(),
		PLCCommands: plc.NewGunCommandServiceWithTransport(testPLCConfig(), newAPITransport(), slog.Default()),
		Logger:      slog.Default(),
	})
	body := []byte(`{"mode":0,"target_azimuth":120,"target_elevation":30}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guns/1/commands/manual", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestManualPLCCommandAccepted(t *testing.T) {
	transport := newAPITransport()
	auditor := &auditSink{id: 17}
	router := newAuthorizedRouter(Dependencies{
		Config:      config.Defaults(),
		History:     auditor,
		PLCCommands: plc.NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default()),
		Logger:      slog.Default(),
	})
	body := []byte(`{"target_azimuth":120,"target_elevation":30,"fire_request":false,"safety_fire_permissive":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guns/1/commands/manual", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var result plc.CommandResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !result.Accepted || result.SubmittedSequence != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if auditor.recordedID != 17 {
		t.Fatalf("recorded audit id = %d, want 17", auditor.recordedID)
	}
}

func TestManualPLCCommandStartsLifecycleAsynchronously(t *testing.T) {
	transport := newAPITransport()
	auditor := &auditSink{id: 23}
	cfg := testPLCConfig()
	tracker := plc.NewCommandLifecycleTracker(cfg, &slowStatusReader{}, auditor, slog.Default())
	defer tracker.Close()
	router := newAuthorizedRouter(Dependencies{
		Config:       config.Defaults(),
		History:      auditor,
		PLCCommands:  plc.NewGunCommandServiceWithTransport(cfg, transport, slog.Default()),
		PLCLifecycle: tracker,
		Logger:       slog.Default(),
	})
	body := []byte(`{"target_azimuth":120,"target_elevation":30,"fire_request":false,"safety_fire_permissive":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guns/1/commands/manual", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	started := time.Now()
	router.ServeHTTP(rec, req)
	if elapsed := time.Since(started); elapsed > 50*time.Millisecond {
		t.Fatalf("manual command response blocked on lifecycle for %s", elapsed)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestManualFireDeniedBySafetyBeforeCommandSubmission(t *testing.T) {
	cfg := config.Defaults()
	cfg.SafetyPLC.Enforcement = true
	transport := newAPITransport()
	safetyService := safety.NewServiceWithTransport(cfg.SafetyPLC, nil, nil, nil)
	arb := arbitration.NewService(plc.NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default()), slog.Default())
	auditor := &auditSink{}
	router := newAuthorizedRouter(Dependencies{
		Config:      cfg,
		History:     auditor,
		PLCCommands: plc.NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default()),
		Arbitration: arb,
		SafetyPLC:   safetyService,
		Logger:      slog.Default(),
	})
	body := []byte(`{"target_azimuth":120,"target_elevation":30,"fire_request":true,"safety_fire_permissive":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guns/1/commands/manual", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if transport.accepted != 0 || transport.writeCount() != 0 {
		t.Fatalf("transport accepted=%d writes=%d, want no command write", transport.accepted, transport.writeCount())
	}
	if status := arb.Status(); status.ActiveOperation != arbitration.OperationNone {
		t.Fatalf("arbitration status = %+v, want no active operation", status)
	}
	if got := auditor.safetyEventCount(); got != 1 {
		t.Fatalf("safety denial events = %d, want 1", got)
	}
}

func TestManualMovementAllowedWhenSafetyUnavailableWithEnforcement(t *testing.T) {
	cfg := config.Defaults()
	cfg.SafetyPLC.Enforcement = true
	transport := newAPITransport()
	safetyService := safety.NewServiceWithTransport(cfg.SafetyPLC, nil, nil, nil)
	router := newAuthorizedRouter(Dependencies{
		Config:      cfg,
		PLCCommands: plc.NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default()),
		SafetyPLC:   safetyService,
		Logger:      slog.Default(),
	})
	body := []byte(`{"target_azimuth":120,"target_elevation":30,"fire_request":false,"safety_fire_permissive":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guns/1/commands/manual", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if transport.accepted != 1 {
		t.Fatalf("transport accepted sequence = %d, want movement command", transport.accepted)
	}
}

func TestManualExtendedMotionDeniedWhenSafetyUnavailable(t *testing.T) {
	cfg := config.Defaults()
	cfg.SafetyPLC.Enforcement = true
	transport := newAPITransport()
	router := newAuthorizedRouter(Dependencies{
		Config:      cfg,
		PLCCommands: plc.NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default()),
		Logger:      slog.Default(),
	})
	body := []byte(`{"target_azimuth":120,"target_elevation":-10,"fire_request":false,"admin_override":true,"safety_extended_permissive":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guns/1/commands/manual", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if transport.writeCount() != 0 {
		t.Fatalf("writes = %d, want no command write", transport.writeCount())
	}
}

func TestManualExtendedMotionUsesAuthoritativeSafetyDecision(t *testing.T) {
	cfg := config.Defaults()
	cfg.SafetyPLC.Enforcement = true
	cfg.SafetyPLC.PollInterval = time.Millisecond
	cfg.SafetyPLC.StaleAfter = time.Second
	cfg.SafetyPLC.StatusRegisterType = "holding"
	transport := newAPITransport()
	auditor := &auditSink{}
	safetyTransport := &apiSafetyTransport{words: apiSafetyWords()}
	safetyService := safety.NewServiceWithTransport(cfg.SafetyPLC, safetyTransport, nil, slog.Default())
	safetyService.Start()
	defer safetyService.Close()
	waitSafetyReady(t, safetyService)
	router := newAuthorizedRouter(Dependencies{
		Config:      cfg,
		History:     auditor,
		PLCCommands: plc.NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default()),
		SafetyPLC:   safetyService,
		Logger:      slog.Default(),
	})
	body := []byte(`{"target_azimuth":120,"target_elevation":-10,"fire_request":false,"admin_override":true,"safety_extended_permissive":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guns/1/commands/manual", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got, ok := auditor.CommandAudit.Payload["safety_extended_permissive"].(bool); !ok || !got {
		t.Fatalf("audit payload = %+v, want authoritative extended permissive", auditor.CommandAudit.Payload)
	}
	if transport.payloadContains(1024+plc.RegisterSafetyExtendedPermissive, 1) {
		t.Fatalf("payload writes = %+v, command must not overwrite live safety lease", transport.writeSnapshot())
	}
}

func TestManualFirePermissiveSafetyOverridesClientPermissive(t *testing.T) {
	cfg := config.Defaults()
	cfg.SafetyPLC.Enforcement = true
	cfg.SafetyPLC.PollInterval = time.Millisecond
	cfg.SafetyPLC.StaleAfter = time.Second
	cfg.SafetyPLC.StatusRegisterType = "holding"
	transport := newAPITransport()
	safetyTransport := &apiSafetyTransport{words: apiSafetyWords()}
	safetyService := safety.NewServiceWithTransport(cfg.SafetyPLC, safetyTransport, nil, slog.Default())
	safetyService.Start()
	defer safetyService.Close()
	waitSafetyReady(t, safetyService)
	router := newAuthorizedRouter(Dependencies{
		Config:      cfg,
		PLCCommands: plc.NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default()),
		SafetyPLC:   safetyService,
		Logger:      slog.Default(),
	})
	body := []byte(`{"target_azimuth":120,"target_elevation":30,"fire_request":true,"safety_fire_permissive":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guns/1/commands/manual", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if transport.accepted != 1 {
		t.Fatalf("transport accepted sequence = %d, want fire command", transport.accepted)
	}
	if transport.payloadContains(1024+plc.RegisterSafetyFirePermissive, 1) {
		t.Fatalf("payload writes = %+v, command must not overwrite live safety lease", transport.writeSnapshot())
	}
}

func TestManualPLCCommandResolvesConfiguredGun(t *testing.T) {
	cfg := config.Defaults()
	transport1 := newAPITransport()
	transport2 := newAPITransport()
	registry, err := guns.NewRegistry([]*guns.Instance{
		{
			Definition: guns.Definition{ID: 1, Name: "Gun 1", PLC: testPLCConfig(), SafetyGunID: 1},
			Commands:   plc.NewGunCommandServiceForGunWithTransport(1, testPLCConfig(), transport1, slog.Default()),
		},
		{
			Definition: guns.Definition{ID: 2, Name: "Gun 2", PLC: testPLCConfig(), SafetyGunID: 2},
			Commands:   plc.NewGunCommandServiceForGunWithTransport(2, testPLCConfig(), transport2, slog.Default()),
		},
	})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	router := newAuthorizedRouter(Dependencies{Config: cfg, Guns: registry, Logger: slog.Default()})
	body := []byte(`{"target_azimuth":120,"target_elevation":30,"fire_request":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guns/2/commands/manual", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if transport1.writeCount() != 0 {
		t.Fatalf("gun 1 writes = %d, want 0", transport1.writeCount())
	}
	if transport2.writeCount() == 0 || transport2.accepted != 1 {
		t.Fatalf("gun 2 writes=%d accepted=%d, want command on gun 2", transport2.writeCount(), transport2.accepted)
	}
}

func TestManualPLCCommandUnknownConfiguredGun(t *testing.T) {
	registry, err := guns.NewRegistry([]*guns.Instance{
		{Definition: guns.Definition{ID: 1, Name: "Gun 1"}, Commands: plc.NewGunCommandServiceWithTransport(testPLCConfig(), newAPITransport(), slog.Default())},
	})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	router := newAuthorizedRouter(Dependencies{Config: config.Defaults(), Guns: registry, Logger: slog.Default()})
	body := []byte(`{"target_azimuth":120,"target_elevation":30,"fire_request":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guns/2/commands/manual", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestSafetyStatusUnavailableResponse(t *testing.T) {
	router := newAuthorizedRouter(Dependencies{Config: config.Defaults(), Logger: slog.Default()})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/safety", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var response safety.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Connected || response.Fresh || response.StateName != "offline" {
		t.Fatalf("response = %+v, want offline unavailable", response)
	}
}

func TestSafetyGlobalControlRequestContract(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantWrites []apiSafetyWrite
	}{
		{
			name:       "canonical maintenance_mode accepted",
			body:       `{"maintenance_mode":false}`,
			wantStatus: http.StatusOK,
			wantWrites: []apiSafetyWrite{{83, 0}},
		},
		{
			name:       "legacy global_maintenance_mode accepted",
			body:       `{"global_maintenance_mode":true}`,
			wantStatus: http.StatusOK,
			wantWrites: []apiSafetyWrite{{83, 1}},
		},
		{
			name:       "conflicting aliases rejected",
			body:       `{"maintenance_mode":false,"global_maintenance_mode":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown field rejected",
			body:       `{"maintenance_mode":false,"gun_enable":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty object rejected",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed json rejected",
			body:       `{"maintenance_mode":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "explicit false global_enable is written",
			body:       `{"global_enable":false}`,
			wantStatus: http.StatusOK,
			wantWrites: []apiSafetyWrite{{81, 0}},
		},
		{
			name:       "multiple json values rejected",
			body:       `{"global_enable":true} {"emergency_stop":false}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body rejected",
			body:       ``,
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, transport, closeService := newSafetyAPIRouter(t)
			defer closeService()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/safety/global", bytes.NewReader([]byte(tt.body)))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
			if got := transport.writesSnapshot(); !equalAPIWrites(got, tt.wantWrites) {
				t.Fatalf("writes = %+v, want %+v", got, tt.wantWrites)
			}
		})
	}
}

func TestSafetyGunControlRequestContract(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		wantStatus int
		wantWrites []apiSafetyWrite
	}{
		{
			name:       "canonical enabled accepted",
			path:       "/api/v1/safety/guns/1",
			body:       `{"enabled":true}`,
			wantStatus: http.StatusOK,
			wantWrites: []apiSafetyWrite{{96, 1}},
		},
		{
			name:       "canonical maintenance_inhibit accepted",
			path:       "/api/v1/safety/guns/1",
			body:       `{"maintenance_inhibit":true}`,
			wantStatus: http.StatusOK,
			wantWrites: []apiSafetyWrite{{97, 1}},
		},
		{
			name:       "canonical external_inhibit accepted",
			path:       "/api/v1/safety/guns/1",
			body:       `{"external_inhibit":false}`,
			wantStatus: http.StatusOK,
			wantWrites: []apiSafetyWrite{{98, 0}},
		},
		{
			name:       "legacy aliases accepted",
			path:       "/api/v1/safety/guns/1",
			body:       `{"gun_enabled":true,"gun_maintenance_inhibit":false,"gun_external_inhibit":true}`,
			wantStatus: http.StatusOK,
			wantWrites: []apiSafetyWrite{{96, 1}, {97, 0}, {98, 1}},
		},
		{
			name:       "conflicting aliases rejected",
			path:       "/api/v1/safety/guns/1",
			body:       `{"enabled":true,"gun_enabled":false}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown field rejected",
			path:       "/api/v1/safety/guns/1",
			body:       `{"gun_enable":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty object rejected",
			path:       "/api/v1/safety/guns/1",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid gun id rejected",
			path:       "/api/v1/safety/guns/9",
			body:       `{"enabled":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "explicit false enabled is written",
			path:       "/api/v1/safety/guns/1",
			body:       `{"enabled":false}`,
			wantStatus: http.StatusOK,
			wantWrites: []apiSafetyWrite{{96, 0}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, transport, closeService := newSafetyAPIRouter(t)
			defer closeService()
			req := httptest.NewRequest(http.MethodPut, tt.path, bytes.NewReader([]byte(tt.body)))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
			if got := transport.writesSnapshot(); !equalAPIWrites(got, tt.wantWrites) {
				t.Fatalf("writes = %+v, want %+v", got, tt.wantWrites)
			}
		})
	}
}

func TestFaultResetWaitsForSequenceAckAndLifecycleSuccess(t *testing.T) {
	transport := newResetRaceTransport([]plc.GunStatus{
		apiStatus(24, 7, true, 2),
		apiStatus(24, 7, true, 2),
		apiStatus(24, 7, true, 2),
		apiStatus(25, 7, true, 2),
		apiStatus(25, 1, false, 0),
	})
	cfg := testPLCConfig()
	cfg.CommandAckTimeout = 100 * time.Millisecond
	cfg.PollInterval = time.Millisecond
	cfg.LifecycleCompletionTimeout = 100 * time.Millisecond
	cfg.LifecyclePollInterval = time.Millisecond
	commands := plc.NewGunCommandServiceWithTransport(cfg, transport, slog.Default())
	arb := arbitration.NewService(commands, slog.Default())
	auditor := &auditSink{id: 31, resetID: 44, resetUpdates: make(chan history.FaultResetUpdate, 4)}
	tracker := plc.NewCommandLifecycleTracker(cfg, commands, auditor, slog.Default())
	defer tracker.Close()
	router := newAuthorizedRouter(Dependencies{
		Config:       config.Defaults(),
		History:      auditor,
		PLCCommands:  commands,
		PLCLifecycle: tracker,
		Arbitration:  arb,
		Logger:       slog.Default(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/guns/1/fault-reset", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Result plc.CommandResult `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Result.Accepted || response.Result.SubmittedSequence != 25 || response.Result.AcceptedSequence != 25 {
		t.Fatalf("unexpected reset response: %+v", response.Result)
	}
	if auditor.CommandAudit.SubmittedSequence != 25 || auditor.CommandAudit.Result != "accepted" {
		t.Fatalf("command audit = %+v", auditor.CommandAudit)
	}

	var final history.FaultResetUpdate
	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case update := <-auditor.resetUpdates:
			final = update
			if update.Result == string(arbitration.ResetResultSucceeded) {
				goto done
			}
		case <-deadline:
			t.Fatalf("timed out waiting for succeeded reset update; last=%+v", final)
		}
	}
done:
	if final.SubmittedSequence == nil || *final.SubmittedSequence != 25 {
		t.Fatalf("final reset submitted sequence = %v, want 25", final.SubmittedSequence)
	}
	if final.FaultCodeAfter == nil || *final.FaultCodeAfter != 0 {
		t.Fatalf("final reset fault code after = %v, want 0", final.FaultCodeAfter)
	}
	status := arb.Status()
	if status.LastFaultResetSequence == nil || *status.LastFaultResetSequence != 25 {
		t.Fatalf("arbitration last reset sequence = %v, want 25", status.LastFaultResetSequence)
	}
	if status.LastFaultResetResult != string(arbitration.ResetResultSucceeded) {
		t.Fatalf("arbitration status = %+v", status)
	}
}

func TestGunAutoAPIEnableAndSubmitTarget(t *testing.T) {
	auto := autocontrol.NewController(config.Defaults().PLC, &apiAutoCommands{}, nil, nil, nil, nil, slog.Default())
	auto.Start()
	defer auto.Close()
	router := newAuthorizedRouter(Dependencies{Config: config.Defaults(), AutoGun1: auto, Logger: slog.Default()})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/guns/1/auto", bytes.NewReader([]byte(`{"enabled":true}`)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("enable status = %d, body=%s", rec.Code, rec.Body.String())
	}

	body := []byte(`{"target_id":9001,"azimuth":220.5,"elevation":40,"priority":50,"target_class":"test_fragment","detected_at":"2026-06-28T17:30:00Z"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/targets", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("target status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var status autocontrol.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode target response: %v", err)
	}
	if status.QueuedTarget == nil || status.QueuedTarget.TargetID != 9001 {
		t.Fatalf("status = %+v", status)
	}
}

func TestGunAutoAPITargetValidation(t *testing.T) {
	auto := autocontrol.NewController(config.Defaults().PLC, &apiAutoCommands{}, nil, nil, nil, nil, slog.Default())
	auto.Start()
	defer auto.Close()
	router := newAuthorizedRouter(Dependencies{Config: config.Defaults(), AutoGun1: auto, Logger: slog.Default()})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/targets", bytes.NewReader([]byte(`{"target_id":0,"azimuth":220,"elevation":40,"priority":50}`)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func testPLCConfig() config.PLCConfig {
	return config.PLCConfig{
		Host:              "openplc-control-gun1",
		Port:              502,
		UnitID:            1,
		CommandAckTimeout: 100 * time.Millisecond,
		PollInterval:      time.Millisecond,
	}
}

type apiAutoCommands struct{}

func (c *apiAutoCommands) ReadStatus(ctx context.Context) (plc.GunStatus, error) {
	return plc.GunStatus{State: 1, StateName: "Ready", Ready: true}, nil
}

func (c *apiAutoCommands) Submit(ctx context.Context, gunID int, command plc.GunCommand) (plc.CommandResult, error) {
	return plc.CommandResult{}, nil
}

type auditSink struct {
	mu         sync.Mutex
	id         int64
	resetID    int64
	recordedID int64
	plc.CommandAudit
	completionID int64
	completion   plc.CommandAuditCompletion
	resetRecord  history.FaultResetRecord
	resetUpdate  history.FaultResetUpdate
	resetUpdates chan history.FaultResetUpdate
	safetyEvents []safety.Event
}

func (s *auditSink) Name() string                                                       { return "test" }
func (s *auditSink) WriteEvent(ctx context.Context, event model.OperationalEvent) error { return nil }
func (s *auditSink) Health() model.HistoryHealth {
	return model.HistoryHealth{Name: "test", Healthy: true, Connected: true}
}
func (s *auditSink) Close() error { return nil }
func (s *auditSink) RecordGunCommand(ctx context.Context, audit plc.CommandAudit) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CommandAudit = audit
	if s.id == 0 {
		s.id = 1
	}
	s.recordedID = s.id
	return s.id, nil
}
func (s *auditSink) UpdateGunCommandCompletion(ctx context.Context, id int64, completion plc.CommandAuditCompletion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completionID = id
	s.completion = completion
	return nil
}

func (s *auditSink) UpdateGunCommandLifecycleEvidence(ctx context.Context, id int64, evidence plc.CommandLifecycleEvidence) error {
	return nil
}

func (s *auditSink) RecordGunFaultReset(ctx context.Context, record history.FaultResetRecord) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetRecord = record
	if s.resetID == 0 {
		s.resetID = 1
	}
	return s.resetID, nil
}
func (s *auditSink) UpdateGunFaultReset(ctx context.Context, id int64, update history.FaultResetUpdate) error {
	s.mu.Lock()
	s.resetUpdate = update
	ch := s.resetUpdates
	s.mu.Unlock()
	if ch != nil {
		select {
		case ch <- update:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
func (s *auditSink) RecordSafetyEvent(ctx context.Context, event safety.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.safetyEvents = append(s.safetyEvents, event)
	return nil
}
func (s *auditSink) safetyEventCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.safetyEvents)
}

type slowStatusReader struct{}

func (r *slowStatusReader) ReadStatus(ctx context.Context) (plc.GunStatus, error) {
	select {
	case <-time.After(200 * time.Millisecond):
		return plc.GunStatus{AcceptedSequence: 1, State: 1, StateName: "Ready"}, nil
	case <-ctx.Done():
		return plc.GunStatus{}, ctx.Err()
	}
}

type apiTransport struct {
	mu       sync.Mutex
	accepted uint32
	writes   []apiPLCWrite
}

type apiPLCWrite struct {
	address uint16
	values  []uint16
	value   uint16
	single  bool
}

func newAPITransport() *apiTransport {
	return &apiTransport{}
}

func (t *apiTransport) ReadHoldingRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	words := make([]uint16, quantity)
	low, high := plc.SplitUint32(t.accepted)
	words[plc.RegisterAcceptedSequenceLow-20] = low
	words[plc.RegisterAcceptedSequenceHigh-20] = high
	return words, nil
}

func (t *apiTransport) WriteMultipleRegisters(ctx context.Context, address uint16, values []uint16) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.writes = append(t.writes, apiPLCWrite{address: address, values: append([]uint16(nil), values...)})
	return nil
}

func (t *apiTransport) WriteSingleRegister(ctx context.Context, address, value uint16) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.writes = append(t.writes, apiPLCWrite{address: address, value: value, single: true})
	t.accepted = uint32(value)
	return nil
}

func (t *apiTransport) writeCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.writes)
}

func (t *apiTransport) writeSnapshot() []apiPLCWrite {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]apiPLCWrite, len(t.writes))
	for i, write := range t.writes {
		out[i] = write
		out[i].values = append([]uint16(nil), write.values...)
	}
	return out
}

func (t *apiTransport) payloadContains(register uint16, value uint16) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, write := range t.writes {
		if write.single {
			if write.address == register && write.value == value {
				return true
			}
			continue
		}
		if register < write.address {
			continue
		}
		idx := int(register - write.address)
		if idx >= 0 && idx < len(write.values) && write.values[idx] == value {
			return true
		}
	}
	return false
}

type apiSafetyWrite struct {
	address uint16
	value   uint16
}

type apiSafetyTransport struct {
	mu     sync.Mutex
	words  []uint16
	writes []apiSafetyWrite
}

func newSafetyAPIRouter(t *testing.T) (http.Handler, *apiSafetyTransport, func()) {
	t.Helper()
	cfg := config.Defaults()
	cfg.SafetyPLC.PollInterval = time.Millisecond
	cfg.SafetyPLC.StaleAfter = time.Second
	cfg.SafetyPLC.StatusRegisterType = "holding"
	transport := &apiSafetyTransport{words: apiSafetyWords()}
	service := safety.NewServiceWithTransport(cfg.SafetyPLC, transport, nil, slog.Default())
	service.Start()
	waitSafetyReady(t, service)
	router := newAuthorizedRouter(Dependencies{Config: cfg, SafetyPLC: service, Logger: slog.Default()})
	return router, transport, service.Close
}

func waitSafetyReady(t *testing.T, service *safety.Service) {
	t.Helper()
	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		snap := service.Snapshot()
		if snap.Connected && snap.Fresh && snap.SchemaSupported {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("test safety service did not become ready")
}

func apiSafetyWords() []uint16 {
	words := make([]uint16, safety.StatusRegisterCount)
	words[0] = 1
	words[1] = 44
	words[2] = uint16(safety.StateReady)
	words[3] = 1
	words[4] = 1
	for gunID := 1; gunID <= 8; gunID++ {
		base := 16 + (gunID-1)*8
		words[base] = 1
		words[base+3] = 1
	}
	return words
}

func (t *apiSafetyTransport) ReadInputRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	return t.read(address, quantity), nil
}

func (t *apiSafetyTransport) ReadHoldingRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	return t.read(address, quantity), nil
}

func (t *apiSafetyTransport) read(address, quantity uint16) []uint16 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]uint16(nil), t.words[address:address+quantity]...)
}

func (t *apiSafetyTransport) WriteSingleRegister(ctx context.Context, address, value uint16) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.writes = append(t.writes, apiSafetyWrite{address: address, value: value})
	return nil
}

func (t *apiSafetyTransport) writesSnapshot() []apiSafetyWrite {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]apiSafetyWrite(nil), t.writes...)
}

func equalAPIWrites(a, b []apiSafetyWrite) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type resetRaceTransport struct {
	mu       sync.Mutex
	statuses []plc.GunStatus
	last     plc.GunStatus
}

func newResetRaceTransport(statuses []plc.GunStatus) *resetRaceTransport {
	transport := &resetRaceTransport{statuses: append([]plc.GunStatus(nil), statuses...)}
	if len(statuses) > 0 {
		transport.last = statuses[len(statuses)-1]
	}
	return transport
}

func (t *resetRaceTransport) ReadHoldingRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	status := t.last
	if len(t.statuses) > 0 {
		status = t.statuses[0]
		t.statuses = t.statuses[1:]
		t.last = status
	}
	words := make([]uint16, quantity)
	put := func(register uint16, value uint16) {
		idx := int(register - 20)
		if idx >= 0 && idx < len(words) {
			words[idx] = value
		}
	}
	low, high := plc.SplitUint32(status.AcceptedSequence)
	put(plc.RegisterAcceptedSequenceLow, low)
	put(plc.RegisterAcceptedSequenceHigh, high)
	put(plc.RegisterState, status.State)
	put(plc.RegisterCurrentAzimuth, plc.Uint16FromInt16(status.CurrentAzimuthTenths))
	put(plc.RegisterCurrentElevation, plc.Uint16FromInt16(status.CurrentElevationTenths))
	put(plc.RegisterReady, boolToWord(status.Ready))
	put(plc.RegisterFaultActive, boolToWord(status.FaultActive))
	put(plc.RegisterFaultCode, status.FaultCode)
	shotLow, shotHigh := plc.SplitUint32(status.ShotSequence)
	put(plc.RegisterShotSequenceLow, shotLow)
	put(plc.RegisterShotSequenceHigh, shotHigh)
	return words, nil
}

func (t *resetRaceTransport) WriteMultipleRegisters(ctx context.Context, address uint16, values []uint16) error {
	return nil
}

func (t *resetRaceTransport) WriteSingleRegister(ctx context.Context, address, value uint16) error {
	return nil
}

func apiStatus(accepted uint32, state uint16, fault bool, faultCode uint16) plc.GunStatus {
	return plc.GunStatus{
		AcceptedSequence:       accepted,
		State:                  state,
		StateName:              plc.StateName(state),
		CurrentAzimuthTenths:   1200,
		CurrentAzimuth:         120,
		CurrentElevationTenths: 300,
		CurrentElevation:       30,
		Ready:                  state == 1,
		FaultActive:            fault,
		FaultCode:              faultCode,
		FaultName:              plc.FaultName(faultCode),
	}
}

func boolToWord(value bool) uint16 {
	if value {
		return 1
	}
	return 0
}
