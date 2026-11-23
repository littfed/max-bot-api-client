-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    active_mode_id TEXT NOT NULL DEFAULT 'shop',
    role TEXT NOT NULL DEFAULT 'user',
    timezone TEXT NOT NULL DEFAULT 'UTC+3',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_identities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    platform_user_id TEXT NOT NULL,
    username TEXT,
    first_name TEXT,
    phone TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(platform, platform_user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_identities_user_id ON user_identities(user_id);

CREATE TABLE IF NOT EXISTS user_mode_states (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode_id TEXT NOT NULL,
    current_step TEXT NOT NULL DEFAULT '',
    payload TEXT NOT NULL DEFAULT '{}',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, mode_id)
);

CREATE TABLE IF NOT EXISTS mode_configs (
    mode_id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    maintenance_message TEXT NOT NULL DEFAULT 'Режим временно находится на техническом обслуживании',
    daily_quota INTEGER NOT NULL DEFAULT 100,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS leads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    source_mode TEXT NOT NULL,
    tested_modes TEXT NOT NULL DEFAULT '[]',
    contact TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'new',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_leads_user_id ON leads(user_id);
CREATE INDEX IF NOT EXISTS idx_leads_status ON leads(status);
CREATE INDEX IF NOT EXISTS idx_leads_created_at ON leads(created_at DESC);

CREATE TABLE IF NOT EXISTS mascot_phrases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    priority INTEGER NOT NULL DEFAULT 4,
    trigger_type TEXT NOT NULL DEFAULT 'tap',
    text TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_mascot_phrases_active ON mascot_phrases(is_active, priority);

CREATE TABLE IF NOT EXISTS products (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price INTEGER NOT NULL,
    category TEXT NOT NULL,
    item_type TEXT NOT NULL DEFAULT 'digital',
    content_payload TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS services (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    duration_minutes INTEGER NOT NULL DEFAULT 30,
    price INTEGER NOT NULL DEFAULT 0,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS time_slots (
    id TEXT PRIMARY KEY,
    service_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    slot_time TIMESTAMP NOT NULL,
    is_booked BOOLEAN NOT NULL DEFAULT 0,
    booked_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_time_slots_service ON time_slots(service_id, slot_time);

CREATE TABLE IF NOT EXISTS bookings (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    service_id TEXT NOT NULL REFERENCES services(id),
    slot_time TIMESTAMP NOT NULL,
    status TEXT NOT NULL DEFAULT 'confirmed',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_bookings_user_id ON bookings(user_id);

CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    event_time TIMESTAMP NOT NULL,
    total_seats INTEGER NOT NULL DEFAULT 50,
    booked_seats INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS event_registrations (
    id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ticket_qr TEXT NOT NULL,
    registered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_event_regs_user ON event_registrations(user_id);
CREATE INDEX IF NOT EXISTS idx_event_regs_event ON event_registrations(event_id);

CREATE TABLE IF NOT EXISTS onboarding_submissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    answers TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_onboarding_user_id ON onboarding_submissions(user_id);

CREATE TABLE IF NOT EXISTS knowledge_documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'faq',
    mode_scope TEXT NOT NULL DEFAULT 'global',
    content TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    doc_id INTEGER NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
    mode_scope TEXT NOT NULL DEFAULT 'global',
    chunk_text TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_scope ON knowledge_chunks(mode_scope);

CREATE TABLE IF NOT EXISTS support_tickets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    mode_scope TEXT NOT NULL DEFAULT 'global',
    subject TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_support_tickets_user ON support_tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_support_tickets_status ON support_tickets(status);

CREATE TABLE IF NOT EXISTS support_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id INTEGER NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender_type TEXT NOT NULL,
    message_text TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_support_msgs_ticket ON support_messages(ticket_id);

CREATE TABLE IF NOT EXISTS chat_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id INTEGER NOT NULL DEFAULT 1,
    tone_id INTEGER NOT NULL DEFAULT 1,
    messages TEXT NOT NULL DEFAULT '[]',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id)
);

CREATE TABLE IF NOT EXISTS admin_audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    admin_id INTEGER NOT NULL,
    action TEXT NOT NULL,
    target_mode TEXT,
    payload_diff TEXT,
    ip TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_admin ON admin_audit_logs(admin_id, created_at DESC);

-- Seeds for mode_configs
INSERT OR IGNORE INTO mode_configs (mode_id, title, is_active, maintenance_message, daily_quota) VALUES
('shop', 'Магазин', 1, 'Режим временно находится на техническом обслуживании', 100),
('reception', 'Ресепшен', 1, 'Режим временно находится на техническом обслуживании', 100),
('reminders', 'Напоминания', 0, 'Модуль находится в разработке (Этап 2)', 100),
('channels', 'Каналы', 0, 'Модуль находится в разработке (Этап 2)', 100),
('giveaways', 'Конкурсы', 0, 'Модуль находится в разработке (Этап 2)', 100),
('support', 'Поддержка', 1, 'Режим временно находится на техническом обслуживании', 100),
('docs', 'Документы', 0, 'Модуль находится в разработке (Этап 2)', 100),
('chat', 'Общение', 1, 'Режим временно находится на техническом обслуживании', 100);

-- Seeds for mascot_phrases
INSERT INTO mascot_phrases (priority, trigger_type, text, is_active) VALUES
(1, 'urgent', 'Внимание! Проверьте важные обновления по вашим заказам и бронированиям.', 1),
(2, 'progress', 'Отличный прогресс! Исследуйте все доступные режимы платформы Bot Hub.', 1),
(3, 'time', 'Рады видеть вас! Чем Bot Hub может помочь вам сегодня?', 1),
(4, 'tap', 'Привет! Я бот-маскот MAX. Нажмите на любой режим, чтобы начать тестирование.', 1),
(4, 'tap', 'Хотите подключить такого же умного бота к вашему бизнесу? Жмите «Хочу такого бота»!', 1);

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
