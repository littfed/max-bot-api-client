import React, { useState, useEffect } from 'react';
import { Header } from './components/common/Header';
import { ToastContainer } from './components/common/Toast';
import { LeadModal } from './components/common/LeadModal';
import { HubGrid } from './components/hub/HubGrid';
import { ShopView } from './components/modes/ShopView';
import { ReceptionView } from './components/modes/ReceptionView';
import { SupportView } from './components/modes/SupportView';
import { ChatView } from './components/modes/ChatView';
import { AdminPanel } from './components/admin/AdminPanel';
import { apiClient } from './api/client';
import {
  User,
  ModeItem,
  MascotState,
  ToastMessage,
  BootstrapResponse,
} from './types';

// Default 8 modes list conforming to the Bot Hub spec
const initial8Modes: ModeItem[] = [
  { id: 'shop', title: 'Магазин', is_active: true, daily_quota: 100 },
  { id: 'reception', title: 'Ресепшен', is_active: true, daily_quota: 100 },
  {
    id: 'reminders',
    title: 'Напоминания',
    is_active: false,
    maintenance_message: 'Модуль находится в разработке (Этап 2)',
    daily_quota: 100,
  },
  {
    id: 'channels',
    title: 'Каналы',
    is_active: false,
    maintenance_message: 'Модуль находится в разработке (Этап 2)',
    daily_quota: 100,
  },
  {
    id: 'giveaways',
    title: 'Конкурсы',
    is_active: false,
    maintenance_message: 'Модуль находится в разработке (Этап 2)',
    daily_quota: 100,
  },
  { id: 'support', title: 'Поддержка', is_active: true, daily_quota: 100 },
  {
    id: 'docs',
    title: 'Документы',
    is_active: false,
    maintenance_message: 'Модуль находится в разработке (Этап 2)',
    daily_quota: 100,
  },
  { id: 'chat', title: 'Общение', is_active: true, daily_quota: 100 },
];

export const App: React.FC = () => {
  const [user, setUser] = useState<User | null>({
    id: 1,
    active_mode_id: 'shop',
    role: 'admin', // Allows viewing admin by default in dev/demo
    timezone: 'UTC+3',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  });

  const [modes, setModes] = useState<ModeItem[]>(initial8Modes);
  const [activeModeId, setActiveModeId] = useState<string>('shop');
  const [currentView, setCurrentView] = useState<string>('hub'); // 'hub' | 'admin' | mode_id
  const [testedModes, setTestedModes] = useState<string[]>(['shop']);

  const [mascot, setMascot] = useState<MascotState>({
    phrase: 'Привет! Я бот-маскот MAX. Нажмите на любой режим, чтобы начать тестирование.',
    priority: 4,
    emotion: 'friendly',
  });

  const [isLeadModalOpen, setIsLeadModalOpen] = useState(false);
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  const [isLoadingBootstrap, setIsLoadingBootstrap] = useState(true);

  // Initialize Telegram WebApp / Bootstrap
  useEffect(() => {
    if (typeof window !== 'undefined' && window.Telegram?.WebApp) {
      window.Telegram.WebApp.ready?.();
      window.Telegram.WebApp.expand?.();
    }

    loadBootstrap();
  }, []);

  const loadBootstrap = async () => {
    try {
      setIsLoadingBootstrap(true);
      const data: BootstrapResponse = await apiClient.getBootstrap();
      if (data.user) setUser(data.user);
      if (data.active_mode) setActiveModeId(data.active_mode);
      if (data.modes && data.modes.length > 0) setModes(data.modes);
      if (data.mascot) setMascot(data.mascot);
    } catch (err: any) {
      console.warn('Bootstrap API call failed or offline, using fallback state:', err);
    } finally {
      setIsLoadingBootstrap(false);
    }
  };

  // Toast helpers
  const addToast = (type: 'info' | 'success' | 'warning' | 'error', text: string) => {
    const id = Math.random().toString(36).substring(2, 9);
    setToasts((prev) => [...prev, { id, type, text }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 4500);
  };

  const handleDismissToast = (id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  };

  // Mode Selection
  const handleSelectMode = async (mode: ModeItem) => {
    // Record tested mode
    setTestedModes((prev) => Array.from(new Set([...prev, mode.id])));

    try {
      // Synchronize active mode with backend
      await apiClient.setUserMode(mode.id);
      setActiveModeId(mode.id);
    } catch (_) {
      // In offline mode, still activate locally
      setActiveModeId(mode.id);
    }

    setCurrentView(mode.id);
  };

  // Maintenance mode tap handling
  const handleMaintenanceClick = (mode: ModeItem) => {
    const msg =
      mode.maintenance_message ||
      "Модуль находится в разработке (Этап 2). Оставьте заявку через 'Хочу такого бота'";
    addToast('warning', msg);
  };

  // Mascot tap handling
  const handleTapMascot = async () => {
    try {
      const res = await apiClient.tapMascot();
      setMascot({
        phrase: res.phrase,
        priority: res.priority || 4,
        emotion: (res.emotion as any) || 'happy',
      });
    } catch (_) {
      const randomPhrases = [
        'Попробуйте модуль Магазина — там есть демо-симуляция оплаты с QR-кодом!',
        'На Ресепшене можно забронировать слот времени с защитой от овербукинга.',
        'В Поддержке работает RAG-поиск с эскалацией на живого оператора.',
        'В Общении можно сменить роль LLM на Маркетолога или Критика!',
      ];
      setMascot({
        phrase: randomPhrases[Math.floor(Math.random() * randomPhrases.length)],
        priority: 4,
        emotion: 'happy',
      });
    }
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col justify-between selection:bg-indigo-500/30 selection:text-indigo-200">
      {/* Sticky Header */}
      <Header
        user={user}
        currentView={currentView}
        modes={modes}
        onOpenLeadModal={() => setIsLeadModalOpen(true)}
        onOpenAdmin={() => setCurrentView('admin')}
        onReturnToHub={() => setCurrentView('hub')}
      />

      {/* Main Body */}
      <main className="flex-1 pb-10 safe-bottom">
        {currentView === 'hub' && (
          <HubGrid
            modes={modes}
            activeModeId={activeModeId}
            mascot={mascot}
            onSelectMode={handleSelectMode}
            onMaintenanceClick={handleMaintenanceClick}
            onTapMascot={handleTapMascot}
          />
        )}

        {currentView === 'admin' && (
          <AdminPanel
            onBack={() => setCurrentView('hub')}
            onSuccessToast={(msg) => addToast('success', msg)}
            onErrorToast={(msg) => addToast('error', msg)}
          />
        )}

        {currentView === 'shop' && (
          <ShopView
            onBack={() => setCurrentView('hub')}
            onSuccessToast={(msg) => addToast('success', msg)}
            onErrorToast={(msg) => addToast('error', msg)}
          />
        )}

        {currentView === 'reception' && (
          <ReceptionView
            onBack={() => setCurrentView('hub')}
            onSuccessToast={(msg) => addToast('success', msg)}
            onErrorToast={(msg) => addToast('error', msg)}
          />
        )}

        {currentView === 'support' && (
          <SupportView
            currentActiveMode={activeModeId}
            onBack={() => setCurrentView('hub')}
            onSuccessToast={(msg) => addToast('success', msg)}
            onErrorToast={(msg) => addToast('error', msg)}
          />
        )}

        {currentView === 'chat' && (
          <ChatView
            onBack={() => setCurrentView('hub')}
            onSuccessToast={(msg) => addToast('success', msg)}
            onErrorToast={(msg) => addToast('error', msg)}
          />
        )}
      </main>

      {/* Global Modals & Toasts */}
      <LeadModal
        isOpen={isLeadModalOpen}
        onClose={() => setIsLeadModalOpen(false)}
        sourceMode={currentView === 'hub' ? activeModeId : currentView}
        testedModes={testedModes}
        onSuccessToast={(msg) => addToast('success', msg)}
      />

      <ToastContainer toasts={toasts} onDismiss={handleDismissToast} />
    </div>
  );
};

export default App;
