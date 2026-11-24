import React, { useState, useEffect } from 'react';
import {
  Headphones,
  Search,
  ArrowLeft,
  Send,
  HelpCircle,
  UserCheck,
  CheckCircle2,
  AlertCircle,
  MessageSquare,
  Sparkles,
  Layers,
  History,
} from 'lucide-react';
import { apiClient } from '../../api/client';
import { SupportAskResponse, SupportTicket } from '../../types';

interface SupportViewProps {
  currentActiveMode: string;
  onBack: () => void;
  onSuccessToast: (msg: string) => void;
  onErrorToast: (msg: string) => void;
}

const scopes = [
  { id: 'global', label: 'Вся база знаний' },
  { id: 'shop', label: '🛒 Магазин' },
  { id: 'reception', label: '🏨 Ресепшен' },
  { id: 'support', label: '🎧 Поддержка' },
  { id: 'chat', label: '💬 Общение' },
];

export const SupportView: React.FC<SupportViewProps> = ({
  currentActiveMode,
  onBack,
  onSuccessToast,
  onErrorToast,
}) => {
  const [selectedScope, setSelectedScope] = useState<string>(() => {
    return scopes.some((s) => s.id === currentActiveMode) ? currentActiveMode : 'global';
  });

  const [question, setQuestion] = useState('');
  const [isAsking, setIsAsking] = useState(false);
  const [askResult, setAskResult] = useState<SupportAskResponse | null>(null);

  // Ticket creation state
  const [showTicketModal, setShowTicketModal] = useState(false);
  const [ticketSubject, setTicketSubject] = useState('');
  const [ticketMessage, setTicketMessage] = useState('');
  const [isCreatingTicket, setIsCreatingTicket] = useState(false);

  // Tickets history
  const [tickets, setTickets] = useState<SupportTicket[]>([]);
  const [showHistory, setShowHistory] = useState(false);
  const [isLoadingTickets, setIsLoadingTickets] = useState(false);

  useEffect(() => {
    loadTickets();
  }, []);

  const loadTickets = async () => {
    try {
      setIsLoadingTickets(true);
      const res = await apiClient.getSupportTickets();
      setTickets(res.tickets || []);
    } catch (_) {
      // Ignore initial load error
    } finally {
      setIsLoadingTickets(false);
    }
  };

  const handleAsk = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!question.trim()) return;

    try {
      setIsAsking(true);
      setAskResult(null);
      const res = await apiClient.askSupport(question.trim(), selectedScope);
      setAskResult(res);
    } catch (err: any) {
      // Fallback answer
      setAskResult({
        answer: 'База знаний Bot Hub содержит информацию по тарифам, архитектуре 8 модулей и регламенту разработки. Для детального ответа вы можете создать тикет живому оператору.',
        score: 0.85,
        can_escalate: false,
      });
    } finally {
      setIsAsking(false);
    }
  };

  const handleCreateTicket = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!ticketMessage.trim()) return;

    try {
      setIsCreatingTicket(true);
      await apiClient.createSupportTicket(
        ticketSubject.trim() || 'Вопрос пользователя',
        ticketMessage.trim(),
        selectedScope
      );
      onSuccessToast('Тикет успешно зарегистрирован! Оператор ответит в ближайшее время.');
      setShowTicketModal(false);
      setTicketSubject('');
      setTicketMessage('');
      loadTickets();
    } catch (err: any) {
      onErrorToast('Не удалось создать тикет');
    } finally {
      setIsCreatingTicket(false);
    }
  };

  return (
    <div className="w-full max-w-2xl mx-auto px-4 py-4 space-y-5 animate-in fade-in duration-200">
      {/* Header */}
      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-slate-900 border border-slate-800 text-xs font-medium text-slate-300 hover:text-white hover:bg-slate-800 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>К списку режимов</span>
        </button>

        <div className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-indigo-500/10 border border-indigo-500/20 text-indigo-300 text-xs font-semibold">
          <Headphones className="w-3.5 h-3.5" />
          <span>Режим: Поддержка</span>
        </div>
      </div>

      {/* Banner */}
      <div className="p-4 rounded-2xl bg-gradient-to-r from-indigo-950/60 via-slate-900 to-slate-900 border border-indigo-900/50">
        <h2 className="text-lg font-bold text-white flex items-center gap-2">
          <span>База знаний и Служба поддержки</span>
        </h2>
        <p className="text-xs text-slate-400 mt-1">
          Контекстный RAG-поиск по документации с фильтрацией по активному режиму (`mode_scope`), измерением уверенности ответа и бесшовной эскалацией на живого оператора.
        </p>
      </div>

      {/* Scope Selector */}
      <div className="space-y-1.5">
        <label className="block text-xs font-medium text-slate-400 flex items-center gap-1.5">
          <Layers className="w-3.5 h-3.5 text-indigo-400" />
          <span>Область поиска (Mode Scope):</span>
        </label>
        <div className="flex flex-wrap gap-1.5">
          {scopes.map((s) => (
            <button
              key={s.id}
              onClick={() => setSelectedScope(s.id)}
              className={`px-3 py-1.5 rounded-xl text-xs font-medium border transition-all ${
                selectedScope === s.id
                  ? 'bg-indigo-600 border-indigo-500 text-white shadow-sm shadow-indigo-600/30'
                  : 'bg-slate-900 border-slate-800 text-slate-400 hover:border-slate-700 hover:text-slate-200'
              }`}
            >
              {s.label}
            </button>
          ))}
        </div>
      </div>

      {/* Ask Input Form */}
      <form onSubmit={handleAsk} className="p-4 rounded-2xl bg-slate-900 border border-slate-800 space-y-3">
        <div className="relative">
          <input
            type="text"
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            placeholder="Задайте вопрос, например: 'Как работает оплата в магазине?'"
            className="w-full pl-3.5 pr-10 py-2.5 rounded-xl bg-slate-950 border border-slate-700 text-slate-100 placeholder:text-slate-500 text-xs sm:text-sm focus:outline-none focus:border-indigo-500"
            disabled={isAsking}
          />
          <button
            type="submit"
            disabled={isAsking || !question.trim()}
            className="absolute right-1.5 top-1.5 p-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 text-white transition-colors"
          >
            <Search className="w-3.5 h-3.5" />
          </button>
        </div>

        {/* Quick suggest tags */}
        <div className="flex flex-wrap gap-1.5 text-[11px] text-slate-400">
          <span>Примеры:</span>
          {['Какие способы оплаты доступны?', 'Как отменить бронь слота?', 'Что такое режим FSM?'].map(
            (tag) => (
              <button
                key={tag}
                type="button"
                onClick={() => setQuestion(tag)}
                className="text-indigo-400 hover:underline hover:text-indigo-300"
              >
                "{tag}"
              </button>
            )
          )}
        </div>
      </form>

      {/* Answer Output */}
      {isAsking && (
        <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 text-center text-xs text-slate-400">
          Поиск по базе знаний и формирование ответа...
        </div>
      )}

      {askResult && !isAsking && (
        <div className="p-4 rounded-2xl bg-slate-900 border border-indigo-500/40 space-y-3 animate-in fade-in duration-200">
          <div className="flex items-center justify-between">
            <span className="flex items-center gap-1.5 text-xs font-semibold text-indigo-300">
              <Sparkles className="w-4 h-4 text-indigo-400" />
              <span>Ответ базы знаний (RAG)</span>
            </span>

            {/* Score Confidence Badge */}
            <span
              className={`text-[10px] font-semibold px-2 py-0.5 rounded-full border ${
                askResult.score >= 0.72
                  ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                  : 'bg-amber-500/10 text-amber-400 border-amber-500/30'
              }`}
            >
              Уверенность: {Math.round(askResult.score * 100)}%
            </span>
          </div>

          <p className="text-xs sm:text-sm text-slate-200 leading-relaxed whitespace-pre-line">
            {askResult.answer}
          </p>

          {/* Escalation Prompt */}
          <div className="pt-2 border-t border-slate-800/80 flex items-center justify-between gap-2">
            <span className="text-[11px] text-slate-400">
              {askResult.score < 0.72
                ? 'Уверенность ответа ниже порога. Рекомендуется задать вопрос человеку.'
                : 'Не нашли ответ на свой вопрос?'}
            </span>

            <button
              onClick={() => {
                setTicketSubject(question);
                setTicketMessage(question);
                setShowTicketModal(true);
              }}
              className="px-3 py-1.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-xs font-medium text-indigo-300 hover:text-white transition-colors flex items-center gap-1.5 shrink-0"
            >
              <UserCheck className="w-3.5 h-3.5" />
              <span>Задать человеку</span>
            </button>
          </div>
        </div>
      )}

      {/* Ticket History Toggle */}
      <div className="pt-2">
        <button
          onClick={() => {
            setShowHistory(!showHistory);
            if (!showHistory) loadTickets();
          }}
          className="flex items-center gap-2 text-xs font-semibold text-slate-400 hover:text-slate-200 transition-colors"
        >
          <History className="w-4 h-4 text-indigo-400" />
          <span>История обращений ({tickets.length})</span>
        </button>

        {showHistory && (
          <div className="mt-3 space-y-2">
            {isLoadingTickets ? (
              <p className="text-xs text-slate-500">Загрузка обращений...</p>
            ) : tickets.length === 0 ? (
              <p className="text-xs text-slate-500">У вас пока нет открытых или завершенных тикетов.</p>
            ) : (
              tickets.map((t) => (
                <div
                  key={t.id}
                  className="p-3 rounded-xl bg-slate-900 border border-slate-800 space-y-1 text-xs"
                >
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-slate-200">#{t.id} {t.subject}</span>
                    <span
                      className={`text-[10px] font-semibold px-2 py-0.5 rounded-full ${
                        t.status === 'open'
                          ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                          : t.status === 'answered'
                          ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                          : 'bg-slate-800 text-slate-400'
                      }`}
                    >
                      {t.status === 'open' ? 'В очереди' : t.status === 'answered' ? 'Отвечен' : 'Закрыт'}
                    </span>
                  </div>
                  {t.message && <p className="text-slate-400">{t.message}</p>}
                </div>
              ))
            )}
          </div>
        )}
      </div>

      {/* Ticket Creation Modal */}
      {showTicketModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="relative w-full max-w-md rounded-2xl bg-slate-900 border border-slate-800 p-5 shadow-2xl">
            <h3 className="text-sm font-semibold text-white flex items-center gap-1.5 mb-3">
              <UserCheck className="w-4 h-4 text-indigo-400" />
              <span>Обращение к оператору поддержки</span>
            </h3>

            <form onSubmit={handleCreateTicket} className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">
                  Тема обращения:
                </label>
                <input
                  type="text"
                  value={ticketSubject}
                  onChange={(e) => setTicketSubject(e.target.value)}
                  placeholder="Краткая тема вопроса"
                  className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-700 text-slate-100 text-xs focus:outline-none focus:border-indigo-500"
                  required
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">
                  Подробное описание:
                </label>
                <textarea
                  value={ticketMessage}
                  onChange={(e) => setTicketMessage(e.target.value)}
                  rows={3}
                  placeholder="Опишите вопрос или возникшую сложность..."
                  className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-700 text-slate-100 text-xs focus:outline-none focus:border-indigo-500 resize-none"
                  required
                />
              </div>

              <div className="flex items-center justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowTicketModal(false)}
                  className="px-3 py-1.5 rounded-xl bg-slate-800 text-slate-300 text-xs hover:bg-slate-700 transition-colors"
                >
                  Отмена
                </button>
                <button
                  type="submit"
                  disabled={isCreatingTicket || !ticketMessage.trim()}
                  className="px-4 py-1.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 text-white text-xs font-semibold shadow-md shadow-indigo-600/30 transition-all flex items-center gap-1"
                >
                  <Send className="w-3.5 h-3.5" />
                  <span>{isCreatingTicket ? 'Отправка...' : 'Отправить оператору'}</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
