# Audit Report: golang-naming

**Дата:** 2026-11-12  
**Репозиторий:** `github.com/littfed/max-bot-api-client`  
**Статус проверки:** ✅ Соответствует (PASS)  

---

## 1. Обзор требований стандарта `golang-naming`

- **Форматирование идентификаторов:** Строго `MixedCaps` (UpperCamelCase для экспортируемых, lowerCamelCase для внутренних), отсутствие snake_case и ALL_CAPS в константах;
- **Отсутствие повторения имени пакета («Anti-stuttering»):** `maxbot.Client` вместо `maxbot.MaxBotClient`, `gma.Client` вместо `gma.GMAClient`;
- **Именование акронимов:** Единообразный регистр для аббревиатур (`URL`, `HTTP`, `API`, `TTL`, `OCR`);
- **Именование ресиверов методов:** 1–2 буквы (`c`, `b`, `sm`, `m`);
- **Именование функциональных опций:** Префикс `With...` (`WithLogger`, `WithTimeout`).

---

## 2. Результаты анализа кодовой базы

### 2.1. Исключение повторений (Anti-stuttering)
- В пакетах выдержана образцовая лаконичность на стороне вызова:
  - `maxbot.Client` (а не `maxbot.MaxbotClient`)
  - `bot.Bot` и `bot.NewWithClient`
  - `gma.Client` (а не `gma.GMAClient`)
  - `telemetry.Collector` (а не `telemetry.TelemetryCollector`)
  - `payments.Router` (а не `payments.PaymentRouter`)
  - `fsm.MemoryStorage` (а не `fsm.FSMMemoryStorage`)

### 2.2. Константы и регистр акронимов
- Все аббревиатуры согласованы:
  - `ImageURL`, `DefaultBaseURL` (не `ImageUrl` / `image_url`)
  - `APIError`, `adminAPIKey`
  - `DefaultSessionTTL`
- Константы оформлены в `MixedCaps`:
  - `DefaultChatModel`, `DefaultVisionModel`, `DefaultImageModel` (вместо C-стиля `DEFAULT_CHAT_MODEL`).

### 2.3. Ресиверы и ошибки
- Ресиверы методов коротки и отражают тип:
  - `func (c *Client)` в клиентах;
  - `func (c *Context)` в контексте запроса;
  - `func (sm *SessionManager)` в менеджере сессий;
  - `func (b *Bot)` в движке бота.
- Именование ошибок строго разделено:
  - Переменные ошибок: префикс `Err...` (`ErrUnauthorized`, `ErrNotFound`, `ErrTooManyRequests`).
  - Типы ошибок: суффикс `...Error` (`APIError`).

---

## 3. Рекомендации
1. В `pkg/gma/models.go` поле `B64JSON` соответствует общепринятому соглашению, однако альтернативно допустимо `Base64JSON` для максимальной читаемости.

---

## 4. Итоговая оценка
- **Оценка:** 10 / 10
- **Вердикт:** Именование элементов библиотеки идеально вписывается в идиомы экосистемы Go.
