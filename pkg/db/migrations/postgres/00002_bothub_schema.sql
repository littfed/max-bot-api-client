-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    active_mode_id VARCHAR(32) NOT NULL DEFAULT 'shop',
    role VARCHAR(32) NOT NULL DEFAULT 'user',
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC+3',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_identities (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(32) NOT NULL,
    platform_user_id VARCHAR(128) NOT NULL,
    username VARCHAR(128),
    first_name VARCHAR(128),
    phone VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(platform, platform_user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_identities_user_id ON user_identities(user_id);

CREATE TABLE IF NOT EXISTS user_mode_states (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode_id VARCHAR(32) NOT NULL,
    current_step VARCHAR(64) NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, mode_id)
);

CREATE TABLE IF NOT EXISTS mode_configs (
    mode_id VARCHAR(32) PRIMARY KEY,
    title VARCHAR(64) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    maintenance_message TEXT NOT NULL DEFAULT 'Режим временно находится на техническом обслуживании',
    daily_quota INT NOT NULL DEFAULT 100,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS leads (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(32) NOT NULL,
    source_mode VARCHAR(32) NOT NULL,
    tested_modes JSONB NOT NULL DEFAULT '[]'::jsonb,
    contact TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'new',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_leads_user_id ON leads(user_id);
CREATE INDEX IF NOT EXISTS idx_leads_status ON leads(status);
CREATE INDEX IF NOT EXISTS idx_leads_created_at ON leads(created_at DESC);

CREATE TABLE IF NOT EXISTS mascot_phrases (
    id BIGSERIAL PRIMARY KEY,
    priority INT NOT NULL DEFAULT 4,
    trigger_type VARCHAR(32) NOT NULL DEFAULT 'tap',
    text TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS idx_mascot_phrases_active ON mascot_phrases(is_active, priority);

CREATE TABLE IF NOT EXISTS products (
    id VARCHAR(64) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price INT NOT NULL,
    category VARCHAR(64) NOT NULL,
    item_type VARCHAR(32) NOT NULL DEFAULT 'digital',
    content_payload TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS services (
    id VARCHAR(64) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    duration_minutes INT NOT NULL DEFAULT 30,
    price INT NOT NULL DEFAULT 0,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS time_slots (
    id VARCHAR(64) PRIMARY KEY,
    service_id VARCHAR(64) NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    slot_time TIMESTAMPTZ NOT NULL,
    is_booked BOOLEAN NOT NULL DEFAULT FALSE,
    booked_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_time_slots_service ON time_slots(service_id, slot_time);

CREATE TABLE IF NOT EXISTS bookings (
    id VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    service_id VARCHAR(64) NOT NULL REFERENCES services(id),
    slot_time TIMESTAMPTZ NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'confirmed',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bookings_user_id ON bookings(user_id);

CREATE TABLE IF NOT EXISTS events (
    id VARCHAR(64) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    total_seats INT NOT NULL DEFAULT 50,
    booked_seats INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS event_registrations (
    id VARCHAR(64) PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ticket_qr TEXT NOT NULL,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_event_regs_user ON event_registrations(user_id);
CREATE INDEX IF NOT EXISTS idx_event_regs_event ON event_registrations(event_id);

CREATE TABLE IF NOT EXISTS onboarding_submissions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    answers JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_onboarding_user_id ON onboarding_submissions(user_id);

CREATE TABLE IF NOT EXISTS knowledge_documents (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    category VARCHAR(64) NOT NULL DEFAULT 'faq',
    mode_scope VARCHAR(32) NOT NULL DEFAULT 'global',
    content TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id BIGSERIAL PRIMARY KEY,
    doc_id BIGINT NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
    mode_scope VARCHAR(32) NOT NULL DEFAULT 'global',
    chunk_text TEXT NOT NULL,
    tsv TSVECTOR
);

CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_scope ON knowledge_chunks(mode_scope);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_tsv ON knowledge_chunks USING gin(tsv);

CREATE TABLE IF NOT EXISTS support_tickets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(32) NOT NULL,
    mode_scope VARCHAR(32) NOT NULL DEFAULT 'global',
    subject VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_tickets_user ON support_tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_support_tickets_status ON support_tickets(status);

CREATE TABLE IF NOT EXISTS support_messages (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender_type VARCHAR(32) NOT NULL,
    message_text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_msgs_ticket ON support_messages(ticket_id);

CREATE TABLE IF NOT EXISTS chat_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id INT NOT NULL DEFAULT 1,
    tone_id INT NOT NULL DEFAULT 1,
    messages JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id)
);

CREATE TABLE IF NOT EXISTS admin_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    admin_id BIGINT NOT NULL,
    action VARCHAR(64) NOT NULL,
    target_mode VARCHAR(32),
    payload_diff JSONB,
    ip VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_admin ON admin_audit_logs(admin_id, created_at DESC);

-- Seeds for mode_configs
INSERT INTO mode_configs (mode_id, title, is_active, maintenance_message, daily_quota) VALUES
('shop', 'Магазин', TRUE, 'Режим временно находится на техническом обслуживании', 100),
('reception', 'Ресепшен', TRUE, 'Режим временно находится на техническом обслуживании', 100),
('reminders', 'Напоминания', FALSE, 'Модуль находится в разработке (Этап 2)', 100),
('channels', 'Каналы', FALSE, 'Модуль находится в разработке (Этап 2)', 100),
('giveaways', 'Конкурсы', FALSE, 'Модуль находится в разработке (Этап 2)', 100),
('support', 'Поддержка', TRUE, 'Режим временно находится на техническом обслуживании', 100),
('docs', 'Документы', FALSE, 'Модуль находится в разработке (Этап 2)', 100),
('chat', 'Общение', TRUE, 'Режим временно находится на техническом обслуживании', 100)
ON CONFLICT (mode_id) DO NOTHING;

-- Seeds for mascot_phrases
INSERT INTO mascot_phrases (priority, trigger_type, text, is_active) VALUES
(1, 'urgent', 'Внимание! Проверьте важные обновления по вашим заказам и бронированиям.', TRUE),
(2, 'progress', 'Отличный прогресс! Исследуйте все доступные режимы платформы Bot Hub.', TRUE),
(3, 'time', 'Рады видеть вас! Чем Bot Hub может помочь вам сегодня?', TRUE),
(4, 'tap', 'Привет! Я бот-маскот MAX. Нажмите на любой режим, чтобы начать тестирование.', TRUE),
(4, 'tap', 'Хотите подключить такого же умного бота к вашему бизнесу? Жмите «Хочу такого бота»!', TRUE);

-- +goose Down
DROP TABLE IF EXISTS admin_audit_logs;
DROP TABLE IF EXISTS chat_sessions;
DROP TABLE IF EXISTS support_messages;
DROP TABLE IF EXISTS support_tickets;
DROP TABLE IF EXISTS knowledge_chunks;
DROP TABLE IF EXISTS knowledge_documents;
DROP TABLE IF EXISTS onboarding_submissions;
DROP TABLE IF EXISTS event_registrations;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS time_slots;
DROP TABLE IF EXISTS services;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS mascot_phrases;
DROP TABLE IF EXISTS leads;
DROP TABLE IF EXISTS mode_configs;
DROP TABLE IF EXISTS user_mode_states;
DROP TABLE IF EXISTS user_identities;
DROP TABLE IF EXISTS users;
