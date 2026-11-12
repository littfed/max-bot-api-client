# Audit Report: golang-error-handling

**Дата:** 2026-11-12  
**Репозиторий:** `github.com/littfed/max-bot-api-client`  
**Статус проверки:** ✅ Соответствует (PASS)  

---

## 1. Обзор требований стандарта `golang-error-handling`

- **Проверка всех ошибок:** ошибки не должны отбрасываться через `_` без веской причины;
- **Оборачивание контекстом:** использование `fmt.Errorf("...: %w", err)` для сохранения цепочки ошибок (`errors.Is` / `errors.As`);
- **Форматирование сообщений:** ошибки с маленькой буквы, без точки на конце;
- **Идиома единой обработки (Single Handling Rule):** ошибку нужно либо залогировать, либо вернуть наверх, но не делать и то, и другое одновременно;
- **Сентинельные ошибки и кастомные типы:** выделение `ErrNotFound`, `APIError` и типизированный разбор;
- **Структурированное логирование:** использование `log/slog` вместо `fmt.Println` или неструктурированного `log.Printf`.

---

## 2. Результаты анализа кодовой базы

### 2.1. Иерархия ошибок и типизированные структуры
- В [`pkg/maxbot/errors.go`](file:///C:/mimocode/max-bot-api-client/pkg/maxbot/errors.go) реализован каноничный доменный тип `APIError`:
  ```go
  type APIError struct {
      StatusCode int
      ErrorCode  string
      Message    string
  }
  func (e *APIError) Error() string { ... }
  ```
- Реализованы сентинельные ошибки верхнего уровня (`ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrTooManyRequests`, `ErrServer`), поддерживающие проверку через `errors.Is`.

### 2.2. Качественное оборачивание цепочки ошибок (`%w`)
- Во всех методах клиента API ([`pkg/maxbot/client.go`](file:///C:/mimocode/max-bot-api-client/pkg/maxbot/client.go)) и клиента шлюза ([`pkg/gma/client.go`](file:///C:/mimocode/max-bot-api-client/pkg/gma/client.go)) ошибки сетевого взаимодействия и сериализации JSON оборачиваются через спецификатор `%w`:
  ```go
  if err != nil {
      return fmt.Errorf("failed to create http request: %w", err)
  }
  ```
  Это позволяет вызывающему коду инспектировать корень проблемы через `errors.Unwrap` и `errors.Is`.

### 2.3. Идиома единой обработки и `log/slog`
- В [`pkg/bot/bot.go`](file:///C:/mimocode/max-bot-api-client/pkg/bot/bot.go) и [`pkg/bot/middleware.go`](file:///C:/mimocode/max-bot-api-client/pkg/bot/middleware.go) обработка ошибок построена по принципу единой точки:
  - Ошибки из обработчиков событий возвращаются на уровень роутера `bot.Router`.
  - Middleware `Logger(logger)` фиксирует ошибку один раз со всеми структурированными атрибутами (`slog.String("event", ...)`, `slog.Any("error", err)`).
- Проект полностью перешел на стандартную библиотеку `log/slog` (Go 1.21+), устранив неструктурированные вызовы стандартного пакета `log`.

### 2.4. Форматирование текста ошибок
- Все константы и строки ошибок начинаются со строчной буквы и не содержат замыкающих знаков препинания:
  - `"bot token cannot be empty"`
  - `"chat completion request cannot be nil"`
  - `"download returned HTTP status %d"`

---

## 3. Рекомендации
1. В Go 1.26+ в хэндлерах можно задействовать дженерик-хелпер `errors.AsType[T](err)` вместо связки `var target *maxbot.APIError; errors.As(err, &target)`.

---

## 4. Итоговая оценка
- **Оценка:** 10 / 10
- **Вердикт:** Обработка ошибок в проекте полностью соответствует лучшим практикам современного Go.
