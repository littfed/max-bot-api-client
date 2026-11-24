import React from 'react';
import { CheckCircle2, AlertCircle, Info, AlertTriangle, X } from 'lucide-react';
import { ToastMessage } from '../../types';

interface ToastProps {
  toasts: ToastMessage[];
  onDismiss: (id: string) => void;
}

export const ToastContainer: React.FC<ToastProps> = ({ toasts, onDismiss }) => {
  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-16 left-1/2 -translate-x-1/2 z-50 flex flex-col gap-2 w-full max-w-sm px-4 pointer-events-none">
      {toasts.map((toast) => {
        const isError = toast.type === 'error';
        const isWarning = toast.type === 'warning';
        const isSuccess = toast.type === 'success';

        return (
          <div
            key={toast.id}
            className={`pointer-events-auto flex items-start gap-2.5 p-3.5 rounded-2xl border shadow-xl transition-all animate-in fade-in slide-in-from-bottom-2 duration-200 ${
              isError
                ? 'bg-rose-50 border-rose-200 text-rose-900'
                : isWarning
                ? 'bg-amber-50 border-amber-200 text-amber-900'
                : isSuccess
                ? 'bg-teal-50 border-teal-200 text-teal-900'
                : 'bg-white border-slate-200 text-slate-800'
            }`}
          >
            <div className="mt-0.5 shrink-0">
              {isError && <AlertCircle className="w-4 h-4 text-rose-600" />}
              {isWarning && <AlertTriangle className="w-4 h-4 text-amber-600" />}
              {isSuccess && <CheckCircle2 className="w-4 h-4 text-teal-600" />}
              {toast.type === 'info' && <Info className="w-4 h-4 text-sky-600" />}
            </div>
            <p className="text-xs sm:text-sm font-medium leading-snug flex-1">{toast.text}</p>
            <button
              onClick={() => onDismiss(toast.id)}
              className="shrink-0 p-1 rounded-md text-slate-400 hover:text-slate-600 transition-colors"
              aria-label="Закрыть"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
        );
      })}
    </div>
  );
};
