-- +goose Up
CREATE TABLE IF NOT EXISTS reminders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    remind_at TIMESTAMPTZ NOT NULL,
    recurrence_rule VARCHAR(32) NOT NULL DEFAULT 'none',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    source_type VARCHAR(32) NOT NULL DEFAULT 'user',
    source_ref_id VARCHAR(128) NOT NULL DEFAULT '',
    template_id VARCHAR(64),
    ab_variant VARCHAR(8) NOT NULL DEFAULT 'A',
    last_sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reminders_worker 
ON reminders (status, remind_at ASC) 
WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_reminders_user_status 
ON reminders (user_id, status, remind_at ASC);

CREATE TABLE IF NOT EXISTS notification_templates (
    id VARCHAR(64) PRIMARY KEY,
    title VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_ab_test_active BOOLEAN NOT NULL DEFAULT FALSE,
    variant_a_text TEXT NOT NULL,
    variant_a_buttons JSONB NOT NULL DEFAULT '[]'::jsonb,
    variant_b_text TEXT NOT NULL DEFAULT '',
    variant_b_buttons JSONB NOT NULL DEFAULT '[]'::jsonb,
    split_ratio_percent INT NOT NULL DEFAULT 50,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notification_events (
    id BIGSERIAL PRIMARY KEY,
    reminder_id BIGINT REFERENCES reminders(id) ON DELETE SET NULL,
    template_id VARCHAR(64) REFERENCES notification_templates(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    variant VARCHAR(8) NOT NULL DEFAULT 'A',
    event_type VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_events_template 
ON notification_events (template_id, variant, event_type);

INSERT INTO notification_templates (id, title, description, is_ab_test_active, variant_a_text, variant_b_text)
VALUES (
    'booking_reminder',
    'Напоминание о бронировании в Ресепшене',
    'Триггерное уведомление за 2 часа до визита',
    TRUE,
    'Напоминаем, что вы записаны на услугу {service_title} на {slot_time}. Ждём вас!',
    'Привет! 👋 Напоминаем о вашей записи на {service_title} ({slot_time}). Мастер уже готовится к встрече!'
) ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS notification_events;
DROP TABLE IF EXISTS notification_templates;
DROP TABLE IF EXISTS reminders;
