CREATE TABLE IF NOT EXISTS dispatcher_decisions (
	id bigserial PRIMARY KEY,
	target_id bigint NOT NULL,
	target_source text NOT NULL,
	external_id text,
	decision_time timestamptz NOT NULL,
	mode text NOT NULL,
	outcome text NOT NULL,
	selected_gun_id integer,
	no_selection_reason text,
	eligible_gun_count integer NOT NULL,
	scoring_version text NOT NULL,
	candidates jsonb NOT NULL,
	selected_score double precision,
	tie_break_reason text,
	shadow_only boolean NOT NULL DEFAULT false,
	details jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_dispatcher_decisions_time ON dispatcher_decisions (decision_time DESC);
CREATE INDEX IF NOT EXISTS idx_dispatcher_decisions_target ON dispatcher_decisions (target_source, target_id);
CREATE INDEX IF NOT EXISTS idx_dispatcher_decisions_selected_gun ON dispatcher_decisions (selected_gun_id);
CREATE INDEX IF NOT EXISTS idx_dispatcher_decisions_mode_outcome ON dispatcher_decisions (mode, outcome);

CREATE TABLE IF NOT EXISTS target_assignments (
	id bigserial PRIMARY KEY,
	target_id bigint NOT NULL,
	target_source text NOT NULL,
	external_id text,
	gun_id integer NOT NULL,
	state text NOT NULL,
	generation integer NOT NULL,
	assigned_at timestamptz NOT NULL,
	lease_expires_at timestamptz NOT NULL,
	dispatcher_decision_id bigint REFERENCES dispatcher_decisions(id),
	command_sequence bigint,
	engagement_id bigint REFERENCES gun_engagements(id),
	completion_state text,
	release_reason text,
	updated_at timestamptz NOT NULL DEFAULT now(),
	target_payload jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_target_assignments_target ON target_assignments (target_source, target_id);
CREATE INDEX IF NOT EXISTS idx_target_assignments_gun ON target_assignments (gun_id);
CREATE INDEX IF NOT EXISTS idx_target_assignments_state ON target_assignments (state);
CREATE INDEX IF NOT EXISTS idx_target_assignments_lease ON target_assignments (lease_expires_at);
CREATE INDEX IF NOT EXISTS idx_target_assignments_decision ON target_assignments (dispatcher_decision_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_target_assignments_active_target
	ON target_assignments (target_source, target_id)
	WHERE state NOT IN ('completed','released','requeued','terminal_failed','expired');

CREATE UNIQUE INDEX IF NOT EXISTS ux_target_assignments_active_gun
	ON target_assignments (gun_id)
	WHERE state NOT IN ('completed','released','requeued','terminal_failed','expired');
