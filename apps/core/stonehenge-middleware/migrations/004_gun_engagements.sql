CREATE TABLE IF NOT EXISTS gun_engagements (
	id bigserial PRIMARY KEY,
	target_id bigint NOT NULL,
	gun_id integer NOT NULL,
	command_audit_id bigint REFERENCES gun_command_audit(id),
	submitted_sequence bigint,
	source text NOT NULL,
	target_payload jsonb NOT NULL,
	state text NOT NULL,
	result text,
	created_at timestamptz NOT NULL,
	started_at timestamptz,
	completed_at timestamptz,
	error text,
	final_shot_sequence bigint,
	fault_code integer,
	updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_gun_engagements_created_at ON gun_engagements (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_gun_engagements_gun_id ON gun_engagements (gun_id);
CREATE INDEX IF NOT EXISTS idx_gun_engagements_target_id ON gun_engagements (target_id);
CREATE INDEX IF NOT EXISTS idx_gun_engagements_state ON gun_engagements (state);
CREATE INDEX IF NOT EXISTS idx_gun_engagements_command_audit_id ON gun_engagements (command_audit_id);
