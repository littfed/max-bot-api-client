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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="relative w-full max-w-md rounded-3xl bg-white border border-slate-200 p-6 shadow-2xl overflow-hidden">
        <div className="flex items-center justify-between pb-3 border-b border-slate-100">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-teal-50 text-teal-600">
              <Sparkles className="w-5 h-5" />
            </div>
            <h3 className="text-base font-bold text-slate-900">Хочу такого бота!</h3>
          </div>
          <button
            onClick={handleResetAndClose}
            className="p-1 rounded-lg text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors"
            aria-label="Закрыть"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {isSubmitted ? (
          <div className="py-6 flex flex-col items-center text-center">
            <div className="w-12 h-12 rounded-full bg-teal-50 border border-teal-200 text-teal-600 flex items-center justify-center mb-3">
              <CheckCircle2 className="w-7 h-7" />
            </div>
            <h4 className="text-lg font-bold text-slate-900 mb-1">Заявка принята!</h4>
            <p className="text-xs sm:text-sm text-slate-600 max-w-xs mb-5">
              Спасибо за интерес! Наш архитектор изучит контекст вашего запроса и свяжется с вами с расчетом решения.
            </p>
            <button
              onClick={handleResetAndClose}
              className="w-full py-2.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] text-sm font-semibold text-white shadow-sm transition-all"
            >
              Вернуться в приложение
            </button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="mt-4 space-y-4">
            <p className="text-xs text-slate-500 leading-relaxed">
              Оставьте контакты для обсуждения разработки аналогичного виртуального бота, Mini App или интеграции с вашими системами.
            </p>

            {/* Context metadata preview */}
            <div className="p-3 rounded-xl bg-slate-50 border border-slate-200 space-y-1.5 text-xs">
              <div className="flex items-center justify-between text-slate-500">
                <span className="flex items-center gap-1.5">
                  <Bot className="w-3.5 h-3.5 text-teal-600" />
                  Платформа:
                </span>
                <span className="font-semibold text-slate-800 uppercase tracking-wide">
                  {platform}
                </span>
              </div>
              <div className="flex items-center justify-between text-slate-500">
                <span className="flex items-center gap-1.5">
                  <Layers className="w-3.5 h-3.5 text-teal-600" />
                  Исходный режим:
                </span>
                <span className="font-semibold text-slate-800">
                  {sourceMode}
                </span>
              </div>
            </div>

            {/* Contact Input */}
            <div>
              <label htmlFor="contact" className="block text-xs font-semibold text-slate-700 mb-1">
                Контакты для связи (Telegram @ или телефон) <span className="text-rose-500">*</span>
              </label>
              <input
                id="contact"
                type="text"
                value={contact}
                onChange={(e) => setContact(e.target.value)}
                placeholder="@username или +7 999 123-45-67"
                required
                className="w-full px-3.5 py-2.5 rounded-xl bg-white border border-slate-200 text-xs sm:text-sm text-slate-900 placeholder:text-slate-400 focus:outline-none focus:border-teal-500"
              />
            </div>

            {/* Comment Input */}
            <div>
              <label htmlFor="comment" className="block text-xs font-semibold text-slate-700 mb-1">
                Что требуется разработать? (опционально)
              </label>
              <textarea
                id="comment"
                rows={3}
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                placeholder="Опишите задачи вашего бизнеса или пожелания..."
                className="w-full px-3.5 py-2.5 rounded-xl bg-white border border-slate-200 text-xs sm:text-sm text-slate-900 placeholder:text-slate-400 focus:outline-none focus:border-teal-500"
              />
            </div>

            {errorMessage && (
              <p className="text-xs text-rose-600 font-medium">
                {errorMessage}
              </p>
            )}

            <button
              type="submit"
              disabled={isSubmitting}
              className="w-full py-3 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] disabled:opacity-50 text-white font-semibold text-sm shadow-sm transition-all flex items-center justify-center gap-1.5"
            >
              <Send className="w-4 h-4" />
              <span>{isSubmitting ? 'Отправка...' : 'Отправить заявку'}</span>
            </button>
          </form>
        )}
      </div>
    </div>
  );
};
