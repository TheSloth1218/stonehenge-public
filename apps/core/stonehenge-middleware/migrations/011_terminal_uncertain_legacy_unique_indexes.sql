DROP INDEX IF EXISTS ux_target_assignments_active_target;
DROP INDEX IF EXISTS ux_target_assignments_active_gun;

CREATE UNIQUE INDEX IF NOT EXISTS ux_target_assignments_active_target
	ON target_assignments (target_source, target_id)
	WHERE state NOT IN ('completed','released','requeued','terminal_failed','terminal_uncertain','expired');

CREATE UNIQUE INDEX IF NOT EXISTS ux_target_assignments_active_gun
	ON target_assignments (gun_id)
	WHERE state NOT IN ('completed','released','requeued','terminal_failed','terminal_uncertain','expired');
