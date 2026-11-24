import React from 'react';
import {
  ShoppingBag,
  Calendar,
  Bell,
  Megaphone,
  Gift,
  Headphones,
  FileText,
  MessageCircle,
  Wrench,
} from 'lucide-react';
import { ModeItem } from '../../types';

interface ModeCardProps {
  mode: ModeItem;
  isActiveMode: boolean;
  onSelect: (mode: ModeItem) => void;
  onMaintenanceClick: (mode: ModeItem) => void;
}

// Icon mapper matching mockup
const modeIcons: Record<string, React.ReactNode> = {
  shop: <ShoppingBag className="w-6 h-6 text-teal-600" />,
  reception: <Calendar className="w-6 h-6 text-teal-600" />,
  reminders: <Bell className="w-6 h-6 text-teal-600" />,
  channels: <Megaphone className="w-6 h-6 text-teal-600" />,
  giveaways: <Gift className="w-6 h-6 text-teal-600" />,
  support: <Headphones className="w-6 h-6 text-teal-600" />,
  docs: <FileText className="w-6 h-6 text-teal-600" />,
  chat: <MessageCircle className="w-6 h-6 text-teal-600" />,
};

// Mode short titles matching mockup
const modeTitles: Record<string, string> = {
  shop: 'Магазин',
  reception: 'Ресепшен',
  reminders: 'Напоминания',
  channels: 'Каналы',
  giveaways: 'Конкурсы',
  support: 'Помощь',
  docs: 'Документы',
  chat: 'Общение',
};

export const ModeCard: React.FC<ModeCardProps> = ({
  mode,
  isActiveMode,
  onSelect,
  onMaintenanceClick,
}) => {
  const isMaintenance = !mode.is_active;
  const icon = isMaintenance ? (
    <div className="relative">
      <div className="opacity-40">{modeIcons[mode.id] || <Gift className="w-6 h-6 text-slate-400" />}</div>
      <Wrench className="w-3.5 h-3.5 text-slate-500 absolute -bottom-0.5 -right-0.5" />
    </div>
  ) : (
    modeIcons[mode.id] || <ShoppingBag className="w-6 h-6 text-teal-600" />
  );

  const title = modeTitles[mode.id] || mode.title;

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
      className={`group relative flex flex-col items-center justify-center text-center p-3 sm:p-3.5 rounded-2xl cursor-pointer transition-all duration-200 select-none min-h-[92px] sm:min-h-[104px] ${
        isActiveMode
          ? 'bg-teal-50/50 border-2 border-teal-500 shadow-sm ring-1 ring-teal-500/30'
          : isMaintenance
          ? 'bg-white border border-slate-200/90 shadow-sm opacity-75 hover:opacity-90'
          : 'bg-white border border-slate-200/90 shadow-sm hover:shadow-md hover:border-teal-400/60 active:scale-98'
      }`}
    >
      {/* Icon Circle */}
      <div
        className={`w-10 h-10 sm:w-11 sm:h-11 rounded-full flex items-center justify-center mb-1.5 transition-transform group-hover:scale-105 ${
          isActiveMode
            ? 'bg-teal-100 text-teal-700'
            : isMaintenance
            ? 'bg-slate-100 text-slate-400'
            : 'bg-teal-50 text-teal-600'
        }`}
      >
        {icon}
      </div>

      {/* Title */}
      <h3 className="font-semibold text-xs sm:text-sm text-slate-900 group-hover:text-teal-700 transition-colors line-clamp-1">
        {title}
      </h3>
    </div>
  );
};
