package main

const miniAppHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
  <title>MAX Mini App Suite</title>
  <style>
    :root {
      --bg: #0c1017;
      --card: #151d28;
      --card-hover: #1c2737;
      --accent: #2563eb;
      --accent-hover: #1d4ed8;
      --text: #f8fafc;
      --muted: #94a3b8;
      --border: #243042;
      --success: #10b981;
      --warning: #f59e0b;
      --danger: #ef4444;
      --purple: #8b5cf6;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
    body { background: var(--bg); color: var(--text); padding-bottom: 74px; min-height: 100vh; display: flex; flex-direction: column; }
    
    /* Top Header */
    header { background: #111827; padding: 12px 16px; border-bottom: 1px solid var(--border); display: flex; justify-content: space-between; align-items: center; position: sticky; top: 0; z-index: 40; }
    .brand { display: flex; align-items: center; gap: 8px; }
    .brand-icon { font-size: 20px; }
    .brand-title { font-size: 15px; font-weight: 700; }
    .user-pill { font-size: 11px; background: rgba(37,99,235,0.15); color: #60a5fa; padding: 4px 10px; border-radius: 9999px; border: 1px solid rgba(37,99,235,0.3); max-width: 180px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

    /* Main Container */
    .container { padding: 14px; max-width: 600px; margin: 0 auto; width: 100%; flex: 1; }

    /* Tabs Content */
    .tab-content { display: none; }
    .tab-content.active { display: block; animation: fadeIn 0.2s ease-in-out; }
    @keyframes fadeIn { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; transform: translateY(0); } }

    /* Search & Categories */
    .search-box { width: 100%; background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 10px 14px; color: var(--text); font-size: 14px; margin-bottom: 12px; }
    .category-pills { display: flex; gap: 8px; overflow-x: auto; padding-bottom: 6px; margin-bottom: 14px; scrollbar-width: none; }
    .pill { background: var(--card); border: 1px solid var(--border); color: var(--muted); padding: 6px 14px; border-radius: 20px; font-size: 13px; cursor: pointer; white-space: nowrap; }
    .pill.active { background: var(--accent); color: white; border-color: var(--accent); font-weight: 600; }

    /* Cards */
    .catalog { display: flex; flex-direction: column; gap: 10px; margin-bottom: 16px; }
    .item-card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 12px; display: flex; align-items: center; justify-content: space-between; gap: 12px; }
    .item-main { display: flex; align-items: center; gap: 12px; flex: 1; cursor: pointer; }
    .item-icon { font-size: 28px; background: rgba(255,255,255,0.04); padding: 6px; border-radius: 10px; min-width: 44px; text-align: center; }
    .item-name { font-weight: 600; font-size: 14px; margin-bottom: 2px; }
    .item-desc { font-size: 12px; color: var(--muted); margin-bottom: 3px; }
    .item-price { font-size: 14px; font-weight: 700; color: #38bdf8; }
    
    .qty-controls { display: flex; align-items: center; gap: 6px; }
    .btn-qty { background: var(--border); color: var(--text); border: none; width: 28px; height: 28px; border-radius: 8px; font-size: 15px; cursor: pointer; }
    .qty-count { font-weight: 600; min-width: 18px; text-align: center; font-size: 14px; }

    /* Cart & Promo */
    .cart-box { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 14px; margin-top: 14px; }
    .promo-input-row { display: flex; gap: 8px; margin-bottom: 10px; }
    .input-promo { flex: 1; background: var(--bg); border: 1px solid var(--border); border-radius: 8px; padding: 8px 12px; color: var(--text); font-size: 13px; }
    .btn-apply-promo { background: #334155; color: white; border: none; padding: 8px 14px; border-radius: 8px; font-size: 13px; cursor: pointer; }
    .summary-row { display: flex; justify-content: space-between; font-size: 13px; color: var(--muted); margin-bottom: 6px; }
    .summary-row.total { font-size: 16px; font-weight: 700; color: var(--text); border-top: 1px solid var(--border); padding-top: 8px; margin-top: 8px; }
    .btn-checkout { background: var(--accent); color: white; border: none; width: 100%; padding: 12px; border-radius: 10px; font-size: 15px; font-weight: 600; cursor: pointer; margin-top: 10px; }

    /* Booking Tab Styles */
    .booking-section { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 16px; margin-bottom: 14px; }
    .section-title { font-size: 15px; font-weight: 700; margin-bottom: 10px; display: flex; align-items: center; gap: 8px; }
    .service-options { display: flex; flex-direction: column; gap: 8px; margin-bottom: 14px; }
    .service-option { background: var(--bg); border: 1px solid var(--border); border-radius: 10px; padding: 10px 12px; cursor: pointer; display: flex; justify-content: space-between; align-items: center; }
    .service-option.selected { border-color: var(--accent); background: rgba(37,99,235,0.08); }
    .slot-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; margin-bottom: 14px; }
    .slot-btn { background: var(--bg); border: 1px solid var(--border); border-radius: 8px; padding: 8px; color: var(--text); font-size: 13px; cursor: pointer; text-align: center; }
    .slot-btn.selected { background: var(--accent); color: white; border-color: var(--accent); font-weight: 600; }
    .input-field { width: 100%; background: var(--bg); border: 1px solid var(--border); color: var(--text); padding: 10px; border-radius: 8px; font-size: 14px; margin-bottom: 10px; }

    /* AI Studio Tab Styles */
    .subnav { display: flex; gap: 8px; margin-bottom: 14px; }
    .subnav-btn { flex: 1; background: var(--card); border: 1px solid var(--border); color: var(--muted); padding: 8px; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; text-align: center; }
    .subnav-btn.active { background: var(--purple); color: white; border-color: var(--purple); }
    .ai-card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 14px; margin-bottom: 14px; }
    .chip-cloud { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 12px; }
    .chip { font-size: 11px; background: rgba(255,255,255,0.06); color: var(--muted); padding: 4px 8px; border-radius: 6px; cursor: pointer; }
    .chip:hover, .chip.selected { background: rgba(139,92,246,0.3); color: #c4b5fd; }
    .image-preview-container { min-height: 200px; background: rgba(0,0,0,0.4); border: 1px dashed var(--border); border-radius: 10px; display: flex; align-items: center; justify-content: center; overflow: hidden; margin-bottom: 12px; position: relative; }
    .image-preview-container img { width: 100%; height: auto; max-height: 320px; object-fit: cover; display: block; }
    .btn-action-purple { background: linear-gradient(135deg, #8b5cf6, #3b82f6); color: white; border: none; padding: 12px; border-radius: 10px; font-size: 14px; font-weight: 600; cursor: pointer; width: 100%; margin-bottom: 8px; }
    .btn-chat-send { background: #3b82f6; color: white; border: none; padding: 10px 18px; border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; }
    .chat-bubble { background: var(--bg); border: 1px solid var(--border); border-radius: 10px; padding: 12px; font-size: 13px; line-height: 1.5; white-space: pre-wrap; margin-top: 10px; }

    /* Studio Tab (Admin/Demo) */
    .studio-card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 14px; margin-bottom: 14px; }
    .field-label { font-size: 13px; font-weight: 600; color: var(--muted); margin-bottom: 6px; display: block; }
    .channel-select { width: 100%; background: var(--bg); border: 1px solid var(--border); color: var(--text); padding: 10px; border-radius: 8px; font-size: 14px; margin-bottom: 12px; }
    .prompt-area { width: 100%; background: var(--bg); border: 1px solid var(--border); color: var(--text); padding: 10px; border-radius: 8px; font-size: 14px; height: 80px; margin-bottom: 10px; resize: vertical; }
    .btn-ai-gen { background: linear-gradient(135deg, #8b5cf6, #3b82f6); color: white; border: none; padding: 10px 14px; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; width: 100%; margin-bottom: 12px; }
    .preview-box { background: rgba(0,0,0,0.3); border: 1px dashed var(--border); border-radius: 8px; padding: 12px; font-size: 13px; line-height: 1.5; margin-bottom: 12px; white-space: pre-wrap; }
    .btn-publish { background: var(--success); color: white; border: none; padding: 12px; border-radius: 10px; font-size: 15px; font-weight: 700; width: 100%; cursor: pointer; }
    .demo-badge { background: rgba(245,158,11,0.15); color: #fbbf24; border: 1px solid rgba(245,158,11,0.3); border-radius: 8px; padding: 8px 12px; font-size: 12px; margin-bottom: 12px; line-height: 1.4; }

    /* Orders, Wheel & Quiz */
    .order-card { background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 12px; margin-bottom: 10px; }
    .order-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
    .order-id { font-weight: 700; font-size: 14px; }
    .status-badge { font-size: 11px; padding: 2px 8px; border-radius: 9999px; }
    .status-accepted { background: rgba(16,185,129,0.15); color: #34d399; }
    .wheel-card { background: linear-gradient(135deg, rgba(37,99,235,0.1), rgba(139,92,246,0.1)); border: 1px solid rgba(139,92,246,0.3); border-radius: 12px; padding: 16px; text-align: center; margin-bottom: 14px; }
    .wheel-visual { font-size: 44px; margin: 8px 0; display: inline-block; transition: transform 2s cubic-bezier(0.17, 0.67, 0.12, 0.99); }
    .btn-spin { background: #8b5cf6; color: white; border: none; padding: 10px 20px; border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; }
    
    .quiz-card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 16px; margin-bottom: 14px; }
    .quiz-q { font-size: 14px; font-weight: 600; margin-bottom: 8px; }
    .quiz-opts { display: flex; flex-direction: column; gap: 6px; margin-bottom: 12px; }
    .quiz-opt { background: var(--bg); border: 1px solid var(--border); border-radius: 8px; padding: 8px 12px; font-size: 13px; cursor: pointer; text-align: left; color: var(--text); }
    .quiz-opt.correct { background: rgba(16,185,129,0.2); border-color: var(--success); color: #34d399; }
    .quiz-opt.wrong { background: rgba(239,68,68,0.2); border-color: var(--danger); color: #f87171; }

    /* Bottom Navigation */
    nav.bottom-nav { position: fixed; bottom: 0; left: 0; right: 0; height: 60px; background: #111827; border-top: 1px solid var(--border); display: flex; justify-content: space-around; align-items: center; z-index: 50; }
    .nav-item { display: flex; flex-direction: column; align-items: center; justify-content: center; color: var(--muted); text-decoration: none; font-size: 11px; flex: 1; height: 100%; cursor: pointer; }
    .nav-item.active { color: #60a5fa; font-weight: 600; }
    .nav-icon { font-size: 18px; margin-bottom: 2px; }

    /* Modal */
    .modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.7); display: none; align-items: center; justify-content: center; z-index: 60; padding: 16px; }
    .modal-overlay.open { display: flex; }
    .modal-card { background: #1e293b; border: 1px solid var(--border); border-radius: 16px; max-width: 440px; width: 100%; padding: 20px; }
  </style>
</head>
<body>
  <header>
    <div class="brand">
      <span class="brand-icon">⚡</span>
      <div class="brand-title">MAX SuperApp Suite</div>
    </div>
    <div class="user-pill" id="user-badge">Гость (Демо)</div>
  </header>

  <div class="container">
    <!-- TAB 1: STORE -->
    <div class="tab-content active" id="tab-store">
      <input type="text" class="search-box" id="search-input" placeholder="🔍 Поиск мерча и гаджетов..." oninput="filterCatalog()">
      
      <div class="category-pills">
        <button class="pill active" onclick="setCategory('all', this)">Все товары</button>
        <button class="pill" onclick="setCategory('merch', this)">Мерч</button>
        <button class="pill" onclick="setCategory('tech', this)">Гаджеты</button>
        <button class="pill" onclick="setCategory('books', this)">Книги</button>
      </div>

      <div class="catalog" id="catalog-list"></div>

      <div class="cart-box">
        <div class="promo-input-row">
          <input type="text" class="input-promo" id="promo-code" placeholder="Промокод (напр. PROMO-25-MAX)">
          <button class="btn-apply-promo" onclick="applyPromo()">Применить</button>
        </div>
        <div class="summary-row"><span>Товаров в корзине:</span><span id="cart-qty">0 шт.</span></div>
        <div class="summary-row"><span>Скидка:</span><span id="cart-discount">0 ₽</span></div>
        <div class="summary-row total"><span>Итого к оплате:</span><span id="cart-total">0 ₽</span></div>
        <button class="btn-checkout" onclick="submitCart()">Оформить заказ в бот 🚀</button>
      </div>
    </div>

    <!-- TAB 2: BOOKING -->
    <div class="tab-content" id="tab-booking">
      <div class="booking-section">
        <div class="section-title"><span>📋</span> 1. Выберите услугу</div>
        <div class="service-options" id="booking-services">
          <div class="service-option selected" onclick="selectService(this, 'srv_audit', 'Архитектурный аудит Go / MAX', 5000)">
            <div>
              <div style="font-weight: 600; font-size: 14px;">💻 Архитектурный аудит Go / MAX</div>
              <div style="font-size: 12px; color: var(--muted);">Разбор отказоустойчивости, FSM и очередей (60 мин)</div>
            </div>
            <div style="font-weight: 700; color: #38bdf8;">5 000 ₽</div>
          </div>
          <div class="service-option" onclick="selectService(this, 'srv_review', 'Code Review & Рефакторинг', 3500)">
            <div>
              <div style="font-weight: 600; font-size: 14px;">🔍 Code Review & Рефакторинг</div>
              <div style="font-size: 12px; color: var(--muted);">Аудит пулл-реквестов, горутин и контекстов (45 мин)</div>
            </div>
            <div style="font-weight: 700; color: #38bdf8;">3 500 ₽</div>
          </div>
          <div class="service-option" onclick="selectService(this, 'srv_turnkey', 'Разработка MAX-бота под ключ', 15000)">
            <div>
              <div style="font-weight: 600; font-size: 14px;">🤖 Разработка MAX-бота под ключ</div>
              <div style="font-size: 12px; color: var(--muted);">Проектирование логики, WebApp и деплой на Dokploy (90 мин)</div>
            </div>
            <div style="font-weight: 700; color: #38bdf8;">15 000 ₽</div>
          </div>
          <div class="service-option" onclick="selectService(this, 'srv_coffee', 'Дегустация specialty кофе', 0)">
            <div>
              <div style="font-weight: 600; font-size: 14px;">☕ Дегустация specialty кофе</div>
              <div style="font-size: 12px; color: var(--muted);">Беседа о высоких нагрузках в уютной кофейне (30 мин)</div>
            </div>
            <div style="font-weight: 700; color: #10b981;">Бесплатно</div>
          </div>
        </div>

        <div class="section-title"><span>👨‍💻</span> 2. Выберите специалиста</div>
        <select class="channel-select" id="booking-specialist">
          <option value="Алексей (Lead Go)">Алексей — Lead Go / Архитектор MAX Bot API</option>
          <option value="Екатерина (Solutions Architect)">Екатерина — Solutions Architect & Security</option>
          <option value="Михаил (DevOps)">Михаил — DevOps & HighLoad Engineer</option>
        </select>

        <div class="section-title"><span>📅</span> 3. Слот времени</div>
        <div class="slot-grid" id="booking-slots">
          <button class="slot-btn selected" onclick="selectSlot(this, 'Завтра, 11:00')">Завтра 11:00</button>
          <button class="slot-btn" onclick="selectSlot(this, 'Завтра, 14:30')">Завтра 14:30</button>
          <button class="slot-btn" onclick="selectSlot(this, 'Завтра, 17:00')">Завтра 17:00</button>
          <button class="slot-btn" onclick="selectSlot(this, 'Четверг, 12:00')">Четв 12:00</button>
          <button class="slot-btn" onclick="selectSlot(this, 'Четверг, 15:30')">Четв 15:30</button>
          <button class="slot-btn" onclick="selectSlot(this, 'Пятница, 16:00')">Пятн 16:00</button>
        </div>

        <div class="section-title"><span>📝</span> 4. Контакты</div>
        <input type="text" class="input-field" id="booking-name" placeholder="Ваше имя">
        <input type="tel" class="input-field" id="booking-phone" placeholder="Номер телефона (+7...)">
        <input type="text" class="input-field" id="booking-comment" placeholder="Тема вопроса или комментарий">

        <button class="btn-checkout" onclick="submitBooking()">Забронировать через WebBridge 📅</button>
      </div>
    </div>

    <!-- TAB 3: AI STUDIO -->
    <div class="tab-content" id="tab-ai">
      <div class="subnav">
        <button class="subnav-btn active" id="btn-sub-image" onclick="switchAISub('image')">🎨 Генератор артов</button>
        <button class="subnav-btn" id="btn-sub-chat" onclick="switchAISub('chat')">💬 Быстрый AI Чат</button>
      </div>

      <!-- AI Sub: Image Gen -->
      <div id="ai-sub-image">
        <div class="ai-card">
          <label class="field-label">Опишите желаемое изображение:</label>
          <textarea class="prompt-area" id="ai-img-prompt" placeholder="Например: Неоновый маскот Go Gopher в шлеме космического инженера..."></textarea>
          
          <div style="font-size: 12px; color: var(--muted); margin-bottom: 6px;">Быстрые стили:</div>
          <div class="chip-cloud">
            <span class="chip selected" onclick="selectStyle(this, 'Cyberpunk')">⚡ Киберпанк</span>
            <span class="chip" onclick="selectStyle(this, '3D Render')">🧊 3D Render</span>
            <span class="chip" onclick="selectStyle(this, 'Anime')">🌸 Аниме</span>
            <span class="chip" onclick="selectStyle(this, 'Photorealistic')">📷 Фотореализм</span>
            <span class="chip" onclick="selectStyle(this, 'Pixel Art')">👾 Pixel Art</span>
          </div>

          <button class="btn-action-purple" id="btn-gen-img" onclick="generateAIImage(false)">✨ Сгенерировать арт</button>

          <div class="image-preview-container" id="img-preview-box">
            <div id="img-placeholder-text" style="font-size: 13px; color: var(--muted); text-align: center; padding: 20px;">
              Здесь появится сгенерированное изображение
            </div>
            <img id="ai-generated-img" style="display: none;" src="" alt="AI Art">
          </div>

          <button class="btn-checkout" id="btn-send-chat-img" style="display: none;" onclick="generateAIImage(true)">📲 Отправить изображение в чат MAX</button>
        </div>
      </div>

      <!-- AI Sub: LLM Chat -->
      <div id="ai-sub-chat" style="display: none;">
        <div class="ai-card">
          <label class="field-label">Модель нейросети:</label>
          <select class="channel-select" id="ai-chat-model">
            <option value="auto">auto — Умная маршрутизация (Smart Gateway)</option>
            <option value="fast">fast — Быстрые ответы (Gemini 2.5 Flash)</option>
            <option value="best-coding">best-coding — Программирование и код (GLM / Claude)</option>
            <option value="reasoning">reasoning — Глубокое рассуждение (DeepSeek R1 / o1)</option>
          </select>

          <div style="font-size: 12px; color: var(--muted); margin-bottom: 6px;">Популярные вопросы:</div>
          <div class="chip-cloud">
            <span class="chip" onclick="setChatPrompt('Как работает WebBridge в MAX Bot API?')">🌐 Как работает WebBridge?</span>
            <span class="chip" onclick="setChatPrompt('Напиши пример HTTP хэндлера на Go')">🐹 Роут на Go</span>
            <span class="chip" onclick="setChatPrompt('Как защитить initData с помощью HMAC-SHA256?')">🔐 HMAC-SHA256</span>
          </div>

          <div style="display: flex; gap: 8px;">
            <input type="text" class="input-field" id="ai-chat-prompt" style="margin-bottom: 0;" placeholder="Задайте вопрос нейросети...">
            <button class="btn-chat-send" onclick="sendAIChat()">Отправить</button>
          </div>

          <div class="chat-bubble" id="ai-chat-result" style="display: none;"></div>
        </div>
      </div>
    </div>

    <!-- TAB 4: STUDIO -->
    <div class="tab-content" id="tab-studio">
      <div class="studio-card">
        <div class="demo-badge" id="studio-demo-badge">
          ℹ️ <b>SMM Студия каналов:</b> Генерация и разметка постов доступны всем. Публикация в реальные каналы защищена (доступна владельцу бота).
        </div>

        <label class="field-label">Целевой канал для публикации:</label>
        <select class="channel-select" id="studio-channel">
          <option value="-78368186601503">📢 Канал «Тест» (-78368186601503)</option>
          <option value="-78548596658207">📢 Канал «littfed» (-78548596658207)</option>
          <option value="0">🧪 Демо-канал (песочница)</option>
        </select>

        <label class="field-label">Тема публикации или промпт:</label>
        <textarea class="prompt-area" id="studio-prompt" placeholder="Например: Анонс новой витрины Mini App и скидок на мерч Go Gopher..."></textarea>
        
        <button class="btn-ai-gen" onclick="generateAIPost()">✨ Сгенерировать текст через Gemini</button>

        <label class="field-label">Текст поста (Markdown):</label>
        <textarea class="prompt-area" id="studio-body" style="height: 110px;"></textarea>

        <div style="display: flex; gap: 12px; margin-bottom: 12px; font-size: 13px;">
          <label><input type="checkbox" id="studio-pin" checked> Закрепить в канале 📌</label>
          <label><input type="checkbox" id="studio-rx" checked> Добавить реакции 👍🔥</label>
        </div>

        <label class="field-label">Предпросмотр поста в MAX:</label>
        <div class="preview-box" id="studio-preview">Текст поста появится здесь...</div>

        <button class="btn-publish" onclick="publishPostToChannel()">🚀 Опубликовать в канал</button>
      </div>
    </div>

    <!-- TAB 5: REWARDS & ORDERS -->
    <div class="tab-content" id="tab-orders">
      <!-- Wheel of fortune -->
      <div class="wheel-card">
        <div style="font-weight: 700; font-size: 15px;">🎁 Колесо бонусов MAX</div>
        <p style="font-size: 12px; color: var(--muted); margin-top: 2px;">Испытайте удачу и получите промокод со скидкой до 25%!</p>
        <div><span class="wheel-visual" id="wheel-icon">🎡</span></div>
        <button class="btn-spin" onclick="spinWheel()">Крутить колесо 🎯</button>
        <div id="wheel-result" style="font-size: 13px; font-weight: 600; color: #34d399; margin-top: 8px;"></div>
      </div>

      <!-- Interactive Quiz -->
      <div class="quiz-card" id="quiz-block">
        <div style="font-weight: 700; font-size: 15px; margin-bottom: 4px;">🧠 MAX Bot API Квиз</div>
        <p style="font-size: 12px; color: var(--muted); margin-bottom: 12px;">Ответьте правильно и активируйте эксклюзивную скидку 30%!</p>
        
        <div id="quiz-body">
          <div class="quiz-q" id="quiz-question">Загрузка вопроса...</div>
          <div class="quiz-opts" id="quiz-options"></div>
        </div>
        <div id="quiz-result" style="display: none; font-size: 13px; font-weight: 600; color: #34d399; margin-top: 8px;"></div>
      </div>

      <!-- History of Orders & Bookings -->
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">
        <div style="font-weight: 700; font-size: 15px;">📦 Заказы и Записи</div>
        <button onclick="loadOrders()" style="background: none; border: none; color: #60a5fa; font-size: 12px; cursor: pointer;">🔄 Обновить</button>
      </div>
      <div id="orders-list">
        <div style="font-size: 13px; color: var(--muted); text-align: center; padding: 20px;">Заказы загружаются...</div>
      </div>
    </div>
  </div>

  <!-- Bottom Navigation -->
  <nav class="bottom-nav">
    <div class="nav-item active" onclick="switchTab('store', this)">
      <div class="nav-icon">🛍️</div>
      <div>Магазин</div>
    </div>
    <div class="nav-item" onclick="switchTab('booking', this)">
      <div class="nav-icon">📅</div>
      <div>Запись</div>
    </div>
    <div class="nav-item" onclick="switchTab('ai', this)">
      <div class="nav-icon">✨</div>
      <div>AI Студия</div>
    </div>
    <div class="nav-item" id="nav-studio-btn" onclick="switchTab('studio', this)">
      <div class="nav-icon">✍️</div>
      <div>Студия</div>
    </div>
    <div class="nav-item" onclick="switchTab('orders', this)">
      <div class="nav-icon">🎁</div>
      <div>Бонусы</div>
    </div>
  </nav>

  <!-- Product Modal -->
  <div class="modal-overlay" id="product-modal" onclick="closeModal(event)">
    <div class="modal-card" onclick="event.stopPropagation()">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
        <h3 id="modal-title" style="font-size: 17px;">Товар</h3>
        <button onclick="closeModal()" style="background: none; border: none; color: var(--muted); font-size: 18px; cursor: pointer;">✕</button>
      </div>
      <div id="modal-desc" style="font-size: 13px; color: var(--muted); line-height: 1.5; margin-bottom: 16px;"></div>
      <div style="display: flex; justify-content: space-between; align-items: center;">
        <div id="modal-price" style="font-size: 18px; font-weight: 700; color: #38bdf8;">0 ₽</div>
        <button id="modal-export-btn" style="background: #3b82f6; color: white; border: none; padding: 8px 12px; border-radius: 8px; font-size: 12px; cursor: pointer;" onclick="exportProductToChannel()">📢 Анонсировать в канал</button>
      </div>
    </div>
  </div>

  <script>
    const OWNER_ID = 396393759;
    let currentUser = { id: 0, first_name: 'Пользователь', username: '' };
    let discountPercent = 0;
    let activeModalItem = null;
    let selectedStyle = 'Cyberpunk';
    let currentArtworkBase64 = null;

    // Booking state
    let selectedBookingService = { id: 'srv_audit', name: 'Архитектурный аудит Go / MAX', price: 5000 };
    let selectedBookingSlot = 'Завтра, 11:00';

    const products = [
      { id: 'gopher_hoodie', name: 'Худи Go Gopher Edition', category: 'merch', price: 3990, icon: '🐹', desc: 'Теплое хлопковое худи с вышивкой талисмана Go Gopher. Ограниченная серия MAX Dev.' },
      { id: 'tech_tshirt', name: 'Футболка MAX Ecosystem', category: 'merch', price: 1990, icon: '👕', desc: 'Премиальный хлопок, светоотражающий логотип платформы MAX Messenger.' },
      { id: 'barista_coffee', name: 'Кофемашина Barista 15bar', category: 'tech', price: 4990, icon: '☕', desc: 'Итальянская помпа 15 бар, капучинатор, быстрый нагрев за 25 секунд.' },
      { id: 'pro_anc_headphones', name: 'Наушники Pro ANC Wireless', category: 'tech', price: 2490, icon: '🎧', desc: 'Активное шумоподавление до 35дБ, 36 часов работы, Bluetooth 5.4.' },
      { id: 'stickers_pack', name: 'Стикерпак Bot Masters', category: 'merch', price: 490, icon: '✨', desc: 'Виниловые стикеры с маскотами MAX, кодом и мемами программистов.' },
      { id: 'go_patterns_book', name: 'Книга «Паттерны микросервисов в Go»', category: 'books', price: 1490, icon: '📖', desc: 'Практическое руководство по созданию отказоустойчивых ботов и бэкендов.' }
    ];

    const cart = {};

    function parseInitData() {
      try {
        let raw = window.location.search.substring(1);
        if (!raw && window.location.hash) {
          raw = window.location.hash.substring(1).replace('tgWebAppData=', '');
        }
        if (!raw && window.Max && window.Max.WebApp && window.Max.WebApp.initData) {
          raw = window.Max.WebApp.initData;
        }
        if (raw) {
          const params = new URLSearchParams(raw);
          const userStr = params.get('user');
          if (userStr) {
            currentUser = JSON.parse(decodeURIComponent(userStr));
          } else if (params.get('user_id')) {
            currentUser.id = parseInt(params.get('user_id'), 10);
            currentUser.first_name = params.get('first_name') || 'Пользователь';
          }
          if (params.get('promo')) {
            document.getElementById('promo-code').value = params.get('promo');
            applyPromo();
          }
          if (params.get('tab')) {
            const targetBtn = document.querySelector('[onclick*="' + params.get('tab') + '"]');
            if (targetBtn) switchTab(params.get('tab'), targetBtn);
          }
        }
      } catch (e) {
        console.error('InitData parse error:', e);
      }

      if (currentUser.first_name) {
        const isOwner = currentUser.id === OWNER_ID;
        document.getElementById('user-badge').innerText = currentUser.first_name + (isOwner ? ' (Владелец ⭐)' : ' (Гость)');
        document.getElementById('booking-name').value = currentUser.first_name;
        if (isOwner) {
          const badge = document.getElementById('studio-demo-badge');
          if (badge) {
            badge.style.background = 'rgba(16,185,129,0.15)';
            badge.style.color = '#34d399';
            badge.style.borderColor = 'rgba(16,185,129,0.3)';
            badge.innerHTML = '⭐ <b>Режим Администратора:</b> Полный доступ к публикации во все каналы бота.';
          }
        }
      }
    }

    function renderCatalog(items) {
      const list = document.getElementById('catalog-list');
      list.innerHTML = '';
      items.forEach(p => {
        const qty = cart[p.id] || 0;
        const div = document.createElement('div');
        div.className = 'item-card';
        div.innerHTML = ` + "`" + `
          <div class="item-main" onclick="openModal('${p.id}')">
            <div class="item-icon">${p.icon}</div>
            <div>
              <div class="item-name">${p.name}</div>
              <div class="item-desc">${p.desc.substring(0, 42)}...</div>
              <div class="item-price">${p.price.toLocaleString('ru-RU')} ₽</div>
            </div>
          </div>
          <div class="qty-controls">
            <button class="btn-qty" onclick="changeQty('${p.id}', -1)">-</button>
            <span class="qty-count" id="qty-${p.id}">${qty}</span>
            <button class="btn-qty" onclick="changeQty('${p.id}', 1)">+</button>
          </div>
        ` + "`" + `;
        list.appendChild(div);
      });
    }

    let activeCategory = 'all';
    function setCategory(cat, btn) {
      activeCategory = cat;
      document.querySelectorAll('.pill').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      filterCatalog();
    }

    function filterCatalog() {
      const q = document.getElementById('search-input').value.toLowerCase();
      const filtered = products.filter(p => {
        const matchCat = activeCategory === 'all' || p.category === activeCategory;
        const matchQ = p.name.toLowerCase().includes(q) || p.desc.toLowerCase().includes(q);
        return matchCat && matchQ;
      });
      renderCatalog(filtered);
    }

    function changeQty(id, delta) {
      cart[id] = Math.max(0, (cart[id] || 0) + delta);
      const el = document.getElementById('qty-' + id);
      if (el) el.innerText = cart[id];
      updateTotals();
    }

    function updateTotals() {
      let count = 0;
      let total = 0;
      for (const [id, qty] of Object.entries(cart)) {
        if (qty > 0) {
          const p = products.find(x => x.id === id);
          if (p) {
            count += qty;
            total += p.price * qty;
          }
        }
      }
      const discount = Math.round(total * (discountPercent / 100));
      const finalTotal = Math.max(0, total - discount);

      document.getElementById('cart-qty').innerText = count + ' шт.';
      document.getElementById('cart-discount').innerText = (discount > 0 ? '-' + discount.toLocaleString('ru-RU') + ' ₽ (' + discountPercent + '%)' : '0 ₽');
      document.getElementById('cart-total').innerText = finalTotal.toLocaleString('ru-RU') + ' ₽';
    }

    function applyPromo() {
      const code = document.getElementById('promo-code').value.trim().toUpperCase();
      if (code.includes('30') || code.includes('QUIZ')) {
        discountPercent = 30;
        alert('🎉 Промокод применен: Скидка 30%!');
      } else if (code.includes('25') || code === 'PROMO-25-MAX') {
        discountPercent = 25;
        alert('🎉 Промокод применен: Скидка 25%!');
      } else if (code.includes('15') || code.includes('LUCK')) {
        discountPercent = 15;
        alert('🎉 Промокод применен: Скидка 15%!');
      } else if (code) {
        discountPercent = 10;
        alert('🎉 Промокод применен: Скидка 10%!');
      }
      updateTotals();
    }

    async function submitCart() {
      let totalItems = 0;
      for (const q of Object.values(cart)) totalItems += q;
      if (totalItems === 0) {
        alert('Корзина пуста. Добавьте товары кнопками +');
        return;
      }

      const payload = {
        action: 'submit_cart',
        init_data: window.location.search.substring(1),
        payload: {
          items: cart,
          discount_percent: discountPercent,
          total_price: parseInt(document.getElementById('cart-total').innerText.replace(/[^0-9]/g, ''), 10)
        }
      };

      try {
        const res = await fetch('/api/webapp/action', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        const data = await res.json();
        if (data.success) {
          alert('✅ Заказ оформлен! Номер: ' + (data.data.order_id || 'WEB') + '. Подтверждение отправлено в бот.');
          loadOrders();
        } else {
          alert('Ошибка оформления: ' + (data.error || 'попробуйте снова'));
        }
      } catch (e) {
        alert('Ошибка связи с сервером WebBridge');
      }
    }

    function switchTab(tab, btn) {
      document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
      document.querySelectorAll('.nav-item').forEach(b => b.classList.remove('active'));
      const target = document.getElementById('tab-' + tab);
      if (target) target.classList.add('active');
      if (btn) btn.classList.add('active');
      if (tab === 'orders') loadOrders();
    }

    // Modal
    function openModal(id) {
      const p = products.find(x => x.id === id);
      if (!p) return;
      activeModalItem = p;
      document.getElementById('modal-title').innerText = p.icon + ' ' + p.name;
      document.getElementById('modal-desc').innerText = p.desc;
      document.getElementById('modal-price').innerText = p.price.toLocaleString('ru-RU') + ' ₽';
      document.getElementById('product-modal').classList.add('open');
    }

    function closeModal(e) {
      document.getElementById('product-modal').classList.remove('open');
    }

    async function exportProductToChannel() {
      if (!activeModalItem) return;
      const channelID = document.getElementById('studio-channel').value;
      if (currentUser.id !== OWNER_ID && channelID !== '0') {
        alert('ℹ️ В демо-режиме публикация доступна в демо-канал или владельцу бота.');
      }
      const res = await fetch('/api/webapp/publish_post', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          channel_id: parseInt(channelID, 10),
          user_id: currentUser.id,
          text: '🛍️ **Анонс товара: ' + activeModalItem.name + '**\n\n' + activeModalItem.desc + '\n\n💰 **Цена:** ' + activeModalItem.price + ' ₽\n\nЗаказывайте прямо в мессенджере через Mini App!',
          pin: true,
          reactions: true
        })
      });
      const data = await res.json();
      if (data.success) {
        alert('📢 Товар опубликован! ID сообщения: ' + data.message_id);
        closeModal();
      } else {
        alert('Информация: ' + (data.error || 'Готово'));
      }
    }

    // Booking Handlers
    function selectService(el, id, name, price) {
      document.querySelectorAll('.service-option').forEach(o => o.classList.remove('selected'));
      el.classList.add('selected');
      selectedBookingService = { id, name, price };
    }

    function selectSlot(el, slot) {
      document.querySelectorAll('.slot-btn').forEach(b => b.classList.remove('selected'));
      el.classList.add('selected');
      selectedBookingSlot = slot;
    }

    async function submitBooking() {
      const name = document.getElementById('booking-name').value.trim() || currentUser.first_name;
      const phone = document.getElementById('booking-phone').value.trim();
      const spec = document.getElementById('booking-specialist').value;
      const comment = document.getElementById('booking-comment').value.trim();

      const payload = {
        action: 'book_service',
        init_data: window.location.search.substring(1),
        payload: {
          service_id: selectedBookingService.id,
          service_name: selectedBookingService.name,
          price: selectedBookingService.price,
          specialist: spec,
          slot: selectedBookingSlot,
          user_name: name,
          phone: phone,
          comment: comment
        }
      };

      try {
        const res = await fetch('/api/webapp/action', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        const data = await res.json();
        if (data.success) {
          alert('📅 Запись успешно подтверждена! Номер брони: ' + (data.data.booking_id || 'BOK') + ' на ' + selectedBookingSlot + '. Карточка отправлена в чат.');
          loadOrders();
        } else {
          alert('Ошибка записи: ' + (data.error || 'попробуйте снова'));
        }
      } catch (e) {
        alert('Ошибка связи с сервером WebBridge');
      }
    }

    // AI Studio Handlers
    function switchAISub(sub) {
      document.getElementById('ai-sub-image').style.display = sub === 'image' ? 'block' : 'none';
      document.getElementById('ai-sub-chat').style.display = sub === 'chat' ? 'block' : 'none';
      document.getElementById('btn-sub-image').classList.toggle('active', sub === 'image');
      document.getElementById('btn-sub-chat').classList.toggle('active', sub === 'chat');
    }

    function selectStyle(el, style) {
      document.querySelectorAll('.chip-cloud .chip').forEach(c => c.classList.remove('selected'));
      el.classList.add('selected');
      selectedStyle = style;
    }

    async function generateAIImage(sendToChat) {
      const prompt = document.getElementById('ai-img-prompt').value.trim() || 'Маскот Go Gopher в футуристичном шлеме';
      const btnGen = document.getElementById('btn-gen-img');
      const placeholder = document.getElementById('img-placeholder-text');
      const imgEl = document.getElementById('ai-generated-img');
      const btnSend = document.getElementById('btn-send-chat-img');

      btnGen.innerText = sendToChat ? '⏳ Отправляем в чат...' : '⏳ Нейросеть генерирует арт...';
      btnGen.disabled = true;

      try {
        const res = await fetch('/api/webapp/generate_image', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            prompt: prompt,
            style: selectedStyle,
            user_id: currentUser.id,
            send_to_chat: sendToChat
          })
        });
        const data = await res.json();
        if (data.success && data.image_url) {
          imgEl.src = data.image_url;
          imgEl.style.display = 'block';
          placeholder.style.display = 'none';
          btnSend.style.display = 'block';
          if (sendToChat) {
            alert('🎨 Арт успешно отправлен в ваш чат мессенджера MAX!');
          }
        } else {
          alert('Не удалось сгенерировать изображение');
        }
      } catch (e) {
        alert('Ошибка связи с AI Gateway');
      } finally {
        btnGen.innerText = '✨ Сгенерировать арт';
        btnGen.disabled = false;
      }
    }

    function setChatPrompt(text) {
      document.getElementById('ai-chat-prompt').value = text;
      sendAIChat();
    }

    async function sendAIChat() {
      const prompt = document.getElementById('ai-chat-prompt').value.trim();
      if (!prompt) return;
      const model = document.getElementById('ai-chat-model').value;
      const resBubble = document.getElementById('ai-chat-result');
      
      resBubble.style.display = 'block';
      resBubble.innerText = '⏳ Думаю... (' + model + ')';

      try {
        const res = await fetch('/api/webapp/ai_chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            prompt: prompt,
            model: model,
            user_id: currentUser.id
          })
        });
        const data = await res.json();
        if (data.success && data.text) {
          resBubble.innerHTML = '<div style="font-size: 11px; color: #60a5fa; margin-bottom: 6px;">🧠 ' + data.model + ' • ' + (data.latency_ms || 350) + 'ms</div>' + data.text;
        } else {
          resBubble.innerText = 'Ошибка получения ответа';
        }
      } catch (e) {
        resBubble.innerText = 'Ошибка связи с сервером AI';
      }
    }

    // Studio AI & Publish
    async function generateAIPost() {
      const prompt = document.getElementById('studio-prompt').value.trim() || 'Анонс магазина и новых поступлений мерча MAX';
      document.getElementById('studio-preview').innerText = '⏳ Gemini генерирует пост...';
      try {
        const res = await fetch('/api/webapp/generate_post', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ prompt: prompt, user_id: currentUser.id })
        });
        const data = await res.json();
        if (data.success && data.text) {
          document.getElementById('studio-body').value = data.text;
          document.getElementById('studio-preview').innerText = data.text;
        } else {
          const fallback = '🚀 **Обновление витрины MAX Store!**\n\nЭксклюзивный мерч для разработчиков и участников сообщества доступен в Mini App.\n\n• Доставка по РФ\n• Оплата через СБП\n• Скидка 15% по промокоду PROMO-15-MAX';
          document.getElementById('studio-body').value = fallback;
          document.getElementById('studio-preview').innerText = fallback;
        }
      } catch (e) {
        document.getElementById('studio-preview').innerText = 'Ошибка связи с AI Gateway';
      }
    }

    document.getElementById('studio-body').addEventListener('input', function() {
      document.getElementById('studio-preview').innerText = this.value || 'Текст поста появится здесь...';
    });

    async function publishPostToChannel() {
      const text = document.getElementById('studio-body').value.trim();
      if (!text) {
        alert('Введите текст публикации');
        return;
      }
      const channelID = parseInt(document.getElementById('studio-channel').value, 10);
      const pin = document.getElementById('studio-pin').checked;
      const rx = document.getElementById('studio-rx').checked;

      if (currentUser.id !== OWNER_ID && channelID !== 0) {
        alert('ℹ️ В демо-режиме пост промодерирован и симулирован! Полная публикация в живой канал доступна администратору.');
        return;
      }

      const res = await fetch('/api/webapp/publish_post', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          channel_id: channelID,
          user_id: currentUser.id,
          text: text,
          pin: pin,
          reactions: rx
        })
      });
      const data = await res.json();
      if (data.success) {
        alert('🚀 Пост успешно опубликован в канале (MessageID: ' + data.message_id + ')!');
      } else {
        alert('Ответ сервера: ' + (data.error || 'Успешно'));
      }
    }

    // Wheel of fortune
    async function spinWheel() {
      const icon = document.getElementById('wheel-icon');
      icon.style.transform = 'rotate(' + (1440 + Math.floor(Math.random() * 360)) + 'deg)';
      try {
        const res = await fetch('/api/webapp/spin', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ user_id: currentUser.id })
        });
        const data = await res.json();
        setTimeout(() => {
          document.getElementById('wheel-result').innerText = '🎉 Победа! Ваш промокод: ' + data.code + ' (скидка ' + data.discount + '%)!';
          document.getElementById('promo-code').value = data.code;
          applyPromo();
        }, 2000);
      } catch (e) {
        document.getElementById('wheel-result').innerText = 'Промокод: PROMO-15-LUCK (скидка 15%)';
      }
    }

    // Quiz
    const quizQuestions = [
      {
        q: '1. Какой тип кнопки в MAX Bot API открывает встроенный WebApp?',
        options: ['open_app', 'link', 'callback', 'inline_query'],
        correct: 0
      },
      {
        q: '2. Какой алгоритм защищает целостность параметров initData?',
        options: ['HMAC-SHA256', 'MD5-Digest', 'AES-128-CBC', 'RSA-4096'],
        correct: 0
      },
      {
        q: '3. Как бот загружает изображение в CDN MAX перед отправкой?',
        options: ['UploadMedia(image)', 'base64 в JSON запросе', 'FTP upload', 'через сторонний хостинг'],
        correct: 0
      }
    ];
    let currentQuizIndex = 0;
    let quizScore = 0;

    function renderQuiz() {
      if (currentQuizIndex >= quizQuestions.length) {
        document.getElementById('quiz-body').style.display = 'none';
        const res = document.getElementById('quiz-result');
        res.style.display = 'block';
        res.innerHTML = '🏆 <b>Квиз пройден: ' + quizScore + ' из ' + quizQuestions.length + '!</b><br>Ваш промокод: <code style="background: rgba(16,185,129,0.2); padding: 2px 6px; border-radius: 4px;">QUIZ-PRO-30</code> (скидка 30%)<br><button onclick="applyQuizPromo()" style="margin-top: 8px; background: #10b981; color: white; border: none; padding: 6px 12px; border-radius: 6px; font-size: 12px; cursor: pointer;">Применить к корзине</button>';
        return;
      }
      const item = quizQuestions[currentQuizIndex];
      document.getElementById('quiz-question').innerText = item.q;
      const optsBox = document.getElementById('quiz-options');
      optsBox.innerHTML = '';
      item.options.forEach((opt, idx) => {
        const btn = document.createElement('button');
        btn.className = 'quiz-opt';
        btn.innerText = opt;
        btn.onclick = () => answerQuiz(idx, btn);
        optsBox.appendChild(btn);
      });
    }

    function answerQuiz(idx, btn) {
      const item = quizQuestions[currentQuizIndex];
      if (idx === item.correct) {
        btn.classList.add('correct');
        quizScore++;
      } else {
        btn.classList.add('wrong');
      }
      setTimeout(() => {
        currentQuizIndex++;
        renderQuiz();
      }, 700);
    }

    function applyQuizPromo() {
      document.getElementById('promo-code').value = 'QUIZ-PRO-30';
      applyPromo();
      switchTab('store', document.querySelector('.nav-item'));
    }

    // Orders List
    async function loadOrders() {
      const box = document.getElementById('orders-list');
      try {
        const res = await fetch('/api/webapp/orders?user_id=' + currentUser.id);
        const data = await res.json();
        if (data.orders && data.orders.length > 0) {
          box.innerHTML = '';
          data.orders.forEach(o => {
            const isBooking = o.delivery_type === 'booking';
            const div = document.createElement('div');
            div.className = 'order-card';
            div.innerHTML = ` + "`" + `
              <div class="order-header">
                <span class="order-id">${isBooking ? '📅 Бронь #' + o.id : '🛍️ Заказ #' + o.id}</span>
                <span class="status-badge status-${o.status}">${o.status === 'accepted' ? 'Принят 🟢' : o.status}</span>
              </div>
              <div style="font-size: 13px; color: var(--text); margin-bottom: 2px;">
                ${o.items && o.items.length > 0 ? o.items[0].title : 'Товар'}
              </div>
              ${o.address ? '<div style="font-size: 11px; color: var(--muted); margin-bottom: 4px;">' + o.address + '</div>' : ''}
              <div style="font-size: 13px; color: var(--muted); margin-bottom: 4px;">Сумма: <b>${o.total > 0 ? o.total.toLocaleString('ru-RU') + ' ₽' : 'Бесплатно'}</b></div>
              <div style="font-size: 11px; color: #64748b;">${new Date(o.created_at).toLocaleString('ru-RU')}</div>
            ` + "`" + `;
            box.appendChild(div);
          });
        } else {
          box.innerHTML = '<div style="font-size: 13px; color: var(--muted); text-align: center; padding: 20px;">Пока нет оформленных заказов или записей.</div>';
        }
      } catch (e) {
        box.innerHTML = '<div style="font-size: 13px; color: var(--muted); text-align: center; padding: 20px;">Список пуст</div>';
      }
    }

    // Initialization
    parseInitData();
    renderCatalog(products);
    updateTotals();
    renderQuiz();
  </script>
</body>
</html>`
