-- +goose Up
ALTER TABLE post_drafts ADD COLUMN text_model TEXT NOT NULL DEFAULT '';
ALTER TABLE post_drafts ADD COLUMN image_history TEXT NOT NULL DEFAULT '[]';
ALTER TABLE post_drafts ADD COLUMN text_history TEXT NOT NULL DEFAULT '[]';

-- +goose Down
-- SQLite keeps columns on down or requires table rebuild; no-op for compatibility.
SELECT 1;
