# MAX Bot API (`dev.max.ru`) and Open-Source Ecosystem Research Report

> **Date:** November 2026  
> **Target System:** MAX Messenger Platform Bot API (`dev.max.ru`)  
> **Status:** Completed  
> **Output Path:** `docs/research/dev-max-ru-api-and-ecosystem.md`  

---

## 1. Executive Summary

MAX Messenger is an enterprise and consumer messaging platform and super-app ecosystem in the Russian Federation developed under VK. The developer portal at [dev.max.ru](https://dev.max.ru/docs/) exposes the official **MAX Bot API**, which enables automated chat-bots, interactive mini-apps, and enterprise integrations.

As of mid-2026, the Bot API has undergone critical architectural evolutions:
1. **Migration to `platform-api2.max.ru`:** The legacy endpoints (`botapi.max.ru` and `platform-api.max.ru`) were decommissioned on July 19, 2026. All API traffic is routed through `https://platform-api2.max.ru`, secured exclusively with Russian Trusted Root CA certificates (Минцифры РФ).
2. **Strict Header-Based Authentication:** Tokens can no longer be passed in URL query strings; bots must supply the raw access token in the standard `Authorization: <token>` HTTP header (without any `Bearer ` prefix).
3. **Addressing Nuances:** 1-on-1 private dialogues are addressed via `?user_id=...`, while group chats and channels are addressed via `?chat_id=...`. The deprecated `GET /chats` list method has been removed in favor of `POST /subscriptions`.
4. **Subscription Stacking Pitfall:** Webhooks do not overwrite existing registrations; calling `POST /subscriptions` registers an additional webhook, creating event duplication unless prior subscriptions are purged via `GET /subscriptions` and `DELETE /subscriptions`.
5. **Emerging Multi-Language Ecosystem:** While the official `max-messenger` organization on GitHub provides reference SDKs for Go, TypeScript, and Python, community developers have introduced high-level frameworks (e.g. `aiomax` in Python, `max-io` in TypeScript, `Max.BotClient` in .NET, and `max-bot-api-php` in PHP).

This report consolidates primary documentation, community postmortems, OpenAPI schemas, and cross-language library benchmarks to guide the implementation of `max-bot-api-client`.

---

## 2. Overview of MAX Platform & Developer Ecosystem

### 2.1 Platform Background & Verification Model
* **Developer Portal:** [https://dev.max.ru/docs/](https://dev.max.ru/docs/)
* **Parent Organization:** VK digital ecosystem.
* **Developer Registration & Verification:** Creating bots and obtaining production API tokens requires business verification through the **MAX для бизнеса** (MAX for Business) portal (`business.max.ru`) and Gosuslugi (Госуслуги) state authentication. Access is currently restricted to registered legal entities, individual entrepreneurs (ИП), and self-employed professionals residing in the Russian Federation.
* **Bot Profile Constraints:**
  * **Bot Display Name:** 1 to 59 characters (Latin, Cyrillic, numbers; emojis prohibited).
  * **Bot Username:** 11 to 60 characters, lowercase alphanumeric/underscore, mandatory `bot` or `_bot` suffix.
  * **Avatar:** Up to 5 MB, 500x500 px, 1:1 aspect ratio (JPG/PNG).
  * **Description:** Up to 200 characters.

### 2.2 Base URLs & Network/TLS Infrastructure
* **Active Base URL:** `https://platform-api2.max.ru`
* **Decommissioned URLs:**
  * `https://botapi.max.ru` (Decommissioned early 2026)
  * `https://platform-api.max.ru` (Let's Encrypt endpoint, decommissioned July 19, 2026)
* **TLS / CA Certificate Requirement:**  
  The `platform-api2.max.ru` domain is issued certificates through the Russian National Certification Authority (Минцифры / Russian Trusted Root CA). Standard Linux distributions (Ubuntu, Alpine, Debian) and runtime environments (Go, Node.js, Python, Docker images) that rely solely on Mozilla's NSS CA bundle will fail TLS handshakes with `x509: certificate signed by unknown authority`.  
  * **Client Requirement:** Any production client must either document the installation of Russian Root CA certificates into the system truststore (`ca-certificates`), bundle the root and intermediate CA certificates within the library, or allow users to inject a custom `*tls.Config` / CA cert pool.

### 2.3 Authentication Methods
* **API Requests:**
  * Requests must include the raw bot access token in the `Authorization` header:
    ```http
    Authorization: 9a8b7c6d5e4f3a2b1c0d...
    ```
  * *Important:* Do **not** prepend `Bearer `; the token must be transmitted as a raw string.
  * *Query parameters:* Passing tokens like `?token=...` or `?access_token=...` is completely disabled and returns `401 Unauthorized`.
* **Webhook Ingress Authentication:**
  * When configuring webhooks via `POST /subscriptions`, developers can supply a `"secret": "my-secret-string"`.
  * MAX delivers incoming webhook requests with the header:
    ```http
    X-Max-Bot-Api-Secret: my-secret-string
    ```
  * Ingress servers must validate this header using constant-time string comparison (e.g. `crypto/subtle.ConstantTimeCompare`).

---

## 3. Bot API Architecture & HTTP Endpoints

The MAX Bot API is a JSON-based RESTful API with HTTP verbs (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`).

### 3.1 Endpoint Reference Matrix

| Method | Endpoint | Query Parameters | Description |
|---|---|---|---|
| **Identity** | | | |
| `GET` | `/me` | None | Fetch authenticated bot profile (`user_id`, `name`, `username`, `is_bot`) |
| **Messages** | | | |
| `POST` | `/messages` | `user_id` OR `chat_id` | Send a new message (text, media, attachments, buttons) |
| `GET` | `/messages` | `chat_id` OR `message_ids` | Retrieve a list of messages from a chat |
| `GET` | `/messages/{messageId}` | None | Retrieve a single message by ID |
| `PUT` | `/messages` | `message_id` | Edit an existing message sent by the bot |
| `DELETE` | `/messages` | `message_id` | Delete a message (bot's own in 1-on-1, or any in group if admin) |
| **Interactive Callbacks** | | | |
| `POST` | `/answers` | `callback_id` | Answer an inline keyboard button click event |
| **Chats & Channels** | | | |
| `GET` | `/chats/{chatId}` | None | Get chat metadata (title, icon, type, participants count) |
| `PATCH` | `/chats/{chatId}` | None | Update chat information (title, icon, pinned message) |
| `GET` | `/chats/{chatId}/members` | `user_ids`, `marker`, `count` | List chat members with pagination and role filtering |
| `POST` | `/chats/{chatId}/members` | None | Add members to a chat (`user_ids`) |
| `DELETE` | `/chats/{chatId}/members` | `user_id` | Remove a member from a chat |
| `DELETE` | `/chats/{chatId}/members/me` | None | Bot leaves the chat |
| `GET` | `/chats/{chatId}/members/admins`| None | List chat administrators |
| `POST` | `/chats/{chatId}/members/admins`| None | Grant administrator rights to members |
| `PUT` | `/chats/{chatId}/pin` | None | Pin a message in a group chat or channel |
| `DELETE` | `/chats/{chatId}/pin` | None | Unpin the currently pinned message |
| `POST` | `/chats/{chatId}/actions` | None | Send a typing or upload action indicator (`typing_on`, etc.) |
| **Webhooks (Subscriptions)** | | | |
| `GET` | `/subscriptions` | None | List all currently active webhook subscriptions |
| `POST` | `/subscriptions` | None | Register a new webhook subscription |
| `DELETE` | `/subscriptions` | `url` | Unregister a webhook subscription by URL |
| **Long Polling** | | | |
| `GET` | `/updates` | `marker`, `timeout`, `limit`, `types` | Fetch event updates sequentially via long-polling |
| **Media Uploads** | | | |
| `POST` | `/uploads` | `type` (`image`, `video`, `audio`, `file`) | Initialize a file upload session and receive upload URL + token |

---

### 3.2 Detailed Method Specifications

#### 1. Identity: `GET /me`
```bash
curl -X GET "https://platform-api2.max.ru/me" \
     -H "Authorization: {token}"
```
**Response (200 OK):**
```json
{
  "user_id": 98765432,
  "first_name": "Support Bot",
  "name": "Support Bot",
  "username": "support_bot",
  "is_bot": true,
  "last_activity_time": 1718000000
}
```

#### 2. Sending Messages: `POST /messages`
* **Addressing Rule:**
  * For private 1-on-1 chats: `POST /messages?user_id=12345678`
  * For group chats and channels: `POST /messages?chat_id=87654321` (or negative IDs for groups)
* **Request Body (`NewMessageBody`):**
```json
{
  "text": "Hello, **world**! Check out [our docs](https://dev.max.ru).",
  "format": "markdown",
  "disable_link_preview": false,
  "attachments": [
    {
      "type": "inline_keyboard",
      "payload": {
        "buttons": [
          [
            {
              "type": "callback",
              "text": "Click Me",
              "payload": "btn_clicked_action"
            },
            {
              "type": "link",
              "text": "Visit Portal",
              "url": "https://dev.max.ru"
            }
          ]
        ]
      }
    }
  ]
}
```
**Response (200 OK):**
```json
{
  "message": {
    "message_id": "msg_01J9...",
    "sender": {
      "user_id": 98765432,
      "name": "Support Bot",
      "username": "support_bot",
      "is_bot": true
    },
    "recipient": {
      "chat_id": null,
      "user_id": 12345678
    },
    "timestamp": 1718000005,
    "body": {
      "text": "Hello, **world**!...",
      "attachments": [...]
    }
  }
}
```

#### 3. Editing & Deleting Messages: `PUT /messages` and `DELETE /messages`
* Both methods target the message via a query parameter `?message_id=...` rather than a path variable.
* **Editing:** `PUT https://platform-api2.max.ru/messages?message_id={message_id}`
  * Body: `{"text": "Updated text", "format": "markdown", "attachments": [...]}`
  * *Rules:* Messages containing `inline_keyboard` can be edited at any time. Plain messages in 1-on-1 chats can only be edited within **7 days** of delivery. In group chats, admins can edit messages at any time.
* **Deleting:** `DELETE https://platform-api2.max.ru/messages?message_id={message_id}`
  * *Rules:* In 1-on-1 chats, bots can only delete their own messages. In group chats/channels, an admin bot can delete any member's message.

#### 4. Callback Query Answers: `POST /answers`
When a user clicks an inline button with `"type": "callback"`, MAX dispatches a `message_callback` update with a unique `callback_id`. The bot must acknowledge it within seconds:
```bash
curl -X POST "https://platform-api2.max.ru/answers?callback_id={callback_id}" \
     -H "Authorization: {token}" \
     -H "Content-Type: application/json" \
     -d '{
       "message": {
         "text": "Action processed successfully!",
         "attachments": []
       },
       "notification": "Updated!"
     }'
```
* `message`: (Optional) Replaces the message containing the button.
* `notification`: (Optional) Single-shot toast notification / popup displayed to the user.

#### 5. Two-Step Media Uploads: `POST /uploads`
Uploading media (images, video, audio, files) uses a two-phase protocol:
1. **Initialize Session:**
   ```bash
   curl -X POST "https://platform-api2.max.ru/uploads?type=file" \
        -H "Authorization: {token}"
   ```
   **Response:**
   ```json
   {
     "url": "https://storage.max.ru/upload/session/...",
     "token": "file_tok_abc123456..."
   }
   ```
2. **Binary Upload:**
   * **Multipart:** Send `POST {url}` with multipart form data (`data=@filename.pdf`).
   * **Resumable Upload:** Send chunks via binary payload with byte offsets for files > 100 MB.
3. **Attach to Message:**
   Use the returned `token` in the message's `attachments` array:
   ```json
   {
     "attachments": [
       {
         "type": "file",
         "payload": {
           "token": "file_tok_abc123456..."
         }
       }
     ]
   }
   ```
   * *Token Reusability:* Upload tokens can be cached and reused across multiple messages and chats without re-uploading the file.
   * *Direct URL for Images:* Images can alternatively specify `"payload": {"url": "https://example.com/image.png"}` without going through `/uploads`.

---

## 4. Message Types, Keyboards & Deep Links

### 4.1 Message Formatting
* Supported formats: `markdown` or `html`.
* Max text length: **4,000 characters**.
* Markdown dialect supports standard bold (`**bold**`), italic (`*italic*`), code blocks (` ``` `), inline code (`` `code` ``), strikethrough (`~strike~`), and hyperlinks (`[text](url)`).

### 4.2 Attachments
Messages support a heterogeneous array of `attachments`. Supported types:
1. `image`: Max 50 MB, up to 7680x7680 px.
2. `video`: Max 250 MB.
3. `audio`: Audio tracks and voice messages.
4. `file`: Generic documents up to 4 GB.
5. `sticker`: Sticker attachment with pack ID and sticker code.
6. `contact`: VCard or structured contact (`name`, `phone`).
7. `location`: Coordinates (`latitude`, `longitude`).
8. `share`: Rich external link preview card.
9. `inline_keyboard`: Interactive button grid.

### 4.3 Inline Keyboard Specifications
The `inline_keyboard` attachment contains a 2D matrix of buttons (`Button[][]`).

#### Grid Constraints:
* **Max Total Buttons:** 210 buttons per keyboard.
* **Max Rows:** 30 rows.
* **Max Buttons Per Row (Standard Buttons):** Up to 7 buttons per row (for `callback`, `message`, `clipboard`).
* **Max Buttons Per Row (Special Buttons):** Up to 3 buttons per row (for `link`, `open_app`, `request_geo_location`, `request_contact`).

#### Supported Button Types:
| Button Type | Required Fields | Behavior |
|---|---|---|
| `callback` | `type`, `text`, `payload` | Dispatches `message_callback` update to the bot with the payload string. |
| `link` | `type`, `text`, `url` | Opens an external HTTPS URL in the client's browser (max 2048 chars). |
| `open_app` | `type`, `text`, `app_id`, `params` | Launches a MAX Mini App inside the messenger modal view. |
| `clipboard` | `type`, `text`, `data` | Copies the string in `data` to the user's OS clipboard. |
| `request_geo_location` | `type`, `text` | Prompts the user to share GPS coordinates with the bot. |
| `request_contact` | `type`, `text` | Prompts the user to share their verified phone number. |
| `message` | `type`, `text` | Sends the specified text directly as a message from the user. |

### 4.4 Deep Links
MAX supports contextual entry points through deep links:
1. **Bot Launch Deep Link:**
   ```
   https://max.ru/<bot_username>?start=<payload>
   ```
   * Payload limit: **128 characters** (alphanumeric, underscores, hyphens).
   * Generates a `bot_started` event containing the payload.
2. **Mini App Deep Link:**
   ```
   https://max.ru/<bot_username>?startapp=<payload>
   ```
   * Payload limit: **512 characters**.
   * Passed directly to the Mini App iframe context via `MAX Bridge`.

---

## 5. Event Handling & Webhooks

### 5.1 Event Delivery Models

MAX provides two mutually exclusive mechanisms for event ingestion:

1. **Long Polling (`GET /updates`) - Development & Debugging:**
   * Parameters:
     * `marker` *(int64, optional)*: Sequence offset of the last confirmed update. Passing `null` fetches latest; passing the last received marker commits prior events.
     * `timeout` *(int, 0–90, default 30)*: Server-side hold duration in seconds.
     * `limit` *(int, 1–1000, default 100)*: Batch size.
     * `types` *(string[], optional)*: Filter for specific update types.
   * Long-polling is strictly recommended for local development or sandbox environments.

2. **Webhooks (`POST /subscriptions`) - Production Deployment:**
   * Ingress must be an HTTPS server listening with a valid SSL/TLS certificate.
   * Supports `secret` token verification via `X-Max-Bot-Api-Secret`.

### 5.2 Critical Webhook Lifecycle Caveats

Community engineering postmortems (notably *"Бот в MAX молчит: четыре грабли Bot API"*) reveal several critical production quirks:

> [!CAUTION]
> **1. Subscription Stacking (No Automatic Overwrite):**  
> `POST /subscriptions` does **not** overwrite an existing webhook URL. It registers an additional subscription. If your CI/CD redeploys a container and calls `POST /subscriptions`, MAX will send duplicate POST requests for every event.  
> **Solution:** The client library must provide a robust `SetWebhook` workflow: call `GET /subscriptions`, iterate and call `DELETE /subscriptions` for existing endpoints, and then register the new URL.

> [!WARNING]
> **2. Eight-Hour Drop Policy:**  
> If a bot's webhook endpoint fails to return an HTTP `200 OK` (e.g. returns 5xx or times out) consistently over an **8-hour window**, the MAX platform automatically deletes the subscription without notification.  
> **Solution:** Bots should implement health monitoring and self-healing subscription renewal jobs.

> [!NOTE]
> **3. Multi-Tenant Payload Anonymity:**  
> The incoming `Update` JSON payload does not explicitly specify the receiving bot's `bot_id`. For multi-tenant ingress servers hosting hundreds of bots on one endpoint, route bots by distinctive URL paths (`/webhook/bot/{token}`) or secret keys.

### 5.3 Event Types (`update_type`)

| Event Name | Trigger Condition | Key Payload Fields |
|---|---|---|
| `message_created` | User sends a text, file, or media in 1-on-1, group, or channel. | `message` (`message_id`, `sender`, `recipient`, `body`) |
| `message_edited` | User or bot modifies an existing message. | `message` with updated `body` and `edited_at` |
| `message_removed` | A message is deleted. | `message_id`, `chat_id`, `user_id`, `timestamp` |
| `message_callback` | User clicks an inline `callback` button. | `callback_id`, `payload`, `user`, `message` |
| `bot_started` | User presses "Start" or enters via `?start=...` deep link. | `user`, `chat_id`, `payload` (nullable) |
| `bot_stopped` | User blocks or stops the bot in 1-on-1 dialogue. | `user`, `timestamp` |
| `bot_added` | Bot is added to a group chat or channel. | `chat_id`, `user` (who added the bot) |
| `bot_removed` | Bot is kicked/removed from a group chat or channel. | `chat_id`, `user` (who removed the bot) |
| `user_added` | A new member joins or is added to a group chat with the bot. | `chat_id`, `user`, `inviter` |
| `user_removed` | A member leaves or is kicked from a group chat. | `chat_id`, `user` |
| `chat_title_changed` | The title of a group chat or channel is edited. | `chat_id`, `title`, `user` |
| `dialog_removed` | User clears/deletes the entire conversation history. | `user_id`, `timestamp` |

---

## 6. Limits, Quotas & Error Handling

### 6.1 Rate Limits & Throttling
* **Method Rate Limits:**
  * Message sending (`POST /messages`): Recommended not to exceed **20–30 RPS** across the entire bot token.
  * Message editing (`PUT /messages`): Max **2 requests per second** per single chat/dialogue.
  * Message deletion (`DELETE /messages`): Max **2 requests per second** per single chat/dialogue.
  * Callback answers (`POST /answers`): Max **2 requests per second** per single chat/dialogue.
* **HTTP 429 Too Many Requests:**
  * Server responds with HTTP status 429 when rate limits are breached. Clients should implement exponential backoff with jitter and respect the `Retry-After` header.

### 6.2 File Size & Content Quotas
* **Image uploads:** 50 MB (formats: JPG, PNG, WEBP, GIF; dimensions: max 7680x7680 px).
* **Video uploads:** 250 MB (MP4, MOV).
* **Audio / Voice:** 100 MB.
* **Generic Documents:** 4 GB.
* **Message Text Length:** 4,000 UTF-8 characters.
* **Deep link payload:** 128 characters for bots; 512 characters for mini-apps.

### 6.3 Standard Error Payload
When an error occurs, `platform-api2.max.ru` returns a non-2xx HTTP code (400, 401, 403, 404, 429, 500) with a structured JSON body:
```json
{
  "code": "attachment.not.ready",
  "message": "Key: errors.process.attachment.file.not.processed",
  "description": "The file upload token provided has not completed asynchronous transcoding."
}
```
* **`code`** (string): Stable machine-readable error token (e.g. `bad.request`, `unauthorized`, `chat.not.found`, `message.too.long`, `rate.limit.exceeded`).
* **`message`** (string): System localization key or internal diagnosis.
* **`description`** (string, optional): Human-readable developer message.

---

## 7. Open-Source Ecosystem Analysis

The open-source community around MAX Messenger spans official reference implementations, community-driven frameworks, and enterprise connectors.

### 7.1 Cross-Language Ecosystem Overview

| Language | Repository / Package | Creator / Type | Maintenance / Status | Key Features & Architecture Patterns |
|---|---|---|---|---|
| **Go** | [`max-messenger/max-bot-api-client-go`](https://github.com/max-messenger/max-bot-api-client-go) | Official (`max-messenger`) | Active (v2) | Low-level HTTP client, typed models, basic long-polling, file upload helper. Minimalistic. |
| **Go** | [`max-messenger/maxbot`](https://github.com/max-messenger/maxbot) | Official (`max-messenger`) | Active | High-level bot framework built on `max-bot-api-client-go`. Includes routers, handlers, and FSM. |
| **TypeScript** | [`max-messenger/max-bot-api-client-ts`](https://github.com/max-messenger/max-bot-api-client-ts) | Official (`max-messenger`) | Active | Typed Axios/Fetch-based client, ESM/CJS dual-build, OpenAPI generated models. |
| **TypeScript** | [`xTCry/max-io`](https://github.com/xTCry/max-io) | Community (`xTCry`) | Very Active | Modern framework inspired by `grammY` / `telegraf` / `GramIO`. Typed context, middleware pipeline, scenes, sessions, NestJS module (`nestjs-max`). |
| **Python** | [`max-messenger/max-botapi-python`](https://github.com/max-messenger/max-botapi-python) (`maxapi`) | Official (`max-messenger`) | Active | Asyncio-based client, long-polling and webhook dispatchers. |
| **Python** | [`dpnspn/aiomax`](https://github.com/dpnspn/aiomax) (`aiomax` on PyPI) | Community (`dpnspn`) | Active | Modeled directly after `aiogram 3.x`. Routers, filters, FSM memory storage, aiohttp webhook server. |
| **Python** | [`green-api/maxbot-api-client-python`](https://github.com/green-api/maxbot-api-client-python) | Community / Commercial | Active | Green API integration wrapper for omnichannel support. |
| **PHP** | [`max-messenger-bot/max-bot-api-php`](https://github.com/max-messenger-bot/max-bot-api-php) | Community | Active | Object-oriented SDK, strict typing (PHP 8.2+), DTOs, Guzzle-based transport. |
| **PHP** | [`BushlanovDev/max-bot-api-client-php`](https://github.com/BushlanovDev/max-bot-api-client-php) | Community | Maintained | Simple wrapper with Laravel service provider and webhook controller. |
| **C# / .NET** | [`Max.BotClient`](https://www.nuget.org/packages/Max.BotClient) | Community | Active | .NET Standard 2.0 / .NET 10.0, builder pattern, HttpClient factory, Polly retry policies, DI extension. |
| **C# / .NET** | [`SaaSoft.MAX.Bot`](https://github.com/SaaSoft/SaaSoft.MAX.Bot) | Community | Maintained | ASP.NET Core webhook middleware, long-polling background worker. |
| **Schema** | [`max-messenger/api-schema`](https://github.com/max-messenger/api-schema) | Official (`max-messenger`) | Active | Official OpenAPI 3.0 specification for all REST endpoints and JSON schemas. |
| **Schema** | [`xTCry/max-dev-openapi-schema`](https://github.com/xTCry/max-dev-openapi-schema) | Community (`xTCry`) | Active | Community mirror tracking schema diffs, Swagger UI hosting, and JSON schema patches. |

---

### 7.2 Detailed Evaluation of Key Libraries

#### 1. Go: `max-messenger/max-bot-api-client-go/v2`
* **Architecture:** Traditional Golang SDK layout. A monolithic `Client` struct holding an `http.Client` and token, with methods corresponding 1:1 to API verbs (`SendMessage`, `GetUpdates`, `GetMe`, `AnswerCallback`).
* **Strengths:** Lightweight, zero unnecessary dependencies, official support.
* **Weaknesses:**
  * Lacks built-in handling of the Russian Root CA bundle; if running on Alpine Linux without extra setup, calls fail silently or return TLS errors.
  * Does not provide an automated webhook management loop (pruning old subscriptions before adding new ones).
  * Update unmarshaling is basic; discriminating polymorphic attachments requires manual type assertions.

#### 2. TypeScript: `xTCry/max-io`
* **Architecture:** Composable middleware architecture heavily inspired by modern Telegram bot frameworks like `grammY`. Every update creates a `Context` object with convenience methods (`ctx.reply()`, `ctx.answer()`).
* **Strengths:** Excellent developer experience, rich typings, scene and session management, first-class NestJS integration.
* **Weaknesses:** Higher abstraction overhead; tied to Node.js/TypeScript ecosystem.

#### 3. Python: `dpnspn/aiomax`
* **Architecture:** Mirrors `aiogram 3.x` architecture (`Dispatcher`, `Router`, `Filter`, `FSMContext`). Uses `pydantic` v2 for data parsing and validation.
* **Strengths:** Intuitive for developers migrating from Telegram bots, asynchronous I/O with `aiohttp`, rich state machine support.
* **Weaknesses:** Moderate maintenance velocity, dependency on pydantic v2 performance.

#### 4. .NET: `Max.BotClient`
* **Architecture:** Production-ready C# SDK utilizing modern .NET idioms (`IHttpClientFactory`, `IAsyncEnumerable` for updates, `Polly` resilience pipelines).
* **Strengths:** Robust handling of 429 rate-limiting with semaphore backoff; explicit documentation regarding Минцифры root CA setup on Windows/Linux servers.

---

## 8. Comparative Analysis: Telegram Bot API vs. MAX Bot API

Because most developers migrate or bridge bots from Telegram, understanding key architectural divergences is essential:

| Feature / Dimension | Telegram Bot API | MAX Bot API (`dev.max.ru`) |
|---|---|---|
| **Base URL** | `https://api.telegram.org/bot<token>/` | `https://platform-api2.max.ru` |
| **Auth Location** | Token in URL path | Raw token in `Authorization` header |
| **SSL / TLS Certificate** | Public CA (DigiCert / Let's Encrypt) | Russian Trusted CA (Минцифры РФ) |
| **1-on-1 Dialogue Target** | `chat_id` (identical to `user_id`) | `?user_id=...` (query parameter) |
| **Group Chat Target** | `chat_id` (negative integer) | `?chat_id=...` (query parameter) |
| **Webhook Registration** | `POST /setWebhook` (automatically replaces old) | `POST /subscriptions` (**appends**; creates duplicates if unpruned) |
| **Webhook Secret Header** | `X-Telegram-Bot-Api-Secret-Token` | `X-Max-Bot-Api-Secret` |
| **Webhook Failure Policy** | Backoff retry queue (retained for days) | Auto-deleted after 8 hours of 5xx / timeout |
| **Update Polling** | `GET /getUpdates` (`offset`) | `GET /updates` (`marker` int64) |
| **Callback Response** | `answerCallbackQuery` (toast/alert only) | `POST /answers` (can update message AND show notification simultaneously) |
| **Inline Keyboards** | Array of arrays of buttons | Array of arrays inside `attachments` element |
| **Message Length** | 4,096 characters | 4,000 characters |
| **File Uploading** | Multipart direct to endpoint | 2-step: `POST /uploads` to get URL+token, then upload binary |

---

## 9. Key Recommendations for `max-bot-api-client`

To build an idiomatic, resilient, production-ready client library in this repository (`max-bot-api-client`), the following architectural pillars should be implemented:

### 1. Robust TLS & Network Configuration
* **Минцифры Root CA Handling:**  
  Provide an optional built-in root CA certificate pool loader (e.g. `WithRussianTrustedCA()` option or automatic fallback) so developers can run in Docker containers (Alpine/Debian) without manual OS-level certificate installation.
* **Configurable Base URL:**  
  Default to `https://platform-api2.max.ru`, but allow overriding via functional options (essential for mock testing and enterprise gateways).

### 2. Dual Addressing Abstraction (`user_id` vs `chat_id`)
* Avoid forcing library users to remember whether an ID is a `user_id` or `chat_id`.
* Implement a unified recipient model:
  ```go
  type Recipient interface {
      Apply(req *http.Request)
  }
  func ToUser(userID int64) Recipient
  func ToChat(chatID int64) Recipient
  ```
* Provide high-level dispatchers: `client.SendMessage(ctx, ToUser(123), msg)` and `client.SendMessage(ctx, ToChat(456), msg)`.

### 3. Safe Webhook Registration (`SetWebhook` with Pruning)
* Prevent duplicate event delivery by wrapping webhook setup into an atomic operation:
  ```go
  func (c *Client) SetWebhook(ctx context.Context, config WebhookConfig) error {
      // 1. GET /subscriptions
      // 2. DELETE /subscriptions for existing matching/old URLs
      // 3. POST /subscriptions with new URL, secret, and update_types
  }
  ```
* Implement incoming webhook verification middleware validating `X-Max-Bot-Api-Secret` in constant time.

### 4. Resilient Long-Polling Engine
* Implement an idiomatic update channel runner:
  ```go
  func (c *Client) GetUpdatesChan(ctx context.Context, opts PollOptions) (<-chan Update, <-chan error)
  ```
* Automatically track and persist the `marker`, manage exponential backoff on network failures, and shut down gracefully on `ctx.Done()`.

### 5. Two-Step Media Upload Helper
* Abstract the two-step upload protocol behind clean stream/reader functions:
  ```go
  func (c *Client) UploadFile(ctx context.Context, mediaType MediaType, filename string, r io.Reader) (*UploadResult, error)
  ```
* Enable automatic attachment construction (`NewFileAttachment(token)` or `NewImageAttachmentURL(url)`).

### 6. Rate Limiting, Retries & Error Decoding
* Build an internal rate limiter (client-side token bucket or semaphore) respecting the 2 RPS limit for message edits, deletions, and answers.
* Wrap all HTTP responses in a strongly-typed `APIError` decoding `code`, `message`, and HTTP status code.

### 7. Comprehensive Model Serialization & Tests
* Provide strong typing for all 12 update types and 9 attachment variants.
* Build table-driven unit tests with mocked HTTP responses matching real MAX API JSON payloads (using `httptest.Server`).

---

## 10. Primary Sources & References

1. **Official MAX Developer Portal & Documentation:**
   * Documentation Index: [https://dev.max.ru/docs/](https://dev.max.ru/docs/)
   * Bot API Preparation & Guide: [https://dev.max.ru/docs/chatbots/bots-coding/prepare](https://dev.max.ru/docs/chatbots/bots-coding/prepare)
   * API Methods & Webhook Reference: [https://dev.max.ru/docs-api](https://dev.max.ru/docs-api)
   * Platform Legal & Requirement Rules: [https://dev.max.ru/docs/legal/rules](https://dev.max.ru/docs/legal/rules)
2. **Official Schemas & GitHub Repositories:**
   * OpenAPI Specification: [https://github.com/max-messenger/api-schema](https://github.com/max-messenger/api-schema)
   * Official Go Client: [https://github.com/max-messenger/max-bot-api-client-go](https://github.com/max-messenger/max-bot-api-client-go)
   * Official TypeScript Client: [https://github.com/max-messenger/max-bot-api-client-ts](https://github.com/max-messenger/max-bot-api-client-ts)
   * Official Python Client: [https://github.com/max-messenger/max-botapi-python](https://github.com/max-messenger/max-botapi-python)
3. **Community Frameworks & Open-Source Implementations:**
   * TypeScript `max-io` & OpenAPI Mirror: [https://github.com/xTCry/max-io](https://github.com/xTCry/max-io) and [https://github.com/xTCry/max-dev-openapi-schema](https://github.com/xTCry/max-dev-openapi-schema)
   * Python `aiomax` Framework: [https://github.com/dpnspn/aiomax](https://github.com/dpnspn/aiomax)
   * PHP Bot API SDK: [https://github.com/max-messenger-bot/max-bot-api-php](https://github.com/max-messenger-bot/max-bot-api-php)
   * .NET Bot SDK: [https://www.nuget.org/packages/Max.BotClient](https://www.nuget.org/packages/Max.BotClient)
4. **Community Engineering Postmortems & Guides (Habr):**
   * *«Бот в MAX молчит: четыре грабли Bot API, на которых я потерял неделю»* ([Habr](https://habr.com))
   * *«Переносим бота из Telegram в MAX: что отличается в API и как не переписывать логику»* ([Habr](https://habr.com))
   * *«Практическое махоботоводство в 2026 году»* ([Habr](https://habr.com))
