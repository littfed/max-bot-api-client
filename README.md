# max-bot-api-client

[![Go Reference](https://pkg.go.dev/badge/github.com/littfed/max-bot-api-client.svg)](https://pkg.go.dev/github.com/littfed/max-bot-api-client)
[![Go Report Card](https://goreportcard.com/badge/github.com/littfed/max-bot-api-client)](https://goreportcard.com/report/github.com/littfed/max-bot-api-client)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Идиоматичная, типизированная и быстрая клиентская библиотека на **Go (Golang)** для разработки чат-ботов на платформе **MAX Messenger** ([dev.max.ru](https://dev.max.ru/docs/)).

---

## ⚡ Особенности библиотеки

* **Сертификаты Минцифры РФ «из коробки» (`Zero-Config TLS`):**  
  Официальный корневой и промежуточный сертификаты Национального удостоверяющего центра встроены прямо в бинарник через `//go:embed`. Библиотека работает без ошибок `x509: certificate signed by unknown authority` на любой ОС и в любых минималистичных Docker-контейнерах (`scratch`, `alpine`, `distroless`) без необходимости настраивать системный truststore.
* **Двухуровневая архитектура:**
  1. `pkg/maxbot` — строго типизированный низкоуровневый REST API клиент (`platform-api2.max.ru`), модели данных, API-ошибки и Fluent Builder для клавиатур.
  2. `pkg/bot` — высокоуровневый движок бота: цикл Long Polling с автоинкрементом маркера, роутинг команд (`/ping`), фильтрация текста, обработка инлайн-колбэков и middleware (Recovery, Logger).
* **Умная адресация `Recipient`:**  
  Скрывает разницу между отправкой в личные сообщения (`?user_id=...`) и групповые чаты (`?chat_id=...`). В хэндлере метод `c.Reply("pong")` автоматически отправляет ответ в нужный контекст.
* **Fluent Builder интерактивных клавиатур:**  
  Удобное создание кнопок `callback`, `link`, `open_app` (Mini Apps), `clipboard` (копирование реферальной ссылки), `request_contact` (телефон) и `request_geo_location` (GPS).
* **Сквозная телеметрия и трекинг кнопок (`pkg/telemetry`):**  
  Автоматическое отслеживание кликов по кнопкам с замером времени отклика пользователя (Time-to-Click, TTK), обогащение внешних ссылок UTM-метками, кольцевой буфер событий в памяти и встроенный Prometheus / OpenMetrics экспортер (`/metrics`).
* **Конечные автоматы и сессии диалогов (`pkg/fsm`):**  
  Управление состояниями пользователей, изоляция сессий (`SessionKey`), пошаговые сценарии (`Wizard`) с поддержкой возврата («назад»), отмены и автоистечения таймаутов, а также постоянные хранилища (`MemoryStorage`, `SQLStorage`, `RedisStorage`).
* **Электронная коммерция и платежи СБП (`pkg/payments`):**  
  Генерация диплинков Системы Быстрых Платежей (`https://qr.nspk.ru/...`), загрузка динамических QR-кодов в MAX, маршрутизатор вебхуков с криптографической валидацией подписи Т-Банк (SHA-256) и ЮKassa.
* **Модерация каналов и антиспам-гейт (`pkg/guard`):**  
  Управление участниками (`GetChatMembers`, `KickChatMember`, `LeaveChat`), интерактивная капча («Я человек») для новых участников с автокиком спам-ботов по таймауту и планировщик отложенных постов (`PostScheduler`).
* **Экосистема Mini Apps & WebBridge (`pkg/webapp`):**  
  Генерация кнопок `open_app` со строгой валидацией слаг-имен (защита от 404 LinkPK), криптографическая проверка HMAC-SHA256 подписей `initData` и двусторонний HTTP-мост для приема заказов из WebView.
* **Поддержка Deep Linking & Referral:**  
  Парсинг стартовых параметров из событий `bot_started` и команд `/start <payload>`.

---

## 🌟 Флагманский Бот-Витрина (Showcase Bot)

В репозитории реализован полноценный интерактивный бот-витрина [`cmd/showcase-bot`](docs/showcase-bot.md), наглядно демонстрирующий все ключевые сценарии платформы в одном приложении:

1. **🛍️ Интернет-магазин & СБП:** FSM-визард заказа, генерация реальной ссылки СБП (НСПК) и кнопка «🧪 Симулировать оплату (Sandbox)».
2. **📅 Онлайн-запись на услуги:** календарь слотов, системный запрос телефона (`request_contact`) и подтверждение бронирования.
3. **🧠 Интерактивный Квиз:** викторина, замер реакции пользователя (Time-to-Click) и копирование результата в буфер обмена (`clipboard`).
4. **📱 Mini App & WebBridge:** встроенное веб-приложение (HTML5/JS на `:8080/webapp`), `open_app` запуск и валидация HMAC-SHA256 подписи.
5. **🛡️ Модерация комьюнити:** капча-гейт «Я человек» с 30-секундным таймером, защита от спам-ботов и автокик.
6. **🔍 Режим «Под капотом» (`/dev`):** детальные карточки с API-методами и JSON-пейлоадами на каждом шаге.

### Запуск шоурума одной командой:

```bash
# Локальный демонстрационный режим (веб-сервер на http://localhost:8080):
go run ./cmd/showcase-bot

# Боевой запуск в MAX Messenger с вашим токеном:
go run ./cmd/showcase-bot -token ВАШ_ТОКЕН_БОТА
```

Подробное руководство читайте в документе [docs/showcase-bot.md](docs/showcase-bot.md).

---

## 🚀 Быстрый старт: Ping-Pong бот

### 1. Установка

```bash
go get github.com/littfed/max-bot-api-client
```

### 2. Запуск минимального бота для проверки связи

Создайте файл `main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func main() {
	token := os.Getenv("MAX_BOT_TOKEN")
	if token == "" {
		log.Fatal("Задайте переменную окружения MAX_BOT_TOKEN")
	}

	// Инициализация бота (сертификаты Минцифры подключаются автоматически)
	b, err := bot.New(token)
	if err != nil {
		log.Fatalf("Ошибка создания бота: %v", err)
	}

	// Обработка /start и события bot_started
	b.OnStart(func(c *bot.Context) error {
		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("🏓 Нажми меня", "btn_ping"))

		_, err := c.ReplyKeyboard("👋 Привет! Я Ping-Pong бот в MAX.", kb)
		return err
	})

	// Обработка команды /ping
	b.OnCommand("ping", func(c *bot.Context) error {
		_, err := c.Reply("pong! 🏓")
		return err
	})

	// Обработка текстового сообщения "ping"
	b.OnText("ping", func(c *bot.Context) error {
		_, err := c.Reply("pong! 🏓")
		return err
	})

	// Обработка нажатия на инлайн-кнопку
	b.OnCallbackPayload("btn_ping", func(c *bot.Context) error {
		_ = c.AnswerNotification("Понг! 🏓")
		_, err := c.Reply("Вы нажали кнопку: **pong!**")
		return err
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Println("🚀 Бот запущен в режиме Long Polling...")
	if err := b.StartLongPolling(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Ошибка: %v", err)
	}
}
```

### 3. Запуск готового примера

В репозитории уже есть готовый пример с поддержкой `.env` и флагов:

```bash
# Через флаг
go run ./examples/01_ping_pong -token ВАШ_ТОКЕН_БОТА

# Или через переменную окружения
export MAX_BOT_TOKEN="ВАШ_ТОКЕН_БОТА"
go run ./examples/01_ping_pong
```

---

## 📂 Структура проекта

```
max-bot-api-client/
├── certs/
│   ├── certs.go                  # go:embed бандла Минцифры и настройка *tls.Config
│   └── mincifry_bundle.pem       # Root & Sub CA сертификаты Минцифры РФ
├── pkg/
│   ├── maxbot/                   # Низкоуровневый REST API клиент
│   │   ├── client.go             # GetMe, SendMessage, GetUpdates, AnswerCallback...
│   │   ├── models.go             # User, Chat, Message, Update, Callback...
│   │   ├── recipient.go          # ToUser(id), ToChat(id), Recipient
│   │   ├── keyboards.go          # Fluent Builder для InlineKeyboard
│   │   └── errors.go             # Типизированные ошибки APIError (401, 429, 404)
│   ├── bot/                      # Высокоуровневый движок ботов
│   │   ├── bot.go                # Long Polling цикл, WebhookHandler
│   │   ├── context.go            # Helper-контекст: Reply, ReplyKeyboard, Command...
│   │   ├── router.go             # Роутер хэндлеров по командам, тексту и колбэкам
│   │   └── middleware.go         # Recovery, Logger
│   ├── telemetry/                # Сквозная телеметрия и трекинг кнопок (Prometheus экспортер)
│   ├── fsm/                      # Сессии и FSM: Memory, SQLite/SQL, Redis, Wizard-навигатор
│   ├── payments/                 # Платежи СБП (НСПК), шлюзы Т-Банк / ЮKassa, роутер вебхуков
│   ├── guard/                    # Модерация участников, интерактивная капча, автопостинг
│   └── webapp/                   # Mini Apps: валидация open_app, HMAC-SHA256 initData, WebBridge
├── internal/
│   └── livetest/                 # Модуль live-тестирования ботов (Suite, Matcher, HTML)
├── cmd/
│   ├── showcase-bot/             # 🌟 Флагманский бот-витрина всех возможностей MAX
│   └── bot-tester/               # CLI раннер с красивой таблицей и скриншотами
├── examples/
│   ├── 01_ping_pong/             # Минимальный Ping-Pong бот
│   ├── 02_feature_tour/          # Интерактивный тур по возможностям MAX API
│   ├── 03_telemetry_tour/        # Демонстрация трекинга воронки и Prometheus метрик
│   └── 04_advanced_features_tour/# Продвинутый интернет-магазин: FSM визард + СБП + Mini App
└── docs/research/                # Документация и детальные исследования платформы
```

---

## 🛠 Тестирование

### 1. Модульные тесты (Unit Tests)

Все пакеты покрыты тестами с использованием мок-серверов и валидацией живого TLS-соединения с `platform-api2.max.ru`:

```bash
go test -v ./...
```

### 2. Live-тестирование («по живому») и CLI-раннер

В репозитории реализован полноценный модуль live-тестирования `internal/livetest` и консольная утилита `cmd/bot-tester` для сквозной проверки бота против боевого API MAX:

* **Проверка API:** `GetMe`, регистрация меню команд `SetCommands`, 2-этапная загрузка медиа `UploadMedia` в `iu.oneme.ru`.
* **Жизненный цикл сообщений:** Отправка сообщений, пин/анпин, редактирование текста и гарантированное удаление (cleanup) без мусора в чате.
* **Human-in-the-Loop (HITL):** Интерактивная проверка кнопок разных цветов (`positive`, `negative`, `primary`) в реальном диалоге с замером задержки (RTT).
* **Визуальные отчеты и скриншоты:** Генерация Markdown-сводки (`reports/live_test_*.md`), автономного HTML-чата в дизайне веб-клиента MAX (`reports/live_test_*.html`) и автоматических PNG-скриншотов (`reports/screenshots/chat_*.png`).

#### Запуск через CLI раннер:

```bash
# Автономный прогон с генерацией HTML-отчета и PNG-скриншота
go run cmd/bot-tester/main.go -interactive=false

# Полный интерактивный прогон с участием человека (отправка кнопок в MAX)
go run cmd/bot-tester/main.go -interactive=true -user-id=396393759
```

#### Запуск через `go test`:

```bash
go test -v -tags=live ./internal/livetest/...
```

---

## 📄 Лицензия

MIT License.
