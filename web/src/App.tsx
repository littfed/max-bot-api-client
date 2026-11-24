import React, { useState, useEffect } from 'react';
import { Header } from './components/common/Header';
import { BottomBar } from './components/common/BottomBar';
import { ToastContainer } from './components/common/Toast';
import { LeadModal } from './components/common/LeadModal';
import { MaintenanceModal } from './components/hub/MaintenanceModal';
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

// Default 8 modes list conforming to the Bot Hub spec & mockups
const initial8Modes: ModeItem[] = [
  { id: 'shop', title: 'Магазин', is_active: true, daily_quota: 100 },
  { id: 'reception', title: 'Ресепшен', is_active: true, daily_quota: 100 },
  {
    id: 'reminders',
    title: 'Напоминания',
    is_active: false,
    maintenance_message: 'Модуль «Напоминания» временно отключён администратором для обновления. Скоро вернётся в работу!',
    daily_quota: 100,
  },
  {
    id: 'channels',
    title: 'Каналы',
    is_active: false,
    maintenance_message: 'Модуль «Каналы» временно отключён администратором для обновления. Скоро вернётся в работу!',
    daily_quota: 100,
  },
  {
    id: 'giveaways',
    title: 'Конкурсы',
    is_active: false,
    maintenance_message: 'Модуль «Конкурсы» временно отключён администратором для обновления. Скоро вернётся в работу!',
    daily_quota: 100,
  },
  { id: 'support', title: 'Помощь', is_active: true, daily_quota: 100 },
  {
    id: 'docs',
    title: 'Документы',
    is_active: false,
    maintenance_message: 'Модуль «Документы» временно отключён администратором для обновления. Скоро вернётся в работу!',
    daily_quota: 100,
  },
  { id: 'chat', title: 'Общение', is_active: true, daily_quota: 100 },
];

export const App: React.FC = () => {
  const [user, setUser] = useState<User | null>({
    id: 1,
    active_mode_id: 'chat',
    role: 'admin', // Allows viewing admin button in demo
    timezone: 'UTC+3',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  });

  const [modes, setModes] = useState<ModeItem[]>(initial8Modes);
  const [activeModeId, setActiveModeId] = useState<string>('chat');
  const [currentView, setCurrentView] = useState<string>('hub'); // 'hub' | 'admin' | mode_id
  const [testedModes, setTestedModes] = useState<string[]>(['chat']);

  // Mascot state with speech cloud text from mockup
  const [mascot, setMascot] = useState<MascotState>({
    phrase: 'Загляни в Документы — разберу любой чек за 3 сек!',
    priority: 4,
    emotion: 'friendly',
  });

  // Modals state
  const [isLeadModalOpen, setIsLeadModalOpen] = useState(false);
  const [maintenanceTargetMode, setMaintenanceTargetMode] = useState<ModeItem | null>(null);
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  const [isLoadingBootstrap, setIsLoadingBootstrap] = useState(true);

  // Initialize Telegram/MAX WebApp & Bootstrap
  useEffect(() => {
    const notifyReady = () => {
      if (typeof window !== 'undefined') {
        const win = window as any;
        try {
          if (win.WebApp && typeof win.WebApp.ready === 'function') {
            win.WebApp.ready();
          }
          if (win.Telegram?.WebApp && typeof win.Telegram.WebApp.ready === 'function') {
            win.Telegram.WebApp.ready();
            win.Telegram.WebApp.expand?.();
          }
          if (win.max && typeof win.max.ready === 'function') {
            win.max.ready();
          }
          if (win.MaxWebApp && typeof win.MaxWebApp.ready === 'function') {
            win.MaxWebApp.ready();
          }
        } catch (e) {
          console.error('Error invoking ready:', e);
        }
      }
    };

    notifyReady();
    loadBootstrap().finally(() => {
      notifyReady();
    });
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
      console.warn('Bootstrap API call offline or unauthenticated, using fallback state:', err);
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
    }, 4000);
  };

  const handleDismissToast = (id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  };

  // Mode Selection
  const handleSelectMode = async (mode: ModeItem) => {
    setTestedModes((prev) => Array.from(new Set([...prev, mode.id])));

    if (activeModeId !== mode.id) {
      try {
        await apiClient.setUserMode(mode.id);
        setActiveModeId(mode.id);
      } catch (_) {
        setActiveModeId(mode.id);
      }
    }

    setCurrentView(mode.id);
  };

  // Maintenance mode tap handling: open Bottom Sheet matching Mockup 3
  const handleMaintenanceClick = (mode: ModeItem) => {
    setMaintenanceTargetMode(mode);
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
      const phrases = [
        'Загляни в Документы — разберу любой чек за 3 сек!',
        'В Магазине работает демо-оплата с генерацией QR-кода!',
        'На Ресепшене можно забронировать слот времени без овербукинга.',
        'В Поддержке подключена база знаний с эскалацией на оператора.',
        'В Общении можно сменить роль ИИ на Маркетолога или Критика!',
      ];
      setMascot({
        phrase: phrases[Math.floor(Math.random() * phrases.length)],
        priority: 4,
        emotion: 'happy',
      });
    }
  };

  return (
    <div className="min-h-screen bg-[#f8fafc] text-slate-900 flex flex-col justify-between selection:bg-teal-500/20 selection:text-teal-900">
      {/* Sticky Top Header (Dark Navy #0b1329 matching mockups) */}
      <Header
        user={user}
        currentView={currentView}
        modes={modes}
        onOpenLeadModal={() => setIsLeadModalOpen(true)}
        onOpenAdmin={() => setCurrentView('admin')}
        onReturnToHub={() => setCurrentView('hub')}
      />

      {/* Main Content Area */}
      <main className={`flex-1 ${currentView === 'chat' ? 'flex flex-col min-h-0' : 'pb-4 safe-bottom'}`}>
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

      {/* Sticky Bottom Bar (Active Mode + Back to Chat + Admin Button) */}
      <BottomBar
        user={user}
        modes={modes}
        activeModeId={activeModeId}
        currentView={currentView}
        onOpenAdmin={() => setCurrentView('admin')}
        onOpenChat={() => {
          setCurrentView('chat');
          const chatMode = modes.find((m) => m.id === 'chat');
          if (chatMode && activeModeId !== 'chat') {
            handleSelectMode(chatMode);
          }
        }}
      />

      {/* Maintenance Bottom Sheet Modal matching Mockup 3 */}
      <MaintenanceModal
        isOpen={Boolean(maintenanceTargetMode)}
        mode={maintenanceTargetMode}
        onClose={() => setMaintenanceTargetMode(null)}
        onRequestNotify={(mode) => {
          setIsLeadModalOpen(true);
        }}
      />

      {/* Lead Modal ("Хочу такого бота") */}
      <LeadModal
        isOpen={isLeadModalOpen}
        onClose={() => setIsLeadModalOpen(false)}
        sourceMode={currentView === 'hub' ? activeModeId : currentView}
        testedModes={testedModes}
        onSuccessToast={(msg) => addToast('success', msg)}
      />

      {/* Toasts */}
      <ToastContainer toasts={toasts} onDismiss={handleDismissToast} />
    </div>
  );
};

export default App;
