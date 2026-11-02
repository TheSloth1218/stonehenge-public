package history

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/model"
	"stonehenge-middleware/internal/plc"
	"stonehenge-middleware/internal/safety"
)

const shutdownDrainTimeout = 10 * time.Second

type PostgresHistorySink struct {
	cfg       config.HistoryConfig
	pool      *pgxpool.Pool
	queue     chan model.OperationalEvent
	stop      chan struct{}
	done      chan struct{}
	logger    *slog.Logger
	mu        sync.RWMutex
	closed    bool
	connected bool
	migrated  bool
	lastError string
	lastOK    *time.Time

	successfulWrites atomic.Uint64
	retries          atomic.Uint64
	failedWrites     atomic.Uint64
	droppedEvents    atomic.Uint64
}

func NewPostgresHistorySink(cfg config.HistoryConfig, logger *slog.Logger) (*PostgresHistorySink, error) {
	if logger == nil {
		logger = slog.Default()
	}
	poolCfg, err := pgxpool.ParseConfig(postgresConnString(cfg))
	if err != nil {
		return nil, err
	}
	poolCfg.MaxConns = cfg.PostgresMaxConnections
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, err
	}
	s := &PostgresHistorySink{
		cfg:    cfg,
		pool:   pool,
		queue:  make(chan model.OperationalEvent, cfg.WriteQueueSize),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		logger: logger,
	}
	migrationCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.ensureMigrations(migrationCtx); err != nil {
		s.setError(err)
		logger.Warn("postgres history migrations pending", "error", err)
	}
	go s.worker()
	return s, nil
}

func postgresConnString(cfg config.HistoryConfig) string {
	values := map[string]string{
		"host":     cfg.PostgresHost,
		"port":     strconv.Itoa(cfg.PostgresPort),
		"dbname":   cfg.PostgresDB,
		"user":     cfg.PostgresUser,
		"password": cfg.PostgresPassword,
		"sslmode":  cfg.PostgresSSLMode,
	}
	parts := make([]string, 0, len(values))
	for key, value := range values {
		parts = append(parts, key+"="+quoteConnValue(value))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func quoteConnValue(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
}

func (s *PostgresHistorySink) Name() string {
	return "postgres"
}

func (s *PostgresHistorySink) WriteEvent(ctx context.Context, event model.OperationalEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.EventUUID == "" {
		event.EventUUID = operationalEventUUID(event)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil
	}
	select {
	case s.queue <- event:
		return nil
	default:
		s.droppedEvents.Add(1)
		s.logger.Warn("postgres history queue full; dropping event", "event_id", event.ID, "event_uuid", event.EventUUID, "event_type", event.EventType)
		return nil
	}
}

func operationalEventUUID(event model.OperationalEvent) string {
	key := fmt.Sprintf("%d", event.ID)
	if event.ID == 0 {
		encoded, err := json.Marshal(event)
		if err == nil {
			key = string(encoded)
		} else {
			key = fmt.Sprintf("%s:%d", event.EventType, event.Timestamp.UnixNano())
		}
	}
	return deterministicUUID("stonehenge-operational-event", key)
}

func (s *PostgresHistorySink) Health() model.HistoryHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return model.HistoryHealth{
		Name:             s.Name(),
		Adapter:          s.Name(),
		Healthy:          s.connected && s.lastError == "",
		Connected:        s.connected,
		QueueDepth:       len(s.queue),
		SuccessfulWrites: s.successfulWrites.Load(),
		Retries:          s.retries.Load(),
		FailedWrites:     s.failedWrites.Load(),
		DroppedEvents:    s.droppedEvents.Load(),
		LastWriteAt:      s.lastOK,
		LastSuccessAt:    s.lastOK,
		LastError:        s.lastError,
	}
}

func (s *PostgresHistorySink) Ready(ctx context.Context) bool {
	if err := s.pool.Ping(ctx); err != nil {
		s.setError(err)
		return false
	}
	s.setOK()
	return true
}

func (s *PostgresHistorySink) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	close(s.stop)
	close(s.queue)
	select {
	case <-s.done:
	case <-time.After(shutdownDrainTimeout):
		s.logger.Warn("postgres history sink shutdown drain timed out", "queue_depth", len(s.queue))
	}
	s.pool.Close()
	return nil
}

func (s *PostgresHistorySink) worker() {
	defer close(s.done)
	for event := range s.queue {
		s.writeWithRetry(event)
	}
}

func (s *PostgresHistorySink) writeWithRetry(event model.OperationalEvent) {
	for {
		select {
		case <-s.stop:
			return
		default:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := s.writeEvent(ctx, event)
		cancel()
		if err == nil {
			s.successfulWrites.Add(1)
			s.setOK()
			return
		}
		s.retries.Add(1)
		s.failedWrites.Add(1)
		s.setError(err)
		s.logger.Warn("postgres history write failed; retrying", "event_uuid", event.EventUUID, "event_type", event.EventType, "error", err)
		timer := time.NewTimer(s.cfg.RetryInterval)
		select {
		case <-timer.C:
		case <-s.stop:
			timer.Stop()
			return
		}
	}
}

func (s *PostgresHistorySink) writeEvent(ctx context.Context, event model.OperationalEvent) error {
	if err := s.ensureMigrations(ctx); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.applyEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresHistorySink) applyEvent(ctx context.Context, tx pgx.Tx, event model.OperationalEvent) error {
	switch event.EventType {
	case model.EventFragmentReceived:
		if err := insertFragmentTrack(ctx, tx, event); err != nil {
			return err
		}
		return insertGunEvent(ctx, tx, event)
	case model.EventTargetAssigned:
		if event.FragmentID != nil && event.GunID != nil {
			if _, err := tx.Exec(ctx, `UPDATE fragment_tracks SET assigned_gun_id=$1, status='ASSIGNED' WHERE id=$2`, *event.GunID, eventFragmentUUID(event)); err != nil {
				return err
			}
		}
		return insertGunEvent(ctx, tx, event)
	case model.EventGunFired:
		if err := insertShotRecord(ctx, tx, event); err != nil {
			return err
		}
		return insertGunEvent(ctx, tx, event)
	case model.EventInterceptRecorded:
		if event.FragmentID != nil {
			if _, err := tx.Exec(ctx, `UPDATE fragment_tracks SET status='INTERCEPTED', intercepted_at=$1 WHERE id=$2`, event.Timestamp, eventFragmentUUID(event)); err != nil {
				return err
			}
		}
		return insertGunEvent(ctx, tx, event)
	case model.EventFragmentMissed:
		if event.FragmentID != nil {
			if _, err := tx.Exec(ctx, `UPDATE fragment_tracks SET status='MISSED', missed_at=$1 WHERE id=$2`, event.Timestamp, eventFragmentUUID(event)); err != nil {
				return err
			}
		}
		return insertGunEvent(ctx, tx, event)
	case model.EventGunSlewStarted, model.EventGunOnTarget, model.EventFireRequested,
		model.EventSafetyPermissiveGranted, model.EventSafetyPermissiveBlocked,
		model.EventGunModeChanged, model.EventGunMaintenanceChanged, model.EventAdminOverrideChanged,
		model.EventGunFaulted, model.EventFaultCleared:
		return insertGunEvent(ctx, tx, event)
	default:
		return insertGunEvent(ctx, tx, event)
	}
}

func insertFragmentTrack(ctx context.Context, tx pgx.Tx, event model.OperationalEvent) error {
	if event.FragmentID == nil {
		return nil
	}
	receivedAt := detailTime(event, "received_at", event.Timestamp)
	observedAt := detailTime(event, "observed_at", event.Timestamp)
	impactAt := detailTime(event, "impact_at", observedAt)
	status := detailString(event, "status", string(model.FragmentActive))
	_, err := tx.Exec(ctx, `
INSERT INTO fragment_tracks (
	id, external_id, source_id, received_at, observed_at, bearing, elevation, map_x, map_y,
	priority, terrain_type, strategic_zone, impact_at, status, assigned_gun_id, intercepted_at, missed_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
ON CONFLICT (id) DO UPDATE SET
	external_id=EXCLUDED.external_id,
	source_id=EXCLUDED.source_id,
	received_at=EXCLUDED.received_at,
	observed_at=EXCLUDED.observed_at,
	bearing=EXCLUDED.bearing,
	elevation=EXCLUDED.elevation,
	map_x=EXCLUDED.map_x,
	map_y=EXCLUDED.map_y,
	priority=EXCLUDED.priority,
	terrain_type=EXCLUDED.terrain_type,
	strategic_zone=EXCLUDED.strategic_zone,
	impact_at=EXCLUDED.impact_at,
	status=EXCLUDED.status,
	assigned_gun_id=EXCLUDED.assigned_gun_id,
	intercepted_at=EXCLUDED.intercepted_at,
	missed_at=EXCLUDED.missed_at`,
		eventFragmentUUID(event),
		optionalString(event.ExternalFragmentID),
		detailString(event, "source_id", ""),
		receivedAt,
		observedAt,
		optionalFloat(event.Azimuth),
		optionalFloat(event.Elevation),
		detailFloat(event, "map_x", 0),
		detailFloat(event, "map_y", 0),
		detailString(event, "priority", ""),
		detailString(event, "terrain_type", ""),
		detailString(event, "strategic_zone", ""),
		impactAt,
		status,
		detailIntPtr(event, "assigned_gun_id"),
		detailTimePtr(event, "intercepted_at"),
		detailTimePtr(event, "missed_at"),
	)
	return err
}

func insertGunEvent(ctx context.Context, tx pgx.Tx, event model.OperationalEvent) error {
	details, err := json.Marshal(event.Details)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO gun_events (
	id, event_time, event_type, severity, gun_id, fragment_id, external_fragment_id,
	azimuth, elevation, mode, shell_type, shot_sequence, details
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT (id) DO NOTHING`,
		event.EventUUID,
		event.Timestamp,
		event.EventType,
		event.Severity,
		event.GunID,
		eventFragmentUUIDPtr(event),
		optionalString(event.ExternalFragmentID),
		event.Azimuth,
		event.Elevation,
		event.Mode,
		event.ShellType,
		event.ShotSequence,
		details,
	)
	return err
}

func insertShotRecord(ctx context.Context, tx pgx.Tx, event model.OperationalEvent) error {
	if event.GunID == nil || event.Azimuth == nil || event.Elevation == nil || event.ShellType == nil || event.ShotSequence == nil {
		return insertGunEvent(ctx, tx, event)
	}
	tag, err := tx.Exec(ctx, `
INSERT INTO shot_records (
	id, gun_id, fragment_id, external_fragment_id, fired_at, azimuth, elevation,
	shell_type, shot_sequence, result
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT (id) DO NOTHING`,
		event.EventUUID,
		*event.GunID,
		eventFragmentUUIDPtr(event),
		optionalString(event.ExternalFragmentID),
		event.Timestamp,
		*event.Azimuth,
		*event.Elevation,
		*event.ShellType,
		*event.ShotSequence,
		detailString(event, "result", "FIRED"),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	standard := 0
	special := 0
	if *event.ShellType == model.ShellStandard {
		standard = 1
	} else {
		special = 1
	}
	_, err = tx.Exec(ctx, `
INSERT INTO gun_runtime_summary (
	gun_id, total_shots, standard_shells_fired, special_shells_fired, last_fired_at, updated_at
) VALUES ($1,1,$2,$3,$4,$4)
ON CONFLICT (gun_id) DO UPDATE SET
	total_shots=gun_runtime_summary.total_shots + 1,
	standard_shells_fired=gun_runtime_summary.standard_shells_fired + EXCLUDED.standard_shells_fired,
	special_shells_fired=gun_runtime_summary.special_shells_fired + EXCLUDED.special_shells_fired,
	last_fired_at=EXCLUDED.last_fired_at,
	updated_at=EXCLUDED.updated_at`,
		*event.GunID,
		standard,
		special,
		event.Timestamp,
	)
	return err
}

func (s *PostgresHistorySink) ensureMigrations(ctx context.Context) error {
	s.mu.RLock()
	migrated := s.migrated
	s.mu.RUnlock()
	if migrated {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.migrated {
		return nil
	}
	if err := RunMigrations(ctx, s.pool, s.cfg.MigrationsPath); err != nil {
		return err
	}
	s.migrated = true
	return nil
}

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, version); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresHistorySink) ListFragments(ctx context.Context, filter FragmentFilter) ([]FragmentTrack, error) {
	args := []interface{}{}
	clauses := []string{}
	if filter.Start != nil {
		args = append(args, *filter.Start)
		clauses = append(clauses, fmt.Sprintf("received_at >= $%d", len(args)))
	}
	if filter.End != nil {
		args = append(args, *filter.End)
		clauses = append(clauses, fmt.Sprintf("received_at <= $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	query := `SELECT id::text, COALESCE(external_id,''), COALESCE(source_id,''), received_at, observed_at,
		bearing, elevation, map_x, map_y, priority, terrain_type, strategic_zone, impact_at, status,
		assigned_gun_id, intercepted_at, missed_at FROM fragment_tracks`
	query = appendWhere(query, clauses)
	args = append(args, filter.Limit, filter.Offset)
	query += fmt.Sprintf(" ORDER BY received_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FragmentTrack{}
	for rows.Next() {
		var row FragmentTrack
		if err := rows.Scan(&row.ID, &row.ExternalID, &row.SourceID, &row.ReceivedAt, &row.ObservedAt, &row.Bearing, &row.Elevation, &row.MapX, &row.MapY, &row.Priority, &row.TerrainType, &row.StrategicZone, &row.ImpactAt, &row.Status, &row.AssignedGunID, &row.InterceptedAt, &row.MissedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresHistorySink) ListShots(ctx context.Context, filter ShotFilter) ([]ShotRecord, error) {
	args := []interface{}{}
	clauses := []string{}
	if filter.Start != nil {
		args = append(args, *filter.Start)
		clauses = append(clauses, fmt.Sprintf("fired_at >= $%d", len(args)))
	}
	if filter.End != nil {
		args = append(args, *filter.End)
		clauses = append(clauses, fmt.Sprintf("fired_at <= $%d", len(args)))
	}
	query := `SELECT id::text, gun_id, fragment_id::text, external_fragment_id, fired_at, azimuth, elevation,
		shell_type, shot_sequence, result FROM shot_records`
	query = appendWhere(query, clauses)
	args = append(args, filter.Limit, filter.Offset)
	query += fmt.Sprintf(" ORDER BY fired_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ShotRecord{}
	for rows.Next() {
		var row ShotRecord
		if err := rows.Scan(&row.ID, &row.GunID, &row.FragmentID, &row.ExternalFragmentID, &row.FiredAt, &row.Azimuth, &row.Elevation, &row.ShellType, &row.ShotSequence, &row.Result); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresHistorySink) ListGunEvents(ctx context.Context, gunID int, filter EventFilter) ([]GunEvent, error) {
	args := []interface{}{gunID}
	clauses := []string{"gun_id = $1"}
	if filter.Start != nil {
		args = append(args, *filter.Start)
		clauses = append(clauses, fmt.Sprintf("event_time >= $%d", len(args)))
	}
	if filter.End != nil {
		args = append(args, *filter.End)
		clauses = append(clauses, fmt.Sprintf("event_time <= $%d", len(args)))
	}
	if filter.EventType != "" {
		args = append(args, filter.EventType)
		clauses = append(clauses, fmt.Sprintf("event_type = $%d", len(args)))
	}
	query := `SELECT id::text, event_time, event_type, severity, gun_id, fragment_id::text, external_fragment_id,
		azimuth, elevation, mode, shell_type, shot_sequence, details FROM gun_events`
	query = appendWhere(query, clauses)
	args = append(args, filter.Limit, filter.Offset)
	query += fmt.Sprintf(" ORDER BY event_time DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GunEvent{}
	for rows.Next() {
		var row GunEvent
		var details []byte
		if err := rows.Scan(&row.ID, &row.EventTime, &row.EventType, &row.Severity, &row.GunID, &row.FragmentID, &row.ExternalFragmentID, &row.Azimuth, &row.Elevation, &row.Mode, &row.ShellType, &row.ShotSequence, &details); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(details, &row.Details)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresHistorySink) GetGunSummary(ctx context.Context, gunID int) (GunRuntimeSummary, bool, error) {
	var row GunRuntimeSummary
	err := s.pool.QueryRow(ctx, `SELECT gun_id, total_shots, standard_shells_fired, special_shells_fired, last_fired_at, last_service_at, updated_at FROM gun_runtime_summary WHERE gun_id=$1`, gunID).Scan(&row.GunID, &row.TotalShots, &row.StandardShellsFired, &row.SpecialShellsFired, &row.LastFiredAt, &row.LastServiceAt, &row.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return GunRuntimeSummary{}, false, nil
	}
	return row, err == nil, err
}

func (s *PostgresHistorySink) RecordGunCommand(ctx context.Context, audit plc.CommandAudit) (int64, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return 0, err
	}
	payload, err := json.Marshal(audit.Payload)
	if err != nil {
		return 0, err
	}
	if audit.Timestamp.IsZero() {
		audit.Timestamp = time.Now().UTC()
	}
	shotSequence := interface{}(nil)
	if audit.ShotSequence != 0 {
		shotSequence = audit.ShotSequence
	}
	var id int64
	err = s.pool.QueryRow(ctx, `
INSERT INTO gun_command_audit (
	command_time, gun_id, command_source, submitted_sequence, command_payload,
	result, resulting_state, fault_code, shot_sequence, error
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id`,
		audit.Timestamp,
		audit.GunID,
		audit.Source,
		audit.SubmittedSequence,
		payload,
		audit.Result,
		audit.ResultingState,
		audit.FaultCode,
		shotSequence,
		optionalAuditError(audit.Error),
	).Scan(&id)
	return id, err
}

func (s *PostgresHistorySink) UpdateGunCommandCompletion(ctx context.Context, auditID int64, completion plc.CommandAuditCompletion) error {
	if err := s.ensureMigrations(ctx); err != nil {
		return err
	}
	if completion.CompletionTime.IsZero() {
		completion.CompletionTime = time.Now().UTC()
	}
	shotSequence := nullableUint32Bigint(completion.ShotSequence)
	tag, err := s.pool.Exec(ctx, `
UPDATE gun_command_audit
SET result=$2,
	resulting_state=$3,
	fault_code=$4,
	shot_sequence=$5,
	error=$6,
	completion_time=$7,
	updated_at=now()
WHERE id=$1`,
		auditID,
		completion.Result,
		completion.ResultingState,
		completion.FaultCode,
		shotSequence,
		optionalAuditError(completion.Error),
		completion.CompletionTime,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("gun command audit id %d not found", auditID)
	}
	return nil
}

func (s *PostgresHistorySink) UpdateGunCommandLifecycleEvidence(ctx context.Context, auditID int64, evidence plc.CommandLifecycleEvidence) error {
	if err := s.ensureMigrations(ctx); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE gun_command_audit
SET baseline_accepted_sequence=COALESCE($2, baseline_accepted_sequence),
	confirmed_accepted_sequence=COALESCE($3, confirmed_accepted_sequence),
	baseline_shot_sequence=COALESCE($4, baseline_shot_sequence),
	last_observed_shot_sequence=COALESCE($5, last_observed_shot_sequence),
	shot_observed=COALESCE($6, shot_observed),
	last_observed_state=COALESCE($7, last_observed_state),
	firing_observed=COALESCE($8, firing_observed),
	lockout_observed=COALESCE($9, lockout_observed),
	ready_after_command_observed=COALESCE($10, ready_after_command_observed),
	command_committed_at=COALESCE($11, command_committed_at),
	last_lifecycle_observation_at=COALESCE($12, last_lifecycle_observation_at),
	lifecycle_uncertainty_reason=COALESCE(NULLIF($13,''), lifecycle_uncertainty_reason),
	updated_at=now()
WHERE id=$1`,
		auditID,
		nullableUint32Bigint(evidence.BaselineAcceptedSequence),
		nullableUint32Bigint(evidence.ConfirmedAcceptedSequence),
		nullableUint32Bigint(evidence.BaselineShotSequence),
		nullableUint32Bigint(evidence.LastObservedShotSequence),
		optionalBool(evidence.ShotObserved),
		nullableUint16Int(evidence.LastObservedState),
		optionalBool(evidence.FiringObserved),
		optionalBool(evidence.LockoutObserved),
		optionalBool(evidence.ReadyAfterCommandObserved),
		optionalTimeValue(evidence.CommandCommittedAt),
		optionalTimeValue(evidence.LastLifecycleObservationTime),
		evidence.UncertaintyReason,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("gun command audit id %d not found", auditID)
	}
	return nil
}

func (s *PostgresHistorySink) LatestGunCommandSequence(ctx context.Context, gunID int) (uint32, bool, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return 0, false, err
	}
	var sequence pgtype.Int8
	err := s.pool.QueryRow(ctx, `
SELECT max(submitted_sequence)::bigint
FROM gun_command_audit
WHERE gun_id=$1 AND submitted_sequence > 0`,
		gunID,
	).Scan(&sequence)
	if err != nil {
		return 0, false, err
	}
	if !sequence.Valid {
		return 0, false, nil
	}
	return uint32(sequence.Int64), true, nil
}

func (s *PostgresHistorySink) GetGunCommand(ctx context.Context, id int64) (GunCommandRecord, bool, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return GunCommandRecord{}, false, err
	}
	var out GunCommandRecord
	var payload []byte
	var shotSequence pgtype.Int8
	var completionTime pgtype.Timestamptz
	var updatedAt pgtype.Timestamptz
	var baselineAccepted pgtype.Int8
	var confirmedAccepted pgtype.Int8
	var baselineShot pgtype.Int8
	var lastObservedShot pgtype.Int8
	var shotObserved pgtype.Bool
	var lastObservedState pgtype.Int4
	var firingObserved pgtype.Bool
	var lockoutObserved pgtype.Bool
	var readyAfterCommandObserved pgtype.Bool
	var commandCommittedAt pgtype.Timestamptz
	var lastLifecycleObservationAt pgtype.Timestamptz
	err := s.pool.QueryRow(ctx, `
SELECT id, command_time, gun_id, command_source, submitted_sequence, command_payload,
	result, resulting_state, fault_code, shot_sequence, COALESCE(error,''), completion_time, updated_at,
	baseline_accepted_sequence, confirmed_accepted_sequence, baseline_shot_sequence,
	last_observed_shot_sequence, shot_observed, last_observed_state, firing_observed,
	lockout_observed, ready_after_command_observed, command_committed_at,
	last_lifecycle_observation_at, COALESCE(lifecycle_uncertainty_reason,'')
FROM gun_command_audit
WHERE id=$1`, id).Scan(
		&out.ID,
		&out.CommandTime,
		&out.GunID,
		&out.CommandSource,
		&out.SubmittedSequence,
		&payload,
		&out.Result,
		&out.ResultingState,
		&out.FaultCode,
		&shotSequence,
		&out.Error,
		&completionTime,
		&updatedAt,
		&baselineAccepted,
		&confirmedAccepted,
		&baselineShot,
		&lastObservedShot,
		&shotObserved,
		&lastObservedState,
		&firingObserved,
		&lockoutObserved,
		&readyAfterCommandObserved,
		&commandCommittedAt,
		&lastLifecycleObservationAt,
		&out.LifecycleEvidence.UncertaintyReason,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return GunCommandRecord{}, false, nil
	}
	if err != nil {
		return GunCommandRecord{}, false, err
	}
	if shotSequence.Valid {
		v := uint32(shotSequence.Int64)
		out.ShotSequence = &v
	}
	if completionTime.Valid {
		v := completionTime.Time.UTC()
		out.CompletionTime = &v
	}
	if updatedAt.Valid {
		v := updatedAt.Time.UTC()
		out.UpdatedAt = &v
	}
	if baselineAccepted.Valid {
		v := uint32(baselineAccepted.Int64)
		out.LifecycleEvidence.BaselineAcceptedSequence = &v
	}
	if confirmedAccepted.Valid {
		v := uint32(confirmedAccepted.Int64)
		out.LifecycleEvidence.ConfirmedAcceptedSequence = &v
	}
	if baselineShot.Valid {
		v := uint32(baselineShot.Int64)
		out.LifecycleEvidence.BaselineShotSequence = &v
	}
	if lastObservedShot.Valid {
		v := uint32(lastObservedShot.Int64)
		out.LifecycleEvidence.LastObservedShotSequence = &v
	}
	if shotObserved.Valid {
		out.LifecycleEvidence.ShotObserved = shotObserved.Bool
	}
	if lastObservedState.Valid {
		v := uint16(lastObservedState.Int32)
		out.LifecycleEvidence.LastObservedState = &v
	}
	if firingObserved.Valid {
		out.LifecycleEvidence.FiringObserved = firingObserved.Bool
	}
	if lockoutObserved.Valid {
		out.LifecycleEvidence.LockoutObserved = lockoutObserved.Bool
	}
	if readyAfterCommandObserved.Valid {
		out.LifecycleEvidence.ReadyAfterCommandObserved = readyAfterCommandObserved.Bool
	}
	if commandCommittedAt.Valid {
		v := commandCommittedAt.Time.UTC()
		out.LifecycleEvidence.CommandCommittedAt = &v
	}
	if lastLifecycleObservationAt.Valid {
		v := lastLifecycleObservationAt.Time.UTC()
		out.LifecycleEvidence.LastLifecycleObservationTime = &v
	}
	_ = json.Unmarshal(payload, &out.CommandPayload)
	return out, true, nil
}

func (s *PostgresHistorySink) RecordDispatcherDecision(ctx context.Context, decision DispatcherDecisionRecord) (int64, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return 0, err
	}
	candidates, err := json.Marshal(decision.Candidates)
	if err != nil {
		return 0, err
	}
	details, err := json.Marshal(decision.Details)
	if err != nil {
		return 0, err
	}
	if decision.DecisionTime.IsZero() {
		decision.DecisionTime = time.Now().UTC()
	}
	var id int64
	err = s.pool.QueryRow(ctx, `
INSERT INTO dispatcher_decisions (
	target_id, target_source, external_id, decision_time, mode, outcome, selected_gun_id,
	no_selection_reason, eligible_gun_count, scoring_version, candidates, selected_score,
	tie_break_reason, shadow_only, details
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
RETURNING id`,
		decision.TargetID, decision.TargetSource, optionalAuditError(decision.ExternalID), decision.DecisionTime,
		decision.Mode, decision.Outcome, decision.SelectedGunID, optionalAuditError(decision.NoSelectionReason),
		decision.EligibleGunCount, decision.ScoringVersion, candidates, decision.SelectedScore,
		optionalAuditError(decision.TieBreakReason), decision.ShadowOnly, details,
	).Scan(&id)
	return id, err
}

func (s *PostgresHistorySink) ListDispatcherDecisions(ctx context.Context, filter DispatcherDecisionFilter) ([]DispatcherDecisionRecord, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return nil, err
	}
	args := []interface{}{}
	clauses := []string{}
	if filter.TargetID != nil {
		args = append(args, *filter.TargetID)
		clauses = append(clauses, fmt.Sprintf("target_id = $%d", len(args)))
	}
	if filter.SelectedGunID != nil {
		args = append(args, *filter.SelectedGunID)
		clauses = append(clauses, fmt.Sprintf("selected_gun_id = $%d", len(args)))
	}
	if filter.Mode != "" {
		args = append(args, filter.Mode)
		clauses = append(clauses, fmt.Sprintf("mode = $%d", len(args)))
	}
	if filter.Outcome != "" {
		args = append(args, filter.Outcome)
		clauses = append(clauses, fmt.Sprintf("outcome = $%d", len(args)))
	}
	if filter.Start != nil {
		args = append(args, *filter.Start)
		clauses = append(clauses, fmt.Sprintf("decision_time >= $%d", len(args)))
	}
	if filter.End != nil {
		args = append(args, *filter.End)
		clauses = append(clauses, fmt.Sprintf("decision_time <= $%d", len(args)))
	}
	query := `SELECT id, target_id, target_source, COALESCE(external_id,''), decision_time, mode, outcome,
		selected_gun_id, COALESCE(no_selection_reason,''), eligible_gun_count, scoring_version, candidates,
		selected_score, COALESCE(tie_break_reason,''), shadow_only, details FROM dispatcher_decisions`
	query = appendWhere(query, clauses)
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	args = append(args, filter.Limit, filter.Offset)
	query += fmt.Sprintf(" ORDER BY decision_time DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DispatcherDecisionRecord{}
	for rows.Next() {
		row, err := scanDispatcherDecision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresHistorySink) GetDispatcherDecision(ctx context.Context, id int64) (DispatcherDecisionRecord, bool, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return DispatcherDecisionRecord{}, false, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, target_id, target_source, COALESCE(external_id,''), decision_time, mode, outcome,
		selected_gun_id, COALESCE(no_selection_reason,''), eligible_gun_count, scoring_version, candidates,
		selected_score, COALESCE(tie_break_reason,''), shadow_only, details FROM dispatcher_decisions WHERE id=$1`, id)
	if err != nil {
		return DispatcherDecisionRecord{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return DispatcherDecisionRecord{}, false, rows.Err()
	}
	row, err := scanDispatcherDecision(rows)
	return row, err == nil, err
}

func (s *PostgresHistorySink) CreateTargetAssignment(ctx context.Context, assignment TargetAssignmentRecord) (TargetAssignmentRecord, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return TargetAssignmentRecord{}, err
	}
	payload, err := json.Marshal(assignment.TargetPayload)
	if err != nil {
		return TargetAssignmentRecord{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TargetAssignmentRecord{}, err
	}
	defer tx.Rollback(ctx)
	var generation int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(max(generation),0)+1 FROM target_assignments WHERE target_id=$1 AND target_source=$2`, assignment.TargetID, assignment.TargetSource).Scan(&generation); err != nil {
		return TargetAssignmentRecord{}, err
	}
	now := time.Now().UTC()
	if assignment.AssignedAt.IsZero() {
		assignment.AssignedAt = now
	}
	if assignment.UpdatedAt.IsZero() {
		assignment.UpdatedAt = now
	}
	assignment.Generation = generation
	err = tx.QueryRow(ctx, `
INSERT INTO target_assignments (
	target_id, target_source, external_id, gun_id, state, generation, assigned_at, lease_expires_at,
	lease_owner, last_lease_renewed_at, lease_renewal_failures, last_lease_renewal_error,
	dispatcher_decision_id, command_sequence, command_audit_id, engagement_id, completion_state, release_reason,
	reconciliation_result, uncertainty_reason, updated_at, target_payload
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
RETURNING id`,
		assignment.TargetID, assignment.TargetSource, optionalAuditError(assignment.ExternalID), assignment.GunID,
		assignment.State, assignment.Generation, assignment.AssignedAt, assignment.LeaseExpiresAt,
		optionalAuditError(assignment.LeaseOwner), optionalTime(assignment.LastLeaseRenewedAt), assignment.LeaseRenewalFailures,
		optionalAuditError(assignment.LastLeaseRenewalError), assignment.DispatcherDecisionID,
		nullableUint32Bigint(assignment.CommandSequence), optionalInt64(assignment.CommandAuditID), optionalInt64(assignment.EngagementID),
		optionalAuditError(assignment.CompletionState), optionalAuditError(assignment.ReleaseReason),
		optionalAuditError(assignment.ReconciliationResult), optionalAuditError(assignment.UncertaintyReason), assignment.UpdatedAt, payload,
	).Scan(&assignment.ID)
	if err != nil {
		return TargetAssignmentRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TargetAssignmentRecord{}, err
	}
	return assignment, nil
}

func (s *PostgresHistorySink) UpdateTargetAssignment(ctx context.Context, id int64, generation int, update TargetAssignmentUpdate) error {
	if err := s.ensureMigrations(ctx); err != nil {
		return err
	}
	if update.UpdatedAt.IsZero() {
		update.UpdatedAt = time.Now().UTC()
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE target_assignments
SET state=COALESCE(NULLIF($3,''), state),
	lease_expires_at=COALESCE($4, lease_expires_at),
	lease_owner=CASE WHEN $17 THEN '' ELSE COALESCE($5, lease_owner) END,
	last_lease_renewed_at=CASE WHEN $17 THEN NULL ELSE COALESCE($6, last_lease_renewed_at) END,
	lease_renewal_failures=COALESCE($7, lease_renewal_failures),
	last_lease_renewal_error=COALESCE($8, last_lease_renewal_error),
	command_sequence=COALESCE($9, command_sequence),
	command_audit_id=COALESCE($10, command_audit_id),
	engagement_id=COALESCE($11, engagement_id),
	completion_state=COALESCE(NULLIF($12,''), completion_state),
	release_reason=COALESCE(NULLIF($13,''), release_reason),
	reconciliation_result=COALESCE(NULLIF($14,''), reconciliation_result),
	uncertainty_reason=COALESCE(NULLIF($15,''), uncertainty_reason),
	updated_at=$16
WHERE id=$1 AND generation=$2`,
		id, generation, update.State, update.LeaseExpiresAt, optionalStringPtr(update.LeaseOwner), optionalTime(update.LastLeaseRenewedAt),
		optionalIntPtr(update.LeaseRenewalFailures), optionalStringPtr(update.LastLeaseRenewalError),
		nullableUint32Bigint(update.CommandSequence), optionalInt64(update.CommandAuditID), optionalInt64(update.EngagementID),
		update.CompletionState, update.ReleaseReason, update.ReconciliationResult, update.UncertaintyReason, update.UpdatedAt, update.ClearLease,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("target assignment id %d generation %d not found", id, generation)
	}
	return nil
}

func (s *PostgresHistorySink) RenewTargetAssignmentLease(ctx context.Context, id int64, generation int, owner string, expiresAt time.Time) (TargetAssignmentRecord, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return TargetAssignmentRecord{}, err
	}
	now := time.Now().UTC()
	rows, err := s.pool.Query(ctx, targetAssignmentSelect()+`
WHERE id=$1 AND generation=$2 AND lease_owner=$3
	AND state NOT IN ('completed','released','requeued','terminal_failed','terminal_uncertain','expired')`, id, generation, owner)
	if err != nil {
		return TargetAssignmentRecord{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		_, _ = s.pool.Exec(ctx, `
UPDATE target_assignments
SET lease_renewal_failures=lease_renewal_failures+1,
	last_lease_renewal_error=$3,
	updated_at=$4
WHERE id=$1 AND generation=$2`, id, generation, "lease renewal rejected", now)
		return TargetAssignmentRecord{}, fmt.Errorf("target assignment id %d generation %d owner %s is not renewable", id, generation, owner)
	}
	row, err := scanTargetAssignment(rows)
	if err != nil {
		return TargetAssignmentRecord{}, err
	}
	rows.Close()
	tag, err := s.pool.Exec(ctx, `
UPDATE target_assignments
SET lease_expires_at=$4,
	last_lease_renewed_at=$5,
	last_lease_renewal_error=NULL,
	updated_at=$5
WHERE id=$1 AND generation=$2 AND lease_owner=$3
	AND state NOT IN ('completed','released','requeued','terminal_failed','terminal_uncertain','expired')`,
		id, generation, owner, expiresAt.UTC(), now)
	if err != nil {
		return TargetAssignmentRecord{}, err
	}
	if tag.RowsAffected() == 0 {
		return TargetAssignmentRecord{}, fmt.Errorf("target assignment id %d generation %d owner %s renewal raced", id, generation, owner)
	}
	row.LeaseExpiresAt = expiresAt.UTC()
	row.LastLeaseRenewedAt = &now
	row.LastLeaseRenewalError = ""
	row.UpdatedAt = now
	return row, nil
}

func (s *PostgresHistorySink) ListTargetAssignments(ctx context.Context, filter TargetAssignmentFilter) ([]TargetAssignmentRecord, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return nil, err
	}
	args := []interface{}{}
	clauses := []string{}
	if filter.TargetID != nil {
		args = append(args, *filter.TargetID)
		clauses = append(clauses, fmt.Sprintf("target_id = $%d", len(args)))
	}
	if filter.GunID != nil {
		args = append(args, *filter.GunID)
		clauses = append(clauses, fmt.Sprintf("gun_id = $%d", len(args)))
	}
	if filter.State != "" {
		args = append(args, filter.State)
		clauses = append(clauses, fmt.Sprintf("state = $%d", len(args)))
	}
	if filter.Active != nil {
		if *filter.Active {
			clauses = append(clauses, "state NOT IN ('completed','released','requeued','terminal_failed','terminal_uncertain','expired')")
		} else {
			clauses = append(clauses, "state IN ('completed','released','requeued','terminal_failed','terminal_uncertain','expired')")
		}
	}
	query := targetAssignmentSelect()
	query = appendWhere(query, clauses)
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	args = append(args, filter.Limit, filter.Offset)
	query += fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TargetAssignmentRecord{}
	for rows.Next() {
		row, err := scanTargetAssignment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresHistorySink) GetTargetAssignment(ctx context.Context, id int64) (TargetAssignmentRecord, bool, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return TargetAssignmentRecord{}, false, err
	}
	rows, err := s.pool.Query(ctx, targetAssignmentSelect()+` WHERE id=$1`, id)
	if err != nil {
		return TargetAssignmentRecord{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return TargetAssignmentRecord{}, false, rows.Err()
	}
	row, err := scanTargetAssignment(rows)
	return row, err == nil, err
}

func (s *PostgresHistorySink) ActiveAssignmentForTarget(ctx context.Context, targetID uint32, source string) (TargetAssignmentRecord, bool, error) {
	return s.oneAssignment(ctx, `target_id=$1 AND target_source=$2 AND state NOT IN ('completed','released','requeued','terminal_failed','terminal_uncertain','expired')`, targetID, source)
}

func (s *PostgresHistorySink) ActiveAssignmentForGun(ctx context.Context, gunID int) (TargetAssignmentRecord, bool, error) {
	return s.oneAssignment(ctx, `gun_id=$1 AND state NOT IN ('completed','released','requeued','terminal_failed','terminal_uncertain','expired')`, gunID)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDispatcherDecision(row scanner) (DispatcherDecisionRecord, error) {
	var out DispatcherDecisionRecord
	var candidates []byte
	var details []byte
	var selectedGun pgtype.Int4
	var selectedScore pgtype.Float8
	if err := row.Scan(&out.ID, &out.TargetID, &out.TargetSource, &out.ExternalID, &out.DecisionTime, &out.Mode, &out.Outcome, &selectedGun, &out.NoSelectionReason, &out.EligibleGunCount, &out.ScoringVersion, &candidates, &selectedScore, &out.TieBreakReason, &out.ShadowOnly, &details); err != nil {
		return out, err
	}
	if selectedGun.Valid {
		v := int(selectedGun.Int32)
		out.SelectedGunID = &v
	}
	if selectedScore.Valid {
		v := selectedScore.Float64
		out.SelectedScore = &v
	}
	_ = json.Unmarshal(candidates, &out.Candidates)
	_ = json.Unmarshal(details, &out.Details)
	return out, nil
}

func targetAssignmentSelect() string {
	return `SELECT id, target_id, target_source, COALESCE(external_id,''), gun_id, state, generation,
		assigned_at, lease_expires_at, COALESCE(lease_owner,''), last_lease_renewed_at,
		lease_renewal_failures, COALESCE(last_lease_renewal_error,''),
		COALESCE(dispatcher_decision_id,0), command_sequence, command_audit_id, engagement_id,
		COALESCE(completion_state,''), COALESCE(release_reason,''),
		COALESCE(reconciliation_result,''), COALESCE(uncertainty_reason,''), updated_at, target_payload
		FROM target_assignments`
}

func scanTargetAssignment(row scanner) (TargetAssignmentRecord, error) {
	var out TargetAssignmentRecord
	var commandSequence pgtype.Int8
	var commandAuditID pgtype.Int8
	var engagementID pgtype.Int8
	var lastRenewed pgtype.Timestamptz
	var payload []byte
	if err := row.Scan(&out.ID, &out.TargetID, &out.TargetSource, &out.ExternalID, &out.GunID, &out.State, &out.Generation, &out.AssignedAt, &out.LeaseExpiresAt, &out.LeaseOwner, &lastRenewed, &out.LeaseRenewalFailures, &out.LastLeaseRenewalError, &out.DispatcherDecisionID, &commandSequence, &commandAuditID, &engagementID, &out.CompletionState, &out.ReleaseReason, &out.ReconciliationResult, &out.UncertaintyReason, &out.UpdatedAt, &payload); err != nil {
		return out, err
	}
	if lastRenewed.Valid {
		v := lastRenewed.Time.UTC()
		out.LastLeaseRenewedAt = &v
	}
	if commandSequence.Valid {
		v := uint32(commandSequence.Int64)
		out.CommandSequence = &v
	}
	if commandAuditID.Valid {
		v := commandAuditID.Int64
		out.CommandAuditID = &v
	}
	if engagementID.Valid {
		v := engagementID.Int64
		out.EngagementID = &v
	}
	_ = json.Unmarshal(payload, &out.TargetPayload)
	return out, nil
}

func (s *PostgresHistorySink) oneAssignment(ctx context.Context, where string, args ...interface{}) (TargetAssignmentRecord, bool, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return TargetAssignmentRecord{}, false, err
	}
	rows, err := s.pool.Query(ctx, targetAssignmentSelect()+` WHERE `+where+` ORDER BY updated_at DESC LIMIT 1`, args...)
	if err != nil {
		return TargetAssignmentRecord{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return TargetAssignmentRecord{}, false, rows.Err()
	}
	row, err := scanTargetAssignment(rows)
	return row, err == nil, err
}

func (s *PostgresHistorySink) RecordGunEngagement(ctx context.Context, record EngagementRecord) (int64, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return 0, err
	}
	payload, err := json.Marshal(record.TargetPayload)
	if err != nil {
		return 0, err
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	var id int64
	err = s.pool.QueryRow(ctx, `
INSERT INTO gun_engagements (
	target_id, gun_id, source, target_payload, state, result, created_at, error
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING id`,
		record.TargetID,
		record.GunID,
		record.Source,
		payload,
		record.State,
		optionalAuditError(record.Result),
		record.CreatedAt,
		optionalAuditError(record.Error),
	).Scan(&id)
	return id, err
}

func (s *PostgresHistorySink) UpdateGunEngagement(ctx context.Context, id int64, update EngagementUpdate) error {
	if err := s.ensureMigrations(ctx); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE gun_engagements
SET command_audit_id=COALESCE($2, command_audit_id),
	submitted_sequence=COALESCE($3, submitted_sequence),
	state=$4,
	result=$5,
	started_at=COALESCE($6, started_at),
	completed_at=COALESCE($7, completed_at),
	error=$8,
	final_shot_sequence=$9,
	fault_code=$10,
	updated_at=now()
WHERE id=$1`,
		id,
		optionalInt64(update.CommandAuditID),
		nullableUint32Bigint(update.SubmittedSequence),
		update.State,
		optionalAuditError(update.Result),
		update.StartedAt,
		update.CompletedAt,
		optionalAuditError(update.Error),
		nullableUint32Bigint(update.FinalShotSequence),
		optionalUint16Int(update.FaultCode),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("gun engagement id %d not found", id)
	}
	return nil
}

func (s *PostgresHistorySink) GetGunEngagement(ctx context.Context, id int64) (GunEngagementRecord, bool, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return GunEngagementRecord{}, false, err
	}
	var out GunEngagementRecord
	var payload []byte
	var commandAuditID pgtype.Int8
	var submittedSequence pgtype.Int8
	var startedAt pgtype.Timestamptz
	var completedAt pgtype.Timestamptz
	var finalShotSequence pgtype.Int8
	var faultCode pgtype.Int4
	err := s.pool.QueryRow(ctx, `
SELECT id, target_id, gun_id, command_audit_id, submitted_sequence, source, target_payload,
	state, COALESCE(result,''), created_at, started_at, completed_at, COALESCE(error,''),
	final_shot_sequence, fault_code, updated_at
FROM gun_engagements
WHERE id=$1`, id).Scan(
		&out.ID,
		&out.TargetID,
		&out.GunID,
		&commandAuditID,
		&submittedSequence,
		&out.Source,
		&payload,
		&out.State,
		&out.Result,
		&out.CreatedAt,
		&startedAt,
		&completedAt,
		&out.Error,
		&finalShotSequence,
		&faultCode,
		&out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return GunEngagementRecord{}, false, nil
	}
	if err != nil {
		return GunEngagementRecord{}, false, err
	}
	if commandAuditID.Valid {
		v := commandAuditID.Int64
		out.CommandAuditID = &v
	}
	if submittedSequence.Valid {
		v := uint32(submittedSequence.Int64)
		out.SubmittedSequence = &v
	}
	if startedAt.Valid {
		v := startedAt.Time.UTC()
		out.StartedAt = &v
	}
	if completedAt.Valid {
		v := completedAt.Time.UTC()
		out.CompletedAt = &v
	}
	if finalShotSequence.Valid {
		v := uint32(finalShotSequence.Int64)
		out.FinalShotSequence = &v
	}
	if faultCode.Valid {
		v := uint16(faultCode.Int32)
		out.FaultCode = &v
	}
	_ = json.Unmarshal(payload, &out.TargetPayload)
	return out, true, nil
}

func (s *PostgresHistorySink) RecordGunFaultReset(ctx context.Context, record FaultResetRecord) (int64, error) {
	if err := s.ensureMigrations(ctx); err != nil {
		return 0, err
	}
	if record.StartedAt.IsZero() {
		record.StartedAt = time.Now().UTC()
	}
	var id int64
	err := s.pool.QueryRow(ctx, `
INSERT INTO gun_fault_resets (
	gun_id, source, prior_mode, requested_resume_auto, fault_code_before, started_at, error
) VALUES ($1,$2,$3,$4,$5,$6,$7)
RETURNING id`,
		record.GunID,
		record.Source,
		record.PriorMode,
		record.RequestedResumeAuto,
		record.FaultCodeBefore,
		record.StartedAt,
		optionalAuditError(record.Error),
	).Scan(&id)
	return id, err
}

func (s *PostgresHistorySink) UpdateGunFaultReset(ctx context.Context, id int64, update FaultResetUpdate) error {
	if err := s.ensureMigrations(ctx); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE gun_fault_resets
SET command_audit_id=COALESCE($2, command_audit_id),
	submitted_sequence=COALESCE($3, submitted_sequence),
	result=$4,
	fault_code_after=$5,
	completed_at=COALESCE($6, completed_at),
	error=$7,
	updated_at=now()
WHERE id=$1`,
		id,
		optionalInt64(update.CommandAuditID),
		nullableUint32Bigint(update.SubmittedSequence),
		optionalAuditError(update.Result),
		optionalUint16Int(update.FaultCodeAfter),
		update.CompletedAt,
		optionalAuditError(update.Error),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("gun fault reset id %d not found", id)
	}
	return nil
}

func (s *PostgresHistorySink) RecordSafetyEvent(ctx context.Context, event safety.Event) error {
	if err := s.ensureMigrations(ctx); err != nil {
		return err
	}
	details, err := json.Marshal(event.Details)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO safety_events (event_time, event_type, gun_id, details)
		VALUES ($1, $2, $3, $4)
	`, event.EventTime, event.EventType, event.GunID, details)
	if err != nil {
		s.setError(err)
		return err
	}
	s.setOK()
	return nil
}

func nullableUint32Bigint(value *uint32) interface{} {
	if value == nil {
		return nil
	}
	return int64(*value)
}

func optionalInt64(value *int64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func optionalIntPtr(value *int) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func optionalStringPtr(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func optionalTime(value *time.Time) interface{} {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

func optionalTimeValue(value time.Time) interface{} {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func optionalUint16Int(value *uint16) interface{} {
	if value == nil {
		return nil
	}
	return int(*value)
}

func nullableUint16Int(value *uint16) interface{} {
	return optionalUint16Int(value)
}

func optionalBool(value *bool) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func appendWhere(query string, clauses []string) string {
	if len(clauses) == 0 {
		return query
	}
	return query + " WHERE " + strings.Join(clauses, " AND ")
}

func optionalAuditError(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func (s *PostgresHistorySink) setOK() {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connected = true
	s.lastError = ""
	s.lastOK = &now
}

func (s *PostgresHistorySink) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connected = false
	s.lastError = err.Error()
}

func deterministicUUID(namespace, key string) string {
	if key == "" {
		key = namespace
	}
	sum := sha1.Sum([]byte(namespace + ":" + key))
	b := sum[:16]
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

func fragmentUUID(fragmentID string) string {
	return deterministicUUID("stonehenge-fragment-track", fragmentID)
}

func eventFragmentUUID(event model.OperationalEvent) string {
	if event.ExternalFragmentID != nil && *event.ExternalFragmentID != "" {
		return deterministicUUID("stonehenge-fragment-track-external", *event.ExternalFragmentID)
	}
	if event.FragmentID != nil {
		return fragmentUUID(*event.FragmentID)
	}
	return deterministicUUID("stonehenge-fragment-track", event.EventUUID)
}

func fragmentUUIDPtr(fragmentID *string) *string {
	if fragmentID == nil {
		return nil
	}
	u := fragmentUUID(*fragmentID)
	return &u
}

func eventFragmentUUIDPtr(event model.OperationalEvent) *string {
	if event.FragmentID == nil && event.ExternalFragmentID == nil {
		return nil
	}
	u := eventFragmentUUID(event)
	return &u
}

func optionalString(value *string) interface{} {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}

func optionalFloat(value *float64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func detailString(event model.OperationalEvent, key, fallback string) string {
	value, ok := event.Details[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

func detailFloat(event model.OperationalEvent, key string, fallback float64) float64 {
	value, ok := event.Details[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case json.Number:
		f, _ := typed.Float64()
		return f
	default:
		return fallback
	}
}

func detailIntPtr(event model.OperationalEvent, key string) *int {
	value, ok := event.Details[key]
	if !ok || value == nil {
		return nil
	}
	var out int
	switch typed := value.(type) {
	case int:
		out = typed
	case int32:
		out = int(typed)
	case int64:
		out = int(typed)
	case float64:
		out = int(typed)
	case json.Number:
		n, err := typed.Int64()
		if err != nil {
			return nil
		}
		out = int(n)
	default:
		return nil
	}
	return &out
}

func detailTime(event model.OperationalEvent, key string, fallback time.Time) time.Time {
	if value := detailTimePtr(event, key); value != nil {
		return *value
	}
	return fallback
}

func detailTimePtr(event model.OperationalEvent, key string) *time.Time {
	value, ok := event.Details[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case time.Time:
		t := typed.UTC()
		return &t
	case string:
		t, err := time.Parse(time.RFC3339Nano, typed)
		if err != nil {
			return nil
		}
		t = t.UTC()
		return &t
	default:
		return nil
	}
}

func isUndefinedTable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01"
}
