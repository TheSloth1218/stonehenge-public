ALTER TABLE gun_command_audit
	ADD COLUMN IF NOT EXISTS completion_time timestamptz,
	ADD COLUMN IF NOT EXISTS updated_at timestamptz;

UPDATE gun_command_audit
SET updated_at = command_time
WHERE updated_at IS NULL;

