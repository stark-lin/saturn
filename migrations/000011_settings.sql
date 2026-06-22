-- This migration creates Settings tables already defined by the Settings domain boundary.
CREATE TABLE system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE user_preferences (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    language TEXT NOT NULL,
    timezone TEXT NOT NULL,
    theme TEXT NOT NULL,
    density TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
