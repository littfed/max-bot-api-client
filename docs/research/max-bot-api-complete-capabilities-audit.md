# MAX Messenger Bot API: Complete Platform Capabilities & Architectural Audit

> **Document Version:** 2.0.0 (Comprehensive Platform Audit)  
> **Target Platform:** MAX Messenger (`dev.max.ru`)  
> **Production Base URL:** `https://platform-api2.max.ru`  
> **Target Reference Schema:** `max-messenger/api-schema` & `xTCry/max-dev-openapi-schema`  
> **SDK References:** `max-messenger/max-bot-api-client-go`, `max-messenger/max-bot-api-client-ts`, `max-messenger/maxbot`  
> **Output Path:** `docs/research/max-bot-api-complete-capabilities-audit.md`  

---

## 1. Executive Overview & Primary Source Inventory

MAX Messenger is an enterprise, government, and consumer messaging super-app platform developed in the Russian Federation under the VK ecosystem. The developer portal at [dev.max.ru](https://dev.max.ru/docs/) provides the official **MAX Bot API**, which allows automated services, corporate backends, mini-apps, and AI agents to converse with users, manage communities, and streamline business processes.

### 1.1 Primary Sources Analyzed

1. **Official Portal Documentation:** `https://dev.max.ru/docs/` (Platform architecture, guides, changelog).
2. **Official OpenAPI / Swagger Specification:**
   - Repository: [`max-messenger/api-schema`](https://github.com/max-messenger/api-schema) (`schema.yaml`).
   - Community Mirror & Diff Tracker: [`xTCry/max-dev-openapi-schema`](https://github.com/xTCry/max-dev-openapi-schema).
3. **Official Client Implementations:**
   - Go: [`max-messenger/max-bot-api-client-go`](https://github.com/max-messenger/max-bot-api-client-go) (v2) & [`max-messenger/maxbot`](https://github.com/max-messenger/maxbot) framework.
   - TypeScript: [`max-messenger/max-bot-api-client-ts`](https://github.com/max-messenger/max-bot-api-client-ts) & [`xTCry/max-io`](https://github.com/xTCry/max-io).
   - Python: [`max-messenger/max-botapi-python`](https://github.com/max-messenger/max-botapi-python) & [`dpnspn/aiomax`](https://github.com/dpnspn/aiomax).
4. **Platform Infrastructure & Verification:**
   - Business Portal: `business.max.ru` (Госуслуги / Gosuslugi verification).
   - Production API Gateway: `https://platform-api2.max.ru` (Russian Trusted Root CA / Минцифры РФ).

---

## 2. Platform Authentication & Transport Layer

### 2.1 The 2026 Gateway Architecture & Cutover
* **Active Gateway:** `https://platform-api2.max.ru`
* **Decommissioned Endpoints:**
  * `https://botapi.max.ru` (Decommissioned early 2026).
  * `https://platform-api.max.ru` (Decommissioned July 19, 2026).
* **TLS Certificate Authority:**
  Certificates on `platform-api2.max.ru` are issued strictly by the Russian National Certification Authority (Головной Удостоверяющий Центр / Минцифры РФ). Standard public truststores (Mozilla NSS in Debian, Alpine, Ubuntu, macOS, Windows) will reject TLS handshakes with `x509: certificate signed by unknown authority`.  
  *All clients must bundle the Russian Root and Intermediate CA certificates (`mincifry_bundle.pem`) or explicitly configure custom TLS root certificate pools.*

### 2.2 Outbound Authentication (Bot to Platform)
* Authentication is handled via the HTTP `Authorization` request header:
  ```http
  Authorization: 9a8b7c6d5e4f3a2b1c0d...
  ```
* **Strict Rule:** The header contains the **raw token string**. Do **not** prepend `Bearer `.
* **Query Parameter Prohibition:** Passing access tokens via URL query parameters (`?access_token=...` or `?token=...`) was decommissioned and returns `HTTP 401 Unauthorized` (`{"code": "verify.token", "message": "No access token"}`).

### 2.3 Inbound Authentication (Platform to Webhook Ingress)
* When configuring a subscription via `POST /subscriptions`, the developer provides a `"secret": "secret_string"` parameter.
* MAX delivers incoming webhook requests with the header:
  ```http
  X-Max-Bot-Api-Secret: secret_string
  ```
* The receiving webhook server must validate this header using constant-time string comparison (`crypto/subtle.ConstantTimeCompare`) before parsing the payload.

---

## 3. What the Bot Can DO (All Outbound Actions & Endpoints)

The MAX Bot API is a JSON REST API utilizing standard HTTP methods (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`).

### 3.1 Comprehensive Endpoint Reference Matrix

| Category | Method | Endpoint | Primary Parameters | Description |
|---|---|---|---|---|
| **Identity** | `GET` | `/me` | None | Fetch authenticated bot profile, username, name, avatar, and commands |
| **Commands** | `PATCH` | `/me/commands` | Body: `commands[]` | Update or reset the bot's suggestion menu commands (max 32 commands) |
| **Messaging** | `POST` | `/messages` | `user_id` OR `chat_id` (Query) | Send text, media attachments, link forwards/replies, inline keyboards |
| **History** | `GET` | `/messages` | `chat_id` OR `message_ids`, `count`, `from`, `to` | Retrieve message history from group chat or channel |
| **Message** | `GET` | `/messages/{messageId}` | `messageId` (Path) | Fetch a specific message by its unique ID |
| **Edit** | `PUT` | `/messages` | `message_id` (Query), Body | Edit text, formatting, attachments, or links of an existing bot message |
| **Delete** | `DELETE` | `/messages` | `message_id` (Query) | Delete a message (bot's own in 1-on-1, or any message if group admin) |
| **Presence** | `POST` | `/chats/{chatId}/actions`| `chatId` (Path), Body: `action` | Dispatch presence status (`typing_on`, `sending_photo`, `sending_video`, etc.) |
| **Pin** | `GET` | `/chats/{chatId}/pin` | `chatId` (Path) | Retrieve the currently pinned message in a chat or channel |
| **Pin** | `PUT` | `/chats/{chatId}/pin` | `chatId` (Path), Body: `message_id`, `notify` | Pin a specific message in a chat or channel |
| **Pin** | `DELETE`| `/chats/{chatId}/pin` | `chatId` (Path) | Unpin the pinned message in a chat or channel |
| **Chat Meta** | `GET` | `/chats/{chatId}` | `chatId` (Path) | Retrieve chat metadata (title, icon, type, participants count, description) |
| **Chat Meta** | `PATCH` | `/chats/{chatId}` | `chatId` (Path), Body | Update chat title, icon URL, pinned message, description, notify flag |
| **Members** | `GET` | `/chats/{chatId}/members` | `chatId` (Path), `marker`, `count` | Paginated list of chat participants and their roles |
| **Members** | `POST` | `/chats/{chatId}/members` | `chatId` (Path), Body: `user_ids[]` | Add members to a group chat |
| **Members** | `DELETE`| `/chats/{chatId}/members` | `chatId` (Path), `user_id` (Query) | Kick a user from a group chat |
| **Leave** | `DELETE`| `/chats/{chatId}/members/me` | `chatId` (Path) | Bot voluntarily leaves the chat |
| **Admins** | `GET` | `/chats/{chatId}/members/admins`| `chatId` (Path) | List all administrators and their permission masks |
| **Admins** | `POST` | `/chats/{chatId}/members/admins`| `chatId` (Path), Body: `admins[]` | Grant/modify admin permissions and custom titles (`alias`) |
| **Admins** | `DELETE`| `/chats/{chatId}/members/admins/{userId}`| `chatId`, `userId` (Path) | Demote an administrator back to a standard participant |
| **Comments** | `POST` | `/messages/{messageId}/comments` | `messageId` (Path), Body | Post a comment under a channel publication |
| **Comments** | `GET` | `/messages/{messageId}/comments` | `messageId` (Path), `count`, `marker` | Retrieve comments under a specific channel publication |
| **Comments** | `GET` | `/messages/{messageId}/comments/{commentId}` | `messageId`, `commentId` (Path) | Retrieve a single comment by ID |
| **Comments** | `PUT` | `/messages/{messageId}/comments` | `commentId` (Query), Body | Edit a previously posted comment |
| **Comments** | `DELETE`| `/messages/{messageId}/comments` | `commentId` (Query) | Delete a comment |
| **Callbacks** | `POST` | `/answers` | `callback_id` (Query), Body | Respond to inline button callback (toast alert and/or message edit) |
| **Uploads** | `POST` | `/uploads` | `type` (`image`, `video`, `audio`, `file`) | Obtain presigned upload URL and attachment token |
| **Webhooks** | `GET` | `/subscriptions` | None | List all currently active webhook registrations |
| **Webhooks** | `POST` | `/subscriptions` | Body: `url`, `secret`, `update_types[]` | Register a new webhook endpoint |
| **Webhooks** | `DELETE`| `/subscriptions` | `url` (Query) | Unregister a webhook URL |
| **Long Poll** | `GET` | `/updates` | `marker`, `limit`, `timeout`, `types` | Fetch events sequentially via HTTP long polling |

---

### 3.2 Detailed Method Specifications & Payloads

#### 1. Identity & Commands: `GET /me` and `PATCH /me/commands`

##### Fetch Profile: `GET /me`
```bash
curl -X GET "https://platform-api2.max.ru/me" \
     -H "Authorization: {token}"
```
**Response (200 OK):**
```json
{
  "user_id": 98765432,
  "first_name": "Delivery Bot",
  "name": "Delivery Bot",
  "username": "delivery_service_bot",
  "is_bot": true,
  "avatar_url": "https://storage.max.ru/avatars/bot_98765432.png",
  "commands": [
    {
      "command": "start",
      "description": "Start dialogue with bot"
    },
    {
      "command": "order",
      "description": "Create a new order"
    }
  ]
}
```

##### Configure Bot Commands Menu: `PATCH /me/commands`
Sets the command suggestion list shown in the messenger's input field menu (supports up to 32 commands).
```bash
curl -X PATCH "https://platform-api2.max.ru/me/commands" \
     -H "Authorization: {token}" \
     -H "Content-Type: application/json" \
     -d '{
       "commands": [
         {
           "command": "start",
           "description": "Start or restart interaction"
         },
         {
           "command": "help",
           "description": "User guide and FAQ"
         },
         {
           "command": "status",
           "description": "Check active order tracking"
         }
       ]
     }'
```
*Note: Passing an empty array `{"commands": []}` removes all registered command suggestions.*

---

#### 2. Messaging: `POST /messages`

* **Target Addressing Parameter:**
  * **Private 1-on-1 dialogue:** Query parameter `?user_id={userId}` (positive `int64`).
  * **Group chat or channel:** Query parameter `?chat_id={chatId}` (group chat IDs can be negative or positive `int64`).
* **Request Body Structure (`NewMessageBody`):**
```json
{
  "text": "Hello, **Ivan**! Your order [#1042](https://example.com) is ready.",
  "format": "markdown",
  "notify": true,
  "disable_link_preview": false,
  "link": {
    "type": "reply",
    "mid": "mid.1718000005.12345"
  },
  "attachments": [
    {
      "type": "image",
      "payload": {
        "url": "https://images.example.com/receipt.png"
      }
    },
    {
      "type": "inline_keyboard",
      "payload": {
        "buttons": [
          [
            {
              "type": "callback",
              "text": "Confirm Receipt",
              "payload": "confirm_order_1042",
              "intent": "positive"
            },
            {
              "type": "callback",
              "text": "Cancel Order",
              "payload": "cancel_order_1042",
              "intent": "negative"
            }
          ]
        ]
      }
    }
  ]
}
```

* **Key Parameter Rules:**
  * `text`: String up to **4,000 UTF-8 characters**.
  * `format`: Either `"markdown"` or `"html"`.
  * `notify`: Boolean (default `true`). In channels, disabling notifications (`notify: false`) is rejected by the server.
  * `link`: Defines message hierarchy:
    * `type`: `"reply"` (quoted reply) or `"forward"` (forwarded message badge).
    * `mid`: The ID of the target message.
  * `attachments`: Heterogeneous array of media, files, keyboards, contacts, or locations.

---

#### 3. Reading Message History: `GET /messages` & `GET /messages/{messageId}`

##### Batch History: `GET /messages`
Retrieves history from a group chat or channel. The calling bot must have administrator privileges with `read_all_messages` rights.
```bash
curl -X GET "https://platform-api2.max.ru/messages?chat_id=-1002345678&count=20" \
     -H "Authorization: {token}"
```
* **Parameters:**
  * `chat_id` *(int64)*: ID of the chat/channel. Mutually exclusive with `message_ids`.
  * `message_ids` *(string)*: Comma-separated message IDs (e.g. `mid.1,mid.2`).
  * `count` *(int)*: Maximum number of items to return (1 to 100, default 50).
  * `from` *(int64)*: Timestamp upper bound in milliseconds (messages returned in reverse chronological order).
  * `to` *(int64)*: Timestamp lower bound in milliseconds.

##### Single Message Lookup: `GET /messages/{messageId}`
```bash
curl -X GET "https://platform-api2.max.ru/messages/mid.1718000005.12345" \
     -H "Authorization: {token}"
```

---

#### 4. Modifying & Deleting Messages: `PUT /messages` and `DELETE /messages`

##### Edit Message: `PUT /messages`
Edits the content, keyboard, or formatting of a previously sent message.
```bash
curl -X PUT "https://platform-api2.max.ru/messages?message_id=mid.1718000005.12345" \
     -H "Authorization: {token}" \
     -H "Content-Type: application/json" \
     -d '{
       "text": "Order #1042 has been **delivered**.",
       "format": "markdown",
       "attachments": []
     }'
```
* **Editing Window Rules:**
  * Messages containing interactive `inline_keyboard` attachments can be updated indefinitely (no time limit).
  * Plain text messages in private 1-on-1 dialogues can only be edited within **7 calendar days** of transmission.
  * In group chats and channels, administrators can edit messages without a time restriction.

##### Delete Message: `DELETE /messages`
```bash
curl -X DELETE "https://platform-api2.max.ru/messages?message_id=mid.1718000005.12345" \
     -H "Authorization: {token}"
```
* **Deletion Scope Rules:**
  * In 1-on-1 chats: The bot can only delete messages sent by itself.
  * In group chats and channels: A bot with administrator permissions can delete messages from any participant.

---

#### 5. Chat Presence Actions: `POST /chats/{chatId}/actions`

Dispatches real-time presence indicators to the top bar of a chat or dialogue.
```bash
curl -X POST "https://platform-api2.max.ru/chats/-1002345678/actions" \
     -H "Authorization: {token}" \
     -H "Content-Type: application/json" \
     -d '{
       "action": "typing_on"
     }'
```
* **Supported `action` Enum Values:**
  * `"typing_on"`: Bot is actively typing text or computing an AI response.
  * `"sending_photo"`: Bot is preparing and uploading an image.
  * `"sending_video"`: Bot is processing or uploading a video clip.
  * `"sending_audio"`: Bot is recording or sending an audio/voice note.
  * `"sending_file"`: Bot is uploading a document or archive.
* *Presence Duration:* The indicator expires automatically after approximately **10 seconds** unless renewed by another request.

---

#### 6. Pinning Messages: `PUT`, `GET`, `DELETE /chats/{chatId}/pin`

* **Get Pinned Message:** `GET https://platform-api2.max.ru/chats/{chatId}/pin`
* **Pin a Message:**
  ```bash
  curl -X PUT "https://platform-api2.max.ru/chats/{chatId}/pin" \
       -H "Authorization: {token}" \
       -H "Content-Type: application/json" \
       -d '{
         "message_id": "mid.1718000005.12345",
         "notify": true
       }'
  ```
* **Unpin Message:** `DELETE https://platform-api2.max.ru/chats/{chatId}/pin`
* *Requirements:* The bot must be an administrator with `pin_message` permission in the chat.

---

#### 7. Managing Chat Metadata: `GET` and `PATCH /chats/{chatId}`

##### Get Chat Info: `GET /chats/{chatId}`
Returns chat properties, participant count, type (`dialog`, `chat`, `channel`), and permissions.

##### Update Chat Settings: `PATCH /chats/{chatId}`
```bash
curl -X PATCH "https://platform-api2.max.ru/chats/{chatId}" \
     -H "Authorization: {token}" \
     -H "Content-Type: application/json" \
     -d '{
       "title": "VIP Customer Support Community",
       "description": "Official support channel for premium clients.",
       "icon": {
         "url": "https://storage.example.com/icons/support.png"
       },
       "pin": "mid.1718000005.12345",
       "notify": false
     }'
```
* **Parameters:**
  * `title`: Chat title string (up to 255 chars).
  * `description`: Detailed description (up to 16,000 characters).
  * `icon`: Image object containing `url` or upload `token`.
  * `pin`: Message ID to set as pinned.
  * `notify`: Boolean controlling participant notification about the change.

---

#### 8. Membership Management

* **List Members:** `GET /chats/{chatId}/members?count=50&marker={marker}`  
  Returns array of participants with fields `user_id`, `name`, `username`, `is_admin`, `is_creator`, `is_bot`.
* **Add Members:** `POST /chats/{chatId}/members` with `{"user_ids": [12345678, 87654321]}`.
* **Kick Member:** `DELETE /chats/{chatId}/members?user_id=12345678`.
* **Bot Self-Leave:** `DELETE /chats/{chatId}/members/me`.

---

#### 9. Administrator Rights & Permissions

* **List Admins:** `GET /chats/{chatId}/members/admins`
* **Promote / Update Administrator Rights:** `POST /chats/{chatId}/members/admins`
```bash
curl -X POST "https://platform-api2.max.ru/chats/{chatId}/members/admins" \
     -H "Authorization: {token}" \
     -H "Content-Type: application/json" \
     -d '{
       "admins": [
         {
           "user_id": 12345678,
           "alias": "Senior Support",
           "permissions": [
             "read_all_messages",
             "add_remove_members",
             "add_admins",
             "change_chat_info",
             "pin_message",
             "write"
           ]
         }
       ]
     }'
```
* **Permission Flags Breakdown:**
  * `"read_all_messages"`: Allows reading all messages in the chat history.
  * `"add_remove_members"`: Can invite new participants or kick existing ones.
  * `"add_admins"`: Can promote other users to administrators.
  * `"change_chat_info"`: Can edit chat title, avatar, and description.
  * `"pin_message"`: Can pin or unpin messages.
  * `"write"`: Can post messages in restricted channels.
* **Demote Administrator:** `DELETE /chats/{chatId}/members/admins/{userId}`

---

#### 10. Channel Publication Comments

For channels with discussion threads enabled:
* **Add Comment:** `POST /messages/{messageId}/comments` (Body matches `NewMessageBody`).
* **List Comments:** `GET /messages/{messageId}/comments?count=50&marker={marker}`.
* **Get Comment:** `GET /messages/{messageId}/comments/{commentId}`.
* **Edit Comment:** `PUT /messages/{messageId}/comments?comment_id={commentId}`.
* **Delete Comment:** `DELETE /messages/{messageId}/comments?comment_id={commentId}`.

---

#### 11. Answering Interactive Callbacks: `POST /answers`

When a user taps an inline button of type `"callback"`, MAX dispatches a `message_callback` update with a unique `callback_id`. The bot must call `POST /answers` within **10 seconds** to dismiss the loading state on the button.

```bash
curl -X POST "https://platform-api2.max.ru/answers?callback_id={callback_id}" \
     -H "Authorization: {token}" \
     -H "Content-Type: application/json" \
     -d '{
       "notification": "Status updated successfully!",
       "message": {
         "text": "Your ticket #402 has been **assigned** to an agent.",
         "format": "markdown",
         "attachments": []
       }
     }'
```

* **Unique MAX Feature:** Unlike Telegram's `answerCallbackQuery` (which only shows an alert/toast and requires a separate `editMessageText` call), MAX allows updating the message content **and** displaying an alert popup **in a single atomic HTTP request**.
* **Payload Fields:**
  * `notification` *(string, optional)*: Single-shot toast popup displayed to the user.
  * `message` *(object, optional)*: Complete `NewMessageBody` replacing the message where the button was pressed.

---

#### 12. Two-Step Media & File Uploads: `POST /uploads`

Media uploads (larger than external URLs) follow a two-phase protocol:

```
[Bot Client]                         [MAX Platform API]                     [MAX Storage Gateway]
     |                                        |                                        |
     |---- 1. POST /uploads?type={type} ----->|                                        |
     |<--- 2. { url, token } -----------------|                                        |
     |                                                                                 |
     |---- 3. POST multipart file data ----------------------------------------------->|
     |<--- 4. HTTP 200 OK -------------------------------------------------------------|
     |                                        |                                        |
     |---- 5. POST /messages (token) -------->|                                        |
     |<--- 6. HTTP 200 { message_id } --------|                                        |
```

##### Step 1: Initialize Upload Session
```bash
curl -X POST "https://platform-api2.max.ru/uploads?type=file" \
     -H "Authorization: {token}"
```
* **Supported `type` Query Parameter Enum:**
  * `"image"`: JPG, PNG, WEBP, GIF, TIFF, BMP, HEIC (up to 50 MB, 7680x7680 px).
  * `"video"`: MP4, MOV, AVI, MKV (up to 250 MB).
  * `"audio"`: MP3, AAC, OGG, WAV, voice messages (up to 100 MB).
  * `"file"`: Generic binary documents, PDF, ZIP (up to 4 GB).
* **Response Body:**
```json
{
  "url": "https://vu.maxcdn.ru/upload/session/f9a8b7...",
  "token": "tok_file_9918273645abcdef..."
}
```

##### Step 2: Binary / Multipart Upload
Send a `POST` request to the presigned `url` with standard `multipart/form-data` containing the file under field name `file` (or `data`).

##### Step 3: Attach Token to Message
Reference the returned `token` in the `attachments` array of `POST /messages`:
```json
{
  "attachments": [
    {
      "type": "file",
      "payload": {
        "token": "tok_file_9918273645abcdef..."
      }
    }
  ]
}
```
* **Token Reuse Capability:** Upload tokens are durable assets. Once uploaded, a single token can be broadcast to thousands of chats and recipients without re-uploading the file.
* **Direct External URL Bypass:** Images do not strictly require `/uploads`. They can be passed directly via URL:
  ```json
  {"type": "image", "payload": {"url": "https://example.com/photo.jpg"}}
  ```

---

#### 13. Inline Keyboards: Specification Matrix & Button Types

Keyboards are transmitted as attachments with `"type": "inline_keyboard"`. The payload contains a 2D array of buttons (`Button[][]`).

```json
{
  "type": "inline_keyboard",
  "payload": {
    "buttons": [
      [
        {"type": "callback", "text": "Approve", "payload": "act_approve", "intent": "positive"},
        {"type": "callback", "text": "Reject", "payload": "act_reject", "intent": "negative"}
      ],
      [
        {"type": "clipboard", "text": "Copy Promo", "data": "PROMO2026"},
        {"type": "link", "text": "Website", "url": "https://example.com"}
      ],
      [
        {"type": "request_contact", "text": "Share Phone Number"},
        {"type": "request_geo_location", "text": "Send Delivery GPS", "quick": true}
      ]
    ]
  }
}
```

##### Inline Keyboard Layout Rules & Limits:
* **Max Total Buttons:** 210 buttons per keyboard attachment.
* **Max Rows:** 30 rows.
* **Max Buttons Per Row (Standard Buttons):** Up to 7 buttons per row (for `callback`, `clipboard`, `message`).
* **Max Buttons Per Row (Interactive/Special):** Up to 3 buttons per row (for `link`, `open_app`, `request_contact`, `request_geo_location`).

##### Exhaustive Button Types Table:

| Button Type | Required Fields | Optional Fields | Action & Behavioral Semantics |
|---|---|---|---|
| **`callback`** | `type`, `text`, `payload` | `intent` | Dispatches a `message_callback` update to the bot with `callback_id` and payload string. |
| **`link`** | `type`, `text`, `url` | None | Opens an external HTTPS URL (max 2048 chars) in the system browser. |
| **`open_app`** | `type`, `text`, `url` | `params` | Launches an embedded MAX Mini App (HTML5 WebView) inside the messenger. |
| **`clipboard`** | `type`, `text`, `data` (or `payload`) | None | Copies the string in `data` directly into the user's OS clipboard with a native checkmark badge. |
| **`request_contact`** | `type`, `text` | None | Prompts user to securely share their verified phone number from their MAX profile. |
| **`request_geo_location`** | `type`, `text` | `quick` (bool) | Prompts user to share GPS coordinates. If `quick: true`, sends immediately without secondary dialog. |
| **`message`** | `type`, `text` | None | Sends the specified text directly as a message from the user account into the chat when tapped. |

##### Visual Styles (`intent`):
The `intent` parameter applies native styling / color-coding to buttons:
* `"default"`: Standard neutral theme button (gray / light slate).
* `"positive"`: Confirm / success action (accent green).
* `"negative"`: Destructive / cancel action (accent red).
* `"primary"`: Highlighted brand action (accent blue).

---

## 4. What Happens TO the Bot (All Inbound Events & Updates)

Events are delivered through either Webhook push (`POST /subscriptions`) or Long Polling pull (`GET /updates`).

### 4.1 Exhaustive Catalog of All Inbound `UpdateType` Events

| Update Type | Trigger Event | Primary Payload Components |
|---|---|---|
| `message_created` | User sends a text, attachment, contact, or location. | `message` (`message_id`, `sender`, `recipient`, `body`) |
| `message_edited` | User or bot modifies an existing message. | `message` with updated `body` and `timestamp` |
| `message_removed` | A message is deleted in a dialogue or group chat. | `message_id`, `chat_id`, `user_id`, `timestamp` |
| `message_callback` | User clicks an inline keyboard `callback` button. | `callback_id`, `payload`, `user`, `message`, `timestamp` |
| `bot_started` | User presses "Start" or launches bot via `?start=...` deep link. | `user`, `chat_id`, `payload` (string), `timestamp` |
| `bot_stopped` | User blocks or stops the bot in a 1-on-1 dialogue. | `user`, `timestamp` |
| `bot_added` | Bot is added to a group chat or channel. | `chat_id`, `user` (inviter), `timestamp` |
| `bot_removed` | Bot is kicked or removed from a group chat or channel. | `chat_id`, `user` (actor), `timestamp` |
| `user_added` | A user joins or is added to a group where the bot is present. | `chat_id`, `user` (new member), `inviter`, `timestamp` |
| `user_removed` | A member leaves or is kicked from a group chat. | `chat_id`, `user` (departed), `timestamp` |
| `chat_title_changed` | The title of a group chat or channel is modified. | `chat_id`, `title` (new string), `user`, `timestamp` |
| `dialog_cleared` | User clears conversation history in 1-on-1 chat. | `user_id`, `timestamp` |
| `dialog_muted` | User mutes notifications for the bot dialogue. | `user_id`, `timestamp` |
| `dialog_unmuted` | User unmutes notifications for the bot dialogue. | `user_id`, `timestamp` |
| `dialog_removed` | User deletes the entire chat dialogue from their list. | `user_id`, `timestamp` |
| `comment_created` | A new comment is posted under a channel publication. | `message` with `recipient.post_id`, `sender`, `body` |
| `comment_edited` | A comment under a channel publication is edited. | `message` with `recipient.post_id`, updated `body` |
| `comment_removed` | A comment under a channel publication is deleted. | `message_id`, `chat_id`, `post_id`, `timestamp` |
| `bot_admin_permissions_changed` | Owner/admin updates the bot's permission mask in a chat. | `chat_id`, `permissions` (string array), `timestamp` |

---

### 4.2 Exact Event JSON Schemas & Go Struct Definitions

#### 1. Incoming `message_created` (Contact & Location Sharing)

##### JSON Payload:
```json
{
  "update_type": "message_created",
  "timestamp": 1718000005123,
  "message": {
    "message_id": "mid.1718000005.99881",
    "timestamp": 1718000005120,
    "sender": {
      "user_id": 15349106,
      "first_name": "Alexander",
      "name": "Alexander Petrov",
      "username": "apetrov",
      "is_bot": false
    },
    "recipient": {
      "chat_id": 0,
      "chat_type": "dialog",
      "user_id": 98765432
    },
    "body": {
      "text": "Here is my location and contact",
      "attachments": [
        {
          "type": "contact",
          "payload": {
            "name": "Alexander Petrov",
            "phone": "+79991234567"
          }
        },
        {
          "type": "location",
          "payload": {
            "latitude": 55.7558,
            "longitude": 37.6173
          }
        }
      ]
    }
  }
}
```

#### 2. Incoming `message_callback` (Button Interaction)

##### JSON Payload:
```json
{
  "update_type": "message_callback",
  "timestamp": 1718000010450,
  "callback": {
    "callback_id": "cb_01J9X8829KLA...",
    "payload": "confirm_order_1042",
    "timestamp": 1718000010448,
    "user": {
      "user_id": 15349106,
      "name": "Alexander Petrov",
      "username": "apetrov",
      "is_bot": false
    },
    "message": {
      "message_id": "mid.1718000005.99881",
      "timestamp": 1718000005120,
      "sender": {
        "user_id": 98765432,
        "is_bot": true
      },
      "body": {
        "text": "Order #1042 is ready.",
        "attachments": []
      }
    }
  }
}
```

#### 3. Incoming `bot_started` (Deep Link Referral / Start)

##### JSON Payload:
```json
{
  "update_type": "bot_started",
  "timestamp": 1718000012000,
  "chat_id": 15349106,
  "user": {
    "user_id": 15349106,
    "first_name": "Alexander",
    "name": "Alexander Petrov",
    "username": "apetrov",
    "is_bot": false
  },
  "payload": "ref_partner_campaign_2026"
}
```

#### 4. Incoming `bot_admin_permissions_changed` (Webhook-Only)

##### JSON Payload:
```json
{
  "update_type": "bot_admin_permissions_changed",
  "timestamp": 1718000020000,
  "chat_id": -1002345678,
  "permissions": [
    "read_all_messages",
    "write",
    "pin_message",
    "change_chat_info"
  ]
}
```

---

### 4.3 Idiomatic Go Structural Models

Below is the complete, production-grade Go structural definition for deserializing all MAX Bot API updates and payloads:

```go
package maxbot

import (
	"encoding/json"
	"time"
)

// Standard MAX Bot API Update Types
const (
	UpdateMessageCreated             = "message_created"
	UpdateMessageCallback            = "message_callback"
	UpdateMessageEdited              = "message_edited"
	UpdateMessageRemoved             = "message_removed"
	UpdateBotStarted                 = "bot_started"
	UpdateBotStopped                 = "bot_stopped"
	UpdateBotAdded                   = "bot_added"
	UpdateBotRemoved                 = "bot_removed"
	UpdateUserAdded                  = "user_added"
	UpdateUserRemoved                = "user_removed"
	UpdateChatTitleChanged           = "chat_title_changed"
	UpdateDialogCleared              = "dialog_cleared"
	UpdateDialogMuted                = "dialog_muted"
	UpdateDialogUnmuted              = "dialog_unmuted"
	UpdateDialogRemoved              = "dialog_removed"
	UpdateCommentCreated             = "comment_created"
	UpdateCommentEdited              = "comment_edited"
	UpdateCommentRemoved             = "comment_removed"
	UpdateBotAdminPermissionsChanged = "bot_admin_permissions_changed"
)

// Update represents an inbound event envelope delivered via Webhook or Long Polling.
type Update struct {
	UpdateType  string       `json:"update_type"`
	Timestamp   int64        `json:"timestamp"`
	Marker      int64        `json:"marker,omitempty"`
	Message     *Message     `json:"message,omitempty"`
	Callback    *Callback    `json:"callback,omitempty"`
	ChatID      int64        `json:"chat_id,omitempty"`
	User        *User        `json:"user,omitempty"`
	Inviter     *User        `json:"inviter,omitempty"`
	Payload     string       `json:"payload,omitempty"` // For bot_started
	Title       string       `json:"title,omitempty"`   // For chat_title_changed
	MessageID   string       `json:"message_id,omitempty"` // For message_removed
	PostID      string       `json:"post_id,omitempty"`    // For comment updates
	Permissions []string     `json:"permissions,omitempty"`// For bot_admin_permissions_changed
}

// Time converts the Unix timestamp (milliseconds) to a standard time.Time.
func (u *Update) Time() time.Time {
	return time.UnixMilli(u.Timestamp)
}

// User represents a user or bot in the MAX ecosystem.
type User struct {
	UserID    int64  `json:"user_id"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Name      string `json:"name,omitempty"`
	Username  string `json:"username,omitempty"`
	IsBot     bool   `json:"is_bot"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// Message represents a delivered or received message entity.
type Message struct {
	MessageID string            `json:"message_id"`
	Timestamp int64             `json:"timestamp"`
	Sender    *User             `json:"sender,omitempty"`
	Recipient *MessageRecipient `json:"recipient,omitempty"`
	Body      MessageBody       `json:"body"`
}

// MessageRecipient indicates the destination dialogue, group chat, or channel.
type MessageRecipient struct {
	ChatID   int64  `json:"chat_id,omitempty"`
	ChatType string `json:"chat_type,omitempty"` // "dialog", "chat", "channel"
	UserID   int64  `json:"user_id,omitempty"`
	PostID   string `json:"post_id,omitempty"`   // For channel comments
}

// MessageBody encapsulates the text, format, link, and attachments.
type MessageBody struct {
	Text        string       `json:"text"`
	Format      string       `json:"format,omitempty"` // "markdown" or "html"
	Link        *MessageLink `json:"link,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// MessageLink represents a replied-to or forwarded message header.
type MessageLink struct {
	Type    string   `json:"type"` // "reply" or "forward"
	MID     string   `json:"mid,omitempty"`
	Message *Message `json:"message,omitempty"`
}

// Attachment represents a generic attachment payload.
type Attachment struct {
	Type    string          `json:"type"` // "image", "video", "audio", "file", "inline_keyboard", "contact", "location"
	Payload json.RawMessage `json:"payload"`
}

// Callback represents the payload of a message_callback event.
type Callback struct {
	CallbackID string   `json:"callback_id"`
	Payload    string   `json:"payload"`
	User       *User    `json:"user"`
	Message    *Message `json:"message,omitempty"`
	Timestamp  int64    `json:"timestamp"`
}

// ContactAttachmentPayload defines the parsed payload for type="contact".
type ContactAttachmentPayload struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	VCF   string `json:"vcf,omitempty"`
}

// LocationAttachmentPayload defines the parsed payload for type="location".
type LocationAttachmentPayload struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
```

---

## 5. Protocol Rules, Limits, Quotas & Error Handling

### 5.1 Throttling & Rate Limits

| Scope | Method / Action | Quota Limit | Behavior on Breach |
|---|---|---|---|
| **Global Token** | All outbound requests | ~30 Requests Per Second | HTTP 429 Too Many Requests |
| **Per Chat / Dialogue** | `POST /messages` | 2 Requests Per Second | HTTP 429 (`rate.limit.exceeded`) |
| **Per Chat / Dialogue** | `PUT /messages` (Edit) | 2 Requests Per Second | HTTP 429 (`rate.limit.exceeded`) |
| **Per Chat / Dialogue** | `DELETE /messages` | 2 Requests Per Second | HTTP 429 (`rate.limit.exceeded`) |
| **Per Callback** | `POST /answers` | 2 Requests Per Second | HTTP 429 (`rate.limit.exceeded`) |
| **Long Polling** | `GET /updates` | 1 Active Connection per Token | HTTP 409 Conflict |

#### Handling HTTP 429 Too Many Requests
When rate limits are exceeded, the API returns:
```http
HTTP/1.1 429 Too Many Requests
Retry-After: 1
Content-Type: application/json

{
  "code": "rate.limit.exceeded",
  "message": "Too many requests to this chat. Limit is 2 RPS.",
  "retry_after": 1
}
```
*Client Requirement:* Clients should read the `Retry-After` header (or JSON `retry_after` field) and execute an exponential backoff with random jitter.

---

### 5.2 Content & Attachment Quotas

* **Message Text Length:** 4,000 UTF-8 characters maximum.
* **Deep Link Parameter (`?start=...`):** 128 characters maximum (alphanumeric, dashes, underscores).
* **Mini App Deep Link (`?startapp=...`):** 512 characters maximum.
* **Images:** Maximum 50 MB; resolution limit 7680x7680 px; formats: JPG, PNG, WEBP, GIF, BMP, HEIC.
* **Videos:** Maximum 250 MB; formats: MP4, MOV, AVI.
* **Audio & Voice:** Maximum 100 MB.
* **Generic Files / Documents:** Maximum 4 GB.
* **Inline Keyboards:**
  * Maximum 210 total buttons.
  * Maximum 30 rows.
  * Maximum 7 standard buttons per row.
  * Maximum 3 interactive/special buttons per row.

---

### 5.3 Standard Error Codes & Diagnostics

When an API call fails, MAX returns a non-2xx HTTP status with a standardized JSON error body:

```json
{
  "code": "chat.not.found",
  "message": "Key: errors.chat.not.found",
  "description": "The specified chat_id does not exist or the bot was removed."
}
```

#### Canonical Error Code Catalog:

| HTTP Status | Error `code` | Description / Root Cause | Recovery Strategy |
|---|---|---|---|
| `400 Bad Request` | `invalid.param` | Invalid query parameter or malformed JSON body. | Validate request against schema; check `chat_id` vs `user_id`. |
| `400 Bad Request` | `message.too.long` | Text exceeds 4,000 UTF-8 characters. | Split message into smaller chunks before dispatching. |
| `400 Bad Request` | `attachment.not.ready` | The upload `token` has not completed backend transcoding. | Retry with 500ms delay. |
| `401 Unauthorized` | `verify.token` | Token missing from `Authorization` header or passed in query string. | Ensure `Authorization: <token>` is present without `Bearer` prefix. |
| `403 Forbidden` | `chat.access.denied` | Bot was kicked from chat or lacks required administrator permissions. | Verify bot membership and admin role before invoking method. |
| `403 Forbidden` | `user.blocked.bot` | User blocked the bot in a 1-on-1 dialogue. | Stop outbound messages to this user; listen for `bot_started`. |
| `404 Not Found` | `chat.not.found` | Target chat or user ID does not exist. | Purge recipient ID from active dispatch queues. |
| `404 Not Found` | `message.not.found` | Target message ID does not exist or was already deleted. | Discard edit/delete operation. |
| `404 Not Found` | `method.not.found` | Calling an obsolete endpoint (e.g. `GET /chats`). | Upgrade client endpoint routes to `platform-api2.max.ru` schema. |
| `409 Conflict` | `polling.conflict` | Multiple long-polling connections running on the same token. | Terminate duplicate bot processes / workers. |
| `429 Too Many Requests`| `rate.limit.exceeded` | Breached 2 RPS per-chat or 30 RPS token limit. | Pause outbound dispatching and wait for `Retry-After`. |
| `500 Internal Error` | `internal.server.error` | Internal platform outage or database lock. | Retry with exponential backoff. |

---

### 5.4 Webhook Lifecycle Caveats

1. **Subscription Stacking (No Overwrite):**
   Calling `POST /subscriptions` does **not** overwrite previous webhooks. It appends an additional subscription URL. If a bot redeploys without deleting old endpoints, MAX will send duplicate webhooks for every event.
   *Resolution:* SDKs must provide an atomic `EnsureWebhook` workflow: call `GET /subscriptions`, iterate and call `DELETE /subscriptions` for existing endpoints, then call `POST /subscriptions`.
2. **The 8-Hour Drop Policy:**
   If a webhook ingress returns 5xx errors or times out continuously for **8 hours**, the MAX platform deletes the subscription automatically.
   *Resolution:* Implement a self-healing health check cron job that regularly polls `GET /subscriptions` and restores the webhook if missing.

---

## 6. Gap Analysis & Comparative Study: MAX vs. Telegram vs. VK

| Dimension / Capability | MAX Messenger (`dev.max.ru`) | Telegram Bot API | VK Bot API | MAX Workaround / Strategic Alternative |
|---|---|---|---|---|
| **Identity & Trust** | **Verified Business / Gosuslugi** required. | Anonymous / Instant via `@BotFather`. | Community-bound (VK Group). | MAX provides unmatched institutional trust for banks, state services, and enterprise brands. |
| **TLS / Security** | **Минцифры Russian Root CA** mandatory. | Global Public CAs (Let's Encrypt). | Global Public CAs. | Complies with Russian federal cybersecurity regulations for critical infrastructure. |
| **Native Payments** | Under enterprise sandbox (Partner Bank SDKs). | **Telegram Stars (`XTR`)** & Providers. | **VK Pay**. | Workaround: Provide payment links via Mini Apps or checkout webviews using `open_app` or `link` buttons. |
| **Inline Query Mode** | Not available. | Yes (`@bot query` in any chat). | Not available. | Workaround: Use Mini Apps or direct bot dialogues. |
| **Forum Topics** | Not available (flat channels/groups). | Forum Topics (`message_thread_id`). | Not available. | Workaround: Channel comments (`POST /messages/{mid}/comments`). |
| **Stickers & Reactions**| Basic stickers; reactions in development. | Full sticker packs, animated emoji reactions. | Stickers and graffiti. | Workaround: Rich media (`image`, `video`) and interactive buttons. |
| **Speech-to-Text** | Server-side voice messages; no native STT API. | Premium Voice-to-Text integration. | Native voice message recognition. | Workaround: Pipe downloaded `.ogg`/`.wav` attachments to external STT (e.g. Sber SaluteSpeech or Yandex SpeechKit). |
| **Native Clipboard Copy**| **First-class button (`clipboard`)**. | Markdown block copy only. | Not available natively. | **MAX Advantage:** One-tap copying of promo codes, order IDs, and payment references. |
| **Direct Contact / Geo**| **First-class inline buttons**. | Reply keyboard only. | Location request button. | **MAX Advantage:** No keyboard switching needed; buttons live directly in the message bubble. |
| **Callback Response** | **Simultaneous Toast + Message Edit** in one roundtrip. | Alert only (`answerCallbackQuery`); requires 2nd call to edit. | Callback response or Snackbar. | **MAX Advantage:** Eliminates network roundtrips and avoids message flickers. |

---

## 7. Architectural Blueprint for `max-bot-api-client`

Based on this audit, the Go SDK implementation in `max-bot-api-client` must incorporate the following core components:

```
max-bot-api-client/
├── certs/
│   ├── mincifry_bundle.pem         <-- Embedded Russian Root & Intermediate CA bundle
│   └── certs.go                    <-- TLSConfig loader injecting system + Минцифры CAs
├── pkg/
│   ├── maxbot/
│   │   ├── client.go               <-- Low-level HTTP client with custom TLS transport
│   │   ├── models.go               <-- Complete Update, Message, Callback, User models
│   │   ├── keyboards.go            <-- Fluent InlineKeyboard builder (all 7 button types)
│   │   ├── errors.go               <-- Typed APIError, IsAPIError, IsTooManyRequests
│   │   ├── recipient.go            <-- Dual addressing (ToUser vs ToChat)
│   │   ├── webhooks.go             <-- EnsureWebhook, Secret validation middleware
│   │   ├── polling.go              <-- Resilient GetUpdatesChan with marker management
│   │   └── uploads.go              <-- Resilient 2-step media upload helper
│   └── bot/
│       ├── bot.go                  <-- High-level Bot instance
│       ├── router.go               <-- Event routing (OnCommand, OnMessage, OnCallback)
│       ├── context.go              <-- Enriched Context (ctx.Reply, ctx.Answer)
│       └── middleware.go           <-- Logging, Recovery, Metrics
```

### Key Library Guarantees:
1. **Zero-Configuration TLS:** Works out-of-the-box on Alpine, Ubuntu, macOS, and Windows containers without requiring manual certificate installation.
2. **Unified Recipient Model:** Safe routing between `user_id` and `chat_id` without query string mistakes.
3. **Atomic Webhook Hygiene:** Prunes stale webhook registrations automatically to avoid duplicate event storms.
4. **Resilient Long Polling:** Handles network disconnects and backoff transparently.
5. **Full Surface Coverage:** Complete support for presence actions, pinning, member management, admin rights, and channel comments.
