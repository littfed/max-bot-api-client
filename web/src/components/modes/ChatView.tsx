import React, { useState, useEffect, useRef } from 'react';
import {
  MessageSquare,
  Send,
  Trash2,
  Check,
  Sparkles,
  Bot,
  User as UserIcon,
} from 'lucide-react';
import { apiClient } from '../../api/client';
import { ChatRole, ChatTone, ChatMessage } from '../../types';

interface ChatViewProps {
  onBack: () => void;
  onSuccessToast: (msg: string) => void;
  onErrorToast: (msg: string) => void;
}

const rolesList = [
  { id: 1, name: 'Универсальный ассистент', desc: 'Универсальный помощник для широкого круга задач' },
  { id: 2, name: 'Бизнес-аналитик / Продакт', desc: 'Анализ гипотез, бизнес-моделей и требований' },
  { id: 3, name: 'Маркетолог / Копирайтер', desc: 'Офферы, воронки продаж и тексты' },
  { id: 4, name: 'Строгий критик идей', desc: 'Поиск уязвимостей, рисков и критических ошибок' },
  { id: 5, name: 'Дружелюбный ментор', desc: 'Поддержка, советы и стратегическое развитие' },
];

const tonesList = [
  { id: 1, name: 'Официально-деловой' },
  { id: 2, name: 'Дружелюбный' },
  { id: 3, name: 'Лаконичный — только суть' },
  { id: 4, name: 'С лёгким юмором' },
];

export const ChatView: React.FC<ChatViewProps> = ({
  onBack,
  onSuccessToast,
  onErrorToast,
}) => {
  const [activeTab, setActiveTab] = useState<'dialog' | 'settings'>('settings');
  const [selectedRole, setSelectedRole] = useState<number>(2); // Default Бизнес-аналитик
  const [selectedTone, setSelectedTone] = useState<number>(1); // Default Официально-деловой

  const [memoryCount, setMemoryCount] = useState(8);
  const maxMemory = 15;

  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      role: 'assistant',
      content: 'Здравствуйте! Я ваш Бизнес-аналитик / Продакт. Опишите продукт или гипотезу, которую нужно разобрать.',
      created_at: new Date().toISOString(),
    },
  ]);
  const [inputText, setInputText] = useState('');
  const [isSending, setIsSending] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSelectRole = (roleId: number) => {
    setSelectedRole(roleId);
    const role = rolesList.find((r) => r.id === roleId);
    onSuccessToast(`Роль переключена на: ${role?.name}`);
    if (messages.length <= 1) {
      setMessages([
        {
          role: 'assistant',
          content: `Здравствуйте! Я ваш ${role?.name}. Готов помочь — напишите ваш вопрос или задачу.`,
          created_at: new Date().toISOString(),
        },
      ]);
    }
  };

  const handleSelectTone = (toneId: number) => {
    setSelectedTone(toneId);
    const tone = tonesList.find((t) => t.id === toneId);
    onSuccessToast(`Тональность: ${tone?.name}`);
  };

  const handleClearHistory = async () => {
    try {
      await apiClient.clearChatSession();
      const role = rolesList.find((r) => r.id === selectedRole);
      setMessages([
        {
          role: 'assistant',
          content: `Память очищена. Здравствуйте! Я ваш ${role?.name}. О чем поговорим?`,
          created_at: new Date().toISOString(),
        },
      ]);
      setMemoryCount(0);
      onSuccessToast('Память диалога успешно очищена');
    } catch (_) {
      setMessages([]);
      setMemoryCount(0);
      onSuccessToast('Память диалога очищена');
    }
  };

  const handleSendMessage = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputText.trim() || isSending) return;

    const userMsg: ChatMessage = {
      role: 'user',
      content: inputText.trim(),
      created_at: new Date().toISOString(),
    };

    setMessages((prev) => [...prev, userMsg]);
    setInputText('');
    setIsSending(true);

    try {
      const res = await apiClient.sendChatMessage(selectedRole, selectedTone, userMsg.content);
      const botMsg: ChatMessage = {
        role: 'assistant',
        content: res.reply,
        created_at: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, botMsg]);
      setMemoryCount((prev) => Math.min(prev + 1, maxMemory));
    } catch (err: any) {
      const errMsg = err?.response?.data?.error || err?.message || 'Не удалось связаться с нейросетью';
      onErrorToast(`Ошибка ИИ: ${errMsg}`);
      const botMsg: ChatMessage = {
        role: 'assistant',
        content: `⚠️ Не удалось получить ответ от нейросети (${errMsg}). Пожалуйста, проверьте подключение или отправьте запрос повторно.`,
        created_at: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, botMsg]);
    } finally {
      setIsSending(false);
    }
  };

  return (
    <div className="w-full max-w-md sm:max-w-lg mx-auto px-4 py-3 sm:py-4 space-y-4 animate-in fade-in duration-200">
      {/* Segmented Control / Tabs matching Mockup */}
      <div className="flex items-center gap-1.5 p-1 rounded-2xl bg-slate-200/60 border border-slate-200">
        <button
          onClick={() => setActiveTab('dialog')}
          className={`flex-1 py-1.5 text-xs font-semibold rounded-xl transition-all ${
            activeTab === 'dialog'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          Диалог
        </button>
        <button
          onClick={() => setActiveTab('settings')}
          className={`flex-1 py-1.5 text-xs font-semibold rounded-xl transition-all ${
            activeTab === 'settings'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          Настройки ИИ
        </button>
      </div>

      {/* --- TAB 1: AI SETTINGS MATCHING MOCKUP --- */}
      {activeTab === 'settings' && (
        <div className="space-y-4">
          {/* Section 1: Role matching Mockup */}
          <div className="p-4 sm:p-5 rounded-2xl bg-white border border-slate-200/90 shadow-sm space-y-2.5">
            <h3 className="font-bold text-sm text-slate-900">
              Роль ассистента
            </h3>

            <div className="space-y-2">
              {rolesList.map((r) => {
                const isSel = selectedRole === r.id;
                return (
                  <div
                    key={r.id}
                    onClick={() => handleSelectRole(r.id)}
                    className={`p-3 rounded-xl border cursor-pointer transition-all flex items-center justify-between text-xs ${
                      isSel
                        ? 'border-teal-500 bg-teal-50/50 text-slate-900 font-semibold'
                        : 'border-slate-200 bg-white text-slate-700 hover:bg-slate-50'
                    }`}
                  >
                    <span>{r.name}</span>
                    <div
                      className={`w-5 h-5 rounded-full flex items-center justify-center shrink-0 ml-2 ${
                        isSel
                          ? 'bg-[#0d9488] text-white'
                          : 'border border-slate-300 bg-white'
                      }`}
                    >
                      {isSel && <Check className="w-3 h-3 stroke-[3]" />}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Section 2: Tone matching Mockup */}
          <div className="p-4 sm:p-5 rounded-2xl bg-white border border-slate-200/90 shadow-sm space-y-2.5">
            <h3 className="font-bold text-sm text-slate-900">
              Тональность ответа
            </h3>

            <div className="flex flex-wrap gap-2">
              {tonesList.map((t) => {
                const isSel = selectedTone === t.id;
                return (
                  <button
                    key={t.id}
                    onClick={() => handleSelectTone(t.id)}
                    className={`px-3.5 py-2 rounded-xl text-xs font-semibold transition-all ${
                      isSel
                        ? 'bg-[#0d9488] text-white shadow-sm'
                        : 'bg-slate-100 hover:bg-slate-200 text-slate-700 border border-slate-200/80'
                    }`}
                  >
                    {t.name}
                  </button>
                );
              })}
            </div>
          </div>

          {/* Section 3: Context Memory matching Mockup */}
          <div className="p-4 sm:p-5 rounded-2xl bg-white border border-slate-200/90 shadow-sm space-y-3">
            <h3 className="font-bold text-sm text-slate-900">
              Память контекста
            </h3>

            <div>
              <div className="flex items-center justify-between text-xs text-slate-700 font-medium mb-1.5">
                <span>Сохранено: {memoryCount} из {maxMemory} пар</span>
              </div>
              <div className="w-full h-2 rounded-full bg-slate-100 overflow-hidden">
                <div
                  className="h-full bg-[#0d9488] transition-all duration-300"
                  style={{ width: `${(memoryCount / maxMemory) * 100}%` }}
                />
              </div>
              <p className="text-[11px] text-slate-400 mt-1.5">
                Смена роли сохраняет контекст беседы
              </p>
            </div>

            <button
              onClick={handleClearHistory}
              className="w-full py-2.5 rounded-xl border border-slate-200 text-slate-700 hover:bg-slate-50 font-semibold text-xs transition-all flex items-center justify-center gap-1.5"
            >
              <Trash2 className="w-3.5 h-3.5 text-slate-500" />
              <span>Очистить память диалога</span>
            </button>
          </div>

          <button
            onClick={() => setActiveTab('dialog')}
            className="w-full py-3 rounded-2xl bg-[#0d9488] hover:bg-[#0f766e] text-white font-semibold text-xs sm:text-sm shadow-sm transition-all flex items-center justify-center gap-2"
          >
            <span>Перейти к диалогу с ИИ →</span>
          </button>
        </div>
      )}

      {/* --- TAB 2: DIALOGUE --- */}
      {activeTab === 'dialog' && (
        <div className="flex flex-col h-[65vh] bg-white rounded-2xl border border-slate-200/90 shadow-sm overflow-hidden">
          {/* Active Role and Tone Header Bar */}
          <div className="px-4 py-2.5 bg-slate-50 border-b border-slate-200/80 flex items-center justify-between text-xs">
            <div className="flex items-center gap-1.5 overflow-hidden text-ellipsis whitespace-nowrap">
              <span className="font-semibold text-teal-800">
                {rolesList.find((r) => r.id === selectedRole)?.name}
              </span>
              <span className="text-slate-400">•</span>
              <span className="text-slate-600 font-medium">
                {tonesList.find((t) => t.id === selectedTone)?.name}
              </span>
            </div>
            <button
              onClick={() => setActiveTab('settings')}
              className="text-[11px] font-semibold text-teal-700 hover:text-teal-900 shrink-0 ml-2 bg-teal-50 hover:bg-teal-100 px-2.5 py-1 rounded-lg transition-colors"
            >
              Настройки ⚙
            </button>
          </div>

          {/* Messages Thread */}
          <div className="flex-1 overflow-y-auto p-4 space-y-3.5">
            {messages.map((msg, i) => (
              <div
                key={i}
                className={`flex gap-2.5 ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
              >
                {msg.role === 'assistant' && (
                  <div className="w-7 h-7 rounded-full bg-teal-100 text-teal-700 flex items-center justify-center shrink-0 mt-0.5 text-xs font-bold">
                    🤖
                  </div>
                )}
                <div
                  className={`max-w-[80%] rounded-2xl px-3.5 py-2.5 text-xs sm:text-sm leading-relaxed ${
                    msg.role === 'user'
                      ? 'bg-[#0d9488] text-white rounded-tr-sm shadow-sm'
                      : 'bg-slate-100 text-slate-900 border border-slate-200/60 rounded-tl-sm'
                  }`}
                >
                  {msg.content}
                </div>
              </div>
            ))}
            {isSending && (
              <div className="flex items-center gap-2 text-xs text-slate-400 pl-9">
                <div className="w-1.5 h-1.5 bg-teal-600 rounded-full animate-bounce" />
                <div className="w-1.5 h-1.5 bg-teal-600 rounded-full animate-bounce [animation-delay:0.2s]" />
                <div className="w-1.5 h-1.5 bg-teal-600 rounded-full animate-bounce [animation-delay:0.4s]" />
                <span>ИИ печатает ответ...</span>
              </div>
            )}
            <div ref={messagesEndRef} />
          </div>

          {/* Input Bar */}
          <form
            onSubmit={handleSendMessage}
            className="p-3 border-t border-slate-100 flex items-center gap-2 bg-slate-50"
          >
            <input
              type="text"
              value={inputText}
              onChange={(e) => setInputText(e.target.value)}
              placeholder="Напишите сообщение..."
              className="flex-1 px-3.5 py-2 rounded-xl bg-white border border-slate-200 text-xs sm:text-sm text-slate-900 placeholder:text-slate-400 focus:outline-none focus:border-teal-500"
            />
            <button
              type="submit"
              disabled={isSending || !inputText.trim()}
              className="p-2.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] disabled:opacity-40 text-white shadow-sm transition-all"
            >
              <Send className="w-4 h-4" />
            </button>
          </form>
        </div>
      )}
    </div>
  );
};
