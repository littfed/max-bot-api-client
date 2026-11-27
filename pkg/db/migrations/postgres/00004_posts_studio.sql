-- +goose Up
CREATE TABLE IF NOT EXISTS brand_profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    niche TEXT NOT NULL DEFAULT '',
    target_audience TEXT NOT NULL DEFAULT '',
    tone_of_voice TEXT NOT NULL DEFAULT '',
    forbidden_words TEXT NOT NULL DEFAULT '',
    default_platforms JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_brand_profiles_user_id
ON brand_profiles (user_id);

CREATE TABLE IF NOT EXISTS post_drafts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    profile_id BIGINT REFERENCES brand_profiles(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'idea',
    raw_idea TEXT NOT NULL DEFAULT '',
    source_image_url TEXT NOT NULL DEFAULT '',
    vision_summary TEXT NOT NULL DEFAULT '',
    goal VARCHAR(64) NOT NULL DEFAULT '',
    tone VARCHAR(64) NOT NULL DEFAULT '',
    target_platforms JSONB NOT NULL DEFAULT '[]'::jsonb,
    hooks JSONB NOT NULL DEFAULT '[]'::jsonb,
    selected_hook_id VARCHAR(64) NOT NULL DEFAULT '',
    image_prompt TEXT NOT NULL DEFAULT '',
    generated_image_url TEXT NOT NULL DEFAULT '',
    variants JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_post_drafts_user_status
ON post_drafts (user_id, status, updated_at DESC);

INSERT INTO mode_configs (mode_id, title, is_active, maintenance_message, daily_quota)
VALUES ('posts', 'Контент-студия', TRUE, '', 50)
ON CONFLICT (mode_id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS post_drafts;
DROP TABLE IF EXISTS brand_profiles;
DELETE FROM mode_configs WHERE mode_id = 'posts';
