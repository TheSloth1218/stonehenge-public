CREATE TABLE IF NOT EXISTS gun_fault_resets (
	id BIGSERIAL PRIMARY KEY,
	gun_id integer NOT NULL,
	source text NOT NULL,
	prior_mode text NOT NULL,
	requested_resume_auto boolean NOT NULL DEFAULT false,
	fault_code_before integer NOT NULL,
	command_audit_id bigint NULL REFERENCES gun_command_audit(id),
	submitted_sequence bigint NULL,
	result text NULL,
	fault_code_after integer NULL,
	started_at timestamptz NOT NULL DEFAULT now(),
	completed_at timestamptz NULL,
	error text NULL,
	updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_gun_fault_resets_gun_time ON gun_fault_resets(gun_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_gun_fault_resets_result ON gun_fault_resets(result);
CREATE INDEX IF NOT EXISTS idx_gun_fault_resets_command_audit_id ON gun_fault_resets(command_audit_id);
