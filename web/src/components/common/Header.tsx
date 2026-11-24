import React from 'react';
import { Sparkles, Shield, Home } from 'lucide-react';
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
  const isAdmin = user && (user.role === 'admin' || user.role === 'superadmin');
  const isInsideMode = currentView !== 'hub' && currentView !== 'admin';
  const currentMode = modes.find((m) => m.id === currentView);

  return (
    <header className="sticky top-0 z-40 bg-slate-950/80 backdrop-blur-md border-b border-slate-800/80 px-4 py-3 safe-top">
      <div className="max-w-2xl mx-auto flex items-center justify-between gap-3">
        {/* Brand & Active Mode Nav */}
        <div className="flex items-center gap-2.5">
          <button
            onClick={onReturnToHub}
            className="flex items-center gap-2 group text-left focus:outline-none"
            title="Главный экран Хаба"
          >
            <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-indigo-600 to-violet-500 flex items-center justify-center text-lg shadow-md shadow-indigo-500/20 group-hover:scale-105 transition-transform">
              🤖
            </div>
            <div>
              <div className="flex items-center gap-1.5">
                <span className="font-bold text-sm tracking-tight text-white group-hover:text-indigo-300 transition-colors">
                  Bot Hub
                </span>
                <span className="text-[10px] font-semibold uppercase tracking-wider px-1.5 py-0.5 rounded-full bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
                  8 в 1
                </span>
              </div>
              <p className="text-[11px] text-slate-400 leading-none">Мультирежимная демо-платформа</p>
            </div>
          </button>

          {/* Current Mode Badge / Return button */}
          {isInsideMode && currentMode && (
            <div className="hidden sm:flex items-center gap-1.5 pl-2 border-l border-slate-800">
              <button
                onClick={onReturnToHub}
                className="flex items-center gap-1 px-2 py-1 rounded-lg bg-slate-800/80 hover:bg-slate-700/80 text-xs font-medium text-slate-300 transition-colors"
              >
                <Home className="w-3.5 h-3.5 text-slate-400" />
                <span>Хаб</span>
              </button>
              <span className="text-slate-600">/</span>
              <span className="text-xs font-semibold text-indigo-400 bg-indigo-950/50 border border-indigo-800/50 px-2 py-0.5 rounded-md">
                {currentMode.title}
              </span>
            </div>
          )}
        </div>

        {/* Actions: Admin button & CTA */}
        <div className="flex items-center gap-2">
          {isAdmin && (
            <button
              onClick={onOpenAdmin}
              className={`p-2 rounded-xl border transition-all ${
                currentView === 'admin'
                  ? 'bg-amber-500/20 border-amber-500/40 text-amber-300 shadow-sm shadow-amber-500/20'
                  : 'bg-slate-900 border-slate-800 hover:border-amber-500/40 text-slate-400 hover:text-amber-300'
              }`}
              title="Панель администратора"
              aria-label="Панель администратора"
            >
              <Shield className="w-4 h-4" />
            </button>
          )}

          <button
            onClick={onOpenLeadModal}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-gradient-to-r from-indigo-500 via-indigo-600 to-violet-600 text-white font-medium text-xs sm:text-sm shadow-md shadow-indigo-600/30 hover:shadow-indigo-600/50 active:scale-95 transition-all"
          >
            <Sparkles className="w-3.5 h-3.5 text-amber-300 animate-pulse" />
            <span>Хочу такого бота</span>
          </button>
        </div>
      </div>
    </header>
  );
};
