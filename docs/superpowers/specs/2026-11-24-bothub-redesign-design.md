# Архитектурная спецификация (Design Doc)
# Мультирежимная демо-платформа «Bot Hub» (Этап 1: MVP)

- **Дата:** 24 ноября 2026 г.
- **Статус:** Утверждён к реализации (Approved)
- **Целевые платформы:** MAX (приоритетная платформа запуска) + Telegram (подготовка на уровне PlatformAdapter)
- **Базовый репозиторий:** `max-bot-api-client`
- **Продакшен стенд:** Dokploy (`max-showcase-bot`, `combot.littfed.ru`)

---

## 1. Контекст, цели и границы скоупа

### 1.1. Бизнес-цель
Преобразование демонстрационного чат-бота `showcase-bot` в полноценную мультирежимную платформу **«Bot Hub»** с концепцией «8 виртуальных ботов в одном». Продукт служит интерактивным портфолио для лидогенерации коммерческих заказов на разработку ботов и Mini App.

### 1.2. Границы Этапа 1 (MVP)
В рамках первого этапа реализуется архитектурное ядро и **4 ключевых режима**:
1. **Режим 1: Магазин** — каталог цифровых товаров и платных подписок, корзина, демо-воронка оплаты с QR-кодом и симуляцией («Я оплатил» / «Я не оплатил»), интеграция ЮKassa.
2. **Режим 2: Стойка регистрации (Ресепшен)** — бронирование услуг и слотов времени с защитой от овербукинга, витрина мероприятий с выдачей QR-билетов, динамическая анкета онбординга.
3. **Режим 6: Помощь и поддержка** — контекстный RAG-поиск по базе знаний на PostgreSQL (с привязкой к активному режиму `mode_scope`), порог уверенности ответа и эскалация на живого оператора (тикет-система).
4. **Режим 8: Свободное общение** — диалог с LLM с выбором из 5 предустановленных ролей и 4 тональностей, управление скользящим окном памяти контекста.
5. **Центральный экран Mini App (Хаб):** симметричная сетка 3–[1+М+1]–3 с интерактивным маскотом в центре и динамическими репликами в облачках (срочные триггеры, подсказки, время суток, интерактив по тапу).
6. **Сквозная лидогенерация:** закрепленная кнопка «Хочу такого бота» со сбором контекстных метаданных клиента.
7. **Скрытая админка:** панель управления рубильниками профилактики (Active / Maintenance) для всех 8 режимов, просмотр лидов и ответы на тикеты поддержки.
*(Режимы 3 «Напоминания», 4 «Каналы», 5 «Конкурсы» и 7 «Документы» отображаются на Хабе в потухшем виде «В разработке / Профилактика» и планируются к реализации на Этапе 2).*

---

## 2. Архитектура системы (Go Modular Monolith)

### 2.1. Структура каталогов
```text
cmd/showcase-bot/
  main.go                 # Точка входа: инициализация БД, Redis, роутинга, запуск HTTP и вебхука
internal/
  core/
    orchestrator.go       # Core Orchestrator: Identity Resolver, маршрутизация, квоты
    fsm.go                # Композитный FSM: ключ (user_id, mode_id), JSONB payload, TTL 24ч
    lock.go               # Распределённый мьютекс Redis (SET NX EX 5) по lock:fsm:{user_id}:{mode_id}
    guard.go              # Maintenance Guard (проверка кэша рубильников режимов)
    events.go             # Доменные структуры событий IncomingMessage, CallbackAction, WebAppAction
  platform/
    adapter.go            # Интерфейс PlatformAdapter (абстракция над мессенджерами)
    max/                  # Реализация адаптера MAX на базе pkg/maxbot
    telegram/             # Реализация интерфейсного адаптера Telegram
  modules/
    module.go             # Интерфейс виртуального бота BotModule
    shop/                 # Модуль Магазина (бизнес-логика, хэндлеры чата и REST API)
    reception/            # Модуль Ресепшена (слоты, ивенты, анкета)
    support/              # Модуль Поддержки (RAG, тикеты)
    chat/                 # Модуль Свободного общения (роли, тональности, память)
  admin/
    service.go            # Сервис скрытой админки: рубильники, лиды, тикеты
web/                      # React 18 + TypeScript + Vite + Tailwind SPA
  dist/                   # Скомпилированный фронтенд (встраивается через go:embed в бинарник)
pkg/                      # Переиспользуемые библиотеки (maxbot, store, fsm, ai, payments)
```

### 2.2. Композитный FSM (Composite State Machine)
- Состояние сессии хранится по композитному ключу `StateKey = (user_id, mode_id)`.
- Переключение режима сохраняет шаг воронки и черновик в таблице `user_mode_states` (JSONB) и синхронизируется в Redis.
- **Защита от Race Conditions:** распределенная блокировка Redis `lock:fsm:{user_id}:{mode_id}` на 5 секунд защищает от параллельных кликов в Mini App и ввода в чате.
- **State Revalidation:** проверка актуальности броней/товаров при возобновлении сессии (TTL 24ч).

### 2.3. Конвейер обработки события в Core Orchestrator
1. **Identity Resolver:** сопоставление `(platform, platform_user_id)` со сквозным `users.id`.
2. **Global Command Interceptor:** обработка `/start`, `/change_me`, `/lead` и сервисных команд.
3. **Redis Lock Acquisition:** захват блокировки `lock:fsm:{user_id}:{active_mode_id}`.
4. **Maintenance Guard:** проверка активности режима в Redis. Если режим на профилактике — возврат заглушки и инлайн-клавиатуры смены режима.
5. **Quota Guard:** проверка суточных лимитов операций.
6. **Module Dispatch:** делегирование события в соответствующий пакет `internal/modules/{mode}`.
7. **Unlock & Persistence:** атомарное обновление FSM в БД/Redis и освобождение мьютекса.

---

## 3. Модель данных и миграции (PostgreSQL + Goose)

Миграция `pkg/db/migrations/postgres/00002_bothub_schema.sql` формирует доменную схему:

### 3.1. Таблицы ядра и навигации
```sql
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    active_mode_id VARCHAR(32) NOT NULL DEFAULT 'shop',
    role VARCHAR(32) NOT NULL DEFAULT 'user', -- user, admin, superadmin, support_operator
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC+3',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_identities (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(32) NOT NULL, -- 'max' / 'telegram'
    platform_user_id VARCHAR(128) NOT NULL,
    username VARCHAR(128),
    first_name VARCHAR(128),
    phone VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(platform, platform_user_id)
);

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
    status VARCHAR(32) NOT NULL DEFAULT 'new', -- 'new', 'in_progress', 'closed'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS mascot_phrases (
    id BIGSERIAL PRIMARY KEY,
    priority INT NOT NULL DEFAULT 4, -- 1: urgent, 2: progress, 3: time, 4: tap
    trigger_type VARCHAR(32) NOT NULL DEFAULT 'tap',
    text TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
```

### 3.2. Таблицы 4 модулей
```sql
-- Магазин
CREATE TABLE IF NOT EXISTS products (
    id VARCHAR(64) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price INT NOT NULL,
    category VARCHAR(64) NOT NULL,
    item_type VARCHAR(32) NOT NULL DEFAULT 'digital', -- digital, subscription
    content_payload TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

-- Ресепшен
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

CREATE TABLE IF NOT EXISTS bookings (
    id VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    service_id VARCHAR(64) NOT NULL REFERENCES services(id),
    slot_time TIMESTAMPTZ NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'confirmed',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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

CREATE TABLE IF NOT EXISTS onboarding_submissions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    answers JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Поддержка и RAG
DO $$ 
BEGIN
    CREATE EXTENSION IF NOT EXISTS vector;
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'pgvector extension not installed, using FTS tsvector fallback';
END $$;

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
    status VARCHAR(32) NOT NULL DEFAULT 'open', -- 'open', 'answered', 'closed'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS support_messages (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender_type VARCHAR(32) NOT NULL, -- 'user', 'operator', 'bot'
    message_text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Свободное общение
CREATE TABLE IF NOT EXISTS chat_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id INT NOT NULL DEFAULT 1,
    tone_id INT NOT NULL DEFAULT 1,
    messages JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id)
);

-- Аудит админки
CREATE TABLE IF NOT EXISTS admin_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    admin_id BIGINT NOT NULL,
    action VARCHAR(64) NOT NULL,
    target_mode VARCHAR(32),
    payload_diff JSONB,
    ip VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 4. Спецификация REST API (`/api/v1/`)

1. `GET /api/v1/bootstrap` — профиль пользователя, активный режим, список 8 режимов со статусом доступности, реплика маскота, возможности платформы.
2. `POST /api/v1/user/mode` — переключение активного режима (`{ "mode_id": "shop" }`), отправка сервисного сообщения в чат, инвалидация кэша.
3. `GET /api/v1/mascot/tap` — получение новой реплики маскота при тапе.
4. `POST /api/v1/lead` — отправка заявки с кнопки «Хочу такого бота» (`contact`, `comment`, `source_mode`, `tested_modes`).
5. `GET /api/v1/shop/catalog` — товары и подписки.
6. `POST /api/v1/shop/orders` — создание заказа.
7. `POST /api/v1/shop/orders/{id}/simulate` — симуляция оплаты (`action: "paid"` / `"abandoned"`).
8. `GET /api/v1/reception/slots` — слоты времени по услуге и дате.
9. `POST /api/v1/reception/book` — бронирование слота с распределенной Redis-блокировкой.
10. `GET /api/v1/reception/events` — ивенты.
11. `POST /api/v1/reception/events/{id}/register` — регистрация на ивент с генерацией QR-билета.
12. `POST /api/v1/reception/onboarding` — сохранение анкеты онбординга.
13. `POST /api/v1/support/ask` — RAG-запрос по базе знаний со скоупом `mode_scope`.
14. `POST /api/v1/support/tickets` — создание тикета живому человеку.
15. `GET /api/v1/support/tickets` — получение тикетов и ответов.
16. `GET /api/v1/chat/config` — доступные роли и тональности.
17. `POST /api/v1/chat/message` — сообщение в чат с LLM с сохранением скользящего окна памяти.
18. `POST /api/v1/chat/clear` — сброс памяти диалога.
19. `GET /api/v1/admin/modes` — получение состояния рубильников 8 режимов и квот.
20. `POST /api/v1/admin/modes/{id}/toggle` — переключение Active/Maintenance и текста заглушки.
21. `GET /api/v1/admin/leads` — лента заявок «Хочу такого бота».
22. `POST /api/v1/admin/leads/{id}/status` — сброс/смена статуса заявки.
23. `GET /api/v1/admin/tickets` — открытые тикеты техподдержки.
24. `POST /api/v1/admin/tickets/{id}/reply` — ответ оператора (маршрутизируется в чат пользователя).

---

## 5. Фронтенд Mini App (React 18 + TypeScript + Tailwind)

### 5.1. Компоновка главного экрана (Хаб)
Симметричная сетка 3–[1+М+1]–3:
- **Верхняя тройка:** 🛒 Магазин, 🏨 Ресепшен, ⏰ Напоминания *(профилактика)*.
- **Средняя тройка:** 📢 Каналы *(профилактика)* | 🤖 **МАСКОТ В ЦЕНТРЕ** | 🎁 Конкурсы *(профилактика)*.
- **Нижняя тройка:** 🎧 Поддержка, 📄 Документы *(профилактика)*, 💬 Общение.

### 5.2. Поведение маскота и карточек
- Маскот парит с лёгкой CSS-анимацией. Тап вызывает отскок (`bounce`) и обновляет облачко цитаты.
- Карточки Этапа 2 отображаются в `grayscale opacity-40` с бейджем «В разработке». Клик по ним выводит всплывающий Toast с предложением оставить заявку через «Хочу такого бота».
- Активный режим выделяется ярким акцентным бордером и дублируется в нижней панели навигации.
- Шапка приложения содержит постоянную кнопку «Хочу такого бота» и скрытую иконку админки для пользователей с ролью `admin`.

---

## 6. Тестирование, сборка и деплой

### 6.1. Тесты
- Юнит- и интеграционные тесты в Go: изоляция FSM, Redis-блокировки, проверка работы рубильников, демо-оплата Магазина, анти-овербукинг слотов Ресепшена, RAG-поиск со скоупом, роли чата, RBAC админки.
- Валидация TypeScript (`npm run build`) без ошибок компиляции.

### 6.2. Dockerfile и Dokploy
- Многостадийный `Dockerfile.showcase-bot`:
  1. `node:20-alpine AS web-builder`: сборка React SPA в `web/dist`.
  2. `golang:alpine AS go-builder`: сборка Go бинарника со встроенной статикой (`embed.FS`).
  3. `alpine:3.20`: защищенный контейнер, non-root пользователь `appuser`, порт 8080.
- Автоматический деплой на Dokploy (`combot.littfed.ru`) при пуше в `master`.

---

## 7. Чеклист самопроверки (Spec Self-Review)
1. **Плейсхолдеры:** Отсутствуют (нет незаполненных TBD, TODO или общих фраз).
2. **Внутренняя согласованность:** Названия таблиц, полей, эндпоинтов и режимов 100% совпадают между разделами архитектуры, схемы БД и API.
3. **Фокус скоупа:** Чётко зафиксированы 4 режима MVP, а остальные 4 безопасно закрыты механизмом профилактики.
4. **Однозначность:** Все неоднозначности (pgvector fallback, обработка гонок FSM через Redis-lock, права админа) специфицированы явно.
