DROP INDEX IF EXISTS idx_target_assignments_active_target;
DROP INDEX IF EXISTS idx_target_assignments_active_gun;

CREATE UNIQUE INDEX IF NOT EXISTS idx_target_assignments_active_target
	ON target_assignments (target_id, target_source)
	WHERE state NOT IN ('completed','released','requeued','terminal_failed','terminal_uncertain','expired');

CREATE UNIQUE INDEX IF NOT EXISTS idx_target_assignments_active_gun
	ON target_assignments (gun_id)
	WHERE state NOT IN ('completed','released','requeued','terminal_failed','terminal_uncertain','expired');
