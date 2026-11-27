# Архитектурная спецификация (Design Doc)
# Режим генерации и исправления изображений «Фотостудия» (`images`)

- **Дата:** 27 ноября 2026 г.
- **Статус:** На ревью (Draft for Review)
- **Платформа:** Bot Hub (`cmd/showcase-bot`), гибридный интерфейс Чат (MAX / Telegram) + Mini App (`web/`)
- **Базовый репозиторий:** `max-bot-api-client`

---

## 1. Контекст, цели и границы MVP

### 1.1. Цель
Добавить в **Bot Hub** 9-й активный режим — **«Фотостудия» (`images`)** для генерации изображений с нуля (Text-to-Image), исправления/стилизации загруженных фотографий (Image-to-Image) и итеративной доработки результата по текстовым инструкциям с историей ревизий. Ключевая особенность — каталог предустановленных сценариев с визуальными примерами «До / После» и запуском в один тап.

### 1.2. Границы скоупа (MVP)
- **Входит в MVP:**
  - 9-й активный режим в `HubGrid` (`id: "images"`, `title: "Фотостудия"`).
  - Каталог из 8 предустановленных сценариев (5 для `edit` и 3 для `generate`) с визуальными примерами (`example_before_url` / `example_after_url`) и готовыми промптами (`example_prompt`) для запуска в 1 тап («Попробовать пример»).
  - Свободный режим: генерация по произвольному промпту или редактирование загруженного фото.
  - Итеративные правки (`refine`) с сохранением версий и откатом (`restore`).
  - Гибридный AI-пайплайн: `/v1/images/edits` (primary) + автоматический фоллбэк `Vision + GenerateImage`.
  - Внешнее объектное хранилище S3/MinIO (`ObjectStorage`) для пользовательских исходников и сгенерированных ревизий, с автоматическим локальным/in-memory фоллбэком при отсутствии S3-конфигурации (dev/тесты).
  - Гибридный интерфейс: полноценный чат-флоу (`internal/modules/images`) и экран Mini App (`web/src/components/modes/ImagesView.tsx`).
- **Не входит в MVP:**
  - Кисть/рисование масок на канвасе (inpainting по маске выделяется в будущую итерацию; в MVP редактирование выполняется по текстовой инструкции на всё изображение).
  - Пакетная обработка нескольких фото за один запрос.
  - Генерация видео или анимаций.

---

## 2. Архитектура системы

### 2.1. Место в модульном монолите Bot Hub
```text
pkg/
  ai/
    client.go                 # + метод EditImage(ctx, *ImageEditRequest) (/v1/images/edits)
    types.go                  # + ImageEditRequest
  db/migrations/postgres/
    00006_images_studio.sql   # Таблицы image_projects и image_revisions
internal/
  storage/
    s3.go                     # Интерфейс ObjectStorage, S3Storage (MinIO/S3) и MemoryStorage
    s3_test.go
  modules/
    images/
      module.go               # BotModule: HandleMessage, HandleCallback (префикс img:)
      service.go              # Единый доменный сервис (используется и ботом, и REST API)
      presets.go              # Декларативный реестр сценариев с примерами
      ai_pipeline.go          # Гибридный AI-пайплайн (Generate, EditImage + Vision fallback)
      store.go                # Репозиторий PostgreSQL (image_projects, image_revisions)
      types.go                # Доменные структуры и DTO
      images_test.go          # Unit-тесты сервиса, пайплайна, сценариев и чат-модуля
  api/
    handlers_images.go        # REST API /api/modules/images/*
    handlers_images_test.go   # Тесты авторизации, ownership, валидации и сквозного flow
web/
  public/presets/             # Статические превью «До / После» для предустановленных сценариев
  src/
    api/images.ts             # Типизированный API-клиент
    types/images.ts           # TypeScript-интерфейсы
    components/modes/
      ImagesView.tsx          # Экран «Фотостудия» (Сценарии / Студия / Мои работы)
```

### 2.2. Интеграция с Core Orchestrator и квотами
- Режим регистрируется в `showcase-bot` как `images` («Фотостудия»).
- Использует стандартный конвейер Оркестратора: `Identity Resolver`, распределённую блокировку `lock:fsm:{user_id}:images`, `Maintenance Guard` и `Quota Guard`.
- Каждая операция генерации (`create` с генерацией/правкой) и итеративной правки (`refine`) списывает 1 единицу дневной квоты пользователя. Операции просмотра, отката (`restore`) и удаления не расходуют квоту.

### 2.3. Хранение медиафайлов (S3 / MinIO)
- **Интерфейс `internal/storage.ObjectStorage`:**
  - `PutObject(ctx context.Context, key string, contentType string, data []byte) (publicURL string, err error)`
  - `GetObject(ctx context.Context, key string) (data []byte, contentType string, err error)`
  - `DeleteObject(ctx context.Context, key string) error`
- **Конфигурация через переменные окружения:** `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_PUBLIC_URL`, `S3_USE_SSL`.
- **Ключи объектов:** `images/{user_id}/{project_id}/source_{uuid}.{ext}` для загруженного исходника и `images/{user_id}/{project_id}/v{version}_{uuid}.png` для сгенерированных ревизий.
- **Фоллбэк:** если `S3_ENDPOINT` не задан, инициализируется потокобезопасный `MemoryStorage` (или локальная директория с раздачей через `/api/modules/images/assets/{key}`), чтобы локальный запуск и unit-тесты работали без внешнего MinIO.
- **Статические примеры сценариев:** хранятся в `web/public/presets/{preset_id}_before.png` и `{preset_id}_after.png` (для `generate`-сценариев только `{preset_id}_after.png`), чтобы открываться в Mini App без задержек и не дублироваться для каждого пользователя в S3.

---

## 3. Модель данных и состояние FSM

### 3.1. Схема PostgreSQL (`00006_images_studio.sql`)
- **`image_projects`:**
  - `id BIGSERIAL PRIMARY KEY`
  - `user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE`
  - `mode VARCHAR(16) NOT NULL` (`generate` | `edit`)
  - `preset_id VARCHAR(64)` (nullable)
  - `title VARCHAR(255) NOT NULL`
  - `source_image_key TEXT` (ключ в S3, nullable)
  - `source_image_url TEXT` (URL в S3, nullable)
  - `current_image_key TEXT`
  - `current_image_url TEXT`
  - `current_version INT NOT NULL DEFAULT 0`
  - `image_model VARCHAR(128) NOT NULL DEFAULT ''`
  - `aspect_ratio VARCHAR(16) NOT NULL DEFAULT '1:1'`
  - `status VARCHAR(32) NOT NULL DEFAULT 'draft'` (`draft` | `processing` | `ready` | `failed`)
  - `error_message TEXT`
  - `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
  - `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
  - Индекс: `idx_image_projects_user_updated ON image_projects(user_id, updated_at DESC)`

- **`image_revisions`:**
  - `id BIGSERIAL PRIMARY KEY`
  - `project_id BIGINT NOT NULL REFERENCES image_projects(id) ON DELETE CASCADE`
  - `version INT NOT NULL`
  - `prompt TEXT NOT NULL`
  - `action_type VARCHAR(24) NOT NULL` (`generate` | `edit` | `refine` | `restore`)
  - `image_key TEXT NOT NULL`
  - `image_url TEXT NOT NULL`
  - `model VARCHAR(128) NOT NULL DEFAULT ''`
  - `pipeline_used VARCHAR(32) NOT NULL` (`images_generations` | `images_edits` | `vision_fallback` | `restore`)
  - `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
  - Уникальный индекс: `UNIQUE (project_id, version)`

### 3.2. Состояние Composite FSM (`user_mode_states`, ключ `(user_id, "images")`)
- `current_step`: `idle` | `preset_browse` | `awaiting_input` | `processing` | `ready`
- `payload`: `{"active_project_id": <int64>, "selected_preset_id": "<string>", "pending_mode": "generate|edit"}`
- Навигация в чате и Mini App синхронизирована через БД: проект, созданный в чате, сразу отображается в «Моих работах» Mini App, и наоборот.

---

## 4. Каталог предустановленных сценариев и AI-пайплайн

### 4.1. Реестр сценариев (`presets.go`)
| ID | Режим | Название | Что делает | Aspect Ratio |
|---|---|---|---|---|
| `bg_replace` | `edit` | Удалить / заменить фон | Выделяет главный объект и помещает его на заданный студийный или сюжетный фон | `1:1` |
| `product_card` | `edit` | Товарная карточка | Превращает обычное фото товара в коммерческий кадр со студийным светом и тенями | `1:1` |
| `retouch_light` | `edit` | Ретушь и свет | Выравнивает экспозицию, убирает шумы и улучшает цветокоррекцию без искажения деталей | `4:3` |
| `object_remove` | `edit` | Удаление объекта | Убирает лишний объект, человека или надпись по текстовой инструкции | `4:3` |
| `style_transfer` | `edit` | Стилизация фото | Переносит снимок в выбранный художественный стиль (3D, аниме, акварель, ретро) | `1:1` |
| `social_poster` | `generate` | Обложка для поста | Создаёт чистую яркую композицию для канала, статьи или анонса | `16:9` |
| `brand_logo` | `generate` | Логотип / иконка | Генерирует минималистичную эмблему или знак бренда на чистом фоне | `1:1` |
| `sticker_pack` | `generate` | Стикер / персонаж | Генерирует выразительного персонажа с чётким контуром на однотонном фоне | `1:1` |

Каждый сценарий в структуре `Preset` содержит:
- `ID`, `Mode`, `Title`, `Description`, `PromptTemplate`, `DefaultAspectRatio`
- `ExamplePrompt` — готовый запрос (например, для `product_card`: *«Поставь кроссовок на минималистичный бетонный подиум с мягким студийным светом»*)
- `ExampleBeforeURL` (для `edit`, например `/presets/product_card_before.png`)
- `ExampleAfterURL` (например `/presets/product_card_after.png`)

### 4.2. Гибридный AI-пайплайн (`ai_pipeline.go`)
1. **Вход `generate` (Text-to-Image):**
   - Пользовательский промпт объединяется с `PromptTemplate` выбранного сценария (если сценарий выбран).
   - Быстрая текстовая модель адаптирует/обогащает промпт (перевод на английский, добавление композиционных деталей при сохранении смысла).
   - Вызов `aiClient.GenerateImage(ctx, &ai.ImageRequest{Prompt, Model, AspectRatio})`.
   - `pipeline_used = "images_generations"`.
2. **Вход `edit` / `refine` (Image-to-Image):**
   - Исходные байты изображения читаются из S3 (`ObjectStorage.GetObject`) по `current_image_key` (для `refine`) или `source_image_key` (для первого `edit`).
   - **Primary:** вызов `aiClient.EditImage(ctx, &ai.ImageEditRequest{ImageBytes, Prompt, Model, AspectRatio})` → `POST /v1/images/edits` (multipart). `pipeline_used = "images_edits"`.
   - **Auto-Fallback:** если `EditImage` возвращает ошибку неподдерживаемого метода/модели (HTTP 404, 405, 501 или ошибка шлюза об отсутствии поддержки `/edits`), запускается фоллбэк:
     1. `aiClient.Vision` анализирует исходную картинку и формирует подробное описание объекта, композиции, цветов и ракурса.
     2. Текстовая модель объединяет Vision-описание с инструкцией пользователя в единый промпт.
     3. `aiClient.GenerateImage` создаёт обновлённое изображение. `pipeline_used = "vision_fallback"`.
3. **Сохранение результата:**
   - Декодирование результата (URL или base64) → загрузка байтов в S3 (`PutObject`) → запись новой строки в `image_revisions` → обновление `current_image_url`, `current_image_key`, `current_version`, `status = "ready"` в `image_projects`.
   - При сбое статус проекта становится `failed` с сохранением `error_message`, при этом предыдущая успешная ревизия (`current_image_url`) остаётся доступной для просмотра и отката.

---

## 5. Чат-интерфейс и REST API

### 5.1. Чат-флоу (`internal/modules/images/module.go`, префикс `img:`)
- **`idle`:** приветственное сообщение с кнопками:
  - `📚 Сценарии с примерами` (`img:presets`)
  - `🎨 Свободная генерация` (`img:mode:generate`)
  - `🪄 Исправить фото` (`img:mode:edit`)
  - `🗂 Мои работы` (`img:history`)
  - `📱 Открыть Фотостудию` (WebApp-кнопка в режим `images`)
- **`preset_browse`:** список сценариев по категориям (`img:cat:edit`, `img:cat:generate`), при выборе сценария (`img:preset:<id>`) отправляется картинка-пример и текст `ExamplePrompt` с кнопками:
  - `🚀 Попробовать пример` (`img:try:<id>`) — для `generate` сразу запускает генерацию по `ExamplePrompt`, для `edit` использует эталонное фото примера.
  - `✏️ Свой запрос / фото` (`img:use:<id>`) — переводит в `awaiting_input`.
- **`awaiting_input` и обработка фото:**
  - Получение фото в чате (в любом шаге режима `images`) скачивает файл через платформенный адаптер, проверяет формат/размер (до 10 МБ), загружает в S3, создаёт проект `edit` и либо применяет выбранный сценарий (если пользователь уже ввёл инструкцию), либо просит описать правку текстом.
- **`ready`:** отправляется готовая картинка с кнопками `↩️ Откатить версию` (`img:undo:<project_id>`), `📚 Другой сценарий` (`img:presets`), `📱 В студию`. Последующее текстовое сообщение пользователя в состоянии `ready` автоматически запускает `refine` активного проекта.

### 5.2. REST API (`internal/api/handlers_images.go`)
- `GET /api/modules/images/presets` — каталог сценариев с примерами.
- `GET /api/modules/images/models` — список доступных image-моделей из GMA.
- `GET /api/modules/images/projects` — список проектов текущего пользователя.
- `GET /api/modules/images/projects/{id}` — проект и его `revisions`.
- `POST /api/modules/images/projects` — создание проекта и запуск первой генерации/правки (поддерживает `multipart/form-data` для загрузки файла `source_image`, JSON с `use_example_source: true` для запуска примера, `mode`, `preset_id`, `prompt`, `aspect_ratio`, `image_model`).
- `POST /api/modules/images/projects/{id}/refine` — итеративная правка (`prompt`, опционально `image_model`).
- `POST /api/modules/images/projects/{id}/restore` — откат к указанной версии `version`.
- `DELETE /api/modules/images/projects/{id}` — удаление проекта (и связанных ключей в S3).

**Валидация и безопасность:**
- Все операции с `projects/{id}` проверяют `project.UserID == authUser.ID` (при несовпадении возвращается `403` / `404`).
- Загружаемые файлы проверяются по `http.DetectContentType` (разрешены `image/jpeg`, `image/png`, `image/webp`) и размеру (`<= 10 МБ`).

---

## 6. Mini App UI (`web/src/components/modes/ImagesView.tsx`)

- **Навигация в Hub:** 9-я карточка в `HubGrid` (`id: "images"`, `title: "Фотостудия"`).
- **Три вкладки внутри `ImagesView`:**
  1. **«Сценарии»:** фильтр (`Все` / `Исправление` / `Генерация`), сетка карточек сценариев. Каждая карточка показывает превью **До / После** (переключатель или сравнение для `edit`, результат для `generate`), пример промпта и кнопки **«Попробовать пример»** (1 тап) и **«Со своим фото / запросом»**.
  2. **«Студия»:** переключатель `Генерация` / `Исправление`, загрузка фото, поле промпта, селектор соотношения сторон (`1:1`, `16:9`, `9:16`, `4:3`) и модели, кнопка запуска. После генерации — просмотр результата, переключатель сравнения **«До / После»**, кнопка скачивания, строка быстрой итеративной правки и лента ревизий (`v1`, `v2`…) с кнопкой **«Вернуть версию»**.
  3. **«Мои работы»:** история проектов пользователя с превью, датой и быстрым переходом в «Студию» для доработки.

---

## 7. Стратегия тестирования

1. **`pkg/ai`:** unit-тест `EditImage` (`httptest.Server`, проверка multipart-полей `image`, `prompt`, `model` и парсинга ответа).
2. **`internal/storage`:** unit-тесты `MemoryStorage` (Put/Get/Delete, ошибки отсутствующего ключа) и парсинга S3-конфигурации.
3. **`internal/modules/images`:**
   - Тесты `ai_pipeline.go`: генерация, успешный `EditImage`, переключение на `vision_fallback` при HTTP 404/501 от `/edits`.
   - Тесты `store.go` и `service.go`: создание проекта, итеративный `refine`, откат `restore`, проверка `presets`, проверка ownership и квот.
   - Тесты `module.go`: обработка команд, загрузка фото, колбэки `img:`.
4. **`internal/api/handlers_images_test.go`:** тесты авторизации, 403/404 при доступе к чужому проекту, отклонение невалидного MIME/превышения 10 МБ, сквозной сценарий `create → refine → restore → delete`.
5. **`web/`:** проверка типов `npx tsc --noEmit` и сборки `npm run build`.
