-- +goose Up
CREATE TABLE IF NOT EXISTS reminders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    remind_at TIMESTAMP NOT NULL,
    recurrence_rule TEXT NOT NULL DEFAULT 'none',
    status TEXT NOT NULL DEFAULT 'pending',
    source_type TEXT NOT NULL DEFAULT 'user',
    source_ref_id TEXT NOT NULL DEFAULT '',
    template_id TEXT,
    ab_variant TEXT NOT NULL DEFAULT 'A',
    last_sent_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reminders_worker 
ON reminders (status, remind_at ASC);

CREATE INDEX IF NOT EXISTS idx_reminders_user_status 
ON reminders (user_id, status, remind_at ASC);

CREATE TABLE IF NOT EXISTS notification_templates (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_ab_test_active INTEGER NOT NULL DEFAULT 0,
    variant_a_text TEXT NOT NULL,
    variant_a_buttons TEXT NOT NULL DEFAULT '[]',
    variant_b_text TEXT NOT NULL DEFAULT '',
    variant_b_buttons TEXT NOT NULL DEFAULT '[]',
    split_ratio_percent INTEGER NOT NULL DEFAULT 50,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS notification_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reminder_id INTEGER REFERENCES reminders(id) ON DELETE SET NULL,
    template_id TEXT REFERENCES notification_templates(id) ON DELETE SET NULL,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    variant TEXT NOT NULL DEFAULT 'A',
    event_type TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notification_events_template 
ON notification_events (template_id, variant, event_type);

INSERT INTO notification_templates (id, title, description, is_ab_test_active, variant_a_text, variant_b_text)
VALUES (
    'booking_reminder',
    'Напоминание о бронировании в Ресепшене',
    'Триггерное уведомление за 2 часа до визита',
    1,
    'Напоминаем, что вы записаны на услугу {service_title} на {slot_time}. Ждём вас!',
    'Привет! 👋 Напоминаем о вашей записи на {service_title} ({slot_time}). Мастер уже готовится к встрече!'
) ON CONFLICT (id) DO NOTHING;

-- Активируем режим Напоминания в mode_configs
UPDATE mode_configs SET is_active = 1, maintenance_message = '' WHERE mode_id = 'reminders';

-- +goose Down
DROP TABLE IF EXISTS notification_events;
DROP TABLE IF EXISTS notification_templates;
DROP TABLE IF EXISTS reminders;
