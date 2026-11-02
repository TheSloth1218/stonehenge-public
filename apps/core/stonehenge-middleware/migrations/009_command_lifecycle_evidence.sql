ALTER TABLE gun_command_audit
	ADD COLUMN IF NOT EXISTS baseline_accepted_sequence bigint,
	ADD COLUMN IF NOT EXISTS confirmed_accepted_sequence bigint,
	ADD COLUMN IF NOT EXISTS baseline_shot_sequence bigint,
	ADD COLUMN IF NOT EXISTS last_observed_shot_sequence bigint,
	ADD COLUMN IF NOT EXISTS shot_observed boolean NOT NULL DEFAULT false,
	ADD COLUMN IF NOT EXISTS last_observed_state integer,
	ADD COLUMN IF NOT EXISTS firing_observed boolean NOT NULL DEFAULT false,
	ADD COLUMN IF NOT EXISTS lockout_observed boolean NOT NULL DEFAULT false,
	ADD COLUMN IF NOT EXISTS ready_after_command_observed boolean NOT NULL DEFAULT false,
	ADD COLUMN IF NOT EXISTS command_committed_at timestamptz,
	ADD COLUMN IF NOT EXISTS last_lifecycle_observation_at timestamptz,
	ADD COLUMN IF NOT EXISTS lifecycle_uncertainty_reason text;

CREATE INDEX IF NOT EXISTS idx_gun_command_audit_lifecycle_observed
	ON gun_command_audit (gun_id, last_lifecycle_observation_at DESC);

CREATE INDEX IF NOT EXISTS idx_gun_command_audit_shot_observed
	ON gun_command_audit (gun_id, shot_observed)
	WHERE shot_observed;
