CREATE TABLE IF NOT EXISTS safety_events (
    id BIGSERIAL PRIMARY KEY,
    event_time TIMESTAMPTZ NOT NULL,
    event_type TEXT NOT NULL,
    gun_id INTEGER NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_safety_events_time ON safety_events (event_time DESC);
CREATE INDEX IF NOT EXISTS idx_safety_events_type ON safety_events (event_type);
CREATE INDEX IF NOT EXISTS idx_safety_events_gun ON safety_events (gun_id);
