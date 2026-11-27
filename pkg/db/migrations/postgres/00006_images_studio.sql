-- +goose Up
CREATE TABLE IF NOT EXISTS image_projects (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode VARCHAR(16) NOT NULL,
    preset_id VARCHAR(64),
    title VARCHAR(255) NOT NULL,
    source_image_key TEXT,
    source_image_url TEXT,
    current_image_key TEXT,
    current_image_url TEXT,
    current_version INT NOT NULL DEFAULT 0,
    image_model VARCHAR(128) NOT NULL DEFAULT '',
    aspect_ratio VARCHAR(16) NOT NULL DEFAULT '1:1',
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_image_projects_user_updated
ON image_projects (user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS image_revisions (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES image_projects(id) ON DELETE CASCADE,
    version INT NOT NULL,
    prompt TEXT NOT NULL,
    action_type VARCHAR(24) NOT NULL,
    image_key TEXT NOT NULL,
    image_url TEXT NOT NULL,
    model VARCHAR(128) NOT NULL DEFAULT '',
    pipeline_used VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, version)
);

INSERT INTO mode_configs (mode_id, title, is_active, maintenance_message, daily_quota)
VALUES ('images', 'Фотостудия', TRUE, '', 50)
ON CONFLICT (mode_id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS image_revisions;
DROP TABLE IF EXISTS image_projects;
DELETE FROM mode_configs WHERE mode_id = 'images';
