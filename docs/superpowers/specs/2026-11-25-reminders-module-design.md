# Архитектурная спецификация (Design Doc)
# Модуль «Напоминания» и Движок уведомлений с A/B-тестированием (Режим 3 Bot Hub)

- **Дата:** 25 ноября 2026 г.
- **Статус:** Согласован (Approved)
- **Целевая платформа:** Bot Hub (MAX + Telegram)
- **Базовый репозиторий:** `max-bot-api-client`
- **Продакшен стенд:** Dokploy (`combot.littfed.ru`)

---

## 1. Контекст, цели и границы скоупа

### 1.1. Бизнес-цель
Активация Режима 3 («Напоминания») в мультирежимной демонстрационной платформе **«Bot Hub»**. Модуль совмещает персональный тайм-менеджер для пользователей с демонстрацией B2B-функционала автоматических триггерных уведомлений (напоминания о записи из Ресепшена, сервисные алерты), а также закладывает масштабируемый фундамент для A/B-тестирования текстов сообщений и последующих массовых рассылок (Режим 4 «Каналы»).

### 1.2. Границы скоупа
В рамках разработки модуля реализуются:
1. **Персональные напоминания:** создание разовых и рекуррентных (ежедневно, по будням, еженедельно) задач с привязкой к часовому поясу пользователя (`users.timezone`).
2. **Служба фоновой доставки (Reminder Worker):** транзакционный опрос PostgreSQL (`FOR UPDATE SKIP LOCKED`) с интервалом 10 секунд, защита от дублей, Rate Limiter (30 rps) и корректный Graceful Shutdown.
3. **Интерактивные действия в чате:** всплывающие карточки напоминаний с кнопками `[✅ Выполнено]`, `[⏰ +15 мин]`, `[⏰ +1 час]`, `[❌ Удалить]`.
4. **Сквозная интеграция с Ресепшеном (Reception):** автоматическое планирование системного напоминания за 2 часа до времени визита при бронировании слота, авто-отмена при аннулировании брони.
5. **Движок A/B-тестирования и аналитики:** шаблоны сообщений с разделением трафика (варианты A/B), сбор телеметрии конверсий (`sent`, `viewed`, `completed`, `snoozed`, `cancelled`).
6. **Пользовательский экран в Mini App (`RemindersView.tsx`):** вкладки «В планах» / «История», фильтры, карточки с бейджами источников, пикер времени с пресетами и кнопка демо-симуляции (10-секундный таймер).
7. **REST API эндпоинты:** полный CRUD и экшены управления напоминаниями под защитой `authMiddleware`.

---

## 2. Архитектура и структура каталогов

```text
internal/
  modules/
    reminders/
      module.go              # Реализация интерфейса BotModule (ID = "reminders")
      fsm.go                 # Обработка FSM-шагов создания задач в чате
      worker.go              # Фоновый планировщик доставки с rate.Limiter и SKIP LOCKED
      service.go             # Бизнес-логика, рекуррентность, CRUD, интеграция с шаблонами
      reminders_test.go      # Unit-тесты FSM, дат и рекуррентности
      worker_test.go         # Интеграционные тесты воркера и конкурентности
  api/
    handlers_reminders.go    # REST API контроллеры /api/v1/reminders
pkg/
  db/
    migrations/postgres/
      00003_reminders_schema.sql  # Схема таблиц reminders, notification_templates, events
web/
  src/
    components/
      modes/
        RemindersView.tsx    # React-компонент вкладки «Напоминания» в Mini App
```

---

## 3. Модель данных и миграции (PostgreSQL)

Миграция `00003_reminders_schema.sql`:

```sql
-- Таблица напоминаний (1-to-1)
CREATE TABLE IF NOT EXISTS reminders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    remind_at TIMESTAMPTZ NOT NULL,
    recurrence_rule VARCHAR(32) NOT NULL DEFAULT 'none', -- 'none', 'daily', 'weekdays', 'weekly'
    status VARCHAR(32) NOT NULL DEFAULT 'pending',       -- 'pending', 'sent', 'completed', 'cancelled'
    source_type VARCHAR(32) NOT NULL DEFAULT 'user',     -- 'user', 'reception', 'demo_simulation'
    source_ref_id VARCHAR(128) NOT NULL DEFAULT '',     -- ID бронирования или внешней сущности
    template_id VARCHAR(64),
    ab_variant VARCHAR(8) NOT NULL DEFAULT 'A',          -- 'A' или 'B'
    last_sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индекс для высоконагруженной выборки воркером
CREATE INDEX IF NOT EXISTS idx_reminders_worker 
ON reminders (status, remind_at ASC) 
WHERE status = 'pending';

-- Индекс для пользовательских выборок в Mini App
CREATE INDEX IF NOT EXISTS idx_reminders_user_status 
ON reminders (user_id, status, remind_at ASC);

-- Шаблоны уведомлений для A/B тестирования и системных триггеров
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

-- События телеметрии и конверсий для аналитики
CREATE TABLE IF NOT EXISTS notification_events (
    id BIGSERIAL PRIMARY KEY,
    reminder_id BIGINT REFERENCES reminders(id) ON DELETE SET NULL,
    template_id VARCHAR(64) REFERENCES notification_templates(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    variant VARCHAR(8) NOT NULL DEFAULT 'A',
    event_type VARCHAR(32) NOT NULL, -- 'sent', 'viewed', 'completed', 'snoozed', 'cancelled'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Начальные шаблоны
INSERT INTO notification_templates (id, title, description, is_ab_test_active, variant_a_text, variant_b_text)
VALUES (
    'booking_reminder',
    'Напоминание о бронировании в Ресепшене',
    'Триггерное уведомление за 2 часа до визита',
    TRUE,
    'Напоминаем, что вы записаны на услугу {service_title} на {slot_time}. Ждём вас!',
    'Привет! 👋 Напоминаем о вашей записи на {service_title} ({slot_time}). Мастер уже готовится к встрече!'
) ON CONFLICT (id) DO NOTHING;
```

---

## 4. Движок планирования и фоновый воркер

### 4.1. Цикл работы воркера
1. Запускается в фоновой горутине при старте бота, принимает контекст с отменой (`ctx.Done()`).
2. Тикер каждые 10 секунд инициирует транзакцию с выборкой пачки до 50 записей:
   ```sql
   SELECT id, user_id, title, description, recurrence_rule, template_id, ab_variant, source_type
   FROM reminders
   WHERE status = 'pending' AND remind_at <= NOW()
   ORDER BY remind_at ASC
   LIMIT 50
   FOR UPDATE SKIP LOCKED;
   ```
3. Для каждого напоминания:
   - Проверяется Rate Limiter (не более 25 сообщений/сек).
   - Получается платформа пользователя (`user_identities`).
   - Собирается текст (персональный или через шаблон с динамическими переменными).
   - Прикрепляются инлайн-кнопки (`rmd:done:<id>`, `rmd:snz:15:<id>`, `rmd:snz:60:<id>`, `rmd:del:<id>`).
   - Отправляется сообщение через `PlatformAdapter`.
   - В `notification_events` логируется событие `sent`.
   - Запись переводится в `status = 'sent'`, обновляется `last_sent_at`.

### 4.2. Обработка колбэков откладывания и выполнения
* **Отложить (`rmd:snz:15` / `rmd:snz:60`):**
  - Запись обновляется: `remind_at = NOW() + INTERVAL`, `status = 'pending'`.
  - Сообщение в чате редактируется с информацией о новом времени.
  - Логируется событие `snoozed`.
* **Выполнить (`rmd:done`):**
  - Запись переводится в `status = 'completed'`.
  - Логируется событие `completed`.
  - Если `recurrence_rule` не `none`, рассчитывается следующая дата (`daily` -> +24ч, `weekdays` -> Пн-Пт, `weekly` -> +7 дней) и создается следующее напоминание в статусе `pending`.
* **Удалить (`rmd:del`):**
  - Запись переводится в `status = 'cancelled'`.

---

## 5. Пользовательские сценарии в чате (FSM)

### 5.1. Шаги диалога создания
- `reminders:wait_title`: Запрос текста задачи (2–255 символов).
- `reminders:wait_time`: Предложение инлайн-кнопок быстрых сроков (`+15 мин`, `+1 час`, `+3 часа`, `Сегодня в 19:00`, `Завтра в 10:00`) либо ввод точного времени текстом.
- `reminders:wait_recurrence`: Выбор повтора (`Однократно`, `Каждый день`, `По будням`, `Раз в неделю`).
- Итог: карточка подтверждения с кнопками действий.

### 5.2. Демо-симуляция триггера
- Кнопка `[⚡ Демо: Тест триггера]` в меню режима создает демо-напоминание с задержкой 10 секунд.
- Позволяет клиентам и лидам мгновенно протестировать механику без ожидания реальных сроков.

---

## 6. Интерфейс в Mini App и REST API

### 6.1. Эндпоинты
- `GET /api/v1/reminders` — получение списка задач пользователя (`?status=active,completed`).
- `POST /api/v1/reminders` — создание нового напоминания.
- `PATCH /api/v1/reminders/{id}/action` — выполнение действия (`complete`, `snooze`, `cancel`).
- `DELETE /api/v1/reminders/{id}` — удаление напоминания.
- `POST /api/v1/reminders/simulate` — запуск демо-триггера с задержкой 10 секунд.
- `GET /api/v1/admin/notifications/ab-stats` — статистика конверсий A/B вариантов.

### 6.2. Компонент `RemindersView.tsx`
- Индикатор режима и счетчики активных/завершенных задач.
- Вкладки переключения «В планах» и «История».
- Кнопка создания с модальным окном (нативный пикер времени + быстрые таблетки времени + селектор повторов).
- Карточки задач с бейджами источников (`[Личное]`, `[Ресепшен]`) и кнопками мгновенного выполнения / откладывания.

---

## 7. Интеграция с Ресепшеном и внешними модулями

1. При бронировании в `internal/modules/reception` вызывается сервис напоминаний, который регистрирует триггер на дату визита минус 2 часа с `source_type = 'reception'` и `source_ref_id = booking_id`.
2. При отмене брони в Ресепшене соответствующее напоминание автоматически переводится в `cancelled`.
3. Модуль конфигурации `mode_configs` обновляется: рубильник для `reminders` переводится в `is_active = true`.

---

## 8. Тестирование и верификация

1. **Unit-тесты:**
   - Валидация FSM переходов в `reminders_test.go`.
   - Тестирование вычисления рекуррентных дат (переход пятница -> понедельник для `weekdays`, переход через границу месяца).
2. **Воркер и конкурентность:**
   - Проверка выборки `SKIP LOCKED` при параллельных симуляциях.
   - Изоляция ошибок API мессенджера (обработка 429 Retry-After).
3. **Интеграционные тесты:**
   - Тестирование REST эндпоинтов `/api/v1/reminders`.
   - Сквозной тест «бронь в Ресепшене -> появление напоминания -> отмена брони».
