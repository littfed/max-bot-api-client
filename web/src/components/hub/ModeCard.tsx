import React from 'react';
import {
  ShoppingBag,
  CalendarCheck,
  BellRing,
  Megaphone,
  Gift,
  Headphones,
  FileText,
  MessageSquare,
  Lock,
  CheckCircle,
  Wrench,
} from 'lucide-react';
import { ModeItem } from '../../types';

interface ModeCardProps {
  mode: ModeItem;
  isActiveMode: boolean;
  onSelect: (mode: ModeItem) => void;
  onMaintenanceClick: (mode: ModeItem) => void;
}

// Icon mapper for 8 modes
const modeIcons: Record<string, React.ReactNode> = {
  shop: <ShoppingBag className="w-5 h-5 sm:w-6 sm:h-6" />,
  reception: <CalendarCheck className="w-5 h-5 sm:w-6 sm:h-6" />,
  reminders: <BellRing className="w-5 h-5 sm:w-6 sm:h-6" />,
  channels: <Megaphone className="w-5 h-5 sm:w-6 sm:h-6" />,
  giveaways: <Gift className="w-5 h-5 sm:w-6 sm:h-6" />,
  support: <Headphones className="w-5 h-5 sm:w-6 sm:h-6" />,
  docs: <FileText className="w-5 h-5 sm:w-6 sm:h-6" />,
  chat: <MessageSquare className="w-5 h-5 sm:w-6 sm:h-6" />,
};

// Mode subtitle descriptions
const modeDescriptions: Record<string, string> = {
  shop: 'Каталог, корзина, демо-оплата',
  reception: 'Запись на слоты, ивенты, онбординг',
  reminders: 'Умные таймеры и пуши',
  channels: 'Автопостинг и реакции',
  giveaways: 'Розыгрыши и проверка подписок',
  support: 'RAG база знаний и операторы',
  docs: 'Генерация КП и счетов',
  chat: 'Диалог с LLM, 5 ролей и стилей',
};

export const ModeCard: React.FC<ModeCardProps> = ({
  mode,
  isActiveMode,
  onSelect,
  onMaintenanceClick,
}) => {
  const isMaintenance = !mode.is_active;
  const icon = modeIcons[mode.id] || <Wrench className="w-5 h-5" />;
  const subtitle = modeDescriptions[mode.id] || 'Виртуальный модуль';

  const handleClick = () => {
    if (isMaintenance) {
      onMaintenanceClick(mode);
    } else {
      onSelect(mode);
    }
  };

  return (
    <div
      onClick={handleClick}
      className={`group relative flex flex-col justify-between p-3.5 sm:p-4 rounded-2xl cursor-pointer transition-all duration-200 select-none ${
        isMaintenance
          ? 'grayscale opacity-40 hover:opacity-60 bg-slate-900/40 border border-slate-800'
          : isActiveMode
          ? 'bg-gradient-to-br from-indigo-950/80 to-slate-900 border-2 border-indigo-500 shadow-xl shadow-indigo-500/20 ring-2 ring-indigo-500/20 scale-[1.02]'
          : 'bg-slate-900/90 border border-slate-800/90 hover:border-indigo-500/50 hover:bg-slate-800/80 hover:shadow-lg hover:shadow-slate-950/50 active:scale-98'
      }`}
    >
      {/* Top row: Icon and status badge */}
      <div className="flex items-start justify-between gap-2 mb-2">
        <div
          className={`w-10 h-10 sm:w-11 sm:h-11 rounded-xl flex items-center justify-center transition-colors ${
            isMaintenance
              ? 'bg-slate-800 text-slate-500'
              : isActiveMode
              ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/40'
              : 'bg-slate-800 text-indigo-400 group-hover:bg-indigo-600 group-hover:text-white'
          }`}
        >
          {icon}
        </div>

        {/* Status Badges */}
        {isActiveMode ? (
          <span className="flex items-center gap-1 text-[10px] sm:text-xs font-semibold px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 animate-pulse">
            <CheckCircle className="w-3 h-3" />
            <span>Активен</span>
          </span>
        ) : isMaintenance ? (
          <span className="flex items-center gap-1 text-[10px] font-medium px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 border border-slate-700">
            <Lock className="w-2.5 h-2.5" />
            <span>Этап 2</span>
          </span>
        ) : (
          <span className="text-[10px] font-medium px-2 py-0.5 rounded-full bg-slate-800/80 text-slate-400 group-hover:text-indigo-300 transition-colors">
            Открыть
          </span>
        )}
      </div>

      {/* Title & Description */}
      <div>
        <h4 className="font-semibold text-xs sm:text-sm text-white group-hover:text-indigo-200 transition-colors line-clamp-1">
          {mode.title}
        </h4>
        <p className="text-[11px] text-slate-400 line-clamp-2 mt-0.5 leading-tight">
          {subtitle}
        </p>
      </div>
    </div>
  );
};
