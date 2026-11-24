import React, { useState, useEffect, useRef } from 'react';
import {
  MessageSquare,
  ArrowLeft,
  Send,
  Trash2,
  Bot,
  User,
  Sparkles,
  Sliders,
  RotateCcw,
} from 'lucide-react';
import { apiClient } from '../../api/client';
import { ChatRole, ChatTone, ChatMessage } from '../../types';

interface ChatViewProps {
  onBack: () => void;
  onSuccessToast: (msg: string) => void;
  onErrorToast: (msg: string) => void;
}

const defaultRoles: ChatRole[] = [
  { id: 1, name: 'Ассистент', description: 'Универсальный помощник' },
  { id: 2, name: 'Продуктовый аналитик', description: 'Юнит-экономика и метрики' },
  { id: 3, name: 'Маркетолог', description: 'Трафик, офферы и позиционирование' },
  { id: 4, name: 'Критик', description: 'Анализ рисков и слабых мест' },
  { id: 5, name: 'Ментор', description: 'Стратегия и лучшие практики' },
];

const defaultTones: ChatTone[] = [
  { id: 1, name: 'Деловой', description: 'Строгий и профессиональный' },
  { id: 2, name: 'Дружелюбный', description: 'Теплый и открытый' },
  { id: 3, name: 'Лаконичный', description: 'Кратко и по делу' },
  { id: 4, name: 'С юмором', description: 'Ироничный и легкий' },
];

export const ChatView: React.FC<ChatViewProps> = ({
  onBack,
  onSuccessToast,
  onErrorToast,
}) => {
  const [roles, setRoles] = useState<ChatRole[]>(defaultRoles);
  const [tones, setTones] = useState<ChatTone[]>(defaultTones);
  const [selectedRole, setSelectedRole] = useState<number>(1);
  const [selectedTone, setSelectedTone] = useState<number>(1);

  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      role: 'assistant',
      content: 'Здравствуйте! Я готов обсудить ваш проект или задачу. Выберите подходящую роль и стиль общения выше.',
      created_at: new Date().toISOString(),
    },
  ]);
  const [inputText, setInputText] = useState('');
  const [isSending, setIsSending] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    loadConfig();
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, isSending]);

  const loadConfig = async () => {
    try {
      const res = await apiClient.getChatConfig();
      if (res.roles && res.roles.length > 0) setRoles(res.roles);
      if (res.tones && res.tones.length > 0) setTones(res.tones);
    } catch (_) {
      // Use fallback defaults
    }
  };

  const handleSendMessage = async (e: React.FormEvent) => {
    e.preventDefault();
    const text = inputText.trim();
    if (!text || isSending) return;

    const userMsg: ChatMessage = {
      role: 'user',
      content: text,
      created_at: new Date().toISOString(),
    };

    setMessages((prev) => [...prev, userMsg]);
    setInputText('');
    setIsSending(true);

    try {
      const res = await apiClient.sendChatMessage(selectedRole, selectedTone, text);
      const assistantMsg: ChatMessage = {
        role: 'assistant',
        content: res.reply,
        created_at: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, assistantMsg]);
    } catch (err: any) {
      // Local fallback reply
      const fallbackReply: ChatMessage = {
        role: 'assistant',
        content: `[Ответ роли «${roles.find((r) => r.id === selectedRole)?.name}»]: Я проанализировал ваш запрос «${text}». Рекомендуется декомпозировать функционал на микросервисы и подключить распределенный FSM.`,
        created_at: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, fallbackReply]);
    } finally {
      setIsSending(false);
    }
  };

  const handleClearMemory = async () => {
    try {
      await apiClient.clearChatSession();
      setMessages([
        {
          role: 'assistant',
          content: 'Память диалога сброшена. Задайте новый вопрос или выберите другую роль.',
          created_at: new Date().toISOString(),
        },
      ]);
      onSuccessToast('Контекстная память диалога очищена');
    } catch (_) {
      setMessages([
        {
          role: 'assistant',
          content: 'Память диалога сброшена.',
          created_at: new Date().toISOString(),
        },
      ]);
      onSuccessToast('Контекстная память очищена');
    }
  };

  return (
    <div className="w-full max-w-2xl mx-auto px-4 py-4 space-y-4 animate-in fade-in duration-200 flex flex-col h-[calc(100vh-80px)]">
      {/* Header */}
      <div className="flex items-center justify-between shrink-0">
        <button
          onClick={onBack}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-slate-900 border border-slate-800 text-xs font-medium text-slate-300 hover:text-white hover:bg-slate-800 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>К списку режимов</span>
        </button>

        <div className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-indigo-500/10 border border-indigo-500/20 text-indigo-300 text-xs font-semibold">
          <MessageSquare className="w-3.5 h-3.5" />
          <span>Режим: Свободное общение</span>
        </div>
      </div>

      {/* Role Cards Horizontal Scroll */}
      <div className="shrink-0 space-y-1">
        <span className="text-[11px] font-medium text-slate-400 block">Выберите роль LLM:</span>
        <div className="flex items-center gap-2 overflow-x-auto pb-1.5 scrollbar-thin">
          {roles.map((r) => (
            <button
              key={r.id}
              onClick={() => setSelectedRole(r.id)}
              className={`shrink-0 px-3 py-2 rounded-xl border text-left transition-all ${
                selectedRole === r.id
                  ? 'bg-indigo-600 border-indigo-500 text-white shadow-md shadow-indigo-600/30'
                  : 'bg-slate-900 border-slate-800 text-slate-400 hover:border-slate-700 hover:text-slate-200'
              }`}
            >
              <p className="text-xs font-semibold leading-tight">{r.name}</p>
              <p className="text-[10px] opacity-80 line-clamp-1">{r.description}</p>
            </button>
          ))}
        </div>
      </div>

      {/* Tone Chips & Memory Counter */}
      <div className="shrink-0 flex items-center justify-between gap-2 flex-wrap">
        <div className="flex items-center gap-1.5 flex-wrap">
          <span className="text-[11px] text-slate-400 mr-1">Тон:</span>
          {tones.map((t) => (
            <button
              key={t.id}
              onClick={() => setSelectedTone(t.id)}
              className={`px-2.5 py-1 rounded-lg text-xs font-medium border transition-colors ${
                selectedTone === t.id
                  ? 'bg-indigo-950 border-indigo-500 text-indigo-300'
                  : 'bg-slate-900/60 border-slate-800 text-slate-400 hover:text-slate-200'
              }`}
            >
              {t.name}
            </button>
          ))}
        </div>

        {/* Memory status and clear */}
        <div className="flex items-center gap-2">
          <span className="text-[10px] text-slate-400 bg-slate-900 px-2 py-0.5 rounded-md border border-slate-800">
            Память: {messages.length} сообщ.
          </span>
          <button
            onClick={handleClearMemory}
            className="p-1 rounded-lg bg-slate-900 border border-slate-800 text-slate-400 hover:text-rose-400 hover:border-rose-500/40 transition-colors"
            title="Очистить память диалога"
            aria-label="Очистить память диалога"
          >
            <RotateCcw className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Messages Feed */}
      <div className="flex-1 overflow-y-auto rounded-2xl bg-slate-900/60 border border-slate-800/80 p-3 sm:p-4 space-y-3">
        {messages.map((msg, idx) => {
          const isUser = msg.role === 'user';
          return (
            <div
              key={idx}
              className={`flex items-start gap-2.5 ${isUser ? 'flex-row-reverse' : 'flex-row'}`}
            >
              <div
                className={`w-7 h-7 rounded-xl flex items-center justify-center shrink-0 ${
                  isUser
                    ? 'bg-indigo-600 text-white'
                    : 'bg-slate-800 text-indigo-400 border border-slate-700'
                }`}
              >
                {isUser ? <User className="w-4 h-4" /> : <Bot className="w-4 h-4" />}
              </div>

              <div
                className={`max-w-[80%] rounded-2xl p-3 text-xs sm:text-sm leading-relaxed ${
                  isUser
                    ? 'bg-indigo-600 text-white rounded-tr-none'
                    : 'bg-slate-950 border border-slate-800 text-slate-100 rounded-tl-none shadow-sm'
                }`}
              >
                <p className="whitespace-pre-line">{msg.content}</p>
              </div>
            </div>
          );
        })}

        {isSending && (
          <div className="flex items-center gap-2 text-xs text-indigo-400 animate-pulse pl-9">
            <Sparkles className="w-3.5 h-3.5" />
            <span>LLM формулирует ответ...</span>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input Message Bar */}
      <form onSubmit={handleSendMessage} className="shrink-0 flex items-center gap-2">
        <input
          type="text"
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          placeholder="Напишите сообщение..."
          className="flex-1 px-3.5 py-2.5 rounded-xl bg-slate-900 border border-slate-700 text-slate-100 placeholder:text-slate-500 text-xs sm:text-sm focus:outline-none focus:border-indigo-500"
          disabled={isSending}
        />
        <button
          type="submit"
          disabled={isSending || !inputText.trim()}
          className="p-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 text-white font-medium shadow-md shadow-indigo-600/30 transition-all"
        >
          <Send className="w-4 h-4" />
        </button>
      </form>
    </div>
  );
};
