import React from 'react';
import { Wrench, Clock, X } from 'lucide-react';
import { ModeItem } from '../../types';

interface MaintenanceModalProps {
  isOpen: boolean;
  mode: ModeItem | null;
  onClose: () => void;
  onRequestNotify: (mode: ModeItem) => void;
}

export const MaintenanceModal: React.FC<MaintenanceModalProps> = ({
  isOpen,
  mode,
  onClose,
  onRequestNotify,
}) => {
  if (!isOpen || !mode) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 bg-slate-900/60 backdrop-blur-sm animate-in fade-in duration-200">
      <div
        className="w-full max-w-md bg-white rounded-t-3xl sm:rounded-3xl p-6 shadow-2xl relative animate-in slide-in-from-bottom duration-200 text-center"
      >
        {/* Drag handle */}
        <div className="w-10 h-1 bg-slate-200 rounded-full mx-auto mb-4 sm:hidden" />

        {/* Close Button */}
        <button
          onClick={onClose}
          className="absolute top-4 right-4 p-1.5 rounded-full text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors"
          title="Закрыть"
        >
          <X className="w-5 h-5" />
        </button>

        {/* Yellow Circle with Wrench Icon */}
        <div className="w-14 h-14 rounded-full bg-amber-100 text-amber-600 flex items-center justify-center mx-auto mb-3 shadow-sm">
          <Wrench className="w-7 h-7 -rotate-45" />
        </div>

        {/* Title */}
        <h3 className="text-lg sm:text-xl font-bold text-slate-900 tracking-tight">
          Раздел на профилактике
        </h3>

        {/* Description */}
        <p className="text-xs sm:text-sm text-slate-600 mt-2 mb-4 leading-relaxed max-w-xs mx-auto">
          Модуль «{mode.title}» временно отключён администратором для обновления. Скоро вернётся в работу!
        </p>

        {/* Info Banner: 7 other modes are working */}
        <div className="flex items-center justify-center gap-1.5 px-3 py-2 rounded-xl bg-teal-50 border border-teal-100 text-teal-800 text-xs font-medium mb-6">
          <Clock className="w-3.5 h-3.5 text-teal-600 shrink-0" />
          <span>Остальные 7 режимов работают в обычном режиме</span>
        </div>

        {/* Action Buttons */}
        <div className="space-y-2.5">
          <button
            onClick={onClose}
            className="w-full py-3 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] active:scale-98 text-white font-semibold text-sm shadow-sm transition-all"
          >
            Выбрать другой режим
          </button>

          <button
            onClick={() => {
              onClose();
              onRequestNotify(mode);
            }}
            className="w-full py-3 rounded-xl border border-teal-600 text-teal-700 hover:bg-teal-50 active:scale-98 font-semibold text-sm transition-all"
          >
            Сообщить, когда откроется
          </button>
        </div>
      </div>
    </div>
  );
};
