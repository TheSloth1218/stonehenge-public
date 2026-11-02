ALTER TABLE target_assignments
	ADD COLUMN IF NOT EXISTS lease_owner text NOT NULL DEFAULT 'dispatcher',
	ADD COLUMN IF NOT EXISTS last_lease_renewed_at timestamptz,
	ADD COLUMN IF NOT EXISTS lease_renewal_failures integer NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS last_lease_renewal_error text,
	ADD COLUMN IF NOT EXISTS command_audit_id bigint REFERENCES gun_command_audit(id),
	ADD COLUMN IF NOT EXISTS reconciliation_result text,
	ADD COLUMN IF NOT EXISTS uncertainty_reason text;

UPDATE target_assignments
SET lease_owner = CASE
	WHEN state IN ('accepted_by_gun','waiting_for_safety','arbitration_pending','command_pending','command_submit_started','command_committed','engaging','observing','reconciling','uncertain') THEN 'executor'
	ELSE 'dispatcher'
END
WHERE lease_owner IS NULL OR lease_owner = '';

CREATE INDEX IF NOT EXISTS idx_target_assignments_lease_owner ON target_assignments (lease_owner);
CREATE INDEX IF NOT EXISTS idx_target_assignments_command_audit ON target_assignments (command_audit_id);
CREATE INDEX IF NOT EXISTS idx_target_assignments_reconciliation ON target_assignments (reconciliation_result);
