CREATE TABLE IF NOT EXISTS fragment_tracks (
	id uuid PRIMARY KEY,
	external_id text UNIQUE,
	source_id text,
	received_at timestamptz NOT NULL,
	observed_at timestamptz NOT NULL,
	bearing double precision NOT NULL,
	elevation double precision NOT NULL,
	map_x double precision NOT NULL,
	map_y double precision NOT NULL,
	priority text NOT NULL,
	terrain_type text NOT NULL,
	strategic_zone text NOT NULL,
	impact_at timestamptz NOT NULL,
	status text NOT NULL,
	assigned_gun_id integer,
	intercepted_at timestamptz,
	missed_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_fragment_tracks_received_at ON fragment_tracks (received_at DESC);
CREATE INDEX IF NOT EXISTS idx_fragment_tracks_observed_at ON fragment_tracks (observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_fragment_tracks_impact_at ON fragment_tracks (impact_at DESC);
CREATE INDEX IF NOT EXISTS idx_fragment_tracks_external_id ON fragment_tracks (external_id);
CREATE INDEX IF NOT EXISTS idx_fragment_tracks_status ON fragment_tracks (status);
CREATE INDEX IF NOT EXISTS idx_fragment_tracks_assigned_gun_id ON fragment_tracks (assigned_gun_id);

CREATE TABLE IF NOT EXISTS gun_events (
	id uuid PRIMARY KEY,
	event_time timestamptz NOT NULL,
	event_type text NOT NULL,
	severity text NOT NULL,
	gun_id integer,
	fragment_id uuid,
	external_fragment_id text,
	azimuth double precision,
	elevation double precision,
	mode text,
	shell_type text,
	shot_sequence bigint,
	details jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_gun_events_event_time ON gun_events (event_time DESC);
CREATE INDEX IF NOT EXISTS idx_gun_events_gun_id ON gun_events (gun_id);
CREATE INDEX IF NOT EXISTS idx_gun_events_event_type ON gun_events (event_type);
CREATE INDEX IF NOT EXISTS idx_gun_events_fragment_id ON gun_events (fragment_id);
CREATE INDEX IF NOT EXISTS idx_gun_events_external_fragment_id ON gun_events (external_fragment_id);

CREATE TABLE IF NOT EXISTS shot_records (
	id uuid PRIMARY KEY,
	gun_id integer NOT NULL,
	fragment_id uuid,
	external_fragment_id text,
	fired_at timestamptz NOT NULL,
	azimuth double precision NOT NULL,
	elevation double precision NOT NULL,
	shell_type text NOT NULL,
	shot_sequence bigint NOT NULL,
	result text NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_shot_records_fired_at ON shot_records (fired_at DESC);
CREATE INDEX IF NOT EXISTS idx_shot_records_gun_id ON shot_records (gun_id);
CREATE INDEX IF NOT EXISTS idx_shot_records_fragment_id ON shot_records (fragment_id);
CREATE INDEX IF NOT EXISTS idx_shot_records_external_fragment_id ON shot_records (external_fragment_id);

CREATE TABLE IF NOT EXISTS gun_runtime_summary (
	gun_id integer PRIMARY KEY,
	total_shots bigint NOT NULL DEFAULT 0,
	standard_shells_fired bigint NOT NULL DEFAULT 0,
	special_shells_fired bigint NOT NULL DEFAULT 0,
	last_fired_at timestamptz,
	last_service_at timestamptz,
	updated_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_gun_runtime_summary_updated_at ON gun_runtime_summary (updated_at DESC);
