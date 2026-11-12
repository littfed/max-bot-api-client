# Отчет аудита: golang-observability

- **Дата:** 2026-11-12
- **Скилл:** `golang-observability`
- **Проект:** `max-bot-api-client`
- **Статус:** Выполнен

---

## 1. Концепция пяти сигналов наблюдаемости

Современная система наблюдаемости в Go опирается на 5 сигналов:
1. **Логи (Logs):** Структурированные события через `log/slog`.
2. **Метрики (Metrics):** Агрегированные счетчики, гистограммы задержек, экспорт в Prometheus.
3. **Трассировка (Traces):** Сквозной контекст запросов (OpenTelemetry).
4. **Профилирование (Profiles):** Анализ узких мест CPU и памяти через `pprof`.
5. **RUM (Real User Monitoring):** Замер пользовательского взаимодействия (клики, переходы, экраны).

---

## 2. Анализ реализации в репозитории

### 2.1 Структурированное логирование (`log/slog`)
- **Статус:** ✅ **Соответствует**
- Проект полностью переведен на официальный стандарт Go `log/slog`:
  - В [cmd/ai-bot/main.go](file:///C:/mimocode/max-bot-api-client/cmd/ai-bot/main.go#L33) сконфигурирован `slog.New(slog.NewTextHandler(...))`.
  - В middleware [pkg/bot/middleware.go](file:///C:/mimocode/max-bot-api-client/pkg/bot/middleware.go) обработчики запросов замеряют длительность и логируют события со структурированными полями: `duration`, `command`, `chat_id`, `user_id`.
  - Вектор утечки секретов: токен бота и ключи API скрываются или логируются только булевым флагом `has_bot_token`.

### 2.2 Метрики Prometheus (`pkg/telemetry/prometheus.go`)
- **Статус:** ✅ **Выдающаяся реализация (Zero-Dependency)**
- В модуле [pkg/telemetry/prometheus.go](file:///C:/mimocode/max-bot-api-client/pkg/telemetry/prometheus.go) реализован собственный экспортер метрик Prometheus Text-Based Exposition Format:
  - `maxbot_button_clicks_total`
  - `maxbot_command_duration_seconds`
  - `maxbot_api_call_duration_seconds`
  - `maxbot_errors_total`
  - `maxbot_active_users`
- Экспортер не зависит от тяжелого `github.com/prometheus/client_golang` и отдает корректный эндпоинт для сбора метрик Prometheus / Grafana.

### 2.3 Мониторинг пользовательского опыта (RUM)
- **Статус:** ✅ **Реализовано в `pkg/telemetry`**
- Пакет `pkg/telemetry` включает:
  - `tracked_keyboard.go`: Замер времени от показа кнопки до клика (Time-to-Click, TTK).
  - Обогащение внешних ссылок UTM-метками.
  - Кольцевой буфер событий `ringbuffer.go` для анализа недавних аномалий поведения пользователей.

### 2.4 Трассировка (Tracing) и профилирование (pprof)
- **Статус:** 💡 **Рекомендация для продакшна**
- **Трассировка:** Контекст `context.Context` проброшен через все уровни приложения. При необходимости интеграции с OpenTelemetry достаточно обернуть HTTP-клиент в OTel-транспорт.
- **pprof:** Для включения диагностического профилирования в `cmd/ai-bot` рекомендуется добавить опциональный запуск сервера pprof по флагу или переменной окружения `ENABLE_PPROF=true` (на внутреннем приватном порту, например `:6060`).

---

## 3. Резюме

Система наблюдаемости проекта находится на исключительно высоком уровне: встроенный структурированный логгер `slog`, кастомный zero-dependency Prometheus-экспортер и специализированный трекинг времени реакции пользователей (TTK) в `pkg/telemetry`.
