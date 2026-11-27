# Архитектурная спецификация (Design Doc)
# Режим-помощник для создания постов «Контент-студия» (`posts`)

- **Дата:** 27 ноября 2026 г.
- **Статус:** Утверждён к реализации (Approved)
- **Платформа:** Bot Hub (`cmd/showcase-bot`), гибридный интерфейс Чат (MAX / Telegram) + Mini App (`web/`)
- **Базовый репозиторий:** `max-bot-api-client`

---

## 1. Контекст, цели и границы MVP

### 1.1. Цель
Создать встроенный в **Bot Hub** режим **«Контент-студия» (`posts`)** — AI-помощник с полным управляемым пайплайном создания поста: от сырой идеи (текст или фото) и быстрого брифа до готового контент-пакета, адаптированного под несколько площадок (**MAX**, **Telegram**, **VK**, **Instagram**) с визуалом и интерактивным превью.

### 1.2. Границы скоупа (MVP)
- **Входит в MVP:**
  - Управляемый пошаговый визард (`идея/фото → бриф → 3 хука на выбор → генерация пакета и визуала → точечные правки → готовый пакет`).
  - Гибридное прохождение: все шаги доступны как через чат-интерфейс (`BotModule`), так и через экран в Mini App (`web/`) с единым состоянием в БД.
  - Вход: текстовая идея и/или фотография (распознавание контекста через Vision).
  - Выход: готовый контент-пакет — адаптированные тексты, хэштеги, CTA, сгенерированное или загруженное изображение, визуальное превью под каждую выбранную соцсеть, копирование в один клик и скачивание визуала.
  - Профили бренда (`brand_profiles`) и история черновиков/постов (`post_drafts`).
- **Не входит в MVP:**
  - Прямой автопостинг и отложенная публикация через API сторонних соцсетей (VK, Instagram и др.).
  - Парсинг внешних ссылок/статей и транскрибация голосовых сообщений.

---

## 2. Архитектура системы

### 2.1. Место в модульном монолите Bot Hub
Модуль размещается в `internal/modules/posts` и реализует интерфейс `modules.BotModule` (`ID() = "posts"`, `Title() = "Контент-студия"`):

```text
internal/
  modules/
    posts/
      module.go         # BotModule: HandleMessage, HandleCallback (префикс pst:)
      service.go        # Единый доменный сервис пайплайна (вызывается и ботом, и REST API)
      platforms.go      # Декларативный реестр PlatformSpec (IG, VK, TG, MAX)
      ai_pipeline.go    # Промпты и структурированные вызовы GMA (Vision, Hooks, Package, Refine, Image)
      store.go          # Репозиторий PostgreSQL для brand_profiles и post_drafts
      posts_test.go     # Unit-тесты пайплайна, валидации PlatformSpec и retry-логики
  api/
    handlers_posts.go       # REST API эндпоинты /api/modules/posts/*
    handlers_posts_test.go  # Тесты авторизации (ownership), валидации и сквозного flow
pkg/db/migrations/postgres/
  00004_posts_studio.sql    # Миграция таблиц brand_profiles и post_drafts
web/src/
  ...                       # Экран «Контент-студия» (Визард/Превью, История, Профиль бренда)
```

- **Интеграция с `Core Orchestrator`:** режим `posts` использует стандартный конвейер Оркестратора — `Identity Resolver`, распределённую блокировку `lock:fsm:{user_id}:posts`, `Maintenance Guard` и `Quota Guard`.
- **Единый источник правды:** `posts.Service` инкапсулирует всю бизнес-логику. Чат-хэндлеры (`module.go`) и HTTP-хэндлеры (`handlers_posts.go`) не дублируют логику и вызывают одни и те же методы сервиса.

### 2.2. Состояние пайплайна и синхронизация Чат ↔ Mini App
Композитный FSM (`user_mode_states` с ключом `(user_id, "posts")`) хранит только навигационное состояние:
- `current_step`: `idle` | `idea_input` | `brief_setup` | `hooks_select` | `generating` | `ready`
- `payload`: `{"active_draft_id": <int64>}`

Все содержательные данные поста живут в `post_drafts`, благодаря чему пользователь может начать пост в чате, продолжить в Mini App и вернуться к правкам обратно в чат без потери контекста.

---

## 3. Сквозной пайплайн (Шаги визарда)

1. **`idle` (Старт режима):** выбор действия — «Новый пост», «Черновики / История» или «Профиль бренда».
2. **`idea_input` (Ввод идеи):** пользователь отправляет текст идеи и/или изображение.
   - Если пришло изображение — вызывается Vision-анализ (`@scout-vision`), результат сохраняется в `vision_summary` и дополняет `raw_idea`.
3. **`brief_setup` (Быстрый бриф):** если у пользователя есть сохранённый `brand_profile`, его параметры (`niche`, `target_audience`, `tone_of_voice`, `default_platforms`) подставляются автоматически. Пользователь подтверждает профиль в один тап или корректирует цель поста (`engagement` | `sales` | `education` | `announcement`), тон и набор целевых площадок.
4. **`hooks_select` (Выбор угла подачи):** AI генерирует 3 контрастных концепта/хука (`title`, `angle`, `hook_text`). Пользователь выбирает один из них либо запрашивает 3 новых варианта.
5. **`generating` → `ready` (Сборка контент-пакета):**
   - AI генерирует адаптированные тексты под каждую выбранную площадку и формирует `image_prompt`.
   - Если пользователь не загружал своё фото (или запросил генерацию обложки), вызывается GMA `/v1/images/generations`.
6. **`ready` (Правки и финальный результат):**
   - Пользователь просматривает готовый пакет (в чате — превью + быстрые кнопки; в Mini App — карточки в стиле каждой соцсети).
   - Доступны точечные AI-правки (пресеты «Короче», «Дерзче», «Усилить CTA» или свободная инструкция) для отдельной площадки или всего пакета, а также ручное редактирование текста в Mini App.

---

## 4. Модель данных (PostgreSQL)

### 4.1. Таблица `brand_profiles`
Хранит настройки стиля автора/бизнеса (один или несколько пресетов на пользователя):
- `id BIGSERIAL PRIMARY KEY`
- `user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE`
- `name VARCHAR(128) NOT NULL DEFAULT 'Основной'`
- `niche TEXT NOT NULL DEFAULT ''`
- `target_audience TEXT NOT NULL DEFAULT ''`
- `tone_of_voice VARCHAR(64) NOT NULL DEFAULT 'friendly'`
- `forbidden_words TEXT NOT NULL DEFAULT ''`
- `default_platforms JSONB NOT NULL DEFAULT '["max","telegram"]'::jsonb`
- `is_default BOOLEAN NOT NULL DEFAULT TRUE`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

### 4.2. Таблица `post_drafts`
Хранит черновики и готовые контент-пакеты:
- `id BIGSERIAL PRIMARY KEY`
- `user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE`
- `profile_id BIGINT REFERENCES brand_profiles(id) ON DELETE SET NULL`
- `status VARCHAR(32) NOT NULL DEFAULT 'idea'` (`idea` | `brief` | `hooks` | `generating` | `ready` | `archived`)
- `raw_idea TEXT NOT NULL DEFAULT ''`
- `source_image_url TEXT NOT NULL DEFAULT ''`
- `vision_summary TEXT NOT NULL DEFAULT ''`
- `goal VARCHAR(32) NOT NULL DEFAULT 'engagement'`
- `tone VARCHAR(64) NOT NULL DEFAULT 'friendly'`
- `target_platforms JSONB NOT NULL DEFAULT '["max","telegram"]'::jsonb`
- `hooks JSONB NOT NULL DEFAULT '[]'::jsonb` — массив объектов `{id, angle, title, hook_text}`
- `selected_hook_id VARCHAR(64) NOT NULL DEFAULT ''`
- `image_prompt TEXT NOT NULL DEFAULT ''`
- `generated_image_url TEXT NOT NULL DEFAULT ''`
- `variants JSONB NOT NULL DEFAULT '{}'::jsonb` — словарь `{ "<platform>": { "text": "...", "hashtags": [...], "cta": "...", "char_count": 0, "updated_at": "..." } }`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Индексы: `idx_brand_profiles_user_id (user_id)`, `idx_post_drafts_user_status (user_id, status, updated_at DESC)`.

---

## 5. Реестр площадок (`PlatformSpec`)

Специфика соцсетей описывается декларативно в `internal/modules/posts/platforms.go` без дублирования кода обработки:

| ID | Название | Лимит символов (с медиа / текст) | Разметка | Хэштеги | Пропорции визуала |
|---|---|---|---|---|---|
| `instagram` | Instagram | 2200 | Plain + переносы | 5–10 в конце | `4:5` (или `1:1`) |
| `vk` | ВКонтакте | 2000 (рекоменд.) | Plain / абзацы | 2–4 тематических | `1:1` (или `16:9`) |
| `telegram` | Telegram | 1024 (подпись) / 4096 | Markdown / HTML | 0–2 навигационных | `16:9` (или `1:1`) |
| `max` | MAX | 1024 (подпись) / 4000 | Markdown / списки | 0–2 | `16:9` (или `1:1`) |

Каждый `PlatformSpec` содержит `StylePrompt` — инструкцию для LLM (структура первого экрана/лида, ритм абзацев, использование эмодзи, формат CTA). Расширение на новую площадку выполняется добавлением записи в карту `PlatformSpecs`.

---

## 6. AI-оркестрация и обработка ошибок

Все вызовы идут через существующий шлюз **GMA (`pkg/gma` / `pkg/ai`)**:

1. **`AnalyzeImage(ctx, imageURL)`:** Vision-запрос для извлечения фактуры, объектов и настроения из пользовательского фото.
2. **`GenerateHooks(ctx, draft, profile)`:** LLM-запрос со строгим JSON-выходом на 3 разноплановых угла подачи.
3. **`GeneratePackage(ctx, draft, profile, specs)`:** единый структурированный JSON-вызов для генерации адаптированных текстов под все выбранные площадки (гарантирует согласованность фактов и экономит латентность) + генерация `image_prompt` на английском языке.
4. **`GenerateVisual(ctx, imagePrompt, aspectRatio)`:** вызов GMA `/v1/images/generations`, сохранение результата и возврат публичного URL.
5. **`RefineVariant(ctx, draft, platformOrAll, instruction)`:** точечная перегенерация текста одной площадки или всего пакета с сохранением остальных параметров.

### Устойчивость и квоты
- **Валидация выхода LLM:** проверка парсинга JSON, проверка `MaxChars` и отсутствия `forbidden_words`. При невалидном JSON выполняется 1 автоматический retry с корректирующим промптом.
- **Graceful degradation:** если тексты сгенерированы, а генерация картинки завершилась ошибкой/таймаутом, черновик переводится в `ready`, пользователь получает готовые тексты и возможность повторить генерацию изображения отдельно.
- **Таймауты:** 30 секунд на текстовые запросы/Vision, 60 секунд на генерацию изображений.
- **Quota Guard:** все AI-шаги учитываются в суточной квоте пользователя (генерация изображений учитывается с повышенным весом или отдельным лимитом).

---

## 7. Интерфейсы: Чат, REST API и Mini App

### 7.1. Чат-интерфейс (`BotModule`, префикс `pst:`)
- `idle`: `[✨ Новый пост]`, `[📂 Черновики]`, `[🎨 Профиль бренда]`, `[📱 Открыть Контент-студию]` (`open_app`).
- `brief_setup`: быстрые переключатели цели и площадок + `[Сгенерировать 3 хука]`.
- `hooks_select`: вывод 3 вариантов + кнопки `[1]`, `[2]`, `[3]`, `[🔄 Ещё 3]`.
- `ready`: отправка сгенерированного изображения + текста основной площадки и кнопок `[✂️ Короче]`, `[🔥 Дерзче]`, `[🖼 Перерисовать]`, `[📱 Все карточки в Mini App]`.

### 7.2. REST API (`internal/api/handlers_posts.go`)
Все эндпоинты требуют валидный `InitData` (HMAC-SHA256) и проверяют принадлежность `draft_id` / `profile_id` текущему `user_id`:
- `GET /api/modules/posts/state` — активный черновик, история черновиков, профили бренда.
- `POST /api/modules/posts/profile` / `PUT /api/modules/posts/profile/:id` — создание и обновление профиля бренда.
- `POST /api/modules/posts/drafts` — создание черновика (текст и/или фото).
- `PATCH /api/modules/posts/drafts/:id/brief` — обновление цели, тона, площадок.
- `POST /api/modules/posts/drafts/:id/hooks` — генерация/перегенерация 3 хуков.
- `POST /api/modules/posts/drafts/:id/generate` — выбор хука и сборка контент-пакета.
- `POST /api/modules/posts/drafts/:id/refine` — точечная AI-правка (одной площадки или всех).
- `PATCH /api/modules/posts/drafts/:id/variants/:platform` — сохранение ручного редактирования текста.
- `POST /api/modules/posts/drafts/:id/image` — перегенерация изображения.

### 7.3. Frontend (`web/src/`)
Экран режима `posts` («Контент-студия») содержит 3 вкладки:
1. **Студия:** степпер визарда; блок визуала (просмотр, скачивание, перегенерация); вкладки площадок (`MAX`, `Telegram`, `VK`, `Instagram`) с **визуальным превью в стиле соцсети**, счётчиком символов относительно `MaxChars`, кнопкой копирования в один клик и панелью быстрых AI-правок.
2. **История:** список черновиков и готовых постов с переходом к редактированию.
3. **Профиль бренда:** редактор ниши, ЦА, Tone of Voice, стоп-слов и площадок по умолчанию.

---

## 8. Тестирование

- **Unit-тесты (`internal/modules/posts/posts_test.go`):**
  - Переходы шагов визарда и синхронизация `active_draft_id`.
  - Проверка `PlatformSpec` (лимиты символов, фильтр стоп-слов).
  - Мок-тесты AI-пайплайна: успешный flow (с фото и без), 1 retry при битом JSON, graceful degradation при сбое генерации изображения.
- **API-тесты (`internal/api/handlers_posts_test.go`):**
  - Проверка authz/ownership (`403`/`404` при попытке доступа к чужому `draft_id` или `profile_id`).
  - Валидация входных параметров и кодов ответа.
- **Проверка сборки:** `go test ./...` и `npm run build` в `web/`.
