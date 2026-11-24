import React from 'react';
import { ArrowLeft, ArrowUpRight } from 'lucide-react';
import { User, ModeItem } from '../../types';

interface HeaderProps {
  user: User | null;
  currentView: string; // 'hub' | 'admin' | mode_id
  modes: ModeItem[];
  onOpenLeadModal: () => void;
  onOpenAdmin: () => void;
  onReturnToHub: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  user,
  currentView,
  modes,
  onOpenLeadModal,
  onOpenAdmin,
  onReturnToHub,
}) => {
  const isInsideMode = currentView !== 'hub';
  const currentMode = modes.find((m) => m.id === currentView);

  const getTitle = () => {
    if (currentView === 'hub') return 'Bot Hub';
    if (currentView === 'admin') return 'Админ-панель';
    if (currentView === 'shop') return 'Магазин';
    if (currentView === 'reception') return 'Стойка регистрации';
    if (currentView === 'support') return 'Помощь';
    if (currentView === 'chat') return 'Общение';
    return currentMode?.title || 'Bot Hub';
  };

  return (
    <header className="sticky top-0 z-40 bg-[#0b1329] text-white px-3.5 sm:px-4 py-2 sm:py-2.5 shadow-md">
      <div className="max-w-md sm:max-w-lg mx-auto flex items-center justify-between gap-3">
        {/* Left: Brand or Back Navigation */}
        <div className="flex items-center gap-2.5 min-w-0">
          {isInsideMode ? (
            <button
              onClick={onReturnToHub}
              className="flex items-center gap-1.5 text-slate-200 hover:text-white transition-colors focus:outline-none"
              title="Вернуться в Хаб"
            >
              <ArrowLeft className="w-5 h-5 text-slate-300" />
              <span className="text-sm font-semibold text-slate-300 hover:text-white">
                {currentView === 'admin' ? 'Хаб' : 'Хаб'}
              </span>
              <span className="text-slate-500 font-light mx-0.5">/</span>
              <span className="text-sm sm:text-base font-bold text-white truncate max-w-[140px] sm:max-w-[180px]">
                {getTitle()}
              </span>
            </button>
          ) : (
            <div className="flex items-center gap-2">
              <div className="w-7 h-7 rounded-lg bg-teal-500/20 border border-teal-500/40 flex items-center justify-center text-teal-300">
                <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <rect x="3" y="11" width="18" height="10" rx="2" />
                  <circle cx="12" cy="5" r="2" />
                  <path d="M12 7v4" />
                  <circle cx="8" cy="16" r="1" fill="currentColor" />
                  <circle cx="16" cy="16" r="1" fill="currentColor" />
                </svg>
              </div>
              <span className="font-bold text-base sm:text-lg tracking-tight text-white">
                Bot Hub
              </span>
            </div>
          )}
        </div>

        {/* Right: CTA Button */}
        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={onOpenLeadModal}
            className="flex items-center gap-1 px-3 sm:px-3.5 py-1.5 rounded-full bg-[#0d9488] hover:bg-[#0f766e] active:scale-95 text-white font-medium text-xs sm:text-sm shadow-sm transition-all"
          >
            <span>Хочу такого бота</span>
            <ArrowUpRight className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </header>
  );
};
