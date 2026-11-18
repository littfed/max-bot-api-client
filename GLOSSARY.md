# GLOSSARY: MAX Bot API Ecosystem

This document defines the canonical domain vocabulary used across the MAX Bot API Client and showcase applications.

### 1. Bot Roles & Permissions
* **Channel Administrator (Администратор канала):** A bot granted administrative privileges in a MAX broadcast channel (`Type: "channel"`). In MAX, regular participants cannot post to channels; the bot requires permissions `write`, `edit`, `delete`, `pin_message`, and `read_all_messages` to broadcast content, publish interactive polls, and manage reactions.
* **Group Privacy Mode (Режим приватности):** The default privacy mechanism in MAX groups (`Type: "chat"`). Without administrator rights, the bot only intercepts messages directly mentioning its username (`@bot_username`), slash commands (`/start`, `/help`), or replies to its own messages.

### 2. Interactive Channel Telemetry
* **Live Post Reactions (`rx:{mid}:{emoji}`):** Dynamic inline keyboard buttons attached to channel posts (e.g. `👍`, `🔥`, `❤️`). When a subscriber clicks a reaction, MAX emits an interactive callback event. The bot engine records the vote in `pkg/store` and updates the message's keyboard in place via `PUT /messages?message_id={mid}` with real-time counters.
* **Interactive Channel Polls (`poll:{id}:{opt_idx}`):** Inline poll posts where subscriber votes dynamically recalculate percentage distribution. The bot updates the message text with live visual ASCII progress bars (`[██████░░░░] 60%`).
* **Subscription-Gated Giveaway (`gw:{id}:join`):** Contest campaigns where participants click to join. The bot verifies channel membership, registers the entry, and delivers an instant promotional discount code via private message.

### 3. Mini App Suite & WebBridge
* **MAX Mini App (HTML5 WebView):** Responsive web applications rendered inside the native MAX Messenger webview via `open_app` buttons or direct HTTPS URLs (`https://combot.littfed.ru/maxapp`).
* **WebBridge Protocol:** Bi-directional bridge facilitating event communication between client-side JavaScript (`window.Max.WebApp` / `fetch('/api/webapp/action')`) and the Go backend server.
* **InitData HMAC-SHA256 Verification:** Cryptographic handshake authenticating user session parameters against the secret bot token hash before granting access to sensitive actions.
* **Channel Studio (Студия каналов):** An owner-gated administrative tab inside the Mini App (restricted to Bot Owner ID `396393759`) enabling Gemini AI post generation, live post preview, and one-click broadcast to connected channels.
* **Data Persistence Layer (`pkg/store`):** A thread-safe, mutex-protected atomic JSON persistence engine in `data/bot_store.json` storing channels, orders, post reactions, poll votes, and giveaway participants across container restarts.
