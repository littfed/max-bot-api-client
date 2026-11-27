-- +goose Up
CREATE TABLE IF NOT EXISTS image_projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode TEXT NOT NULL,
    preset_id TEXT,
    title TEXT NOT NULL,
    source_image_key TEXT,
    source_image_url TEXT,
    current_image_key TEXT,
    current_image_url TEXT,
    current_version INTEGER NOT NULL DEFAULT 0,
    image_model TEXT NOT NULL DEFAULT '',
    aspect_ratio TEXT NOT NULL DEFAULT '1:1',
    status TEXT NOT NULL DEFAULT 'draft',
    error_message TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_image_projects_user_updated
ON image_projects (user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS image_revisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES image_projects(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    prompt TEXT NOT NULL,
    action_type TEXT NOT NULL,
    image_key TEXT NOT NULL,
    image_url TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    pipeline_used TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (project_id, version)
);

INSERT OR IGNORE INTO mode_configs (mode_id, title, is_active, maintenance_message, daily_quota)
VALUES ('images', 'Фотостудия', 1, '', 50);

-- +goose Down
DROP TABLE IF EXISTS image_revisions;
DROP TABLE IF EXISTS image_projects;
DELETE FROM mode_configs WHERE mode_id = 'images';
