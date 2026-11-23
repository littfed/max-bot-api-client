package main

const miniAppHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover">
  <title>MAX Mini App Suite</title>
  <!-- Official MAX Messenger WebApp Bridge (Russian CDN) -->
  <script src="https://st.max.ru/js/max-web-app.js"></script>
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
    * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      user-select: none;
      -webkit-user-select: none;
      -webkit-touch-callout: none;
      -webkit-tap-highlight-color: transparent;
    }
    :focus-visible {
      outline: 2px solid var(--accent);
      outline-offset: 2px;
    }
    button, input, select, textarea, .pill, .slot-btn, .service-option, .nav-item, .chip {
      touch-action: manipulation;
    }
    input, textarea {
      user-select: text;
      -webkit-user-select: text;
    }
    @media (pointer: fine) {
      html, body {
        overscroll-behavior: none;
      }
    }
    body {
      background: var(--bg);
      color: var(--text);
      padding-top: max(12px, env(safe-area-inset-top, 12px));
      padding-bottom: max(84px, calc(84px + env(safe-area-inset-bottom, 0px)));
      min-height: 100vh;
      min-height: var(--tg-viewport-stable-height, 100vh);
      display: flex;
      flex-direction: column;
    }

    /* Accessibility screen-reader only utility */
    .sr-only {
      position: absolute;
      width: 1px;
      height: 1px;
      padding: 0;
      margin: -1px;
      overflow: hidden;
      clip: rect(0, 0, 0, 0);
      white-space: nowrap;
      border: 0;
    }

    /* Skeleton Loading System */
    @keyframes shimmer {
      0% { background-position: -200% 0; }
      100% { background-position: 200% 0; }
    }
    .skeleton {
      background: linear-gradient(90deg, #151d28 25%, #243042 50%, #151d28 75%);
      background-size: 200% 100%;
      animation: shimmer 1.5s infinite;
      border-radius: 8px;
    }
    .skeleton-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 14px;
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 10px;
    }
    .skeleton-img { width: 44px; height: 44px; border-radius: 10px; flex-shrink: 0; }
    .skeleton-lines { flex: 1; display: flex; flex-direction: column; gap: 8px; }
    .skeleton-line { height: 12px; width: 80%; }
    .skeleton-line.short { width: 45%; }

    /* Empty States */
    .empty-state {
      text-align: center;
      padding: 36px 16px;
      background: var(--card);
      border: 1px dashed var(--border);
      border-radius: 14px;
      margin: 12px 0;
    }
    .empty-icon { font-size: 44px; margin-bottom: 8px; }
    .empty-title { font-size: 15px; font-weight: 700; color: var(--text); margin-bottom: 4px; }
    .empty-desc { font-size: 13px; color: var(--muted); max-width: 280px; margin: 0 auto 16px; line-height: 1.4; }
    .btn-empty-action {
      background: var(--accent);
      color: white;
      border: none;
      padding: 10px 20px;
      border-radius: 10px;
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
    }
    .btn-empty-action:hover { background: var(--accent-hover); }

    /* Sticky Bottom MainButton (Thumb Zone) */
    .sticky-main-btn {
      position: fixed;
      bottom: calc(68px + env(safe-area-inset-bottom, 0px));
      left: 50%;
      transform: translate(-50%, 24px);
      width: calc(100% - 24px);
      max-width: 580px;
      height: 50px;
      background: linear-gradient(135deg, #2563eb, #1d4ed8);
      color: white;
      border: none;
      border-radius: 14px;
      padding: 0 16px;
      display: flex;
      justify-content: space-between;
      align-items: center;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      z-index: 48;
      box-shadow: 0 10px 25px -5px rgba(37, 99, 235, 0.5), 0 4px 10px rgba(0, 0, 0, 0.3);
      opacity: 0;
      pointer-events: none;
      transition: opacity 0.25s cubic-bezier(0.16, 1, 0.3, 1), transform 0.25s cubic-bezier(0.16, 1, 0.3, 1);
    }
    .sticky-main-btn.visible {
      opacity: 1;
      pointer-events: auto;
      transform: translate(-50%, 0);
    }
    .sticky-main-btn:active {
      transform: translate(-50%, 1px) scale(0.99);
    }
    .main-btn-left, .main-btn-right {
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .main-btn-right {
      font-weight: 700;
      font-size: 15px;
    }
    .main-btn-arrow {
      font-size: 16px;
      transition: transform 0.2s;
    }
    .sticky-main-btn:hover .main-btn-arrow {
      transform: translateX(3px);
    }

    /* Store Hero & User Pill */
    .store-hero {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 14px;
    }
    .welcome-sub {
      font-size: 11px;
      color: var(--muted);
      display: block;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      font-weight: 600;
      margin-bottom: 2px;
    }
    .welcome-name {
      font-size: 18px;
      font-weight: 700;
      color: var(--text);
      line-height: 1.2;
    }
    .user-pill {
      font-size: 11px;
      background: rgba(37,99,235,0.12);
      color: #60a5fa;
      padding: 4px 10px;
      border-radius: 9999px;
      border: 1px solid rgba(37,99,235,0.25);
      max-width: 180px;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      display: flex;
      align-items: center;
      gap: 5px;
    }

    /* Profile Card in Tab 5 */
    .profile-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 12px 14px;
      margin-bottom: 14px;
      display: flex;
      align-items: center;
      gap: 12px;
    }
    .profile-avatar {
      font-size: 22px;
      background: rgba(37,99,235,0.15);
      color: #60a5fa;
      width: 42px;
      height: 42px;
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      flex-shrink: 0;
    }
    .profile-info { flex: 1; min-width: 0; }
    .profile-name { font-weight: 700; font-size: 15px; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .profile-id { font-size: 12px; color: var(--muted); margin-top: 1px; }
    .profile-badge { font-size: 11px; background: rgba(16,185,129,0.12); color: #34d399; padding: 4px 10px; border-radius: 9999px; border: 1px solid rgba(16,185,129,0.25); white-space: nowrap; font-weight: 600; }

    /* Main Container */
    .container {
      padding: max(14px, env(safe-area-inset-top, 14px)) 14px max(80px, env(safe-area-inset-bottom, 80px)) 14px;
      max-width: 600px;
      margin: 0 auto;
      width: 100%;
      flex: 1;
    }

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
    .btn-qty { background: var(--border); color: var(--text); border: none; width: 30px; height: 30px; border-radius: 8px; font-size: 16px; font-weight: 600; cursor: pointer; display: flex; align-items: center; justify-content: center; }
    .btn-qty:hover { background: var(--card-hover); }
    .qty-count { font-weight: 600; min-width: 20px; text-align: center; font-size: 14px; font-variant-numeric: tabular-nums; }

    /* Cart & Promo */
    .cart-box { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 14px; margin-top: 14px; }
    .promo-input-row { display: flex; gap: 8px; margin-bottom: 10px; }
    .input-promo { flex: 1; background: var(--bg); border: 1px solid var(--border); border-radius: 8px; padding: 8px 12px; color: var(--text); font-size: 13px; }
    .btn-apply-promo { background: #334155; color: white; border: none; padding: 8px 14px; border-radius: 8px; font-size: 13px; cursor: pointer; font-weight: 600; }
    .btn-apply-promo:hover { background: #475569; }
    .summary-row { display: flex; justify-content: space-between; font-size: 13px; color: var(--muted); margin-bottom: 6px; }
    .summary-row.total { font-size: 16px; font-weight: 700; color: var(--text); border-top: 1px solid var(--border); padding-top: 8px; margin-top: 8px; }
    .btn-checkout { background: var(--accent); color: white; border: none; width: 100%; padding: 12px; border-radius: 10px; font-size: 15px; font-weight: 600; cursor: pointer; margin-top: 10px; }
    .btn-checkout:hover { background: var(--accent-hover); }

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
    .btn-action-purple:hover { opacity: 0.95; }
    .btn-chat-send { background: #3b82f6; color: white; border: none; padding: 10px 18px; border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; }
    .btn-chat-send:hover { background: #2563eb; }
    .chat-bubble { background: var(--bg); border: 1px solid var(--border); border-radius: 10px; padding: 12px; font-size: 13px; line-height: 1.5; white-space: pre-wrap; margin-top: 10px; }

    /* Studio Tab (Admin/Demo) */
    .studio-card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 14px; margin-bottom: 14px; }
    .field-label { font-size: 13px; font-weight: 600; color: var(--muted); margin-bottom: 6px; display: block; }
    .channel-select { width: 100%; background: var(--bg); border: 1px solid var(--border); color: var(--text); padding: 10px; border-radius: 8px; font-size: 14px; margin-bottom: 12px; }
    .prompt-area { width: 100%; background: var(--bg); border: 1px solid var(--border); color: var(--text); padding: 10px; border-radius: 8px; font-size: 14px; height: 80px; margin-bottom: 10px; resize: vertical; }
    .btn-ai-gen { background: linear-gradient(135deg, #8b5cf6, #3b82f6); color: white; border: none; padding: 10px 14px; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; width: 100%; margin-bottom: 12px; }
    .btn-ai-gen:hover { opacity: 0.95; }
    .preview-box { background: rgba(0,0,0,0.3); border: 1px dashed var(--border); border-radius: 8px; padding: 12px; font-size: 13px; line-height: 1.5; margin-bottom: 12px; white-space: pre-wrap; }
    .btn-publish { background: var(--success); color: white; border: none; padding: 12px; border-radius: 10px; font-size: 15px; font-weight: 700; width: 100%; cursor: pointer; }
    .btn-publish:hover { opacity: 0.95; }
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
    .btn-spin:hover { background: #7c3aed; }
    
    .quiz-card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 16px; margin-bottom: 14px; }
    .quiz-q { font-size: 14px; font-weight: 600; margin-bottom: 8px; }
    .quiz-opts { display: flex; flex-direction: column; gap: 6px; margin-bottom: 12px; }
    .quiz-opt { background: var(--bg); border: 1px solid var(--border); border-radius: 8px; padding: 8px 12px; font-size: 13px; cursor: pointer; text-align: left; color: var(--text); }
    .quiz-opt.correct { background: rgba(16,185,129,0.2); border-color: var(--success); color: #34d399; }
    .quiz-opt.wrong { background: rgba(239,68,68,0.2); border-color: var(--danger); color: #f87171; }

    /* Bottom Navigation */
    nav.bottom-nav {
      position: fixed;
      bottom: 0;
      left: 0;
      right: 0;
      height: calc(60px + env(safe-area-inset-bottom, 0px));
      padding-bottom: env(safe-area-inset-bottom, 0px);
      background: #111827;
      border-top: 1px solid var(--border);
      display: flex;
      justify-content: space-around;
      align-items: center;
      z-index: 50;
    }
    .nav-item {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      color: var(--muted);
      background: none;
      border: none;
      font: inherit;
      font-size: 11px;
      flex: 1;
      height: 100%;
      cursor: pointer;
    }
    .nav-item.active { color: #60a5fa; font-weight: 600; }
    .nav-icon { font-size: 18px; margin-bottom: 2px; }

    /* Modal */
    .modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.7); display: none; align-items: center; justify-content: center; z-index: 60; padding: 16px; overscroll-behavior: contain; }
    .modal-overlay.open { display: flex; }
    .modal-card { background: #1e293b; border: 1px solid var(--border); border-radius: 16px; max-width: 440px; width: 100%; padding: 20px; }

    /* Modern Toast System (Replacing browser alert) */
    #toast-container {
      position: fixed;
      bottom: calc(75px + env(safe-area-inset-bottom, 0px));
      left: 50%;
      transform: translateX(-50%);
      display: flex;
      flex-direction: column;
      gap: 8px;
      z-index: 100;
      pointer-events: none;
      width: calc(100% - 32px);
      max-width: 440px;
    }
    .toast-msg {
      background: #1e293b;
      border: 1px solid var(--border);
      color: var(--text);
      padding: 12px 16px;
      border-radius: 12px;
      font-size: 13px;
      font-weight: 500;
      box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5);
      display: flex;
      align-items: center;
      gap: 10px;
      animation: toastSlideUp 0.25s cubic-bezier(0.16, 1, 0.3, 1) forwards;
      pointer-events: auto;
    }
    .toast-msg.success { border-color: var(--success); }
    .toast-msg.error { border-color: var(--danger); }
    .toast-msg.fade-out { animation: toastFadeOut 0.3s forwards; }
    @keyframes toastSlideUp {
      from { opacity: 0; transform: translateY(16px); }
      to { opacity: 1; transform: translateY(0); }
    }
    @keyframes toastFadeOut {
      from { opacity: 1; transform: translateY(0); }
      to { opacity: 0; transform: translateY(-8px); }
    }

    /* Gallery & Media Artifacts */
    .gallery-card {
      background: #111827;
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 12px;
      display: flex;
      flex-direction: column;
      gap: 8px;
      transition: border-color 0.15s ease;
    }
    .gallery-card:hover {
      border-color: #4b5563;
    }
    .gallery-card-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      font-size: 11px;
      color: var(--muted);
    }
    .gallery-card-badge {
      padding: 2px 8px;
      border-radius: 6px;
      font-size: 11px;
      font-weight: 600;
    }
    .badge-image { background: rgba(168, 85, 247, 0.15); color: #c084fc; border: 1px solid rgba(168, 85, 247, 0.3); }
    .badge-stt { background: rgba(59, 130, 246, 0.15); color: #60a5fa; border: 1px solid rgba(59, 130, 246, 0.3); }
    .badge-doc { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
    .gallery-card-img {
      width: 100%;
      max-height: 220px;
      object-fit: cover;
      border-radius: 8px;
      margin-top: 4px;
      border: 1px solid var(--border);
    }
    .gallery-card-body {
      font-size: 13px;
      color: #d1d5db;
      line-height: 1.45;
      background: #1e293b;
      padding: 10px;
      border-radius: 8px;
      max-height: 140px;
      overflow-y: auto;
      white-space: pre-wrap;
      word-break: break-word;
    }
    .gallery-card-actions {
      display: flex;
      gap: 8px;
      margin-top: 4px;
      flex-wrap: wrap;
    }
    .btn-card-action {
      background: #1e293b;
      border: 1px solid var(--border);
      color: var(--text);
      padding: 6px 12px;
      border-radius: 8px;
      font-size: 12px;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      gap: 6px;
      transition: background 0.15s ease;
    }
    .btn-card-action:hover {
      background: #334155;
    }

    /* Accessibility: Reduced Motion */
    @media (prefers-reduced-motion: reduce) {
      *, ::before, ::after {
        animation-duration: 0.01ms !important;
        animation-iteration-count: 1 !important;
        transition-duration: 0.01ms !important;
        scroll-behavior: auto !important;
      }
      .wheel-visual {
        transition: none !important;
      }
    }
  </style>
</head>
<body>
  <main class="container">
    <!-- TAB 1: STORE -->
    <section class="tab-content active" id="tab-store" role="tabpanel" aria-labelledby="tab-store-btn">
      <div class="store-hero">
        <div class="user-welcome">
          <span class="welcome-sub">Добро пожаловать</span>
          <h1 class="welcome-name" id="user-display-name">Пользователь</h1>
        </div>
        <div class="user-pill" id="user-badge" aria-label="Профиль">
          <span>🛍️</span>
          <span id="user-badge-text">Клиент</span>
        </div>
      </div>

      <label for="search-input" class="sr-only">Поиск товаров</label>
      <input type="text" class="search-box" id="search-input" placeholder="🔍 Поиск мерча и гаджетов…" oninput="filterCatalog()" aria-label="Поиск товаров">
      
      <div class="category-pills" role="tablist" aria-label="Категории товаров">
        <button type="button" class="pill active" role="tab" aria-selected="true" onclick="setCategory('all', this)">Все товары</button>
        <button type="button" class="pill" role="tab" aria-selected="false" onclick="setCategory('merch', this)">Мерч</button>
        <button type="button" class="pill" role="tab" aria-selected="false" onclick="setCategory('tech', this)">Гаджеты</button>
        <button type="button" class="pill" role="tab" aria-selected="false" onclick="setCategory('books', this)">Книги</button>
      </div>

      <div class="catalog" id="catalog-list" aria-label="Каталог товаров"></div>

      <div class="cart-box" id="cart-box-container">
        <div class="promo-input-row">
          <label for="promo-code" class="sr-only">Промокод</label>
          <input type="text" class="input-promo" id="promo-code" placeholder="Промокод (напр. PROMO-25-MAX)">
          <button type="button" class="btn-apply-promo" onclick="applyPromo()">Применить</button>
        </div>
        <div id="cart-content-wrapper">
          <div class="summary-row"><span>Товаров в корзине:</span><span id="cart-qty">0 шт.</span></div>
          <div class="summary-row"><span>Скидка:</span><span id="cart-discount">0 ₽</span></div>
          <div class="summary-row total"><span>Итого к оплате:</span><span id="cart-total">0 ₽</span></div>
          <button type="button" class="btn-checkout" id="btn-cart-checkout" onclick="submitCart()">Оформить заказ в бот 🚀</button>
        </div>
      </div>
    </section>

    <!-- TAB 2: BOOKING -->
    <section class="tab-content" id="tab-booking" role="tabpanel" aria-labelledby="tab-booking-btn">
      <div class="booking-section">
        <div class="section-title"><span>📋</span> 1. Выберите услугу</div>
        <div class="service-options" id="booking-services" role="radiogroup" aria-label="Список услуг">
          <div class="service-option selected" role="radio" aria-checked="true" tabindex="0" onclick="selectService(this, 'srv_audit', 'Архитектурный аудит Go / MAX', 5000)" onkeydown="if(event.key==='Enter'||event.key===' ')selectService(this, 'srv_audit', 'Архитектурный аудит Go / MAX', 5000)">
            <div>
              <div style="font-weight: 600; font-size: 14px;">💻 Архитектурный аудит Go / MAX</div>
              <div style="font-size: 12px; color: var(--muted);">Разбор отказоустойчивости, FSM и очередей (60 мин)</div>
            </div>
            <div style="font-weight: 700; color: #38bdf8;">5 000 ₽</div>
          </div>
          <div class="service-option" role="radio" aria-checked="false" tabindex="0" onclick="selectService(this, 'srv_review', 'Code Review & Рефакторинг', 3500)" onkeydown="if(event.key==='Enter'||event.key===' ')selectService(this, 'srv_review', 'Code Review & Рефакторинг', 3500)">
            <div>
              <div style="font-weight: 600; font-size: 14px;">🔍 Code Review & Рефакторинг</div>
              <div style="font-size: 12px; color: var(--muted);">Аудит пулл-реквестов, горутин и контекстов (45 мин)</div>
            </div>
            <div style="font-weight: 700; color: #38bdf8;">3 500 ₽</div>
          </div>
          <div class="service-option" role="radio" aria-checked="false" tabindex="0" onclick="selectService(this, 'srv_turnkey', 'Разработка MAX-бота под ключ', 15000)" onkeydown="if(event.key==='Enter'||event.key===' ')selectService(this, 'srv_turnkey', 'Разработка MAX-бота под ключ', 15000)">
            <div>
              <div style="font-weight: 600; font-size: 14px;">🤖 Разработка MAX-бота под ключ</div>
              <div style="font-size: 12px; color: var(--muted);">Проектирование логики, WebApp и деплой на Dokploy (90 мин)</div>
            </div>
            <div style="font-weight: 700; color: #38bdf8;">15 000 ₽</div>
          </div>
          <div class="service-option" role="radio" aria-checked="false" tabindex="0" onclick="selectService(this, 'srv_coffee', 'Дегустация specialty кофе', 0)" onkeydown="if(event.key==='Enter'||event.key===' ')selectService(this, 'srv_coffee', 'Дегустация specialty кофе', 0)">
            <div>
              <div style="font-weight: 600; font-size: 14px;">☕ Дегустация specialty кофе</div>
              <div style="font-size: 12px; color: var(--muted);">Беседа о высоких нагрузках в уютной кофейне (30 мин)</div>
            </div>
            <div style="font-weight: 700; color: #10b981;">Бесплатно</div>
          </div>
        </div>

        <label for="booking-specialist" class="section-title"><span>👨‍💻</span> 2. Выберите специалиста</label>
        <select class="channel-select" id="booking-specialist">
          <option value="Алексей (Lead Go)">Алексей — Lead Go / Архитектор MAX Bot API</option>
          <option value="Екатерина (Solutions Architect)">Екатерина — Solutions Architect & Security</option>
          <option value="Михаил (DevOps)">Михаил — DevOps & HighLoad Engineer</option>
        </select>

        <div class="section-title"><span>📅</span> 3. Слот времени</div>
        <div class="slot-grid" id="booking-slots" role="radiogroup" aria-label="Слоты времени">
          <button type="button" class="slot-btn selected" role="radio" aria-checked="true" onclick="selectSlot(this, 'Завтра, 11:00')">Завтра 11:00</button>
          <button type="button" class="slot-btn" role="radio" aria-checked="false" onclick="selectSlot(this, 'Завтра, 14:30')">Завтра 14:30</button>
          <button type="button" class="slot-btn" role="radio" aria-checked="false" onclick="selectSlot(this, 'Завтра, 17:00')">Завтра 17:00</button>
          <button type="button" class="slot-btn" role="radio" aria-checked="false" onclick="selectSlot(this, 'Четверг, 12:00')">Четв 12:00</button>
          <button type="button" class="slot-btn" role="radio" aria-checked="false" onclick="selectSlot(this, 'Четверг, 15:30')">Четв 15:30</button>
          <button type="button" class="slot-btn" role="radio" aria-checked="false" onclick="selectSlot(this, 'Пятница, 16:00')">Пятн 16:00</button>
        </div>

        <div class="section-title"><span>📝</span> 4. Контакты</div>
        <label for="booking-name" class="field-label">Ваше имя:</label>
        <input type="text" class="input-field" id="booking-name" placeholder="Ваше имя" autocomplete="name">
        
        <label for="booking-phone" class="field-label">Номер телефона:</label>
        <input type="tel" class="input-field" id="booking-phone" placeholder="Номер телефона (+7…)" autocomplete="tel" inputmode="tel">
        
        <label for="booking-comment" class="field-label">Тема вопроса или комментарий:</label>
        <input type="text" class="input-field" id="booking-comment" placeholder="Тема вопроса или комментарий">

        <button type="button" class="btn-checkout" onclick="submitBooking()">Подтвердить запись 📅</button>
      </div>
    </section>

    <!-- TAB 3: AI STUDIO -->
    <section class="tab-content" id="tab-ai" role="tabpanel" aria-labelledby="tab-ai-btn">
      <div class="subnav" role="tablist">
        <button type="button" class="subnav-btn active" id="btn-sub-image" role="tab" aria-selected="true" onclick="switchAISub('image')">🎨 Генератор артов</button>
        <button type="button" class="subnav-btn" id="btn-sub-chat" role="tab" aria-selected="false" onclick="switchAISub('chat')">💬 Быстрый AI Чат</button>
        <button type="button" class="subnav-btn" id="btn-sub-gallery" role="tab" aria-selected="false" onclick="switchAISub('gallery')">🖼️ Медиа & История</button>
      </div>

      <!-- AI Sub: Image Gen -->
      <div id="ai-sub-image">
        <div class="ai-card">
          <label for="ai-img-prompt" class="field-label">Опишите желаемое изображение:</label>
          <textarea class="prompt-area" id="ai-img-prompt" placeholder="Например: Неоновый маскот Go Gopher в шлеме космического инженера…"></textarea>
          
          <div style="font-size: 12px; color: var(--muted); margin-bottom: 6px;">Быстрые стили:</div>
          <div class="chip-cloud" role="radiogroup" aria-label="Стили изображения">
            <span class="chip selected" role="radio" aria-checked="true" tabindex="0" onclick="selectStyle(this, 'Cyberpunk')">⚡ Киберпанк</span>
            <span class="chip" role="radio" aria-checked="false" tabindex="0" onclick="selectStyle(this, '3D Render')">🧊 3D Render</span>
            <span class="chip" role="radio" aria-checked="false" tabindex="0" onclick="selectStyle(this, 'Anime')">🌸 Аниме</span>
            <span class="chip" role="radio" aria-checked="false" tabindex="0" onclick="selectStyle(this, 'Photorealistic')">📷 Фотореализм</span>
            <span class="chip" role="radio" aria-checked="false" tabindex="0" onclick="selectStyle(this, 'Pixel Art')">👾 Pixel Art</span>
          </div>

          <button type="button" class="btn-action-purple" id="btn-gen-img" onclick="generateAIImage(false)">✨ Сгенерировать арт</button>

          <div class="image-preview-container" id="img-preview-box">
            <div id="img-placeholder-text" style="font-size: 13px; color: var(--muted); text-align: center; padding: 20px;">
              Здесь появится сгенерированное изображение
            </div>
            <img id="ai-generated-img" style="display: none;" src="" alt="Сгенерированное нейросетью изображение" width="512" height="512">
          </div>

          <button type="button" class="btn-checkout" id="btn-send-chat-img" style="display: none;" onclick="generateAIImage(true)">📲 Отправить изображение в чат</button>
        </div>
      </div>

      <!-- AI Sub: LLM Chat -->
      <div id="ai-sub-chat" style="display: none;">
        <div class="ai-card">
          <label for="ai-chat-model" class="field-label">Модель нейросети:</label>
          <select class="channel-select" id="ai-chat-model">
            <option value="auto">auto — Умная маршрутизация (Smart Gateway)</option>
            <option value="fast">fast — Быстрые ответы (Gemini 2.5 Flash)</option>
            <option value="best-coding">best-coding — Программирование и код (GLM / Claude)</option>
            <option value="reasoning">reasoning — Глубокое рассуждение (DeepSeek R1 / o1)</option>
          </select>

          <div style="font-size: 12px; color: var(--muted); margin-bottom: 6px;">Популярные вопросы:</div>
          <div class="chip-cloud">
            <span class="chip" tabindex="0" onclick="setChatPrompt('Как оформить заказ и доставку?')">🛍️ Доставка и заказ</span>
            <span class="chip" tabindex="0" onclick="setChatPrompt('Расскажи о бонусах и скидках')">🎁 Скидки и бонусы</span>
            <span class="chip" tabindex="0" onclick="setChatPrompt('Как работает онлайн-запись на консультацию?')">📅 Запись на услугу</span>
          </div>

          <div style="display: flex; gap: 8px;">
            <label for="ai-chat-prompt" class="sr-only">Вопрос нейросети</label>
            <input type="text" class="input-field" id="ai-chat-prompt" style="margin-bottom: 0;" placeholder="Задайте вопрос нейросети…">
            <button type="button" class="btn-chat-send" onclick="sendAIChat()">Отправить</button>
          </div>

          <div class="chat-bubble" id="ai-chat-result" style="display: none;" aria-live="polite"></div>
        </div>
      </div>

      <!-- AI Sub: Gallery & History -->
      <div id="ai-sub-gallery" style="display: none;">
        <div class="ai-card">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
            <div style="font-weight: 700; font-size: 15px;">📁 Медиатека и история</div>
            <button type="button" onclick="loadMediaHistory()" aria-label="Обновить историю" style="background: none; border: none; color: #a78bfa; font-size: 12px; cursor: pointer; display: flex; align-items: center; gap: 4px;">🔄 Обновить</button>
          </div>
          <p style="font-size: 12px; color: var(--muted); margin-bottom: 12px;">Сгенерированные изображения, транскрибация голоса и анализ документов из ваших диалогов с ботом.</p>
          
          <div class="chip-cloud" role="radiogroup" aria-label="Фильтр артефактов" style="margin-bottom: 12px;">
            <span class="chip selected" id="filter-chip-all" role="radio" aria-checked="true" tabindex="0" onclick="filterGalleryType('all', this)">Все материалы</span>
            <span class="chip" id="filter-chip-image" role="radio" aria-checked="false" tabindex="0" onclick="filterGalleryType('image', this)">🎨 Арт</span>
            <span class="chip" id="filter-chip-transcription" role="radio" aria-checked="false" tabindex="0" onclick="filterGalleryType('transcription', this)">🎙️ Голос (STT)</span>
            <span class="chip" id="filter-chip-document" role="radio" aria-checked="false" tabindex="0" onclick="filterGalleryType('document', this)">📄 Документы</span>
          </div>

          <div style="margin-bottom: 14px;">
            <label for="gallery-search" class="sr-only">Поиск по материалам</label>
            <input type="text" class="input-field" id="gallery-search" placeholder="🔍 Поиск по тексту или промпту…" oninput="filterGallerySearch(this.value)" style="margin-bottom: 0;">
          </div>

          <div id="gallery-items-container" aria-label="Список артефактов и истории" style="display: flex; flex-direction: column; gap: 10px;">
            <div style="font-size: 13px; color: var(--muted); text-align: center; padding: 24px;">Загрузка материалов…</div>
          </div>
        </div>
      </div>
    </section>

    <!-- TAB 4: STUDIO -->
    <section class="tab-content" id="tab-studio" role="tabpanel" aria-labelledby="tab-studio-btn">
      <div class="studio-card">
        <div class="demo-badge" id="studio-demo-badge">
          ℹ️ <b>SMM Студия каналов:</b> Генерация и разметка постов доступны всем. Публикация в реальные каналы защищена (доступна владельцу бота).
        </div>

        <label for="studio-channel" class="field-label">Целевой канал для публикации:</label>
        <select class="channel-select" id="studio-channel">
          <option value="-78368186601503">📢 Канал «Тест» (-78368186601503)</option>
          <option value="-78548596658207">📢 Канал «littfed» (-78548596658207)</option>
          <option value="0">🧪 Демо-канал (песочница)</option>
        </select>

        <label for="studio-prompt" class="field-label">Тема публикации или промпт:</label>
        <textarea class="prompt-area" id="studio-prompt" placeholder="Например: Анонс новой витрины Mini App и скидок на мерч Go Gopher…"></textarea>
        
        <button type="button" class="btn-ai-gen" onclick="generateAIPost()">✨ Сгенерировать текст через Gemini</button>

        <label for="studio-body" class="field-label">Текст поста (Markdown):</label>
        <textarea class="prompt-area" id="studio-body" style="height: 110px;"></textarea>

        <div style="display: flex; gap: 12px; margin-bottom: 12px; font-size: 13px;">
          <label><input type="checkbox" id="studio-pin" checked> Закрепить в канале 📌</label>
          <label><input type="checkbox" id="studio-rx" checked> Добавить реакции 👍🔥</label>
        </div>

        <div class="field-label">Предпросмотр поста в MAX:</div>
        <div class="preview-box" id="studio-preview">Текст поста появится здесь…</div>

        <button type="button" class="btn-publish" onclick="publishPostToChannel()">🚀 Опубликовать в канал</button>
      </div>
    </section>

    <!-- TAB 5: REWARDS & ORDERS -->
    <section class="tab-content" id="tab-orders" role="tabpanel" aria-labelledby="tab-orders-btn">
      <!-- Profile Card -->
      <div class="profile-card">
        <div class="profile-avatar" id="profile-avatar" aria-hidden="true">👤</div>
        <div class="profile-info">
          <div class="profile-name" id="profile-name">Пользователь</div>
          <div class="profile-id" id="profile-id">ID: —</div>
        </div>
        <div class="profile-badge" id="profile-status">Клиент MAX</div>
      </div>

      <!-- History of Orders & Bookings -->
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; margin-top: 14px;">
        <div style="font-weight: 700; font-size: 15px;">📦 Мои заказы и записи</div>
        <button type="button" onclick="loadOrders()" aria-label="Обновить список заказов" style="background: none; border: none; color: #60a5fa; font-size: 12px; cursor: pointer;">🔄 Обновить</button>
      </div>
      <div id="orders-list" aria-label="Список оформленных заказов" style="margin-bottom: 18px;">
        <div style="font-size: 13px; color: var(--muted); text-align: center; padding: 20px;">Заказы загружаются…</div>
      </div>

      <!-- Bonuses & Rewards -->
      <div style="font-weight: 700; font-size: 15px; margin-bottom: 10px; margin-top: 6px;">🎁 Бонусы и спецпредложения</div>

      <!-- Wheel of fortune -->
      <div class="wheel-card">
        <div style="font-weight: 700; font-size: 15px;">🎁 Колесо бонусов MAX</div>
        <p style="font-size: 12px; color: var(--muted); margin-top: 2px;">Испытайте удачу и получите промокод со скидкой до 25%!</p>
        <div><span class="wheel-visual" id="wheel-icon" aria-hidden="true">🎡</span></div>
        <button type="button" class="btn-spin" onclick="spinWheel()">Крутить колесо 🎯</button>
        <div id="wheel-result" style="font-size: 13px; font-weight: 600; color: #34d399; margin-top: 8px;" aria-live="polite"></div>
      </div>

      <!-- Interactive Quiz -->
      <div class="quiz-card" id="quiz-block">
        <div style="font-weight: 700; font-size: 15px; margin-bottom: 4px;">🧠 MAX Bot API Квиз</div>
        <p style="font-size: 12px; color: var(--muted); margin-bottom: 12px;">Ответьте правильно и активируйте эксклюзивную скидку 30%!</p>
        
        <div id="quiz-body">
          <div class="quiz-q" id="quiz-question">Загрузка вопроса…</div>
          <div class="quiz-opts" id="quiz-options"></div>
        </div>
        <div id="quiz-result" style="display: none; font-size: 13px; font-weight: 600; color: #34d399; margin-top: 8px;" aria-live="polite"></div>
      </div>
    </section>
  </main>

  <!-- Sticky Bottom MainButton (Thumb Zone) -->
  <button type="button" id="sticky-main-btn" class="sticky-main-btn" onclick="handleMainButtonAction()" aria-label="Оформить заказ">
    <span class="main-btn-left">
      <span class="main-btn-icon" aria-hidden="true">🛍️</span>
      <span>В корзине: <b id="main-btn-qty">0</b> шт.</span>
    </span>
    <span class="main-btn-right">
      <span id="main-btn-total">0 ₽</span>
      <span class="main-btn-arrow" aria-hidden="true">➔</span>
    </span>
  </button>

  <!-- Bottom Navigation -->
  <nav class="bottom-nav" role="tablist" aria-label="Главное меню витрины">
    <button type="button" class="nav-item active" id="tab-store-btn" role="tab" aria-selected="true" aria-controls="tab-store" onclick="switchTab('store', this)">
      <div class="nav-icon" aria-hidden="true">🛍️</div>
      <div>Магазин</div>
    </button>
    <button type="button" class="nav-item" id="tab-booking-btn" role="tab" aria-selected="false" aria-controls="tab-booking" onclick="switchTab('booking', this)">
      <div class="nav-icon" aria-hidden="true">📅</div>
      <div>Запись</div>
    </button>
    <button type="button" class="nav-item" id="tab-ai-btn" role="tab" aria-selected="false" aria-controls="tab-ai" onclick="switchTab('ai', this)">
      <div class="nav-icon" aria-hidden="true">✨</div>
      <div>AI Студия</div>
    </button>
    <button type="button" class="nav-item" id="nav-studio-btn" role="tab" aria-selected="false" aria-controls="tab-studio" onclick="switchTab('studio', this)">
      <div class="nav-icon" aria-hidden="true">✍️</div>
      <div>Студия</div>
    </button>
    <button type="button" class="nav-item" id="tab-orders-btn" role="tab" aria-selected="false" aria-controls="tab-orders" onclick="switchTab('orders', this)">
      <div class="nav-icon" aria-hidden="true">📦</div>
      <div>Заказы</div>
    </button>
  </nav>

  <!-- Product Modal -->
  <div class="modal-overlay" id="product-modal" role="dialog" aria-modal="true" aria-labelledby="modal-title" onclick="closeModal(event)">
    <div class="modal-card" onclick="event.stopPropagation()">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
        <h3 id="modal-title" style="font-size: 17px;">Товар</h3>
        <button type="button" onclick="closeModal()" aria-label="Закрыть" style="background: none; border: none; color: var(--muted); font-size: 18px; cursor: pointer; padding: 4px;">✕</button>
      </div>
      <div id="modal-desc" style="font-size: 13px; color: var(--muted); line-height: 1.5; margin-bottom: 16px;"></div>
      <div style="display: flex; justify-content: space-between; align-items: center;">
        <div id="modal-price" style="font-size: 18px; font-weight: 700; color: #38bdf8;">0 ₽</div>
        <button type="button" id="modal-export-btn" style="background: #3b82f6; color: white; border: none; padding: 8px 12px; border-radius: 8px; font-size: 12px; cursor: pointer;" onclick="exportProductToChannel()">📢 Анонсировать в канал</button>
      </div>
    </div>
  </div>

  <div id="toast-container" aria-live="polite" role="status"></div>

  <script>
    const OWNER_ID = 396393759;
    let currentUser = { id: 0, first_name: 'Пользователь', username: '' };
    let rawInitData = '';
    let discountPercent = 0;
    let activeModalItem = null;
    let selectedStyle = 'Cyberpunk';
    let currentArtworkBase64 = null;
    let currentActiveTab = 'store';

    // Booking state
    let selectedBookingService = { id: 'srv_audit', name: 'Архитектурный аудит Go / MAX', price: 5000 };
    let selectedBookingSlot = 'Завтра, 11:00';

    // Unified Haptic Feedback Engine
    function triggerHaptic(type) {
      try {
        if (window.Telegram && window.Telegram.WebApp && window.Telegram.WebApp.HapticFeedback) {
          const hf = window.Telegram.WebApp.HapticFeedback;
          if (type === 'light' || type === 'medium' || type === 'heavy' || type === 'rigid' || type === 'soft') {
            hf.impactOccurred(type);
            return;
          }
          if (type === 'success' || type === 'warning' || type === 'error') {
            hf.notificationOccurred(type);
            return;
          }
          if (type === 'selection') {
            hf.selectionChanged();
            return;
          }
        }
        if (window.Max && window.Max.WebApp && window.Max.WebApp.HapticFeedback) {
          window.Max.WebApp.HapticFeedback.impactOccurred(type);
          return;
        }
        if (navigator.vibrate) {
          if (type === 'success') navigator.vibrate([20, 50, 20]);
          else if (type === 'error') navigator.vibrate([50, 50, 50]);
          else if (type === 'heavy') navigator.vibrate([25, 40, 25]);
          else if (type === 'selection') navigator.vibrate(10);
          else navigator.vibrate(15);
        }
      } catch (e) {}
    }

    // Modern Toast Notification (replaces blocking browser alert)
    function showToast(text, type = 'info') {
      const container = document.getElementById('toast-container');
      if (!container) return;
      const toast = document.createElement('div');
      toast.className = 'toast-msg ' + type;
      let icon = 'ℹ️';
      if (type === 'success') icon = '✅';
      else if (type === 'error') icon = '⚠️';
      toast.innerHTML = '<span aria-hidden="true">' + icon + '</span><span>' + text + '</span>';
      container.appendChild(toast);
      setTimeout(() => {
        toast.classList.add('fade-out');
        setTimeout(() => toast.remove(), 300);
      }, 3000);
    }

    // Unsaved Changes / Cart Protection
    function updateClosingConfirmation() {
      let totalItems = 0;
      for (const q of Object.values(cart)) totalItems += q;
      const hasUnsaved = totalItems > 0;
      if (window.Telegram && window.Telegram.WebApp) {
        if (hasUnsaved && window.Telegram.WebApp.enableClosingConfirmation) {
          window.Telegram.WebApp.enableClosingConfirmation();
        } else if (!hasUnsaved && window.Telegram.WebApp.disableClosingConfirmation) {
          window.Telegram.WebApp.disableClosingConfirmation();
        }
      }
      if (window.Max && window.Max.WebApp && window.Max.WebApp.enableClosingConfirmation) {
        if (hasUnsaved) window.Max.WebApp.enableClosingConfirmation();
      }
    }

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
        const tg = (window.Telegram && window.Telegram.WebApp) || window.WebApp || (window.Max && window.Max.WebApp) || window.MAX;
        if (tg) {
          if (tg.ready) tg.ready();
          if (tg.expand) tg.expand();
          if (tg.disableVerticalSwipes) tg.disableVerticalSwipes();
          if (tg.initData) rawInitData = tg.initData;
          if (tg.initDataUnsafe && tg.initDataUnsafe.user) {
            currentUser = Object.assign({}, currentUser, tg.initDataUnsafe.user);
          }
        }

        let raw = window.location.search.substring(1);
        if (!raw && window.location.hash) {
          raw = window.location.hash.substring(1).replace(/^#/, '').replace(/^tgWebAppData=/, '');
        }
        if (!raw && rawInitData) {
          raw = rawInitData;
        } else if (raw && !rawInitData) {
          rawInitData = raw;
        }

        if (raw) {
          const params = new URLSearchParams(raw);
          const userStr = params.get('user');
          if (userStr) {
            try {
              currentUser = Object.assign({}, currentUser, JSON.parse(decodeURIComponent(userStr)));
            } catch (_) {
              try {
                currentUser = Object.assign({}, currentUser, JSON.parse(userStr));
              } catch (_) {}
            }
          } else if (params.get('user_id')) {
            currentUser.id = parseInt(params.get('user_id'), 10) || currentUser.id;
            currentUser.first_name = params.get('first_name') || currentUser.first_name;
            if (params.get('username')) currentUser.username = params.get('username');
          }
          if (params.get('promo')) {
            const promoEl = document.getElementById('promo-code');
            if (promoEl) {
              promoEl.value = params.get('promo');
              applyPromo();
            }
          }
          if (params.get('tab')) {
            const targetBtn = document.querySelector('[onclick*="' + params.get('tab') + '"]');
            if (targetBtn) switchTab(params.get('tab'), targetBtn);
          }
        }

        // LocalStorage cache persistence
        if (currentUser.id && currentUser.id !== 0) {
          try {
            localStorage.setItem('max_user', JSON.stringify(currentUser));
            if (rawInitData) localStorage.setItem('max_init_data', rawInitData);
          } catch (_) {}
        } else {
          try {
            const cachedUser = localStorage.getItem('max_user');
            if (cachedUser) {
              const u = JSON.parse(cachedUser);
              if (u && u.id) currentUser = Object.assign({}, currentUser, u);
            }
            const cachedInit = localStorage.getItem('max_init_data');
            if (cachedInit && !rawInitData) rawInitData = cachedInit;
          } catch (_) {}
        }
      } catch (e) {
        console.error('InitData parse error:', e);
      }

      if (currentUser.first_name) {
        const isOwner = currentUser.id === OWNER_ID;
        const nameEl = document.getElementById('user-display-name');
        if (nameEl) nameEl.innerText = currentUser.first_name;

        const badgeText = document.getElementById('user-badge-text');
        if (badgeText) {
          badgeText.innerText = isOwner ? 'Владелец ⭐' : (currentUser.username ? '@' + currentUser.username : 'Клиент');
        }

        const userBadge = document.getElementById('user-badge');
        if (userBadge) {
          userBadge.setAttribute('aria-label', 'Профиль: ' + currentUser.first_name);
        }

        const profName = document.getElementById('profile-name');
        if (profName) profName.innerText = currentUser.first_name + (currentUser.last_name ? ' ' + currentUser.last_name : '');
        const profID = document.getElementById('profile-id');
        if (profID) profID.innerText = currentUser.id ? 'MAX ID: ' + currentUser.id : 'MAX ID: Демо (откройте из бота)';
        const profStatus = document.getElementById('profile-status');
        if (profStatus) profStatus.innerText = isOwner ? 'Владелец ⭐' : 'Клиент MAX';

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
      if (!items || items.length === 0) {
        list.innerHTML = ` + "`" + `
          <div class="empty-state">
            <div class="empty-icon" aria-hidden="true">🔍</div>
            <div class="empty-title">Ничего не найдено</div>
            <p class="empty-desc">Попробуйте изменить поисковый запрос или выбрать другую категорию</p>
          </div>
        ` + "`" + `;
        return;
      }
      items.forEach(p => {
        const qty = cart[p.id] || 0;
        const div = document.createElement('div');
        div.className = 'item-card';
        div.innerHTML = ` + "`" + `
          <div class="item-main" onclick="openModal('${p.id}')">
            <div class="item-icon" aria-hidden="true">${p.icon}</div>
            <div>
              <div class="item-name">${p.name}</div>
              <div class="item-desc">${p.desc.substring(0, 42)}…</div>
              <div class="item-price">${p.price.toLocaleString('ru-RU')}&nbsp;₽</div>
            </div>
          </div>
          <div class="qty-controls">
            <button type="button" class="btn-qty" aria-label="Уменьшить количество" onclick="changeQty('${p.id}', -1)">−</button>
            <span class="qty-count" id="qty-${p.id}" aria-live="polite">${qty}</span>
            <button type="button" class="btn-qty" aria-label="Увеличить количество" onclick="changeQty('${p.id}', 1)">+</button>
          </div>
        ` + "`" + `;
        list.appendChild(div);
      });
    }

    let activeCategory = 'all';
    function setCategory(cat, btn) {
      triggerHaptic('selection');
      activeCategory = cat;
      document.querySelectorAll('.pill').forEach(b => {
        b.classList.remove('active');
        b.setAttribute('aria-selected', 'false');
      });
      btn.classList.add('active');
      btn.setAttribute('aria-selected', 'true');
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
      triggerHaptic('light');
      cart[id] = Math.max(0, (cart[id] || 0) + delta);
      const el = document.getElementById('qty-' + id);
      if (el) el.innerText = cart[id];
      updateTotals();
      updateClosingConfirmation();
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
      document.getElementById('cart-discount').innerText = (discount > 0 ? '−' + discount.toLocaleString('ru-RU') + ' ₽ (' + discountPercent + '%)' : '0 ₽');
      document.getElementById('cart-total').innerText = finalTotal.toLocaleString('ru-RU') + ' ₽';

      // Update Sticky MainButton
      const stickyBtn = document.getElementById('sticky-main-btn');
      const stickyQty = document.getElementById('main-btn-qty');
      const stickyTotal = document.getElementById('main-btn-total');
      if (stickyQty) stickyQty.innerText = count;
      if (stickyTotal) stickyTotal.innerText = finalTotal.toLocaleString('ru-RU') + ' ₽';

      if (stickyBtn) {
        if (count > 0 && currentActiveTab === 'store') {
          stickyBtn.classList.add('visible');
        } else {
          stickyBtn.classList.remove('visible');
        }
      }
    }

    function handleMainButtonAction() {
      triggerHaptic('medium');
      const cartBox = document.getElementById('cart-box-container');
      if (cartBox) {
        cartBox.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
      submitCart();
    }

    function applyPromo() {
      const code = document.getElementById('promo-code').value.trim().toUpperCase();
      if (code.includes('30') || code.includes('QUIZ')) {
        discountPercent = 30;
        showToast('Промокод применен: Скидка 30%!', 'success');
        triggerHaptic('success');
      } else if (code.includes('25') || code === 'PROMO-25-MAX') {
        discountPercent = 25;
        showToast('Промокод применен: Скидка 25%!', 'success');
        triggerHaptic('success');
      } else if (code.includes('15') || code.includes('LUCK')) {
        discountPercent = 15;
        showToast('Промокод применен: Скидка 15%!', 'success');
        triggerHaptic('success');
      } else if (code) {
        discountPercent = 10;
        showToast('Промокод применен: Скидка 10%!', 'success');
        triggerHaptic('success');
      }
      updateTotals();
    }

    async function submitCart() {
      let totalItems = 0;
      for (const q of Object.values(cart)) totalItems += q;
      if (totalItems === 0) {
        showToast('Корзина пуста. Добавьте товары кнопками +', 'error');
        triggerHaptic('error');
        return;
      }

      const currentInit = rawInitData || window.location.search.substring(1) || (localStorage.getItem('max_init_data') || '');
      const payload = {
        action: 'submit_cart',
        init_data: currentInit,
        payload: {
          items: cart,
          discount_percent: discountPercent,
          total_price: parseInt(document.getElementById('cart-total').innerText.replace(/[^0-9]/g, ''), 10)
        }
      };

      try {
        const res = await fetch('/api/webapp/action', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json; charset=utf-8' },
          body: JSON.stringify(payload)
        });
        const data = await res.json();
        if (data.success) {
          const sent = data.data && data.data.sent_to_chat;
          const orderNum = (data.data && data.data.order_id) || 'WEB';
          if (sent) {
            showToast('✅ Заказ оформлен! Номер: ' + orderNum + '. Чек отправлен в чат.', 'success');
          } else {
            showToast('✅ Заказ оформлен! Номер: ' + orderNum + '. Сохранен в истории витрины.', 'info');
          }
          triggerHaptic('success');
          // Clear cart on successful order
          for (const k in cart) cart[k] = 0;
          updateTotals();
          updateClosingConfirmation();
          renderCatalog(products);
          loadOrders();
        } else {
          showToast('Ошибка оформления: ' + (data.error || 'попробуйте снова'), 'error');
          triggerHaptic('error');
        }
      } catch (e) {
        showToast('Ошибка связи с сервером', 'error');
        triggerHaptic('error');
      }
    }

    function switchTab(tab, btn) {
      triggerHaptic('selection');
      currentActiveTab = tab;
      document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
      document.querySelectorAll('.nav-item').forEach(b => {
        b.classList.remove('active');
        b.setAttribute('aria-selected', 'false');
      });
      const target = document.getElementById('tab-' + tab);
      if (target) target.classList.add('active');
      if (btn) {
        btn.classList.add('active');
        btn.setAttribute('aria-selected', 'true');
      }
      
      // Update sticky button visibility based on active tab
      const stickyBtn = document.getElementById('sticky-main-btn');
      if (stickyBtn) {
        let count = 0;
        for (const q of Object.values(cart)) count += q;
        if (count > 0 && currentActiveTab === 'store') {
          stickyBtn.classList.add('visible');
        } else {
          stickyBtn.classList.remove('visible');
        }
      }

      if (tab === 'orders') loadOrders();
    }

    // Modal
    function openModal(id) {
      const p = products.find(x => x.id === id);
      if (!p) return;
      triggerHaptic('selection');
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
        showToast('В демо-режиме публикация доступна в демо-канал или владельцу бота.', 'info');
      }
      const res = await fetch('/api/webapp/publish_post', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json; charset=utf-8' },
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
        showToast('📢 Товар опубликован! ID сообщения: ' + data.message_id, 'success');
        triggerHaptic('success');
        closeModal();
      } else {
        showToast('Информация: ' + (data.error || 'Готово'), 'info');
      }
    }

    // Booking Handlers
    function selectService(el, id, name, price) {
      triggerHaptic('selection');
      document.querySelectorAll('.service-option').forEach(o => {
        o.classList.remove('selected');
        o.setAttribute('aria-checked', 'false');
      });
      el.classList.add('selected');
      el.setAttribute('aria-checked', 'true');
      selectedBookingService = { id, name, price };
    }

    function selectSlot(el, slot) {
      triggerHaptic('selection');
      document.querySelectorAll('.slot-btn').forEach(b => {
        b.classList.remove('selected');
        b.setAttribute('aria-checked', 'false');
      });
      el.classList.add('selected');
      el.setAttribute('aria-checked', 'true');
      selectedBookingSlot = slot;
    }

    async function submitBooking() {
      const name = document.getElementById('booking-name').value.trim() || currentUser.first_name;
      const phone = document.getElementById('booking-phone').value.trim();
      const spec = document.getElementById('booking-specialist').value;
      const comment = document.getElementById('booking-comment').value.trim();

      const currentInit = rawInitData || window.location.search.substring(1) || (localStorage.getItem('max_init_data') || '');
      const payload = {
        action: 'book_service',
        init_data: currentInit,
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
          headers: { 'Content-Type': 'application/json; charset=utf-8' },
          body: JSON.stringify(payload)
        });
        const data = await res.json();
        if (data.success) {
          const sent = data.data && data.data.sent_to_chat;
          const bookingNum = (data.data && data.data.booking_id) || 'BOK';
          if (sent) {
            showToast('📅 Запись успешно подтверждена! Номер: ' + bookingNum + ' на ' + selectedBookingSlot + '. Чек отправлен в чат.', 'success');
          } else {
            showToast('📅 Запись успешно подтверждена! Номер: ' + bookingNum + ' на ' + selectedBookingSlot, 'success');
          }
          triggerHaptic('success');
          loadOrders();
        } else {
          showToast('Ошибка записи: ' + (data.error || 'попробуйте снова'), 'error');
          triggerHaptic('error');
        }
      } catch (e) {
        showToast('Ошибка связи с сервером', 'error');
        triggerHaptic('error');
      }
    }

    // AI Studio Handlers
    let currentGalleryFilter = 'all';
    let allGalleryItems = [];

    function switchAISub(sub) {
      triggerHaptic('selection');
      document.getElementById('ai-sub-image').style.display = sub === 'image' ? 'block' : 'none';
      document.getElementById('ai-sub-chat').style.display = sub === 'chat' ? 'block' : 'none';
      document.getElementById('ai-sub-gallery').style.display = sub === 'gallery' ? 'block' : 'none';
      document.getElementById('btn-sub-image').classList.toggle('active', sub === 'image');
      document.getElementById('btn-sub-image').setAttribute('aria-selected', sub === 'image');
      document.getElementById('btn-sub-chat').classList.toggle('active', sub === 'chat');
      document.getElementById('btn-sub-chat').setAttribute('aria-selected', sub === 'chat');
      document.getElementById('btn-sub-gallery').classList.toggle('active', sub === 'gallery');
      document.getElementById('btn-sub-gallery').setAttribute('aria-selected', sub === 'gallery');

      if (sub === 'gallery') {
        loadMediaHistory();
      }
    }

    function filterGalleryType(type, el) {
      triggerHaptic('selection');
      document.querySelectorAll('#ai-sub-gallery .chip-cloud .chip').forEach(c => {
        c.classList.remove('selected');
        c.setAttribute('aria-checked', 'false');
      });
      if (el) {
        el.classList.add('selected');
        el.setAttribute('aria-checked', 'true');
      }
      currentGalleryFilter = type;
      renderGalleryItems();
    }

    function filterGallerySearch(query) {
      renderGalleryItems(query);
    }

    async function loadMediaHistory() {
      const container = document.getElementById('gallery-items-container');
      if (!container) return;
      container.innerHTML = '<div style="font-size: 13px; color: var(--muted); text-align: center; padding: 24px;">Загрузка материалов…</div>';

      try {
        const uid = (currentUser && currentUser.id) ? currentUser.id : 0;
        const res = await fetch('/api/webapp/history?user_id=' + uid + '&limit=50');
        const data = await res.json();
        if (data.success && Array.isArray(data.items)) {
          allGalleryItems = data.items;
          renderGalleryItems();
        } else {
          container.innerHTML = '<div style="font-size: 13px; color: var(--muted); text-align: center; padding: 20px;">История пуста</div>';
        }
      } catch (err) {
        container.innerHTML = '<div style="font-size: 13px; color: var(--danger); text-align: center; padding: 20px;">Не удалось загрузить историю</div>';
      }
    }

    function renderGalleryItems(searchQuery) {
      const container = document.getElementById('gallery-items-container');
      if (!container) return;

      const q = (searchQuery !== undefined ? searchQuery : (document.getElementById('gallery-search')?.value || '')).toLowerCase().trim();

      const filtered = allGalleryItems.filter(item => {
        if (currentGalleryFilter !== 'all' && item.artifact_type !== currentGalleryFilter) {
          return false;
        }
        if (q) {
          const matchTitle = (item.title || '').toLowerCase().includes(q);
          const matchPrompt = (item.prompt || '').toLowerCase().includes(q);
          const matchContent = (item.content_text || '').toLowerCase().includes(q);
          return matchTitle || matchPrompt || matchContent;
        }
        return true;
      });

      if (filtered.length === 0) {
        container.innerHTML = '<div style="font-size: 13px; color: var(--muted); text-align: center; padding: 30px; background: #111827; border-radius: 12px; border: 1px dashed var(--border);">' +
          '<div style="font-size: 28px; margin-bottom: 8px;">📭</div>' +
          '<b>В этом разделе пока ничего нет</b>' +
          '<p style="font-size: 12px; margin-top: 4px;">Сгенерируйте арт во вкладке выше или отправьте боту голосовое сообщение в чат MAX!</p>' +
          '</div>';
        return;
      }

      container.innerHTML = filtered.map(function(item) {
        var badgeHtml = '';
        var contentHtml = '';
        var createdDate = item.created_at ? new Date(item.created_at).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }) : '';

        if (item.artifact_type === 'image') {
          badgeHtml = '<span class="gallery-card-badge badge-image">🎨 Арт</span>';
          if (item.media_url) {
            contentHtml = '<div style="font-size: 13px; color: #e5e7eb; font-weight: 500;">«' + escapeHtml(item.prompt || item.title) + '»</div>' +
              '<img src="' + item.media_url + '" alt="Арт" class="gallery-card-img" loading="lazy">' +
              '<div class="gallery-card-actions">' +
              '<button type="button" class="btn-card-action" onclick="sendGalleryImageToChat(\'' + escapeQuotes(item.prompt || item.title) + '\', this)">📲 Отправить в чат</button>' +
              '<a href="' + item.media_url + '" download="max_artwork.png" class="btn-card-action" style="text-decoration: none;">💾 Скачать</a>' +
              '</div>';
          } else {
            contentHtml = '<div style="font-size: 13px; color: #e5e7eb;">«' + escapeHtml(item.prompt || item.title) + '»</div>';
          }
        } else if (item.artifact_type === 'transcription') {
          badgeHtml = '<span class="gallery-card-badge badge-stt">🎙️ Голос (STT)</span>';
          contentHtml = '<div class="gallery-card-body">' + escapeHtml(item.content_text || '') + '</div>' +
            '<div class="gallery-card-actions">' +
            '<button type="button" class="btn-card-action" onclick="copyGalleryText(\'' + escapeQuotes(item.content_text || '') + '\', this)">📋 Скопировать текст</button>' +
            '</div>';
        } else {
          badgeHtml = '<span class="gallery-card-badge badge-doc">📄 Документ</span>';
          contentHtml = '<div style="font-size: 13px; font-weight: 600; color: #f3f4f6; margin-bottom: 4px;">' + escapeHtml(item.title || 'Документ') + '</div>' +
            '<div class="gallery-card-body">' + escapeHtml(item.content_text || '') + '</div>' +
            '<div class="gallery-card-actions">' +
            '<button type="button" class="btn-card-action" onclick="copyGalleryText(\'' + escapeQuotes(item.content_text || '') + '\', this)">📋 Скопировать анализ</button>' +
            '</div>';
        }

        return '<div class="gallery-card">' +
          '<div class="gallery-card-header">' +
          badgeHtml +
          '<span>' + createdDate + '</span>' +
          '</div>' +
          contentHtml +
          '</div>';
      }).join('');
    }

    function copyGalleryText(text, btn) {
      triggerHaptic('selection');
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(() => {
          showToast('Текст скопирован в буфер обмена', 'success');
          if (btn) {
            const old = btn.innerHTML;
            btn.innerHTML = '✅ Скопировано!';
            setTimeout(() => { btn.innerHTML = old; }, 2000);
          }
        }).catch(() => {
          showToast('Не удалось скопировать', 'error');
        });
      } else {
        showToast('Буфер обмена недоступен в этом WebView', 'error');
      }
    }

    async function sendGalleryImageToChat(prompt, btn) {
      triggerHaptic('selection');
      if (btn) {
        btn.disabled = true;
        btn.innerText = '⏳ Отправка…';
      }
      try {
        const uid = (currentUser && currentUser.id) ? currentUser.id : 0;
        const res = await fetch('/api/webapp/generate_image', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            prompt: prompt,
            user_id: uid,
            send_to_chat: true
          })
        });
        const d = await res.json();
        if (d.success) {
          showToast('Изображение отправлено в чат MAX!', 'success');
          triggerHaptic('success');
        } else {
          showToast('Ошибка отправки: ' + (d.message || 'попробуйте позже'), 'error');
        }
      } catch (e) {
        showToast('Ошибка сети', 'error');
      } finally {
        if (btn) {
          btn.disabled = false;
          btn.innerText = '📲 Отправить в чат';
        }
      }
    }

    function escapeHtml(str) {
      if (!str) return '';
      return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
    }

    function escapeQuotes(str) {
      if (!str) return '';
      return String(str).replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/"/g, '&quot;').replace(/\n/g, '\\n').replace(/\r/g, '');
    }

    function selectStyle(el, style) {
      triggerHaptic('selection');
      document.querySelectorAll('.chip-cloud .chip').forEach(c => {
        c.classList.remove('selected');
        c.setAttribute('aria-checked', 'false');
      });
      el.classList.add('selected');
      el.setAttribute('aria-checked', 'true');
      selectedStyle = style;
    }

    async function generateAIImage(sendToChat) {
      const prompt = document.getElementById('ai-img-prompt').value.trim() || 'Маскот Go Gopher в футуристичном шлеме';
      const btnGen = document.getElementById('btn-gen-img');
      const placeholder = document.getElementById('img-placeholder-text');
      const imgEl = document.getElementById('ai-generated-img');
      const btnSend = document.getElementById('btn-send-chat-img');

      btnGen.innerText = sendToChat ? '⏳ Отправляем в чат…' : '⏳ Нейросеть генерирует арт…';
      btnGen.disabled = true;

      try {
        const res = await fetch('/api/webapp/generate_image', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json; charset=utf-8' },
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
          loadMediaHistory();
          if (sendToChat) {
            if (data.sent_to_chat) {
              showToast('🎨 Арт успешно отправлен в ваш чат мессенджера MAX!', 'success');
              triggerHaptic('success');
            } else {
              showToast(data.message ? '⚠️ ' + data.message : '⚠️ Арт создан, но не удалось отправить в чат (откройте по ссылке из бота)', 'warning');
              triggerHaptic('warning');
            }
          } else {
            triggerHaptic('light');
          }
        } else {
          showToast('Не удалось сгенерировать изображение', 'error');
          triggerHaptic('error');
        }
      } catch (e) {
        showToast('Ошибка связи с AI Gateway', 'error');
        triggerHaptic('error');
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
      
      triggerHaptic('light');
      resBubble.style.display = 'block';
      resBubble.innerText = '⏳ Думаю… (' + model + ')';

      try {
        const res = await fetch('/api/webapp/ai_chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json; charset=utf-8' },
          body: JSON.stringify({
            prompt: prompt,
            model: model,
            user_id: currentUser.id
          })
        });
        const data = await res.json();
        if (data.success && data.text) {
          resBubble.innerHTML = '<div style="font-size: 11px; color: #60a5fa; margin-bottom: 6px;">🧠 ' + data.model + ' • ' + (data.latency_ms || 350) + 'ms</div>' + data.text;
          triggerHaptic('light');
        } else {
          resBubble.innerText = 'Ошибка получения ответа';
          triggerHaptic('error');
        }
      } catch (e) {
        resBubble.innerText = 'Ошибка связи с сервером AI';
        triggerHaptic('error');
      }
    }

    // Studio AI & Publish
    async function generateAIPost() {
      const prompt = document.getElementById('studio-prompt').value.trim() || 'Анонс магазина и новых поступлений мерча MAX';
      document.getElementById('studio-preview').innerText = '⏳ Gemini генерирует пост…';
      triggerHaptic('light');
      try {
        const res = await fetch('/api/webapp/generate_post', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json; charset=utf-8' },
          body: JSON.stringify({ prompt: prompt, user_id: currentUser.id })
        });
        const data = await res.json();
        if (data.success && data.text) {
          document.getElementById('studio-body').value = data.text;
          document.getElementById('studio-preview').innerText = data.text;
          triggerHaptic('success');
        } else {
          const fallback = '🚀 **Обновление витрины MAX Store!**\n\nЭксклюзивный мерч для разработчиков и участников сообщества доступен в Mini App.\n\n• Доставка по РФ\n• Оплата через СБП\n• Скидка 15% по промокоду PROMO-15-MAX';
          document.getElementById('studio-body').value = fallback;
          document.getElementById('studio-preview').innerText = fallback;
        }
      } catch (e) {
        document.getElementById('studio-preview').innerText = 'Ошибка связи с AI Gateway';
        triggerHaptic('error');
      }
    }

    document.getElementById('studio-body').addEventListener('input', function() {
      document.getElementById('studio-preview').innerText = this.value || 'Текст поста появится здесь…';
    });

    async function publishPostToChannel() {
      const text = document.getElementById('studio-body').value.trim();
      if (!text) {
        showToast('Введите текст публикации', 'error');
        triggerHaptic('error');
        return;
      }
      const channelID = parseInt(document.getElementById('studio-channel').value, 10);
      const pin = document.getElementById('studio-pin').checked;
      const rx = document.getElementById('studio-rx').checked;

      if (currentUser.id !== OWNER_ID && channelID !== 0) {
        showToast('В демо-режиме пост промодерирован и симулирован!', 'info');
        return;
      }

      const res = await fetch('/api/webapp/publish_post', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json; charset=utf-8' },
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
        showToast('🚀 Пост успешно опубликован в канале (ID: ' + data.message_id + ')!', 'success');
        triggerHaptic('success');
      } else {
        showToast('Ответ сервера: ' + (data.error || 'Успешно'), 'info');
      }
    }

    // Wheel of fortune
    async function spinWheel() {
      triggerHaptic('heavy');
      const icon = document.getElementById('wheel-icon');
      icon.style.transform = 'rotate(' + (1440 + Math.floor(Math.random() * 360)) + 'deg)';
      try {
        const res = await fetch('/api/webapp/spin', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json; charset=utf-8' },
          body: JSON.stringify({ user_id: currentUser.id })
        });
        const data = await res.json();
        setTimeout(() => {
          showToast('🎉 Победа! Ваш промокод: ' + data.code + ' (скидка ' + data.discount + '%)!', 'success');
          triggerHaptic('success');
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
        res.innerHTML = '🏆 <b>Квиз пройден: ' + quizScore + ' из ' + quizQuestions.length + '!</b><br>Ваш промокод: <code style="background: rgba(16,185,129,0.2); padding: 2px 6px; border-radius: 4px;">QUIZ-PRO-30</code> (скидка 30%)<br><button type="button" onclick="applyQuizPromo()" style="margin-top: 8px; background: #10b981; color: white; border: none; padding: 6px 12px; border-radius: 6px; font-size: 12px; cursor: pointer;">Применить к корзине</button>';
        return;
      }
      const item = quizQuestions[currentQuizIndex];
      document.getElementById('quiz-question').innerText = item.q;
      const optsBox = document.getElementById('quiz-options');
      optsBox.innerHTML = '';
      item.options.forEach((opt, idx) => {
        const btn = document.createElement('button');
        btn.type = 'button';
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
        triggerHaptic('success');
      } else {
        btn.classList.add('wrong');
        triggerHaptic('error');
      }
      setTimeout(() => {
        currentQuizIndex++;
        renderQuiz();
      }, 700);
    }

    function applyQuizPromo() {
      triggerHaptic('selection');
      document.getElementById('promo-code').value = 'QUIZ-PRO-30';
      applyPromo();
      switchTab('store', document.querySelector('.nav-item'));
    }

    // Orders List with Shimmer Skeletons and Empty States
    async function loadOrders() {
      const box = document.getElementById('orders-list');
      
      // Render Shimmer Skeletons while fetching
      box.innerHTML = ` + "`" + `
        <div class="skeleton-card"><div class="skeleton skeleton-img"></div><div class="skeleton-lines"><div class="skeleton skeleton-line"></div><div class="skeleton skeleton-line short"></div></div></div>
        <div class="skeleton-card"><div class="skeleton skeleton-img"></div><div class="skeleton-lines"><div class="skeleton skeleton-line"></div><div class="skeleton skeleton-line short"></div></div></div>
        <div class="skeleton-card"><div class="skeleton skeleton-img"></div><div class="skeleton-lines"><div class="skeleton skeleton-line"></div><div class="skeleton skeleton-line short"></div></div></div>
      ` + "`" + `;

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
              <div style="font-size: 13px; color: var(--muted); margin-bottom: 4px;">Сумма: <b>${o.total > 0 ? o.total.toLocaleString('ru-RU') + '&nbsp;₽' : 'Бесплатно'}</b></div>
              <div style="font-size: 11px; color: #64748b;">${new Date(o.created_at).toLocaleString('ru-RU')}</div>
            ` + "`" + `;
            box.appendChild(div);
          });
        } else {
          box.innerHTML = ` + "`" + `
            <div class="empty-state">
              <div class="empty-icon" aria-hidden="true">📦</div>
              <div class="empty-title">У вас пока нет заказов</div>
              <p class="empty-desc">Здесь будут отображаться ваши покупки мерча и подтвержденные записи на услуги</p>
              <button type="button" class="btn-empty-action" onclick="switchTab('store', document.getElementById('tab-store-btn'))">Перейти в магазин</button>
            </div>
          ` + "`" + `;
        }
      } catch (e) {
        box.innerHTML = ` + "`" + `
          <div class="empty-state">
            <div class="empty-icon" aria-hidden="true">⚠️</div>
            <div class="empty-title">Не удалось загрузить историю</div>
            <p class="empty-desc">Проверьте соединение или обновите список кнопкой выше</p>
          </div>
        ` + "`" + `;
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
