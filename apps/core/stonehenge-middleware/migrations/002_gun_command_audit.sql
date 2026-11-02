CREATE TABLE IF NOT EXISTS gun_command_audit (
	id bigserial PRIMARY KEY,
	command_time timestamptz NOT NULL,
	gun_id integer NOT NULL,
	command_source text NOT NULL,
	submitted_sequence bigint NOT NULL,
	command_payload jsonb NOT NULL,
	result text NOT NULL,
	resulting_state integer NOT NULL,
	fault_code integer NOT NULL,
	shot_sequence bigint,
	error text
);

CREATE INDEX IF NOT EXISTS idx_gun_command_audit_command_time ON gun_command_audit (command_time DESC);
CREATE INDEX IF NOT EXISTS idx_gun_command_audit_gun_id ON gun_command_audit (gun_id);
CREATE INDEX IF NOT EXISTS idx_gun_command_audit_command_source ON gun_command_audit (command_source);
CREATE INDEX IF NOT EXISTS idx_gun_command_audit_result ON gun_command_audit (result);
