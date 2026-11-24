import React from 'react';
import { MessageSquare, CornerDownLeft, Settings } from 'lucide-react';
import { ModeItem, User } from '../../types';

interface BottomBarProps {
  user: User | null;
  modes: ModeItem[];
  activeModeId: string;
  currentView: string;
  onOpenAdmin: () => void;
  onOpenChat: () => void;
}

export const BottomBar: React.FC<BottomBarProps> = ({
  user,
  modes,
  activeModeId,
  currentView,
  onOpenAdmin,
  onOpenChat,
}) => {
  // Find active mode title (default to Общение)
  const activeMode = modes.find((m) => m.id === activeModeId);
  const activeTitle = activeMode ? activeMode.title : 'Общение';

  const isAlreadyInChat = currentView === 'chat';

  // Handle click on chat/return button
  const handleChatAction = () => {
    if (!isAlreadyInChat) {
      onOpenChat();
      return;
    }

    // If already in chat, close the webapp window to return to MAX messenger
    if (typeof window !== 'undefined') {
      const win = window as any;
      if (typeof win.WebApp?.close === 'function') {
        win.WebApp.close();
      } else if (win.Telegram?.WebApp?.close) {
        win.Telegram.WebApp.close();
      } else if (typeof win.max?.close === 'function') {
        win.max.close();
      } else if (typeof win.close === 'function') {
        win.close();
      }
    }
  };

  return (
    <footer className="sticky bottom-0 z-30 bg-white/95 backdrop-blur-md border-t border-slate-200/90 shadow-lg px-4 py-2.5 safe-bottom">
      <div className="max-w-md sm:max-w-lg mx-auto flex items-center justify-between gap-3">
        {/* Left: Active Mode Indicator with Green Dot */}
        <div className="flex items-center gap-2">
          <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
          <span className="text-xs sm:text-sm font-semibold text-slate-800">
            Режим: <span className="text-slate-900">{activeTitle}</span>
          </span>
        </div>

        {/* Right: Actions */}
        <div className="flex items-center gap-2">
          {/* Chat / Messenger button */}
          <button
            onClick={handleChatAction}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs sm:text-sm font-medium border transition-all shadow-sm active:scale-95 ${
              isAlreadyInChat
                ? 'bg-slate-100 hover:bg-slate-200 text-slate-700 border-slate-200/80'
                : 'bg-teal-50 hover:bg-teal-100 text-teal-800 border-teal-200/80'
            }`}
            title={isAlreadyInChat ? 'Закрыть окно мини-апп и вернуться в чат' : 'Перейти в диалог с ИИ-ассистентом'}
          >
            {isAlreadyInChat ? (
              <>
                <span>В мессенджер</span>
                <CornerDownLeft className="w-3.5 h-3.5 text-slate-500" />
              </>
            ) : (
              <>
                <MessageSquare className="w-3.5 h-3.5 text-teal-600" />
                <span>В чат</span>
              </>
            )}
          </button>

          {/* Admin panel button - always visible in bottom-right corner */}
          <button
            onClick={onOpenAdmin}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs sm:text-sm font-medium border transition-all shadow-sm active:scale-95 ${
              currentView === 'admin'
                ? 'bg-amber-100 border-amber-300 text-amber-900'
                : 'bg-amber-50 hover:bg-amber-100 border-amber-200 text-amber-800'
            }`}
            title="Панель администратора"
          >
            <Settings className="w-3.5 h-3.5 text-amber-700" />
            <span>Админ</span>
          </button>
        </div>
      </div>
    </footer>
  );
};
