package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stonehenge-middleware/images"
	"stonehenge-middleware/internal/arbitration"
	"stonehenge-middleware/internal/autocontrol"
	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/dashboard"
	"stonehenge-middleware/internal/dispatcher"
	"stonehenge-middleware/internal/fragments"
	"stonehenge-middleware/internal/guns"
	"stonehenge-middleware/internal/history"
	"stonehenge-middleware/internal/maintenance"
	"stonehenge-middleware/internal/model"
	"stonehenge-middleware/internal/plc"
	"stonehenge-middleware/internal/safety"
	"stonehenge-middleware/internal/sim"
	"stonehenge-middleware/internal/store"
	"stonehenge-middleware/internal/targetfeed"
	"stonehenge-middleware/internal/telemetry"
	"stonehenge-middleware/internal/testcontrol"
)

const (
	maxRequestBodyBytes = 1 << 20
	maxHistoryPageSize  = 500
	maxHistoryOffset    = 10_000
)

type Dependencies struct {
	Config        config.Config
	Store         *store.MemoryStore
	Engine        *sim.Engine
	History       history.Sink
	Guns          *guns.Registry
	PLCCommands   *plc.GunCommandService
	PLCLifecycle  *plc.CommandLifecycleTracker
	AutoGun1      *autocontrol.Controller
	Arbitration   *arbitration.Service
	TargetFeed    *targetfeed.Manager
	Dispatcher    *dispatcher.Manager
	SafetyPLC     *safety.Service
	AutoTelemetry *telemetry.ModbusServer
	Maintenance   *maintenance.Service
	ControlToken  string
	IngestToken   string
	Logger        *slog.Logger
}

type handler struct {
	deps Dependencies
	mux  *http.ServeMux
}

func NewRouter(deps Dependencies) http.Handler {
	h := &handler{deps: deps, mux: http.NewServeMux()}
	h.routes()
	return h.mux
}

func (h *handler) gunInstance(id int) (*guns.Instance, bool) {
	if h.deps.Guns != nil {
		return h.deps.Guns.Get(id)
	}
	if id != plc.Gun1ID {
		return nil, false
	}
	return &guns.Instance{
		Definition:  guns.Definition{ID: plc.Gun1ID, Name: "Gun 1", PLC: h.deps.Config.PLC, SafetyGunID: 1, AutoEnabled: true, PLCEnabled: true},
		Commands:    h.deps.PLCCommands,
		Lifecycle:   h.deps.PLCLifecycle,
		Auto:        h.deps.AutoGun1,
		Arbitration: h.deps.Arbitration,
	}, true
}

func (h *handler) routes() {
	staticFS, err := fs.Sub(dashboard.StaticFiles, "static")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(staticFS))
	h.mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, staticFS, "index.html")
	})
	h.mux.HandleFunc("GET /maintenance", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, staticFS, "maintenance.html")
	})
	h.mux.HandleFunc("GET /dashboard/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusMovedPermanently)
	})
	h.mux.Handle("GET /dashboard/static/", http.StripPrefix("/dashboard/static/", fileServer))
	h.mux.HandleFunc("GET /dashboard/images/Usean_continent.png", h.useaMap)
	h.mux.HandleFunc("GET /health", h.health)
	h.mux.HandleFunc("GET /ready", h.ready)
	h.mux.HandleFunc("GET /api/v1/status", h.status)
	h.mux.HandleFunc("GET /api/v1/guns", h.guns)
	h.mux.HandleFunc("GET /api/v1/guns/", h.gunRoute)
	h.mux.HandleFunc("POST /api/v1/guns/", h.requireControlToken(h.gunRoute))
	h.mux.HandleFunc("PUT /api/v1/guns/", h.requireControlToken(h.gunRoute))
	h.mux.HandleFunc("GET /api/v1/fragments", h.fragments)
	h.mux.HandleFunc("GET /api/v1/fragments/active", h.activeFragments)
	h.mux.HandleFunc("GET /api/v1/events", h.events)
	h.mux.HandleFunc("GET /api/v1/history/fragments", h.historyFragments)
	h.mux.HandleFunc("GET /api/v1/history/shots", h.historyShots)
	h.mux.HandleFunc("GET /api/v1/history/guns/", h.historyGunRoute)
	h.mux.HandleFunc("POST /api/v1/ingest/fragments", h.requireIngestToken(h.ingestFragment))
	h.mux.HandleFunc("GET /api/v1/target-feed", h.targetFeedStatus)
	h.mux.HandleFunc("GET /api/v1/target-feed/targets", h.targetFeedTargets)
	h.mux.HandleFunc("GET /api/v1/dispatcher", h.dispatcherStatus)
	h.mux.HandleFunc("GET /api/v1/dispatcher/decisions", h.dispatcherDecisions)
	h.mux.HandleFunc("GET /api/v1/dispatcher/decisions/", h.dispatcherDecision)
	h.mux.HandleFunc("GET /api/v1/dispatcher/assignments", h.dispatcherAssignments)
	h.mux.HandleFunc("GET /api/v1/dispatcher/assignments/", h.dispatcherAssignment)
	h.mux.HandleFunc("POST /api/v1/targets", h.requireControlToken(h.submitTarget))
	h.mux.HandleFunc("GET /api/v1/safety", h.safetyStatus)
	h.mux.HandleFunc("PUT /api/v1/safety/global", h.requireControlToken(h.safetyGlobal))
	h.mux.HandleFunc("PUT /api/v1/safety/guns/", h.requireControlToken(h.safetyGun))
	h.mux.HandleFunc("POST /api/v1/safety/reset", h.requireControlToken(h.safetyReset))
	h.mux.HandleFunc("POST /api/v1/sim/start", h.requireControlToken(h.start))
	h.mux.HandleFunc("POST /api/v1/sim/stop", h.requireControlToken(h.stop))
	h.mux.HandleFunc("POST /api/v1/sim/reset", h.requireControlToken(h.reset))
	h.mux.HandleFunc("GET /api/v1/test/pause", h.testPauseStatus)
	h.mux.HandleFunc("POST /api/v1/test/pause", h.testPauseArm)
	h.mux.HandleFunc("POST /api/v1/test/pause/release", h.testPauseRelease)
	h.mux.HandleFunc("POST /api/v1/test/dispatcher/reconcile", h.testDispatcherReconcile)
	h.mux.HandleFunc("POST /api/v1/test/assignments/resolve", h.testResolveAssignment)
	h.mux.HandleFunc("GET /api/v1/maintenance", h.maintenanceStatus)
	h.mux.HandleFunc("POST /api/v1/maintenance/actions", h.maintenanceAction)
}

func (h *handler) requireControlToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !tokenEqual(h.deps.ControlToken, r.Header.Get("X-Stonehenge-Control-Token")) {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "control_token_required"})
			return
		}
		next(w, r)
	}
}

func (h *handler) requireIngestToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || !tokenEqual(h.deps.IngestToken, parts[1]) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "ingest_token_required"})
			return
		}
		next(w, r)
	}
}

func tokenEqual(expected, actual string) bool {
	return expected != "" && len(expected) == len(actual) && subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "service": "stonehenge-middleware"})
}

func (h *handler) ready(w http.ResponseWriter, r *http.Request) {
	if reader, ok := h.deps.History.(history.Reader); ok {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if !reader.Ready(ctx) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"ok": false, "service": "stonehenge-middleware", "history": h.deps.History.Health()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "service": "stonehenge-middleware", "history": h.deps.History.Health()})
}

func (h *handler) useaMap(w http.ResponseWriter, r *http.Request) {
	data, err := images.Files.ReadFile("Usean_continent.png")
	if err != nil {
		writeError(w, http.StatusNotFound, "map image not found")
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeContent(w, r, "Usean_continent.png", time.Time{}, bytes.NewReader(data))
}

func (h *handler) status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.statusWithPLCOverlay(r.Context()))
}

func (h *handler) statusWithPLCOverlay(ctx context.Context) model.Status {
	status := h.deps.Engine.Status()
	if h.deps.Guns == nil {
		return status
	}

	checkedAt := time.Now().UTC()
	byID := make(map[int]int, len(status.Guns))
	for i := range status.Guns {
		byID[status.Guns[i].ID] = i
	}

	var readOK int
	var enabled int
	var lastErr string
	for _, inst := range h.deps.Guns.List() {
		if inst == nil || !inst.Definition.PLCEnabled || inst.Commands == nil {
			continue
		}
		enabled++
		readCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
		plcStatus, err := inst.Commands.ReadStatus(readCtx)
		cancel()
		if err != nil {
			lastErr = err.Error()
			if h.deps.Logger != nil {
				h.deps.Logger.Warn("real plc status overlay failed", "gun_id", inst.Definition.ID, "error", err)
			}
			continue
		}
		idx, ok := byID[inst.Definition.ID]
		if !ok {
			continue
		}
		status.Guns[idx] = overlayGunStatusFromPLC(status.Guns[idx], plcStatus)
		readOK++
	}

	if enabled > 0 {
		status.PLC.Adapter = "openplc"
		status.PLC.Connected = readOK == enabled
		status.PLC.LastCheckedAt = &checkedAt
		if readOK > 0 {
			status.PLC.LastOKAt = &checkedAt
		}
		if readOK != enabled {
			status.PLC.LastError = lastErr
		} else {
			status.PLC.LastError = ""
		}
	}
	status.GeneratedAt = checkedAt
	return status
}

func overlayGunStatusFromPLC(gun model.GunState, status plc.GunStatus) model.GunState {
	gun.CurrentAzimuth = status.CurrentAzimuth
	gun.CurrentElevation = status.CurrentElevation
	gun.TargetAzimuth = status.CurrentAzimuth
	gun.TargetElevation = status.CurrentElevation
	gun.State = publicGunStateFromPLC(status)
	gun.ShotSequence = uint64(status.ShotSequence)
	gun.FaultCode = status.FaultName
	gun.EventSummary = strings.ToLower(status.StateName)
	if status.FaultActive {
		gun.HardLimitStatus = "FAULT"
		if status.FaultName != "" && status.FaultName != "None" {
			gun.SafetyFaultReason = status.FaultName
		}
	} else if gun.HardLimitStatus == "" || strings.EqualFold(gun.HardLimitStatus, "FAULT") {
		gun.HardLimitStatus = "OK"
		gun.SafetyFaultReason = ""
	}
	if status.Firing {
		now := time.Now().UTC()
		gun.LastFireTime = &now
	}
	if status.Ready || status.OnTarget || status.State == 1 {
		gun.LastIdleTime = time.Now().UTC()
	}
	if status.Lockout || status.LockoutSeconds > 0 {
		until := time.Now().UTC().Add(time.Duration(status.LockoutSeconds) * time.Second)
		gun.PostFireLockoutUntil = &until
	} else {
		gun.PostFireLockoutUntil = nil
	}
	if status.Settling || status.SettlingSeconds > 0 {
		until := time.Now().UTC().Add(time.Duration(status.SettlingSeconds) * time.Second)
		gun.SettlingUntil = &until
	} else {
		gun.SettlingUntil = nil
	}
	return gun
}

func publicGunStateFromPLC(status plc.GunStatus) model.GunStateName {
	switch status.State {
	case 0:
		return model.GunStateName("OFFLINE")
	case 1:
		return model.GunIdle
	case 2:
		return model.GunSlewing
	case 3:
		return model.GunOnTargetSettling
	case 4:
		return model.GunFiring
	case 5:
		return model.GunPostFireLockout
	case 6:
		return model.GunStateName("MAINTENANCE")
	case 7:
		return model.GunFaulted
	default:
		if status.StateName != "" {
			return model.GunStateName(strings.ToUpper(strings.ReplaceAll(status.StateName, " ", "_")))
		}
		return model.GunStateName("UNKNOWN")
	}
}

func (h *handler) guns(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.deps.Store.Guns())
}

func (h *handler) fragments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.deps.Store.Fragments())
}

func (h *handler) activeFragments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.deps.Store.ActiveFragments())
}

func (h *handler) events(w http.ResponseWriter, r *http.Request) {
	limit := parseBoundedQueryInt(r, "limit", 100, 1, maxHistoryPageSize)
	writeJSON(w, http.StatusOK, h.deps.Store.Events(limit))
}

func (h *handler) historyFragments(w http.ResponseWriter, r *http.Request) {
	reader, ok := h.historyReader(w)
	if !ok {
		return
	}
	filter, ok := parseFragmentHistoryFilter(w, r)
	if !ok {
		return
	}
	rows, err := reader.ListFragments(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": rows, "limit": filter.Limit, "offset": filter.Offset})
}

func (h *handler) historyShots(w http.ResponseWriter, r *http.Request) {
	reader, ok := h.historyReader(w)
	if !ok {
		return
	}
	base, ok := parseBaseHistoryFilter(w, r)
	if !ok {
		return
	}
	filter := history.ShotFilter{Limit: base.limit, Offset: base.offset, Start: base.start, End: base.end}
	rows, err := reader.ListShots(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": rows, "limit": filter.Limit, "offset": filter.Offset})
}

func (h *handler) historyGunRoute(w http.ResponseWriter, r *http.Request) {
	reader, ok := h.historyReader(w)
	if !ok {
		return
	}
	id, action, ok := parseHistoryGunPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch action {
	case "events":
		base, ok := parseBaseHistoryFilter(w, r)
		if !ok {
			return
		}
		filter := history.EventFilter{Limit: base.limit, Offset: base.offset, Start: base.start, End: base.end, EventType: r.URL.Query().Get("event_type")}
		rows, err := reader.ListGunEvents(r.Context(), id, filter)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": rows, "limit": filter.Limit, "offset": filter.Offset})
	case "summary":
		row, found, err := reader.GetGunSummary(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "gun summary not found")
			return
		}
		writeJSON(w, http.StatusOK, row)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *handler) historyReader(w http.ResponseWriter) (history.Reader, bool) {
	reader, ok := h.deps.History.(history.Reader)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "postgres history is disabled", "adapter": h.deps.History.Name()})
		return nil, false
	}
	return reader, true
}

func (h *handler) start(w http.ResponseWriter, r *http.Request) {
	h.deps.Engine.Start()
	writeJSON(w, http.StatusOK, h.deps.Engine.Status())
}

func (h *handler) stop(w http.ResponseWriter, r *http.Request) {
	h.deps.Engine.Stop()
	writeJSON(w, http.StatusOK, h.deps.Engine.Status())
}

func (h *handler) reset(w http.ResponseWriter, r *http.Request) {
	h.deps.Engine.Reset()
	writeJSON(w, http.StatusOK, h.deps.Engine.Status())
}

func (h *handler) maintenanceStatus(w http.ResponseWriter, r *http.Request) {
	if h.deps.Maintenance == nil || !h.deps.Maintenance.Enabled() {
		writeJSON(w, http.StatusOK, maintenance.Result{Enabled: false, Status: "disabled"})
		return
	}
	if !h.deps.Maintenance.Authorized(r.Header.Get("X-Stonehenge-Maintenance-Token")) {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "maintenance_token_required"})
		return
	}
	writeJSON(w, http.StatusOK, h.deps.Maintenance.Status())
}

func (h *handler) maintenanceAction(w http.ResponseWriter, r *http.Request) {
	if h.deps.Maintenance == nil || !h.deps.Maintenance.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, maintenance.Result{Enabled: false, Status: "disabled", Error: "maintenance controls are disabled"})
		return
	}
	if !h.deps.Maintenance.Authorized(r.Header.Get("X-Stonehenge-Maintenance-Token")) {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "maintenance_token_required"})
		return
	}
	var req maintenance.Request
	if !decodeJSON(w, r, &req) {
		return
	}
	result := h.deps.Maintenance.Run(r.Context(), req)
	if result.Status == "rejected" {
		writeJSON(w, http.StatusBadRequest, result)
		return
	}
	if result.Status == "failed" {
		writeJSON(w, http.StatusBadGateway, result)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (h *handler) gunRoute(w http.ResponseWriter, r *http.Request) {
	id, action, ok := parseGunPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method == http.MethodGet && action == "" {
		gun, ok := h.deps.Store.Gun(id)
		if !ok {
			writeError(w, http.StatusNotFound, "gun not found")
			return
		}
		writeJSON(w, http.StatusOK, gun)
		return
	}
	if r.Method == http.MethodGet && action == "plc-status" {
		h.plcStatus(w, r, id)
		return
	}
	if action == "mode" {
		h.gunMode(w, r, id)
		return
	}
	if action == "auto" {
		h.gunAuto(w, r, id)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	switch action {
	case "commands/manual":
		h.manualPLCCommand(w, r, id)
	case "fault-reset":
		h.faultReset(w, r, id)
	case "manual-target":
		h.manualTarget(w, r, id)
	case "fire-request":
		h.fireRequest(w, id)
	case "fault":
		h.fault(w, r, id)
	case "clear-fault":
		h.clearFault(w, id)
	case "admin-override":
		h.adminOverride(w, r, id)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *handler) plcStatus(w http.ResponseWriter, r *http.Request, id int) {
	inst, ok := h.gunInstance(id)
	if !ok {
		writeError(w, http.StatusNotFound, "gun not configured")
		return
	}
	if inst.Commands == nil {
		writeError(w, http.StatusServiceUnavailable, "plc command service is unavailable")
		return
	}
	status, err := inst.Commands.ReadStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) manualPLCCommand(w http.ResponseWriter, r *http.Request, id int) {
	inst, ok := h.gunInstance(id)
	if !ok {
		writeError(w, http.StatusNotFound, "gun not configured")
		return
	}
	if inst.Commands == nil {
		writeError(w, http.StatusServiceUnavailable, "plc command service is unavailable")
		return
	}
	var req struct {
		Mode                     *plc.CommandMode `json:"mode"`
		TargetAzimuth            *float64         `json:"target_azimuth"`
		TargetElevation          *float64         `json:"target_elevation"`
		FragmentID               *uint32          `json:"fragment_id"`
		FragmentLow              *uint16          `json:"fragment_low"`
		FragmentHigh             *uint16          `json:"fragment_high"`
		FireRequest              bool             `json:"fire_request"`
		Maintenance              bool             `json:"maintenance"`
		AdminOverride            bool             `json:"admin_override"`
		FaultReset               bool             `json:"fault_reset"`
		SafetyFirePermissive     bool             `json:"safety_fire_permissive"`
		SafetyExtendedPermissive bool             `json:"safety_extended_permissive"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.TargetAzimuth == nil {
		writeError(w, http.StatusBadRequest, "target_azimuth is required")
		return
	}
	if req.TargetElevation == nil {
		writeError(w, http.StatusBadRequest, "target_elevation is required")
		return
	}
	if req.Mode != nil && *req.Mode != plc.ModeManual {
		writeError(w, http.StatusBadRequest, "manual command endpoint requires mode 1 (Manual)")
		return
	}
	command := plc.GunCommand{
		Mode:                     plc.ModeManual,
		TargetAzimuth:            *req.TargetAzimuth,
		TargetElevation:          *req.TargetElevation,
		FragmentID:               req.FragmentID,
		FragmentLow:              req.FragmentLow,
		FragmentHigh:             req.FragmentHigh,
		FireRequest:              req.FireRequest,
		Maintenance:              req.Maintenance,
		AdminOverride:            req.AdminOverride,
		FaultReset:               req.FaultReset,
		SafetyFirePermissive:     req.SafetyFirePermissive,
		SafetyExtendedPermissive: req.AdminOverride && req.SafetyExtendedPermissive,
	}
	requiresSafetyDecision := command.FireRequest || command.SafetyExtendedPermissive
	if requiresSafetyDecision && h.deps.Config.SafetyPLC.Enforcement {
		if h.deps.SafetyPLC == nil {
			writeError(w, http.StatusServiceUnavailable, "safety plc service is unavailable")
			return
		}
		decision := h.deps.SafetyPLC.CanFire(id)
		if !decision.Allowed {
			h.recordSafetyDenied(r.Context(), id, decision)
			writeJSON(w, http.StatusForbidden, map[string]interface{}{"error": "safety_denied", "decision": decision})
			return
		}
		command.SafetyFirePermissive = command.FireRequest
	} else if h.deps.Config.SafetyPLC.Enforcement {
		command.SafetyFirePermissive = false
		command.SafetyExtendedPermissive = false
	}
	var lease *arbitration.Lease
	if inst.Arbitration != nil {
		var status arbitration.Status
		var err error
		lease, status, err = inst.Arbitration.AcquireManual(r.Context())
		if err != nil {
			writeArbitrationConflict(w, err, status)
			return
		}
	}
	result, err := inst.Commands.Submit(r.Context(), id, command)
	auditID, auditRecorded := h.recordGunCommand(r.Context(), id, plc.CommandSourceManualAPI, command, result)
	if err != nil {
		if lease != nil {
			lease.Release()
		}
		switch result.ErrorType {
		case "validation":
			writeJSON(w, http.StatusBadRequest, result)
		case "timeout":
			writeJSON(w, http.StatusGatewayTimeout, result)
		case "plc_fault":
			writeJSON(w, http.StatusConflict, result)
		default:
			writeJSON(w, http.StatusServiceUnavailable, result)
		}
		return
	}
	if result.Accepted && auditRecorded {
		h.startGunCommandLifecycleFor(inst, auditID, id, plc.CommandSourceManualAPI, command, result, lease, 0, nil)
	} else if lease != nil {
		lease.Release()
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) recordGunCommand(ctx context.Context, gunID int, source string, command plc.GunCommand, result plc.CommandResult) (int64, bool) {
	auditor, ok := h.deps.History.(history.CommandAuditor)
	if !ok {
		return 0, false
	}
	audit := plc.NewCommandAudit(gunID, source, command, result)
	auditCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	auditID, err := auditor.RecordGunCommand(auditCtx, audit)
	if err != nil {
		if h.deps.Logger != nil {
			h.deps.Logger.Warn("gun plc command audit write failed", "gun_id", gunID, "sequence", result.SubmittedSequence, "error", err)
		}
		return 0, false
	}
	return auditID, true
}

func (h *handler) startGunCommandLifecycle(auditID int64, gunID int, source string, command plc.GunCommand, result plc.CommandResult, lease *arbitration.Lease, timeout time.Duration, completion func(plc.CommandLifecycleEvent)) {
	inst, _ := h.gunInstance(gunID)
	h.startGunCommandLifecycleFor(inst, auditID, gunID, source, command, result, lease, timeout, completion)
}

func (h *handler) startGunCommandLifecycleFor(inst *guns.Instance, auditID int64, gunID int, source string, command plc.GunCommand, result plc.CommandResult, lease *arbitration.Lease, timeout time.Duration, completion func(plc.CommandLifecycleEvent)) {
	lifecycle := h.deps.PLCLifecycle
	if inst != nil && inst.Lifecycle != nil {
		lifecycle = inst.Lifecycle
	}
	if lifecycle == nil {
		if lease != nil {
			lease.Release()
		}
		go h.updateGunCommandLifecycleStartFailure(auditID, gunID, result, "plc lifecycle tracker is unavailable")
		return
	}
	baseline := plc.GunStatus{}
	if result.BaselineStatus != nil {
		baseline = *result.BaselineStatus
	}
	ackStatus := plc.GunStatus{}
	if result.Status != nil {
		ackStatus = *result.Status
	}
	completions := make(chan plc.CommandLifecycleEvent, 1)
	err := lifecycle.Start(plc.CommandLifecycleRequest{
		AuditID:           auditID,
		GunID:             gunID,
		Command:           command,
		SubmittedSequence: result.SubmittedSequence,
		BaselineStatus:    baseline,
		AckStatus:         ackStatus,
		Source:            source,
		Timeout:           timeout,
		Completion:        completions,
	})
	if err != nil {
		if lease != nil {
			lease.Release()
		}
		go h.updateGunCommandLifecycleStartFailure(auditID, gunID, result, err.Error())
		return
	}
	if lease != nil || completion != nil {
		go func() {
			event := <-completions
			if lease != nil {
				lease.Release()
			}
			if completion != nil {
				completion(event)
			}
		}()
	}
}

func (h *handler) updateGunCommandLifecycleStartFailure(auditID int64, gunID int, result plc.CommandResult, message string) {
	auditor, ok := h.deps.History.(history.CommandAuditor)
	if !ok {
		return
	}
	if h.deps.Logger != nil {
		h.deps.Logger.Warn("gun plc command lifecycle tracker start failed", "audit_id", auditID, "gun_id", gunID, "sequence", result.SubmittedSequence, "error", message)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	status := plc.GunStatus{}
	if result.Status != nil {
		status = *result.Status
	}
	if err := auditor.UpdateGunCommandCompletion(ctx, auditID, plc.CommandAuditCompletion{
		CompletionTime: time.Now().UTC(),
		Result:         "protocol_state",
		ResultingState: status.State,
		FaultCode:      status.FaultCode,
		ShotSequence:   uint32Ptr(status.ShotSequence),
		Error:          message,
	}); err != nil && h.deps.Logger != nil {
		h.deps.Logger.Warn("gun plc command lifecycle start failure audit update failed", "audit_id", auditID, "gun_id", gunID, "sequence", result.SubmittedSequence, "error", err)
	}
}

func (h *handler) recordFaultReset(ctx context.Context, status arbitration.Status, resumeAuto bool, plcStatus plc.GunStatus) int64 {
	return h.recordFaultResetForGun(ctx, plc.Gun1ID, status, resumeAuto, plcStatus)
}

func (h *handler) recordFaultResetForGun(ctx context.Context, gunID int, status arbitration.Status, resumeAuto bool, plcStatus plc.GunStatus) int64 {
	auditor, ok := h.deps.History.(history.FaultResetAuditor)
	if !ok {
		return 0
	}
	auditCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	id, err := auditor.RecordGunFaultReset(auditCtx, history.FaultResetRecord{
		GunID:               gunID,
		Source:              plc.CommandSourceOperatorFaultReset,
		PriorMode:           string(status.Mode),
		RequestedResumeAuto: resumeAuto,
		FaultCodeBefore:     plcStatus.FaultCode,
		StartedAt:           time.Now().UTC(),
	})
	if err != nil && h.deps.Logger != nil {
		h.deps.Logger.Warn("gun fault reset audit write failed", "gun_id", gunID, "fault_code", plcStatus.FaultCode, "error", err)
		return 0
	}
	return id
}

func (h *handler) updateFaultReset(ctx context.Context, id int64, update history.FaultResetUpdate) {
	if id <= 0 {
		return
	}
	auditor, ok := h.deps.History.(history.FaultResetAuditor)
	if !ok {
		return
	}
	auditCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := auditor.UpdateGunFaultReset(auditCtx, id, update); err != nil && h.deps.Logger != nil {
		h.deps.Logger.Warn("gun fault reset audit update failed", "reset_id", id, "result", update.Result, "error", err)
	}
}

func uint32Ptr(value uint32) *uint32 {
	return &value
}

func optionalUint32(value uint32, ok bool) *uint32 {
	if !ok {
		return nil
	}
	return &value
}

func optionalInt64(value int64, ok bool) *int64 {
	if !ok {
		return nil
	}
	return &value
}

func nullableID(value int64) interface{} {
	if value <= 0 {
		return nil
	}
	return value
}

func derefPLCStatus(status *plc.GunStatus) plc.GunStatus {
	if status == nil {
		return plc.GunStatus{}
	}
	return *status
}

func resetModeFor(mode arbitration.Mode) plc.CommandMode {
	if mode == arbitration.ModeMaintenance {
		return plc.ModeMaintenance
	}
	return plc.ModeManual
}

func mapFaultResetResult(completion plc.CommandAuditCompletion) arbitration.FaultResetResult {
	switch completion.Result {
	case "completed_no_fire", "completed_fired":
		if completion.FaultCode == 0 && completion.ResultingState == 1 {
			return arbitration.ResetResultSucceeded
		}
		return arbitration.ResetResultFaultPersisted
	case "faulted":
		return arbitration.ResetResultFaultPersisted
	case "completion_timeout":
		return arbitration.ResetResultLifecycleTimeout
	case "protocol_state":
		return arbitration.ResetResultProtocolState
	default:
		return arbitration.ResetResultInternalError
	}
}

func statusCodeForPLCResult(result plc.CommandResult) int {
	switch result.ErrorType {
	case "validation":
		return http.StatusBadRequest
	case "timeout":
		return http.StatusGatewayTimeout
	case "plc_fault", "protocol_state":
		return http.StatusConflict
	default:
		return http.StatusServiceUnavailable
	}
}

func writeArbitrationConflict(w http.ResponseWriter, err error, status arbitration.Status) {
	code := http.StatusConflict
	var conflict arbitration.ConflictError
	reason := err.Error()
	if errors.As(err, &conflict) {
		reason = conflict.Reason
	}
	writeJSON(w, code, map[string]interface{}{
		"error":       "command_conflict",
		"reason":      reason,
		"mode":        status.Mode,
		"arbitration": status,
	})
}

func stringsJoin(left, right string) string {
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	return left + "; " + right
}

func (h *handler) gunAuto(w http.ResponseWriter, r *http.Request, id int) {
	inst, ok := h.gunInstance(id)
	if !ok {
		writeError(w, http.StatusNotFound, "gun not configured")
		return
	}
	if inst.Auto == nil {
		writeError(w, http.StatusServiceUnavailable, "gun auto controller is unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		status := inst.Auto.Status(r.Context())
		if id == plc.Gun1ID && h.deps.AutoTelemetry != nil {
			status.Telemetry = h.deps.AutoTelemetry.Status()
		}
		if inst.Arbitration != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"auto": status, "arbitration": inst.Arbitration.Status()})
			return
		}
		writeJSON(w, http.StatusOK, status)
	case http.MethodPut:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if inst.Arbitration != nil {
			mode := arbitration.ModeManual
			if req.Enabled {
				mode = arbitration.ModeAuto
			}
			status, err := inst.Arbitration.RequestMode(r.Context(), mode)
			if err != nil {
				writeArbitrationConflict(w, err, status)
				return
			}
			inst.Auto.SetEnabled(req.Enabled)
			writeJSON(w, http.StatusOK, map[string]interface{}{"arbitration": status, "auto": inst.Auto.Status(r.Context())})
			return
		}
		writeJSON(w, http.StatusOK, inst.Auto.SetEnabled(req.Enabled))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *handler) gunMode(w http.ResponseWriter, r *http.Request, id int) {
	inst, ok := h.gunInstance(id)
	if !ok {
		if r.Method == http.MethodPost {
			h.setMode(w, r, id)
			return
		}
		writeError(w, http.StatusNotFound, "gun not configured")
		return
	}
	if inst.Arbitration == nil {
		if r.Method == http.MethodPost {
			h.setMode(w, r, id)
			return
		}
		writeError(w, http.StatusServiceUnavailable, "gun arbitration service is unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, inst.Arbitration.Status())
	case http.MethodPut:
		var req struct {
			Mode arbitration.Mode `json:"mode"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		status, err := inst.Arbitration.RequestMode(r.Context(), req.Mode)
		if err != nil {
			writeArbitrationConflict(w, err, status)
			return
		}
		if inst.Auto != nil {
			switch req.Mode {
			case arbitration.ModeAuto:
				inst.Auto.SetEnabled(true)
			case arbitration.ModeManual, arbitration.ModeMaintenance:
				inst.Auto.SetEnabled(false)
			}
		}
		writeJSON(w, http.StatusOK, status)
	case http.MethodPost:
		h.setMode(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *handler) faultReset(w http.ResponseWriter, r *http.Request, id int) {
	inst, ok := h.gunInstance(id)
	if !ok {
		writeError(w, http.StatusNotFound, "gun not configured")
		return
	}
	if inst.Commands == nil || inst.Arbitration == nil {
		writeError(w, http.StatusServiceUnavailable, "gun plc command or arbitration service is unavailable")
		return
	}
	var req struct {
		ResumeAuto *bool `json:"resume_auto"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if !decodeJSON(w, r, &req) {
			return
		}
	}
	status, err := inst.Commands.ReadStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": err.Error()})
		return
	}
	currentArb := inst.Arbitration.Status()
	resumeAuto := currentArb.Mode == arbitration.ModeAutoSuspended && currentArb.AutoEnabledBeforeSuspension
	if req.ResumeAuto != nil {
		resumeAuto = *req.ResumeAuto
	}
	lease, arbStatus, err := inst.Arbitration.AcquireFaultReset(r.Context(), status, resumeAuto)
	if err != nil {
		writeArbitrationConflict(w, err, arbStatus)
		return
	}
	resetID := h.recordFaultResetForGun(r.Context(), id, currentArb, resumeAuto, status)
	command := plc.GunCommand{
		Mode:                     resetModeFor(currentArb.Mode),
		TargetAzimuth:            status.CurrentAzimuth,
		TargetElevation:          status.CurrentElevation,
		FireRequest:              false,
		Maintenance:              false,
		AdminOverride:            false,
		FaultReset:               true,
		SafetyFirePermissive:     false,
		SafetyExtendedPermissive: false,
	}
	result, submitErr := inst.Commands.Submit(r.Context(), id, command)
	auditID, auditRecorded := h.recordGunCommand(r.Context(), id, plc.CommandSourceOperatorFaultReset, command, result)
	h.updateFaultReset(r.Context(), resetID, history.FaultResetUpdate{
		CommandAuditID:    optionalInt64(auditID, auditRecorded),
		SubmittedSequence: optionalUint32(result.SubmittedSequence, result.SubmittedSequence != 0),
		Result:            "accepted",
	})
	if submitErr != nil || !result.Accepted {
		if lease != nil {
			lease.Release()
		}
		resetResult := arbitration.ResetResultAcknowledgmentFailed
		if result.ErrorType == "plc_fault" {
			resetResult = arbitration.ResetResultFaultPersisted
		}
		final := derefPLCStatus(result.Status)
		inst.Arbitration.CompleteFaultReset(resetResult, final, result.Error)
		now := time.Now().UTC()
		faultAfter := final.FaultCode
		h.updateFaultReset(context.Background(), resetID, history.FaultResetUpdate{
			CommandAuditID:    optionalInt64(auditID, auditRecorded),
			SubmittedSequence: optionalUint32(result.SubmittedSequence, result.SubmittedSequence != 0),
			Result:            string(resetResult),
			FaultCodeAfter:    &faultAfter,
			CompletedAt:       &now,
			Error:             result.Error,
		})
		writeJSON(w, statusCodeForPLCResult(result), result)
		return
	}
	if auditRecorded {
		h.startGunCommandLifecycleFor(inst, auditID, id, plc.CommandSourceOperatorFaultReset, command, result, lease, inst.Definition.PLC.FaultResetTimeout, func(event plc.CommandLifecycleEvent) {
			h.completeFaultResetFor(inst, resetID, resumeAuto, event)
		})
	} else {
		if lease != nil {
			lease.Release()
		}
		inst.Arbitration.CompleteFaultReset(arbitration.ResetResultInternalError, derefPLCStatus(result.Status), "fault reset command audit was not recorded")
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"result": result, "arbitration": inst.Arbitration.Status(), "fault_reset_id": nullableID(resetID)})
}

func (h *handler) completeFaultReset(resetID int64, resumeAuto bool, event plc.CommandLifecycleEvent) {
	inst, _ := h.gunInstance(event.GunID)
	h.completeFaultResetFor(inst, resetID, resumeAuto, event)
}

func (h *handler) completeFaultResetFor(inst *guns.Instance, resetID int64, resumeAuto bool, event plc.CommandLifecycleEvent) {
	completion := event.Completion
	final := plc.GunStatus{AcceptedSequence: event.SubmittedSequence, FaultCode: completion.FaultCode, State: completion.ResultingState}
	resetResult := mapFaultResetResult(completion)
	message := completion.Error
	if resetResult == arbitration.ResetResultSucceeded && completion.FaultCode != 0 {
		resetResult = arbitration.ResetResultFaultPersisted
		message = stringsJoin(message, "fault persisted after reset")
	}
	if inst == nil || inst.Arbitration == nil {
		return
	}
	arbStatus := inst.Arbitration.CompleteFaultReset(resetResult, final, message)
	if inst.Auto != nil && resetResult == arbitration.ResetResultSucceeded {
		if arbStatus.Mode == arbitration.ModeAuto {
			inst.Auto.SetEnabled(true)
			inst.Auto.ResumeAfterFaultReset()
		} else {
			inst.Auto.SetEnabled(false)
		}
	}
	now := completion.CompletionTime
	if now.IsZero() {
		now = time.Now().UTC()
	}
	faultAfter := completion.FaultCode
	h.updateFaultReset(context.Background(), resetID, history.FaultResetUpdate{
		SubmittedSequence: optionalUint32(event.SubmittedSequence, true),
		Result:            string(resetResult),
		FaultCodeAfter:    &faultAfter,
		CompletedAt:       &now,
		Error:             message,
	})
	_ = resumeAuto
}

func (h *handler) submitTarget(w http.ResponseWriter, r *http.Request) {
	var target autocontrol.Target
	if !decodeJSON(w, r, &target) {
		return
	}
	if h.deps.TargetFeed != nil {
		result, err := h.deps.TargetFeed.Ingest(r.Context(), targetfeed.Target{
			Source:      targetfeed.SourceManualAPI,
			TargetID:    target.TargetID,
			Azimuth:     target.Azimuth,
			Elevation:   target.Elevation,
			Priority:    target.Priority,
			TargetClass: target.TargetClass,
			DetectedAt:  target.DetectedAt,
			Metadata:    target.Metadata,
		})
		if err != nil {
			writeTargetFeedError(w, err, result)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]interface{}{"feed": result, "target_feed": h.deps.TargetFeed.Status()})
		return
	}
	if h.deps.AutoGun1 == nil {
		writeError(w, http.StatusServiceUnavailable, "gun 1 auto controller is unavailable")
		return
	}
	status, err := h.deps.AutoGun1.SubmitTarget(r.Context(), target)
	if err != nil {
		if errors.Is(err, autocontrol.ErrConflict) {
			writeJSON(w, http.StatusConflict, map[string]interface{}{"error": err.Error(), "auto": status})
			return
		}
		if errors.Is(err, autocontrol.ErrValidation) {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error(), "auto": status})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error(), "auto": status})
		return
	}
	writeJSON(w, http.StatusAccepted, status)
}

func (h *handler) targetFeedStatus(w http.ResponseWriter, r *http.Request) {
	if h.deps.TargetFeed == nil {
		writeJSON(w, http.StatusOK, targetfeed.Status{Source: targetfeed.SourceExternalGenerator, Configured: false})
		return
	}
	writeJSON(w, http.StatusOK, h.deps.TargetFeed.Status())
}

func (h *handler) targetFeedTargets(w http.ResponseWriter, r *http.Request) {
	if h.deps.TargetFeed == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"targets": []targetfeed.Record{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"targets": h.deps.TargetFeed.Targets()})
}

func (h *handler) dispatcherStatus(w http.ResponseWriter, r *http.Request) {
	if h.deps.Dispatcher == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"enabled": false, "mode": "disabled", "ready": false})
		return
	}
	writeJSON(w, http.StatusOK, h.deps.Dispatcher.Status(r.Context()))
}

func (h *handler) dispatcherDecisions(w http.ResponseWriter, r *http.Request) {
	store, ok := h.dispatcherStore(w)
	if !ok {
		return
	}
	filter := history.DispatcherDecisionFilter{Limit: parseBoundedQueryInt(r, "limit", 100, 1, maxHistoryPageSize), Offset: parseBoundedQueryInt(r, "offset", 0, 0, maxHistoryOffset), Mode: r.URL.Query().Get("mode"), Outcome: r.URL.Query().Get("outcome")}
	if raw := r.URL.Query().Get("target_id"); raw != "" {
		if n, err := strconv.ParseUint(raw, 10, 32); err == nil {
			v := uint32(n)
			filter.TargetID = &v
		}
	}
	if raw := r.URL.Query().Get("selected_gun_id"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			filter.SelectedGunID = &n
		}
	}
	rows, err := store.ListDispatcherDecisions(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": rows, "limit": filter.Limit, "offset": filter.Offset})
}

func (h *handler) dispatcherDecision(w http.ResponseWriter, r *http.Request) {
	store, ok := h.dispatcherStore(w)
	if !ok {
		return
	}
	id, ok := parseIDAfterPrefix(w, r.URL.Path, "/api/v1/dispatcher/decisions/")
	if !ok {
		return
	}
	row, found, err := store.GetDispatcherDecision(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "dispatcher decision not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *handler) dispatcherAssignments(w http.ResponseWriter, r *http.Request) {
	store, ok := h.dispatcherStore(w)
	if !ok {
		return
	}
	filter := history.TargetAssignmentFilter{Limit: parseBoundedQueryInt(r, "limit", 100, 1, maxHistoryPageSize), Offset: parseBoundedQueryInt(r, "offset", 0, 0, maxHistoryOffset), State: r.URL.Query().Get("state")}
	if raw := r.URL.Query().Get("target_id"); raw != "" {
		if n, err := strconv.ParseUint(raw, 10, 32); err == nil {
			v := uint32(n)
			filter.TargetID = &v
		}
	}
	if raw := r.URL.Query().Get("gun_id"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			filter.GunID = &n
		}
	}
	if raw := r.URL.Query().Get("active"); raw != "" {
		if b, err := strconv.ParseBool(raw); err == nil {
			filter.Active = &b
		}
	}
	rows, err := store.ListTargetAssignments(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": rows, "limit": filter.Limit, "offset": filter.Offset})
}

func (h *handler) dispatcherAssignment(w http.ResponseWriter, r *http.Request) {
	store, ok := h.dispatcherStore(w)
	if !ok {
		return
	}
	id, ok := parseIDAfterPrefix(w, r.URL.Path, "/api/v1/dispatcher/assignments/")
	if !ok {
		return
	}
	row, found, err := store.GetTargetAssignment(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "dispatcher assignment not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *handler) dispatcherStore(w http.ResponseWriter) (history.DispatcherStore, bool) {
	store, ok := h.deps.History.(history.DispatcherStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "dispatcher persistence is unavailable", "adapter": h.deps.History.Name()})
		return nil, false
	}
	return store, true
}

func (h *handler) testControlsAllowed(w http.ResponseWriter, r *http.Request) bool {
	if !testcontrol.TokenOK(r.Header.Get("X-Stonehenge-Test-Token")) {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "not found"})
		return false
	}
	return true
}

func (h *handler) testPauseStatus(w http.ResponseWriter, r *http.Request) {
	if !h.testControlsAllowed(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, testcontrol.Status())
}

func (h *handler) testPauseArm(w http.ResponseWriter, r *http.Request) {
	if !h.testControlsAllowed(w, r) {
		return
	}
	var req testcontrol.PauseRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	switch req.Point {
	case testcontrol.PointBeforeAutoSubmit, testcontrol.PointAfterCommit, testcontrol.PointAfterShotEvidence:
	default:
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "unsupported pause point"})
		return
	}
	if !testcontrol.Arm(req) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "test controls disabled"})
		return
	}
	writeJSON(w, http.StatusAccepted, testcontrol.Status())
}

func (h *handler) testPauseRelease(w http.ResponseWriter, r *http.Request) {
	if !h.testControlsAllowed(w, r) {
		return
	}
	var req testcontrol.PauseRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !testcontrol.Release(req.Point) {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "pause point not armed"})
		return
	}
	writeJSON(w, http.StatusOK, testcontrol.Status())
}

type testResolveAssignmentRequest struct {
	AssignmentID int64 `json:"assignment_id"`
	Generation   int   `json:"generation"`
}

func (h *handler) testResolveAssignment(w http.ResponseWriter, r *http.Request) {
	if !h.testControlsAllowed(w, r) {
		return
	}
	store, ok := h.dispatcherStore(w)
	if !ok {
		return
	}
	var req testResolveAssignmentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.AssignmentID <= 0 || req.Generation <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "assignment_id and generation are required"})
		return
	}
	row, found, err := store.GetTargetAssignment(r.Context(), req.AssignmentID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "dispatcher assignment not found"})
		return
	}
	if row.Generation != req.Generation {
		writeJSON(w, http.StatusConflict, map[string]interface{}{"error": "assignment generation mismatch", "assignment": row})
		return
	}
	if !terminalAssignmentState(row.State) {
		writeJSON(w, http.StatusConflict, map[string]interface{}{"error": "assignment is not terminal", "assignment": row})
		return
	}
	inst, ok := h.gunInstance(row.GunID)
	if !ok || inst.Auto == nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "gun auto controller not found", "gun_id": row.GunID})
		return
	}
	cleared := inst.Auto.ClearResolvedAssignment(row.ID, row.Generation)
	if !cleared {
		writeJSON(w, http.StatusConflict, map[string]interface{}{"error": "auto controller is not holding this assignment", "assignment": row, "auto": inst.Auto.Status(r.Context())})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"cleared": true, "assignment": row, "auto": inst.Auto.Status(r.Context())})
}

func terminalAssignmentState(state string) bool {
	switch state {
	case "completed", "released", "requeued", "terminal_failed", "terminal_uncertain", "expired":
		return true
	default:
		return false
	}
}

func (h *handler) testDispatcherReconcile(w http.ResponseWriter, r *http.Request) {
	if !h.testControlsAllowed(w, r) {
		return
	}
	if h.deps.Dispatcher == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "dispatcher unavailable"})
		return
	}
	h.deps.Dispatcher.Reconcile(r.Context())
	writeJSON(w, http.StatusOK, h.deps.Dispatcher.Status(r.Context()))
}

func parseBoundedQueryInt(r *http.Request, name string, fallback, minimum, maximum int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < minimum {
		return fallback
	}
	return min(n, maximum)
}

func parseIDAfterPrefix(w http.ResponseWriter, path, prefix string) (int64, bool) {
	raw := strings.TrimPrefix(path, prefix)
	id, err := strconv.ParseInt(strings.Trim(raw, "/"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func (h *handler) safetyStatus(w http.ResponseWriter, r *http.Request) {
	if h.deps.SafetyPLC == nil {
		writeJSON(w, http.StatusOK, safety.Snapshot{
			Enabled:            h.deps.Config.SafetyPLC.Enabled,
			EnforcementEnabled: h.deps.Config.SafetyPLC.Enforcement,
			Configured:         false,
			Connected:          false,
			Fresh:              false,
			State:              safety.StateOffline,
			StateName:          safety.StateOffline.Name(),
			Guns:               []safety.GunStatus{},
			LastError:          "safety plc service is unavailable",
			LastErrorCode:      safety.ErrorControllerUnavailable,
		})
		return
	}
	writeJSON(w, http.StatusOK, h.deps.SafetyPLC.Snapshot())
}

func (h *handler) safetyGlobal(w http.ResponseWriter, r *http.Request) {
	if h.deps.SafetyPLC == nil {
		writeError(w, http.StatusServiceUnavailable, "safety plc service is unavailable")
		return
	}
	req, ok := decodeSafetyGlobalControl(w, r)
	if !ok {
		return
	}
	if err := h.deps.SafetyPLC.SetGlobal(r.Context(), req); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": err.Error(), "safety": h.deps.SafetyPLC.Snapshot()})
		return
	}
	writeJSON(w, http.StatusOK, h.deps.SafetyPLC.Snapshot())
}

func (h *handler) safetyGun(w http.ResponseWriter, r *http.Request) {
	if h.deps.SafetyPLC == nil {
		writeError(w, http.StatusServiceUnavailable, "safety plc service is unavailable")
		return
	}
	id, ok := parseSafetyGunPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, "gun id must be between 1 and 8")
		return
	}
	req, ok := decodeSafetyGunControl(w, r)
	if !ok {
		return
	}
	if err := h.deps.SafetyPLC.SetGun(r.Context(), id, req); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": err.Error(), "safety": h.deps.SafetyPLC.Snapshot()})
		return
	}
	writeJSON(w, http.StatusOK, h.deps.SafetyPLC.Snapshot())
}

func (h *handler) safetyReset(w http.ResponseWriter, r *http.Request) {
	if h.deps.SafetyPLC == nil {
		writeError(w, http.StatusServiceUnavailable, "safety plc service is unavailable")
		return
	}
	if err := h.deps.SafetyPLC.PulseReset(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": err.Error(), "safety": h.deps.SafetyPLC.Snapshot()})
		return
	}
	writeJSON(w, http.StatusAccepted, h.deps.SafetyPLC.Snapshot())
}

func (h *handler) recordSafetyDenied(ctx context.Context, gunID int, decision safety.Decision) {
	auditor, ok := h.deps.History.(safety.EventAuditor)
	if !ok {
		return
	}
	auditCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := auditor.RecordSafetyEvent(auditCtx, safety.Event{
		EventTime: time.Now().UTC(),
		EventType: "fire_denied",
		GunID:     &gunID,
		Details: map[string]any{
			"reason":                decision.Reason,
			"reason_code":           decision.ReasonCode,
			"global_inhibit_reason": decision.GlobalInhibitReason,
			"gun_inhibit_reason":    decision.GunInhibitReason,
		},
	}); err != nil && h.deps.Logger != nil {
		h.deps.Logger.Warn("safety denied event audit write failed", "gun_id", gunID, "error", err)
	}
}

func (h *handler) ingestFragment(w http.ResponseWriter, r *http.Request) {
	if h.deps.Dispatcher != nil && !h.deps.Dispatcher.TargetIngestAllowed(r.Context()) {
		status := h.deps.Dispatcher.Status(r.Context())
		if status.NoEligibleGunReason == string(dispatcher.ReasonTargetQueueFull) {
			writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
				"error": "target_queue_full",
				"state": status,
			})
			return
		}
		w.Header().Set("Retry-After", "30")
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error":               "stack_not_ready",
			"state":               status,
			"retry_after_seconds": 30,
		})
		return
	}
	var req struct {
		ExternalID          string         `json:"external_id"`
		SourceID            string         `json:"source_id"`
		Bearing             float64        `json:"bearing"`
		Elevation           float64        `json:"elevation"`
		MapX                float64        `json:"map_x"`
		MapY                float64        `json:"map_y"`
		Priority            model.Priority `json:"priority"`
		TerrainType         string         `json:"terrain_type"`
		StrategicZone       string         `json:"strategic_zone"`
		TimeToImpactSeconds int            `json:"time_to_impact_seconds"`
		ObservedAt          *time.Time     `json:"observed_at"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	observedAt := time.Now().UTC()
	if req.ObservedAt != nil {
		observedAt = req.ObservedAt.UTC()
	}
	target := fragmentTarget(req.ExternalID, req.SourceID, req.Bearing, req.Elevation, req.Priority, req.TerrainType, req.StrategicZone, req.TimeToImpactSeconds, observedAt)
	fragment, err := h.deps.Engine.IngestFragment(r.Context(), fragments.Input{
		ExternalID:          req.ExternalID,
		Bearing:             req.Bearing,
		Elevation:           req.Elevation,
		MapX:                req.MapX,
		MapY:                req.MapY,
		Priority:            req.Priority,
		TerrainType:         req.TerrainType,
		StrategicZone:       req.StrategicZone,
		TimeToImpactSeconds: req.TimeToImpactSeconds,
		ObservedAt:          observedAt,
	})
	if err != nil {
		status := http.StatusBadRequest
		var duplicate fragments.DuplicateError
		if errors.As(err, &duplicate) {
			if h.deps.TargetFeed != nil && h.deps.Store != nil {
				if _, active := h.deps.Store.ActiveFragmentByExternalID(req.ExternalID); active {
					result, feedErr := h.deps.TargetFeed.Ingest(r.Context(), target)
					if feedErr == nil {
						writeJSON(w, http.StatusAccepted, map[string]interface{}{"duplicate": true, "feed": result, "fragment_error": err.Error()})
						return
					}
				}
			}
			status = http.StatusConflict
		}
		if errors.Is(err, store.ErrFragmentCapacity) || errors.Is(err, fragments.ErrExternalIDCapacity) {
			status = http.StatusTooManyRequests
		}
		writeJSON(w, status, map[string]interface{}{"error": err.Error()})
		return
	}
	if h.deps.TargetFeed != nil {
		result, err := h.deps.TargetFeed.Ingest(r.Context(), target)
		if err != nil {
			writeTargetFeedError(w, err, result)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]interface{}{"fragment": fragment, "feed": result})
		return
	}
	writeJSON(w, http.StatusAccepted, fragment)
}

func fragmentTarget(externalID, sourceID string, bearing, elevation float64, priority model.Priority, terrainType, strategicZone string, tti int, observedAt time.Time) targetfeed.Target {
	source := sourceID
	if source == "" {
		source = targetfeed.SourceExternalGenerator
	}
	return targetfeed.Target{
		Source:      source,
		TargetID:    targetIDFromExternalID(externalID),
		Azimuth:     bearing,
		Elevation:   elevation,
		Priority:    priorityScore(priority),
		TargetClass: "fragment",
		DetectedAt:  observedAt,
		Metadata: map[string]interface{}{
			"external_id":            externalID,
			"terrain_type":           terrainType,
			"strategic_zone":         strategicZone,
			"time_to_impact_seconds": tti,
			"original_priority":      priority,
			"source":                 targetfeed.SourceExternalGenerator,
		},
	}
}

func targetIDFromExternalID(externalID string) uint32 {
	parts := strings.Split(externalID, "-")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if n, err := strconv.ParseUint(last, 10, 32); err == nil && n != 0 {
			return uint32(n)
		}
	}
	id := crc32.ChecksumIEEE([]byte(externalID))
	if id == 0 {
		return 1
	}
	return id
}

func priorityScore(priority model.Priority) int {
	switch priority {
	case model.PriorityCritical:
		return 100
	case model.PriorityHigh:
		return 75
	case model.PriorityMedium:
		return 50
	case model.PriorityLow:
		return 25
	default:
		return 0
	}
}

func writeTargetFeedError(w http.ResponseWriter, err error, result targetfeed.IngestResult) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, targetfeed.ErrQueueFull):
		status = http.StatusTooManyRequests
	case errors.Is(err, targetfeed.ErrStale):
		status = http.StatusGone
	case errors.Is(err, targetfeed.ErrValidation):
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]interface{}{"error": err.Error(), "feed": result})
}

func (h *handler) setMode(w http.ResponseWriter, r *http.Request, id int) {
	var req struct {
		Mode model.GunMode `json:"mode"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	gun, err := h.deps.Engine.SetMode(id, req.Mode)
	writeEngineResult(w, gun, err)
}

func (h *handler) manualTarget(w http.ResponseWriter, r *http.Request, id int) {
	var req struct {
		Azimuth   float64 `json:"azimuth"`
		Elevation float64 `json:"elevation"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	gun, err := h.deps.Engine.SetManualTarget(id, req.Azimuth, req.Elevation)
	writeEngineResult(w, gun, err)
}

func (h *handler) fireRequest(w http.ResponseWriter, id int) {
	gun, err := h.deps.Engine.FireRequest(id)
	writeEngineResult(w, gun, err)
}

func (h *handler) fault(w http.ResponseWriter, r *http.Request, id int) {
	var req struct {
		FaultCode string `json:"fault_code"`
	}
	if !decodeOptionalJSON(w, r, &req) {
		return
	}
	gun, err := h.deps.Engine.FaultGun(id, req.FaultCode)
	writeEngineResult(w, gun, err)
}

func (h *handler) clearFault(w http.ResponseWriter, id int) {
	gun, err := h.deps.Engine.ClearFault(id)
	writeEngineResult(w, gun, err)
}

func (h *handler) adminOverride(w http.ResponseWriter, r *http.Request, id int) {
	var req struct {
		Active     bool       `json:"active"`
		ApprovedBy string     `json:"approved_by"`
		ExpiresAt  *time.Time `json:"expires_at"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	gun, err := h.deps.Engine.SetAdminOverride(id, req.Active, req.ApprovedBy, req.ExpiresAt)
	writeEngineResult(w, gun, err)
}

func parseGunPath(path string) (int, string, bool) {
	trimmed := strings.TrimPrefix(path, "/api/v1/guns/")
	if trimmed == path || trimmed == "" {
		return 0, "", false
	}
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}
	action := ""
	if len(parts) > 1 {
		action = strings.Join(parts[1:], "/")
	}
	return id, action, true
}

func parseSafetyGunPath(path string) (int, bool) {
	trimmed := strings.TrimPrefix(path, "/api/v1/safety/guns/")
	if trimmed == path || trimmed == "" {
		return 0, false
	}
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 1 {
		return 0, false
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil || id < 1 || id > 8 {
		return 0, false
	}
	return id, true
}

func parseHistoryGunPath(path string) (int, string, bool) {
	trimmed := strings.TrimPrefix(path, "/api/v1/history/guns/")
	if trimmed == path || trimmed == "" {
		return 0, "", false
	}
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 {
		return 0, "", false
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}
	return id, parts[1], true
}

type baseHistoryFilter struct {
	limit  int
	offset int
	start  *time.Time
	end    *time.Time
}

func parseFragmentHistoryFilter(w http.ResponseWriter, r *http.Request) (history.FragmentFilter, bool) {
	base, ok := parseBaseHistoryFilter(w, r)
	if !ok {
		return history.FragmentFilter{}, false
	}
	return history.FragmentFilter{
		Limit:  base.limit,
		Offset: base.offset,
		Start:  base.start,
		End:    base.end,
		Status: r.URL.Query().Get("status"),
	}, true
}

func parseBaseHistoryFilter(w http.ResponseWriter, r *http.Request) (baseHistoryFilter, bool) {
	q := r.URL.Query()
	limit := 100
	if raw := q.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return baseHistoryFilter{}, false
		}
		limit = parsed
	}
	if limit > maxHistoryPageSize {
		limit = maxHistoryPageSize
	}
	offset := 0
	if raw := q.Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
			return baseHistoryFilter{}, false
		}
		offset = parsed
	}
	if offset > maxHistoryOffset {
		offset = maxHistoryOffset
	}
	start, ok := parseOptionalTime(w, q.Get("start"), "start")
	if !ok {
		return baseHistoryFilter{}, false
	}
	end, ok := parseOptionalTime(w, q.Get("end"), "end")
	if !ok {
		return baseHistoryFilter{}, false
	}
	return baseHistoryFilter{limit: limit, offset: offset, start: start, end: end}, true
}

func parseOptionalTime(w http.ResponseWriter, raw, name string) (*time.Time, bool) {
	if raw == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, name+" must be an RFC3339 timestamp")
		return nil, false
	}
	parsed = parsed.UTC()
	return &parsed, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)).Decode(dst); err != nil {
		writeJSONDecodeError(w, err, "invalid json body")
		return false
	}
	return true
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)).Decode(dst)
	if errors.Is(err, io.EOF) {
		return true
	}
	if err != nil {
		writeJSONDecodeError(w, err, "invalid json body")
		return false
	}
	return true
}

type safetyGlobalControlRequest struct {
	GlobalEnable                *bool `json:"global_enable,omitempty"`
	EmergencyStop               *bool `json:"emergency_stop,omitempty"`
	MaintenanceMode             *bool `json:"maintenance_mode,omitempty"`
	LegacyGlobalMaintenanceMode *bool `json:"global_maintenance_mode,omitempty"`
	TestFaultRequest            *bool `json:"test_fault_request,omitempty"`
}

type safetyGunControlRequest struct {
	Enabled                  *bool `json:"enabled,omitempty"`
	MaintenanceInhibit       *bool `json:"maintenance_inhibit,omitempty"`
	ExternalInhibit          *bool `json:"external_inhibit,omitempty"`
	LegacyEnabled            *bool `json:"gun_enabled,omitempty"`
	LegacyMaintenanceInhibit *bool `json:"gun_maintenance_inhibit,omitempty"`
	LegacyExternalInhibit    *bool `json:"gun_external_inhibit,omitempty"`
}

func decodeSafetyGlobalControl(w http.ResponseWriter, r *http.Request) (safety.GlobalControl, bool) {
	var raw safetyGlobalControlRequest
	if !decodeStrictJSON(w, r, &raw) {
		return safety.GlobalControl{}, false
	}
	maintenance, ok := mergeBoolAlias(w, "maintenance_mode", raw.MaintenanceMode, "global_maintenance_mode", raw.LegacyGlobalMaintenanceMode)
	if !ok {
		return safety.GlobalControl{}, false
	}
	req := safety.GlobalControl{
		GlobalEnable:          raw.GlobalEnable,
		EmergencyStop:         raw.EmergencyStop,
		GlobalMaintenanceMode: maintenance,
		TestFaultRequest:      raw.TestFaultRequest,
	}
	if req.GlobalEnable == nil && req.EmergencyStop == nil && req.GlobalMaintenanceMode == nil && req.TestFaultRequest == nil {
		writeError(w, http.StatusBadRequest, "at least one safety control field is required")
		return safety.GlobalControl{}, false
	}
	return req, true
}

func decodeSafetyGunControl(w http.ResponseWriter, r *http.Request) (safety.GunControl, bool) {
	var raw safetyGunControlRequest
	if !decodeStrictJSON(w, r, &raw) {
		return safety.GunControl{}, false
	}
	enabled, ok := mergeBoolAlias(w, "enabled", raw.Enabled, "gun_enabled", raw.LegacyEnabled)
	if !ok {
		return safety.GunControl{}, false
	}
	maintenance, ok := mergeBoolAlias(w, "maintenance_inhibit", raw.MaintenanceInhibit, "gun_maintenance_inhibit", raw.LegacyMaintenanceInhibit)
	if !ok {
		return safety.GunControl{}, false
	}
	external, ok := mergeBoolAlias(w, "external_inhibit", raw.ExternalInhibit, "gun_external_inhibit", raw.LegacyExternalInhibit)
	if !ok {
		return safety.GunControl{}, false
	}
	req := safety.GunControl{
		Enabled:            enabled,
		MaintenanceInhibit: maintenance,
		ExternalInhibit:    external,
	}
	if req.Enabled == nil && req.MaintenanceInhibit == nil && req.ExternalInhibit == nil {
		writeError(w, http.StatusBadRequest, "at least one safety control field is required")
		return safety.GunControl{}, false
	}
	return req, true
}

func mergeBoolAlias(w http.ResponseWriter, canonicalName string, canonical *bool, legacyName string, legacy *bool) (*bool, bool) {
	if canonical != nil && legacy != nil {
		if *canonical != *legacy {
			writeError(w, http.StatusBadRequest, canonicalName+" conflicts with legacy alias "+legacyName)
			return nil, false
		}
		return canonical, true
	}
	if canonical != nil {
		return canonical, true
	}
	return legacy, true
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "request body is required")
			return false
		}
		writeJSONDecodeError(w, err, jsonDecodeErrorMessage(err))
		return false
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		writeJSONDecodeError(w, err, jsonDecodeErrorMessage(err))
		return false
	}
	writeError(w, http.StatusBadRequest, "request body must contain a single JSON value")
	return false
}

func writeJSONDecodeError(w http.ResponseWriter, err error, fallback string) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}
	writeError(w, http.StatusBadRequest, fallback)
}

func jsonDecodeErrorMessage(err error) string {
	const prefix = "json: "
	message := err.Error()
	if strings.HasPrefix(message, prefix) {
		return strings.TrimPrefix(message, prefix)
	}
	return message
}

func writeEngineResult(w http.ResponseWriter, gun model.GunState, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, gun)
		return
	}
	if errors.Is(err, sim.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error(), "gun": gun})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]interface{}{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
