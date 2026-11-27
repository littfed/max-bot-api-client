-- +goose Up
ALTER TABLE post_drafts
    ADD COLUMN IF NOT EXISTS text_model VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS image_history JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS text_history JSONB NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE post_drafts
    DROP COLUMN IF EXISTS text_history,
    DROP COLUMN IF EXISTS image_history,
    DROP COLUMN IF EXISTS text_model;
