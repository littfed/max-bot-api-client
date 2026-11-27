-- +goose Up
CREATE TABLE IF NOT EXISTS brand_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    niche TEXT NOT NULL DEFAULT '',
    target_audience TEXT NOT NULL DEFAULT '',
    tone_of_voice TEXT NOT NULL DEFAULT '',
    forbidden_words TEXT NOT NULL DEFAULT '',
    default_platforms TEXT NOT NULL DEFAULT '[]',
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_brand_profiles_user_id
ON brand_profiles (user_id);

CREATE TABLE IF NOT EXISTS post_drafts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    profile_id INTEGER REFERENCES brand_profiles(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'idea',
    raw_idea TEXT NOT NULL DEFAULT '',
    source_image_url TEXT NOT NULL DEFAULT '',
    vision_summary TEXT NOT NULL DEFAULT '',
    goal TEXT NOT NULL DEFAULT '',
    tone TEXT NOT NULL DEFAULT '',
    target_platforms TEXT NOT NULL DEFAULT '[]',
    hooks TEXT NOT NULL DEFAULT '[]',
    selected_hook_id TEXT NOT NULL DEFAULT '',
    image_prompt TEXT NOT NULL DEFAULT '',
    generated_image_url TEXT NOT NULL DEFAULT '',
    variants TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_post_drafts_user_status
ON post_drafts (user_id, status, updated_at DESC);

INSERT OR IGNORE INTO mode_configs (mode_id, title, is_active, maintenance_message, daily_quota)
VALUES ('posts', 'Контент-студия', 1, '', 50);

-- +goose Down
DROP TABLE IF EXISTS post_drafts;
DROP TABLE IF EXISTS brand_profiles;
DELETE FROM mode_configs WHERE mode_id = 'posts';
