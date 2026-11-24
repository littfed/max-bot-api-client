import React, { useState } from 'react';
import { X, Send, Sparkles, CheckCircle2, Bot, Layers } from 'lucide-react';
import { apiClient, getClientPlatform } from '../../api/client';

interface LeadModalProps {
  isOpen: boolean;
  onClose: () => void;
  sourceMode: string;
  testedModes: string[];
  onSuccessToast: (msg: string) => void;
}

export const LeadModal: React.FC<LeadModalProps> = ({
  isOpen,
  onClose,
  sourceMode,
  testedModes,
  onSuccessToast,
}) => {
  const [contact, setContact] = useState('');
  const [comment, setComment] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSubmitted, setIsSubmitted] = useState(false);
  const [errorMessage, setErrorMessage] = useState('');

  if (!isOpen) return null;

  const platform = getClientPlatform();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!contact.trim()) {
      setErrorMessage('Пожалуйста, укажите ваши контактные данные (Telegram @username или телефон)');
      return;
    }

    setErrorMessage('');
    setIsSubmitting(true);

    try {
      await apiClient.submitLead({
        contact: contact.trim(),
        comment: comment.trim(),
        source_mode: sourceMode || 'hub',
        tested_modes: Array.from(new Set([...testedModes, sourceMode].filter(Boolean))),
      });

      setIsSubmitted(true);
      onSuccessToast('Заявка успешно отправлена! Мы свяжемся с вами в течение часа.');
    } catch (err: any) {
      const msg = err.response?.data?.message || 'Не удалось отправить заявку. Попробуйте еще раз.';
      setErrorMessage(msg);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleResetAndClose = () => {
    setIsSubmitted(false);
    setContact('');
    setComment('');
    setErrorMessage('');
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="relative w-full max-w-md rounded-2xl bg-slate-900 border border-slate-800 p-6 shadow-2xl overflow-hidden">
        {/* Accent glow corner */}
        <div className="absolute -top-12 -right-12 w-32 h-32 bg-indigo-600/20 rounded-full blur-2xl pointer-events-none" />

        <div className="flex items-center justify-between pb-3 border-b border-slate-800">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-indigo-500/10 text-indigo-400">
              <Sparkles className="w-5 h-5" />
            </div>
            <h3 className="text-base font-semibold text-white">Хочу такого бота!</h3>
          </div>
          <button
            onClick={handleResetAndClose}
            className="p-1 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors"
            aria-label="Закрыть"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {isSubmitted ? (
          <div className="py-6 flex flex-col items-center text-center">
            <div className="w-12 h-12 rounded-full bg-emerald-500/20 border border-emerald-500/40 text-emerald-400 flex items-center justify-center mb-3">
              <CheckCircle2 className="w-7 h-7" />
            </div>
            <h4 className="text-lg font-bold text-white mb-1">Заявка принята!</h4>
            <p className="text-sm text-slate-300 max-w-xs mb-5">
              Спасибо за интерес! Наш архитектор изучит контекст вашего запроса и свяжется с вами с готовым расчетом решения.
            </p>
            <button
              onClick={handleResetAndClose}
              className="w-full py-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-sm font-medium text-slate-200 transition-colors"
            >
              Вернуться в приложение
            </button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="mt-4 space-y-4">
            <p className="text-xs text-slate-400 leading-relaxed">
              Оставьте контакты для обсуждения разработки аналогичного виртуального бота, Mini App или интеграции с вашими системами.
            </p>

            {/* Context metadata preview */}
            <div className="p-3 rounded-xl bg-slate-950/60 border border-slate-800/80 space-y-1.5 text-xs">
              <div className="flex items-center justify-between text-slate-400">
                <span className="flex items-center gap-1.5">
                  <Bot className="w-3.5 h-3.5 text-indigo-400" />
                  Платформа:
                </span>
                <span className="font-medium text-slate-200 uppercase tracking-wide">
                  {platform}
                </span>
              </div>
              <div className="flex items-center justify-between text-slate-400">
                <span className="flex items-center gap-1.5">
                  <Layers className="w-3.5 h-3.5 text-indigo-400" />
                  Режим просмотра:
                </span>
                <span className="font-medium text-indigo-300">
                  {sourceMode || 'Главный хаб'}
                </span>
              </div>
            </div>

            {/* Contact Input */}
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">
                Контакты для связи <span className="text-rose-400">*</span>
              </label>
              <input
                type="text"
                value={contact}
                onChange={(e) => setContact(e.target.value)}
                placeholder="@username в Telegram или номер телефона"
                className="w-full px-3.5 py-2.5 rounded-xl bg-slate-950 border border-slate-700 text-slate-100 placeholder:text-slate-500 text-sm focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-colors"
                disabled={isSubmitting}
                required
              />
            </div>

            {/* Comment / Idea */}
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">
                Опишите задачу или проект (необязательно)
              </label>
              <textarea
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                rows={3}
                placeholder="Например: нужен интернет-магазин с приемом платежей и синхронизацией со складом..."
                className="w-full px-3.5 py-2 rounded-xl bg-slate-950 border border-slate-700 text-slate-100 placeholder:text-slate-500 text-sm focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 resize-none transition-colors"
                disabled={isSubmitting}
              />
            </div>

            {errorMessage && (
              <p className="text-xs text-rose-400 bg-rose-950/40 border border-rose-800/40 p-2.5 rounded-lg">
                {errorMessage}
              </p>
            )}

            <button
              type="submit"
              disabled={isSubmitting}
              className="w-full flex items-center justify-center gap-2 py-2.5 rounded-xl bg-gradient-to-r from-indigo-500 to-violet-600 hover:from-indigo-600 hover:to-violet-700 text-white font-medium text-sm shadow-lg shadow-indigo-600/30 disabled:opacity-50 disabled:cursor-not-allowed active:scale-95 transition-all"
            >
              {isSubmitting ? (
                <span>Отправка...</span>
              ) : (
                <>
                  <Send className="w-4 h-4" />
                  <span>Отправить заявку</span>
                </>
              )}
            </button>
          </form>
        )}
      </div>
    </div>
  );
};
