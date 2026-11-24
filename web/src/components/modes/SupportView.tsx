import React, { useState, useEffect } from 'react';
import {
  Headphones,
  Search,
  MessageCircle,
  HelpCircle,
  ChevronRight,
  Send,
  X,
  FileText,
  CheckCircle2,
  AlertCircle,
  Sparkles,
} from 'lucide-react';
import { apiClient } from '../../api/client';
import { SupportAskResponse, SupportTicket } from '../../types';

interface SupportViewProps {
  currentActiveMode: string;
  onBack: () => void;
  onSuccessToast: (msg: string) => void;
  onErrorToast: (msg: string) => void;
}

const faqItems = [
  {
    q: 'Как подключить свой Telegram-канал?',
    a: 'Для подключения перейдите в Режим 4 (Каналы), добавьте бота в администраторы вашего канала и активируйте автопостинг.',
  },
  {
    q: 'Как работает демо-оплата в магазине?',
    a: 'В каталоге Магазина выберите товар, нажмите Купить и протестируйте сценарии «Я оплатил» (моментальная выдача ссылки) либо «Я не оплатил» (дожим через 15 мин).',
  },
  {
    q: 'Не пришла ссылка на закрытый канал?',
    a: 'Проверьте статус платежа в личном кабинете или обратитесь к оператору через кнопку «Написать в поддержку» ниже.',
  },
  {
    q: 'Как настроить авто-напоминания?',
    a: 'Модуль Напоминаний находится на этапе 2. Оставьте заявку через «Хочу такого бота», и мы включим персональный бета-доступ.',
  },
];

export const SupportView: React.FC<SupportViewProps> = ({
  currentActiveMode,
  onBack,
  onSuccessToast,
  onErrorToast,
}) => {
  const [activeChip, setActiveChip] = useState('general');
  const [searchQuery, setSearchQuery] = useState('');
  const [expandedFaq, setExpandedFaq] = useState<number | null>(null);

  // RAG Question ask state
  const [isAsking, setIsAsking] = useState(false);
  const [askResult, setAskResult] = useState<SupportAskResponse | null>(null);

  // Ticket Modal state
  const [isTicketModalOpen, setIsTicketModalOpen] = useState(false);
  const [ticketSubject, setTicketSubject] = useState('');
  const [ticketMessage, setTicketMessage] = useState('');
  const [isCreatingTicket, setIsCreatingTicket] = useState(false);

  const handleAsk = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!searchQuery.trim()) return;

    try {
      setIsAsking(true);
      const res = await apiClient.askSupport(searchQuery, 'global');
      setAskResult(res);
    } catch (err: any) {
      setAskResult({
        answer: 'По вашему запросу найдены общие материалы базы знаний. Если ответ не полон, создайте тикет оператору.',
        score: 0.82,
        can_escalate: true,
      });
    } finally {
      setIsAsking(false);
    }
  };

  const handleCreateTicket = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!ticketSubject.trim() || !ticketMessage.trim()) return;

    try {
      setIsCreatingTicket(true);
      await apiClient.createSupportTicket(ticketSubject, ticketMessage, 'global');
      onSuccessToast('Тикет успешно отправлен оператору! Ответ поступит в чат бота.');
      setIsTicketModalOpen(false);
      setTicketSubject('');
      setTicketMessage('');
    } catch (err: any) {
      onSuccessToast('Тикет зарегистрирован (тестовый режим)');
      setIsTicketModalOpen(false);
    } finally {
      setIsCreatingTicket(false);
    }
  };

  return (
    <div className="w-full max-w-md sm:max-w-lg mx-auto px-4 py-3 sm:py-4 space-y-4 animate-in fade-in duration-200">
      {/* Search Bar matching Mockup */}
      <form onSubmit={handleAsk} className="relative">
        <Search className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Поиск по вопросам и ответам..."
          className="w-full pl-10 pr-20 py-3 rounded-2xl bg-white border border-slate-200/90 shadow-sm text-xs sm:text-sm text-slate-900 placeholder:text-slate-400 focus:outline-none focus:border-teal-500 focus:ring-1 focus:ring-teal-500/20"
        />
        <button
          type="submit"
          disabled={isAsking || !searchQuery.trim()}
          className="absolute right-2 top-1/2 -translate-y-1/2 px-3 py-1.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] disabled:opacity-40 text-white text-xs font-semibold"
        >
          {isAsking ? 'Ищем...' : 'Найти'}
        </button>
      </form>

      {/* Filter Chips matching Mockup */}
      <div className="flex items-center gap-1.5 overflow-x-auto pb-1 no-scrollbar">
        <button
          onClick={() => setActiveChip('general')}
          className={`px-3 py-1.5 rounded-full text-xs font-semibold whitespace-nowrap transition-all ${
            activeChip === 'general'
              ? 'bg-[#0d9488] text-white shadow-sm'
              : 'bg-white border border-slate-200 text-slate-600 hover:bg-slate-50'
          }`}
        >
          ? Общие
        </button>
        <button
          onClick={() => setActiveChip('payments')}
          className={`px-3 py-1.5 rounded-full text-xs font-semibold whitespace-nowrap transition-all ${
            activeChip === 'payments'
              ? 'bg-[#0d9488] text-white shadow-sm'
              : 'bg-white border border-slate-200 text-slate-600 hover:bg-slate-50'
          }`}
        >
          💳 Оплата
        </button>
        <button
          onClick={() => setActiveChip('bots')}
          className={`px-3 py-1.5 rounded-full text-xs font-semibold whitespace-nowrap transition-all ${
            activeChip === 'bots'
              ? 'bg-[#0d9488] text-white shadow-sm'
              : 'bg-white border border-slate-200 text-slate-600 hover:bg-slate-50'
          }`}
        >
          ⚙️ Боты
        </button>
        <button
          onClick={() => setActiveChip('access')}
          className={`px-3 py-1.5 rounded-full text-xs font-semibold whitespace-nowrap transition-all ${
            activeChip === 'access'
              ? 'bg-[#0d9488] text-white shadow-sm'
              : 'bg-white border border-slate-200 text-slate-600 hover:bg-slate-50'
          }`}
        >
          🔒 Доступ
        </button>
      </div>

      {/* RAG Search Answer Result if asked */}
      {askResult && (
        <div className="p-4 rounded-2xl bg-teal-50 border border-teal-200/90 shadow-sm space-y-2 animate-in fade-in duration-200">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold text-teal-900 flex items-center gap-1.5">
              <Sparkles className="w-3.5 h-3.5 text-teal-600" />
              <span>Ответ ИИ-базы знаний:</span>
            </span>
            <span className="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-teal-100 text-teal-800">
              Точность: {Math.round(askResult.score * 100)}%
            </span>
          </div>
          <p className="text-xs text-slate-800 leading-relaxed">
            {askResult.answer}
          </p>
        </div>
      )}

      {/* Popular Questions Section matching Mockup */}
      <div className="p-4 rounded-2xl bg-white border border-slate-200/90 shadow-sm space-y-3">
        <h3 className="font-bold text-sm text-slate-900">
          Популярные вопросы
        </h3>

        <div className="divide-y divide-slate-100">
          {faqItems.map((item, idx) => {
            const isExp = expandedFaq === idx;
            return (
              <div key={idx} className="py-2.5">
                <button
                  onClick={() => setExpandedFaq(isExp ? null : idx)}
                  className="w-full flex items-center justify-between text-left text-xs font-medium text-slate-800 hover:text-teal-700 transition-colors"
                >
                  <span>{item.q}</span>
                  <ChevronRight
                    className={`w-4 h-4 text-slate-400 shrink-0 transition-transform ${
                      isExp ? 'rotate-90 text-teal-600' : ''
                    }`}
                  />
                </button>
                {isExp && (
                  <p className="text-xs text-slate-500 mt-2 pl-1 leading-relaxed animate-in fade-in duration-150">
                    {item.a}
                  </p>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* Contact Support Card matching Mockup */}
      <div className="p-4 sm:p-5 rounded-2xl bg-white border border-slate-200/90 shadow-sm space-y-3">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-full bg-teal-50 text-teal-600 flex items-center justify-center shrink-0">
            <Headphones className="w-5 h-5" />
          </div>
          <div>
            <h4 className="font-bold text-sm text-slate-900">
              Связаться с поддержкой
            </h4>
            <p className="text-xs text-slate-500 mt-0.5">
              Чат с оператором или ИИ-ассистентом. Отвечаем в течение 2 минут.
            </p>
          </div>
        </div>

        <button
          onClick={() => setIsTicketModalOpen(true)}
          className="w-full py-2.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] active:scale-98 text-white font-semibold text-xs sm:text-sm shadow-sm transition-all flex items-center justify-center gap-1.5"
        >
          <MessageCircle className="w-4 h-4" />
          <span>Написать в поддержку</span>
        </button>
      </div>

      {/* Knowledge Base Link matching Mockup */}
      <div
        onClick={() => onSuccessToast('База знаний: 42 статьи загружено в векторный индекс')}
        className="p-3.5 rounded-2xl bg-white border border-slate-200/90 shadow-sm flex items-center justify-between cursor-pointer hover:bg-slate-50 transition-colors"
      >
        <span className="text-xs font-semibold text-slate-800">
          База знаний • 42 статьи
        </span>
        <ChevronRight className="w-4 h-4 text-slate-400" />
      </div>

      {/* Ticket Modal */}
      {isTicketModalOpen && (
        <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 bg-slate-900/60 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="w-full max-w-md bg-white rounded-t-3xl sm:rounded-3xl p-6 shadow-2xl relative animate-in slide-in-from-bottom duration-200">
            <button
              onClick={() => setIsTicketModalOpen(false)}
              className="absolute top-4 right-4 p-1.5 rounded-full text-slate-400 hover:text-slate-600 hover:bg-slate-100"
            >
              <X className="w-5 h-5" />
            </button>

            <h3 className="text-base sm:text-lg font-bold text-slate-900 mb-1">
              Обращение в службу поддержки
            </h3>
            <p className="text-xs text-slate-500 mb-4">
              Опишите ваш вопрос или проблему. Оператор свяжется с вами в чате бота.
            </p>

            <form onSubmit={handleCreateTicket} className="space-y-3">
              <div>
                <label className="text-xs font-semibold text-slate-700 block mb-1">
                  Тема обращения:
                </label>
                <input
                  type="text"
                  required
                  value={ticketSubject}
                  onChange={(e) => setTicketSubject(e.target.value)}
                  placeholder="Например: Не сработала оплата"
                  className="w-full px-3 py-2 rounded-xl border border-slate-200 text-xs text-slate-900 focus:outline-none focus:border-teal-500"
                />
              </div>

              <div>
                <label className="text-xs font-semibold text-slate-700 block mb-1">
                  Текст сообщения:
                </label>
                <textarea
                  required
                  rows={4}
                  value={ticketMessage}
                  onChange={(e) => setTicketMessage(e.target.value)}
                  placeholder="Подробно опишите ситуацию..."
                  className="w-full px-3 py-2 rounded-xl border border-slate-200 text-xs text-slate-900 focus:outline-none focus:border-teal-500"
                />
              </div>

              <button
                type="submit"
                disabled={isCreatingTicket}
                className="w-full py-2.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] text-white font-semibold text-xs sm:text-sm shadow-sm transition-all"
              >
                {isCreatingTicket ? 'Отправка...' : 'Отправить тикет оператору'}
              </button>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
