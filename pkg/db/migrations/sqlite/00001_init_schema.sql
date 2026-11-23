-- +goose Up
CREATE TABLE IF NOT EXISTS channels (
    chat_id BIGINT PRIMARY KEY,
    title TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'channel',
    is_admin BOOLEAN NOT NULL DEFAULT 0,
    permissions TEXT NOT NULL DEFAULT '[]',
    last_updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    user_name TEXT NOT NULL,
    items TEXT NOT NULL DEFAULT '[]',
    total INTEGER NOT NULL DEFAULT 0,
    delivery_type TEXT NOT NULL DEFAULT 'delivery',
    address TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'accepted',
    verified BOOLEAN NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC);

CREATE TABLE IF NOT EXISTS reactions (
    post_mid TEXT PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    counts TEXT NOT NULL DEFAULT '{}',
    user_votes TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_reactions_chat_id ON reactions(chat_id);

CREATE TABLE IF NOT EXISTS polls (
    id TEXT PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    message_mid TEXT NOT NULL DEFAULT '',
    question TEXT NOT NULL,
    options TEXT NOT NULL DEFAULT '[]',
    votes TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_polls_chat_id ON polls(chat_id);

CREATE TABLE IF NOT EXISTS giveaways (
    id TEXT PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    title TEXT NOT NULL,
    promo_code TEXT NOT NULL,
    discount_percent INTEGER NOT NULL DEFAULT 0,
    participants TEXT NOT NULL DEFAULT '{}',
    winners TEXT NOT NULL DEFAULT '[]',
    active BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_giveaways_chat_id ON giveaways(chat_id);

CREATE TABLE IF NOT EXISTS media_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id BIGINT NOT NULL,
    artifact_type TEXT NOT NULL,
    title TEXT NOT NULL,
    prompt TEXT NOT NULL DEFAULT '',
    content_text TEXT NOT NULL DEFAULT '',
    media_url TEXT NOT NULL DEFAULT '',
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_media_history_user_id ON media_history(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_media_history_type ON media_history(artifact_type);

-- +goose Down
DROP TABLE IF EXISTS media_history;
DROP TABLE IF EXISTS giveaways;
DROP TABLE IF EXISTS polls;
DROP TABLE IF EXISTS reactions;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS channels;
