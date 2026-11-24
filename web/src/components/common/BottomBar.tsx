import React from 'react';
import { CornerDownLeft, Settings } from 'lucide-react';
import { ModeItem, User } from '../../types';

interface BottomBarProps {
  user: User | null;
  modes: ModeItem[];
  activeModeId: string;
  currentView: string;
  onOpenAdmin: () => void;
}

export const BottomBar: React.FC<BottomBarProps> = ({
  user,
  modes,
  activeModeId,
  currentView,
  onOpenAdmin,
}) => {
  const isAdmin = user && (user.role === 'admin' || user.role === 'superadmin');

  // Find active mode title
  const activeMode = modes.find((m) => m.id === activeModeId);
  const activeTitle = activeMode ? activeMode.title : 'Магазин';

  // Return to bot chat (closing webapp in Telegram or MAX)
  const handleReturnToChat = () => {
    if (typeof window !== 'undefined') {
      if (window.Telegram?.WebApp?.close) {
        window.Telegram.WebApp.close();
      } else if ((window as any).max?.close) {
        (window as any).max.close();
      } else {
        alert('Возврат в чат бота (в клиенте Telegram или MAX это окно закроется)');
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
          {/* Back to Chat button */}
          <button
            onClick={handleReturnToChat}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-slate-100 hover:bg-slate-200 active:scale-95 text-slate-700 text-xs sm:text-sm font-medium border border-slate-200/80 transition-all shadow-sm"
          >
            <span>В чат</span>
            <CornerDownLeft className="w-3.5 h-3.5 text-slate-500" />
          </button>

          {/* Admin panel button if user has permissions */}
          {isAdmin && (
            <button
              onClick={onOpenAdmin}
              className={`flex items-center gap-1 px-2.5 py-1.5 rounded-xl text-xs sm:text-sm font-medium border transition-all shadow-sm ${
                currentView === 'admin'
                  ? 'bg-amber-100 border-amber-300 text-amber-900'
                  : 'bg-amber-50 hover:bg-amber-100 border-amber-200 text-amber-800'
              }`}
              title="Панель администратора"
            >
              <Settings className="w-3.5 h-3.5 text-amber-700" />
              <span>Админ</span>
            </button>
          )}
        </div>
      </div>
    </footer>
  );
};
