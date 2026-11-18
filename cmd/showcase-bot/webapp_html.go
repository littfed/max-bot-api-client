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
    }
    * { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
    body { background: var(--bg); color: var(--text); padding-bottom: 74px; min-height: 100vh; display: flex; flex-direction: column; }
    
    /* Top Header */
    header { background: #111827; padding: 14px 16px; border-bottom: 1px solid var(--border); display: flex; justify-content: space-between; align-items: center; position: sticky; top: 0; z-index: 40; }
    .brand { display: flex; align-items: center; gap: 10px; }
    .brand-icon { font-size: 22px; }
    .brand-title { font-size: 16px; font-weight: 700; }
    .user-pill { font-size: 12px; background: rgba(37,99,235,0.15); color: #60a5fa; padding: 4px 10px; border-radius: 9999px; border: 1px solid rgba(37,99,235,0.3); }

    /* Main Container */
    .container { padding: 16px; max-width: 600px; margin: 0 auto; width: 100%; flex: 1; }

    /* Tabs Content */
    .tab-content { display: none; }
    .tab-content.active { display: block; animation: fadeIn 0.2s ease-in-out; }
    @keyframes fadeIn { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; transform: translateY(0); } }

    /* Search & Categories */
    .search-box { width: 100%; background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 10px 14px; color: var(--text); font-size: 14px; margin-bottom: 12px; }
    .category-pills { display: flex; gap: 8px; overflow-x: auto; padding-bottom: 6px; margin-bottom: 14px; scrollbar-width: none; }
    .pill { background: var(--card); border: 1px solid var(--border); color: var(--muted); padding: 6px 14px; border-radius: 20px; font-size: 13px; cursor: pointer; white-space: nowrap; }
    .pill.active { background: var(--accent); color: white; border-color: var(--accent); font-weight: 600; }

    /* Catalog Cards */
    .catalog { display: flex; flex-direction: column; gap: 12px; margin-bottom: 20px; }
    .item-card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 14px; display: flex; align-items: center; justify-content: space-between; gap: 12px; }
    .item-main { display: flex; align-items: center; gap: 12px; flex: 1; cursor: pointer; }
    .item-icon { font-size: 30px; background: rgba(255,255,255,0.04); padding: 8px; border-radius: 10px; min-width: 48px; text-align: center; }
    .item-name { font-weight: 600; font-size: 15px; margin-bottom: 2px; }
    .item-desc { font-size: 12px; color: var(--muted); margin-bottom: 4px; }
    .item-price { font-size: 14px; font-weight: 700; color: #38bdf8; }
    
    .qty-controls { display: flex; align-items: center; gap: 6px; }
    .btn-qty { background: var(--border); color: var(--text); border: none; width: 30px; height: 30px; border-radius: 8px; font-size: 16px; cursor: pointer; }
    .qty-count { font-weight: 600; min-width: 20px; text-align: center; }

    /* Cart & Promo */
    .cart-box { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 14px; margin-top: 14px; }
    .promo-input-row { display: flex; gap: 8px; margin-bottom: 12px; }
    .input-promo { flex: 1; background: var(--bg); border: 1px solid var(--border); border-radius: 8px; padding: 8px 12px; color: var(--text); font-size: 13px; }
    .btn-apply-promo { background: #334155; color: white; border: none; padding: 8px 14px; border-radius: 8px; font-size: 13px; cursor: pointer; }
    .summary-row { display: flex; justify-content: space-between; font-size: 13px; color: var(--muted); margin-bottom: 6px; }
    .summary-row.total { font-size: 16px; font-weight: 700; color: var(--text); border-top: 1px solid var(--border); padding-top: 8px; margin-top: 8px; }
    .btn-checkout { background: var(--accent); color: white; border: none; width: 100%; padding: 12px; border-radius: 10px; font-size: 15px; font-weight: 600; cursor: pointer; margin-top: 10px; }

    /* Studio Tab (Admin) */
    .studio-card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 16px; margin-bottom: 16px; }
    .field-label { font-size: 13px; font-weight: 600; color: var(--muted); margin-bottom: 6px; display: block; }
    .channel-select { width: 100%; background: var(--bg); border: 1px solid var(--border); color: var(--text); padding: 10px; border-radius: 8px; font-size: 14px; margin-bottom: 12px; }
    .prompt-area { width: 100%; background: var(--bg); border: 1px solid var(--border); color: var(--text); padding: 10px; border-radius: 8px; font-size: 14px; height: 90px; margin-bottom: 10px; resize: vertical; }
    .btn-ai-gen { background: linear-gradient(135deg, #8b5cf6, #3b82f6); color: white; border: none; padding: 10px 14px; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; width: 100%; margin-bottom: 14px; display: flex; align-items: center; justify-content: center; gap: 8px; }
    .preview-box { background: rgba(0,0,0,0.3); border: 1px dashed var(--border); border-radius: 8px; padding: 12px; font-size: 13px; line-height: 1.5; margin-bottom: 14px; white-space: pre-wrap; }
    .btn-publish { background: var(--success); color: white; border: none; padding: 12px; border-radius: 10px; font-size: 15px; font-weight: 700; width: 100%; cursor: pointer; }

    /* Orders & Interactive */
    .order-card { background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 12px; margin-bottom: 10px; }
    .order-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
    .order-id { font-weight: 700; font-size: 14px; }
    .status-badge { font-size: 11px; padding: 2px 8px; border-radius: 9999px; }
    .status-accepted { background: rgba(16,185,129,0.15); color: #34d399; }
    .status-processing { background: rgba(245,158,11,0.15); color: #fbbf24; }

    .wheel-card { background: linear-gradient(135deg, rgba(37,99,235,0.1), rgba(139,92,246,0.1)); border: 1px solid rgba(139,92,246,0.3); border-radius: 12px; padding: 16px; text-align: center; margin-bottom: 16px; }
    .wheel-visual { font-size: 48px; margin: 10px 0; display: inline-block; transition: transform 2s cubic-bezier(0.17, 0.67, 0.12, 0.99); }
    .btn-spin { background: #8b5cf6; color: white; border: none; padding: 10px 20px; border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; }

    /* Bottom Navigation */
    nav.bottom-nav { position: fixed; bottom: 0; left: 0; right: 0; height: 60px; background: #111827; border-top: 1px solid var(--border); display: flex; justify-content: space-around; align-items: center; z-index: 50; }
    .nav-item { display: flex; flex-direction: column; align-items: center; justify-content: center; color: var(--muted); text-decoration: none; font-size: 11px; flex: 1; height: 100%; cursor: pointer; }
    .nav-item.active { color: #60a5fa; font-weight: 600; }
    .nav-icon { font-size: 20px; margin-bottom: 2px; }

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
      <div class="brand-title">MAX Mini App Suite</div>
    </div>
    <div class="user-pill" id="user-badge">Гость</div>
  </header>

  <div class="container">
    <!-- TAB 1: STORE -->
    <div class="tab-content active" id="tab-store">
      <input type="text" class="search-box" id="search-input" placeholder="🔍 Поиск по каталогу..." oninput="filterCatalog()">
      
      <div class="category-pills">
        <button class="pill active" onclick="setCategory('all', this)">Все товары</button>
        <button class="pill" onclick="setCategory('merch', this)">Мерч</button>
        <button class="pill" onclick="setCategory('tech', this)">Гаджеты</button>
        <button class="pill" onclick="setCategory('books', this)">Книги</button>
      </div>

      <div class="catalog" id="catalog-list"></div>

      <div class="cart-box">
        <div class="promo-input-row">
          <input type="text" class="input-promo" id="promo-code" placeholder="Промокод на скидку">
          <button class="btn-apply-promo" onclick="applyPromo()">Применить</button>
        </div>
        <div class="summary-row"><span>Товаров:</span><span id="cart-qty">0 шт.</span></div>
        <div class="summary-row"><span>Скидка:</span><span id="cart-discount">0 ₽</span></div>
        <div class="summary-row total"><span>Итого:</span><span id="cart-total">0 ₽</span></div>
        <button class="btn-checkout" onclick="submitCart()">Отправить заказ в бот 🚀</button>
      </div>
    </div>

    <!-- TAB 2: STUDIO (ADMIN ONLY) -->
    <div class="tab-content" id="tab-studio">
      <div class="studio-card">
        <label class="field-label">Целевой канал для публикации:</label>
        <select class="channel-select" id="studio-channel">
          <option value="-78368186601503">📢 Канал «Тест» (-78368186601503)</option>
          <option value="-78548596658207">📢 Канал «littfed» (-78548596658207)</option>
        </select>

        <label class="field-label">Тема публикации или промпт:</label>
        <textarea class="prompt-area" id="studio-prompt" placeholder="Например: Анонс новой витрины Mini App и скидок на мерч Go Gopher..."></textarea>
        
        <button class="btn-ai-gen" onclick="generateAIPost()">✨ Сгенерировать текст через Gemini</button>

        <label class="field-label">Текст поста (Markdown):</label>
        <textarea class="prompt-area" id="studio-body" style="height: 120px;"></textarea>

        <div style="display: flex; gap: 12px; margin-bottom: 12px; font-size: 13px;">
          <label><input type="checkbox" id="studio-pin" checked> Закрепить в канале 📌</label>
          <label><input type="checkbox" id="studio-rx" checked> Добавить реакции 👍🔥</label>
        </div>

        <label class="field-label">Предпросмотр поста в MAX:</label>
        <div class="preview-box" id="studio-preview">Текст поста появится здесь...</div>

        <button class="btn-publish" onclick="publishPostToChannel()">🚀 Опубликовать в канал</button>
      </div>
    </div>

    <!-- TAB 3: ORDERS & INTERACTIVE -->
    <div class="tab-content" id="tab-orders">
      <!-- Wheel of fortune -->
      <div class="wheel-card">
        <div style="font-weight: 700; font-size: 15px;">🎁 Колесо бонусов MAX</div>
        <p style="font-size: 12px; color: var(--muted); margin-top: 2px;">Выиграйте скидку до 25% на заказ!</p>
        <div><span class="wheel-visual" id="wheel-icon">🎡</span></div>
        <button class="btn-spin" onclick="spinWheel()">Испытать удачу 🎯</button>
        <div id="wheel-result" style="font-size: 13px; font-weight: 600; color: #34d399; margin-top: 8px;"></div>
      </div>

      <!-- User Orders -->
      <div style="font-weight: 700; font-size: 15px; margin-bottom: 10px;">📦 Мои заказы</div>
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
    <div class="nav-item" id="nav-studio-btn" onclick="switchTab('studio', this)">
      <div class="nav-icon">✍️</div>
      <div>Студия</div>
    </div>
    <div class="nav-item" onclick="switchTab('orders', this)">
      <div class="nav-icon">📦</div>
      <div>Заказы</div>
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
        <button id="modal-export-btn" style="display: none; background: #3b82f6; color: white; border: none; padding: 8px 12px; border-radius: 8px; font-size: 12px; cursor: pointer;" onclick="exportProductToChannel()">📢 Анонсировать в канал</button>
      </div>
    </div>
  </div>

  <script>
    const OWNER_ID = 396393759;
    let currentUser = { id: 0, first_name: 'Пользователь', username: '' };
    let discountPercent = 0;
    let activeModalItem = null;

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
          if (params.get('tab') === 'studio') {
            const studioBtn = document.getElementById('nav-studio-btn');
            if (studioBtn) switchTab('studio', studioBtn);
          }
        }
      } catch (e) {
        console.error('InitData parse error:', e);
      }

      if (currentUser.first_name) {
        document.getElementById('user-badge').innerText = currentUser.first_name + (currentUser.id === OWNER_ID ? ' (Владелец ⭐)' : '');
      }

      // Check owner permissions for studio tab
      if (currentUser.id === OWNER_ID) {
        document.getElementById('nav-studio-btn').style.display = 'flex';
      } else {
        // Hide studio for non-owners
        document.getElementById('nav-studio-btn').style.display = 'none';
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
              <div class="item-desc">${p.desc.substring(0, 48)}...</div>
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
      if (code.includes('25') || code === 'PROMO-MAX-25') {
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
      document.getElementById('tab-' + tab).classList.add('active');
      btn.classList.add('active');
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
      if (currentUser.id === OWNER_ID) {
        document.getElementById('modal-export-btn').style.display = 'block';
      }
      document.getElementById('product-modal').classList.add('open');
    }

    function closeModal(e) {
      document.getElementById('product-modal').classList.remove('open');
    }

    async function exportProductToChannel() {
      if (!activeModalItem) return;
      const channelID = document.getElementById('studio-channel').value;
      const res = await fetch('/api/webapp/publish_post', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          channel_id: parseInt(channelID, 10),
          user_id: currentUser.id,
          text: '🛍️ **Анонс товара: ' + activeModalItem.name + '**\n\n' + activeModalItem.desc + '\n\n💰 **Цена:** ' + activeModalItem.price + ' ₽\n\nЗаказывайте прямо в мессенджере в один клик!',
          pin: true,
          reactions: true
        })
      });
      const data = await res.json();
      if (data.success) {
        alert('📢 Товар успешно опубликован в канале!');
        closeModal();
      } else {
        alert('Ошибка публикации: ' + data.error);
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
        alert('Ошибка публикации: ' + data.error);
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

    // Orders List
    async function loadOrders() {
      const box = document.getElementById('orders-list');
      try {
        const res = await fetch('/api/webapp/orders?user_id=' + currentUser.id);
        const data = await res.json();
        if (data.orders && data.orders.length > 0) {
          box.innerHTML = '';
          data.orders.forEach(o => {
            const div = document.createElement('div');
            div.className = 'order-card';
            div.innerHTML = ` + "`" + `
              <div class="order-header">
                <span class="order-id">#${o.id}</span>
                <span class="status-badge status-${o.status}">${o.status === 'accepted' ? 'Принят 🟢' : o.status}</span>
              </div>
              <div style="font-size: 13px; color: var(--muted); margin-bottom: 4px;">Сумма: <b>${o.total.toLocaleString('ru-RU')} ₽</b></div>
              <div style="font-size: 11px; color: #64748b;">${new Date(o.created_at).toLocaleString('ru-RU')}</div>
            ` + "`" + `;
            box.appendChild(div);
          });
        } else {
          box.innerHTML = '<div style="font-size: 13px; color: var(--muted); text-align: center; padding: 20px;">Пока нет оформленных заказов. Закажите мерч во вкладке «Магазин»!</div>';
        }
      } catch (e) {
        box.innerHTML = '<div style="font-size: 13px; color: var(--muted); text-align: center; padding: 20px;">Список заказов пуст</div>';
      }
    }

    // Initialization
    parseInitData();
    renderCatalog(products);
    updateTotals();
  </script>
</body>
</html>`
