package main

const miniAppHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
  <title>MAX Mini App Showroom</title>
  <style>
    :root {
      --bg: #0f141c;
      --card: #182230;
      --accent: #2563eb;
      --accent-hover: #1d4ed8;
      --text: #f8fafc;
      --muted: #94a3b8;
      --border: #334155;
      --success: #10b981;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
    body { background: var(--bg); color: var(--text); padding: 16px; display: flex; flex-direction: column; min-height: 100vh; }
    header { text-align: center; margin-bottom: 20px; }
    h1 { font-size: 20px; font-weight: 700; margin-bottom: 4px; }
    p.sub { font-size: 13px; color: var(--muted); }
    .catalog { display: flex; flex-direction: column; gap: 12px; margin-bottom: 24px; }
    .item { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 14px; display: flex; align-items: center; justify-content: space-between; }
    .item-info { display: flex; align-items: center; gap: 12px; }
    .item-icon { font-size: 28px; background: rgba(255,255,255,0.05); padding: 8px; border-radius: 10px; }
    .item-title { font-weight: 600; font-size: 15px; }
    .item-price { font-size: 13px; color: var(--muted); margin-top: 2px; }
    .qty-controls { display: flex; align-items: center; gap: 8px; }
    .btn-qty { background: var(--border); color: var(--text); border: none; width: 28px; height: 28px; border-radius: 6px; font-size: 16px; cursor: pointer; }
    .qty-count { font-weight: 600; min-width: 16px; text-align: center; }
    footer { margin-top: auto; padding-top: 16px; }
    .summary-card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 14px; margin-bottom: 12px; }
    .summary-row { display: flex; justify-content: space-between; font-size: 14px; margin-bottom: 6px; color: var(--muted); }
    .summary-row.total { font-size: 17px; font-weight: 700; color: var(--text); margin-top: 8px; padding-top: 8px; border-top: 1px solid var(--border); margin-bottom: 0; }
    .btn-checkout { background: var(--accent); color: white; border: none; width: 100%; padding: 14px; border-radius: 10px; font-size: 16px; font-weight: 600; cursor: pointer; transition: background 0.2s; }
    .btn-checkout:hover { background: var(--accent-hover); }
    .status-msg { text-align: center; font-size: 13px; color: var(--muted); margin-top: 8px; }
    .badge { display: inline-block; font-size: 11px; background: rgba(37,99,235,0.2); color: #60a5fa; padding: 2px 8px; border-radius: 9999px; margin-top: 4px; }
  </style>
</head>
<body>
  <header>
    <h1>🛍️ MAX Mini App Корзина</h1>
    <p class="sub">Демонстрация WebBridge & встроенного WebView</p>
    <span class="badge">HMAC-SHA256 Ready</span>
  </header>

  <div class="catalog">
    <div class="item">
      <div class="item-info">
        <div class="item-icon">☕</div>
        <div>
          <div class="item-title">Кофемашина Barista</div>
          <div class="item-price">4 990 ₽</div>
        </div>
      </div>
      <div class="qty-controls">
        <button class="btn-qty" onclick="changeQty('coffee', -1)">-</button>
        <span class="qty-count" id="qty-coffee">1</span>
        <button class="btn-qty" onclick="changeQty('coffee', 1)">+</button>
      </div>
    </div>

    <div class="item">
      <div class="item-info">
        <div class="item-icon">🎧</div>
        <div>
          <div class="item-title">Наушники Pro ANC</div>
          <div class="item-price">2 490 ₽</div>
        </div>
      </div>
      <div class="qty-controls">
        <button class="btn-qty" onclick="changeQty('headphones', -1)">-</button>
        <span class="qty-count" id="qty-headphones">0</span>
        <button class="btn-qty" onclick="changeQty('headphones', 1)">+</button>
      </div>
    </div>
  </div>

  <footer>
    <div class="summary-card">
      <div class="summary-row"><span>Позиций в корзине:</span><span id="total-qty">1 шт.</span></div>
      <div class="summary-row total"><span>Итого к оплате:</span><span id="total-price">4 990 ₽</span></div>
    </div>
    <button class="btn-checkout" onclick="submitOrder()">Отправить заказ в бот 🚀</button>
    <div class="status-msg" id="status-msg">WebBridge готов к отправке</div>
  </footer>

  <script>
    const prices = { coffee: 4990, headphones: 2490 };
    const cart = { coffee: 1, headphones: 0 };

    function changeQty(item, delta) {
      cart[item] = Math.max(0, (cart[item] || 0) + delta);
      document.getElementById('qty-' + item).innerText = cart[item];
      updateTotals();
    }

    function updateTotals() {
      const totalQty = cart.coffee + cart.headphones;
      const totalPrice = (cart.coffee * prices.coffee) + (cart.headphones * prices.headphones);
      document.getElementById('total-qty').innerText = totalQty + ' шт.';
      document.getElementById('total-price').innerText = totalPrice.toLocaleString('ru-RU') + ' ₽';
    }

    async function submitOrder() {
      const status = document.getElementById('status-msg');
      status.innerText = 'Отправка через WebBridge...';

      const payload = {
        action: 'submit_cart',
        items: cart,
        total_price: (cart.coffee * prices.coffee) + (cart.headphones * prices.headphones),
        client_timestamp: Date.now()
      };

      try {
        const resp = await fetch('/api/webapp/action', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            action: 'submit_cart',
            init_data: window.location.search.substring(1) || 'demo=true',
            payload: payload
          })
        });
        const res = await resp.json();
        if (res.success) {
          status.innerText = '✅ Заказ принят! Проверьте чат бота.';
          status.style.color = '#10b981';
        } else {
          status.innerText = 'Ответ: ' + (res.message || 'успешно обработано');
        }
      } catch (e) {
        status.innerText = 'Отправлено на локальный мост.';
      }
    }
  </script>
</body>
</html>
`
