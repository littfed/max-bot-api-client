import React, { useState, useEffect, useRef } from 'react';
import {
  Send,
  Trash2,
  Check,
  Settings,
  HelpCircle,
  ChevronDown,
  X,
} from 'lucide-react';
import { apiClient } from '../../api/client';
import { ChatMessage } from '../../types';

interface ChatViewProps {
  onBack: () => void;
  onSuccessToast: (msg: string) => void;
  onErrorToast: (msg: string) => void;
}

interface RoleMeta {
  id: number;
  shortName: string;
  fullName: string;
  desc: string;
  iconEmoji: string;
}

interface ToneMeta {
  id: number;
  shortName: string;
  fullName: string;
  desc: string;
  iconEmoji: string;
}

const rolesList: RoleMeta[] = [
  {
    id: 1,
    shortName: 'Ассистент',
    fullName: 'Универсальный ассистент',
    desc: 'Универсальный помощник для широкого круга повседневных и деловых задач',
    iconEmoji: '🤖',
  },
  {
    id: 2,
    shortName: 'Аналитик',
    fullName: 'Бизнес-аналитик / Продакт',
    desc: 'Анализ гипотез, бизнес-моделей, юнит-экономики и продуктовых требований',
    iconEmoji: '📊',
  },
  {
    id: 3,
    shortName: 'Маркетолог',
    fullName: 'Маркетолог / Копирайтер',
    desc: 'Офферы, воронки продаж, продающие тексты и позиционирование продукта',
    iconEmoji: '🎯',
  },
  {
    id: 4,
    shortName: 'Критик',
    fullName: 'Строгий критик идей',
    desc: 'Поиск уязвимостей, финансовых дыр, рисков и критических нестыковок',
    iconEmoji: '🧐',
  },
  {
    id: 5,
    shortName: 'Ментор',
    fullName: 'Дружелюбный ментор',
    desc: 'Поддержка, советы, наводящие вопросы и стратегическое развитие',
    iconEmoji: '🌱',
  },
];

const tonesList: ToneMeta[] = [
  {
    id: 1,
    shortName: 'Деловой',
    fullName: 'Официально-деловой',
    desc: 'Формальный стиль, четкая логика, списки и профессиональный тон',
    iconEmoji: '💼',
  },
  {
    id: 2,
    shortName: 'Дружелюбный',
    fullName: 'Дружелюбный',
    desc: 'Теплый и поддерживающий стиль с уместными эмодзи',
    iconEmoji: '😊',
  },
  {
    id: 3,
    shortName: 'Лаконичный',
    fullName: 'Лаконичный — только суть',
    desc: 'Предельно емко и по существу, без лишней воды — 2–4 предложения',
    iconEmoji: '⚡',
  },
  {
    id: 4,
    shortName: 'Юмор',
    fullName: 'С лёгким юмором',
    desc: 'Остроумно, с позитивной иронией и яркими метафорами',
    iconEmoji: '😄',
  },
];

export const ChatView: React.FC<ChatViewProps> = ({
  onBack,
  onSuccessToast,
  onErrorToast,
}) => {
  const [selectedRole, setSelectedRole] = useState<number>(1); // Default: Ассистент
  const [selectedTone, setSelectedTone] = useState<number>(1); // Default: Деловой
  const [showSettingsModal, setShowSettingsModal] = useState(false);
  const [showRoleMenu, setShowRoleMenu] = useState(false);
  const [showToneMenu, setShowToneMenu] = useState(false);
  const [activeInfoRole, setActiveInfoRole] = useState<number | null>(null);

  const [memoryCount, setMemoryCount] = useState(0);
  const maxMemory = 15;

  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      role: 'assistant',
      content: 'Здравствуйте! Я ваш Ассистент. Опишите вопрос или задачу, которую нужно разобрать.',
      created_at: new Date().toISOString(),
    },
  ]);
  const [inputText, setInputText] = useState('');
  const [isSending, setIsSending] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const roleMenuRef = useRef<HTMLDivElement>(null);
  const toneMenuRef = useRef<HTMLDivElement>(null);

  // Close quick popups when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (roleMenuRef.current && !roleMenuRef.current.contains(e.target as Node)) {
        setShowRoleMenu(false);
      }
      if (toneMenuRef.current && !toneMenuRef.current.contains(e.target as Node)) {
        setShowToneMenu(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, []);

  // Load session from server on mount
  useEffect(() => {
    let isMounted = true;
    apiClient
      .getChatSession()
      .then((session) => {
        if (!isMounted) return;
        if (session.role_id) setSelectedRole(session.role_id);
        if (session.tone_id) setSelectedTone(session.tone_id);
        if (session.messages && session.messages.length > 0) {
          setMessages(session.messages);
          setMemoryCount(session.messages.length);
        } else {
          const role = rolesList.find((r) => r.id === (session.role_id || 1));
          setMessages([
            {
              role: 'assistant',
              content: `Здравствуйте! Я ваш ${role?.shortName || 'Ассистент'}. Опишите вопрос или задачу, которую нужно разобрать.`,
              created_at: new Date().toISOString(),
            },
          ]);
        }
      })
      .catch((err) => {
        console.warn('Failed to load chat session from server:', err);
      });

    return () => {
      isMounted = false;
    };
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSelectRole = async (roleId: number) => {
    setSelectedRole(roleId);
    setShowRoleMenu(false);
    const role = rolesList.find((r) => r.id === roleId);
    onSuccessToast(`Роль: ${role?.shortName}`);
    if (messages.length <= 1) {
      setMessages([
        {
          role: 'assistant',
          content: `Здравствуйте! Я ваш ${role?.shortName}. Готов помочь — напишите ваш вопрос или задачу.`,
          created_at: new Date().toISOString(),
        },
      ]);
    }
    try {
      await apiClient.setChatSettings(roleId, selectedTone);
    } catch (err) {
      console.warn('Failed to persist chat role to server:', err);
    }
  };

  const handleSelectTone = async (toneId: number) => {
    setSelectedTone(toneId);
    setShowToneMenu(false);
    const tone = tonesList.find((t) => t.id === toneId);
    onSuccessToast(`Тон: ${tone?.shortName}`);
    try {
      await apiClient.setChatSettings(selectedRole, toneId);
    } catch (err) {
      console.warn('Failed to persist chat tone to server:', err);
    }
  };

  const handleClearHistory = async () => {
    try {
      await apiClient.clearChatSession();
      const role = rolesList.find((r) => r.id === selectedRole);
      setMessages([
        {
          role: 'assistant',
          content: `Память очищена. Здравствуйте! Я ваш ${role?.shortName}. О чем поговорим?`,
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

  const currentRole = rolesList.find((r) => r.id === selectedRole) || rolesList[0];
  const currentTone = tonesList.find((t) => t.id === selectedTone) || tonesList[0];

  return (
    <div className="w-full max-w-md sm:max-w-lg mx-auto px-3 sm:px-4 py-2 sm:py-3 animate-in fade-in duration-200">
      {/* Main Dialogue Container */}
      <div className="flex flex-col h-[74vh] sm:h-[78vh] bg-white rounded-2xl border border-slate-200/90 shadow-sm overflow-hidden relative">
        {/* Top Control Bar: Quick Role/Tone switchers + Settings Gear */}
        <div className="px-3 py-2 bg-slate-50 border-b border-slate-200/80 flex items-center justify-between gap-1.5 text-xs select-none">
          <div className="flex items-center gap-1.5 min-w-0">
            {/* Quick Role Switcher Pill */}
            <div className="relative" ref={roleMenuRef}>
              <button
                type="button"
                onClick={() => {
                  setShowRoleMenu((prev) => !prev);
                  setShowToneMenu(false);
                }}
                className={`flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-semibold transition-all border ${
                  showRoleMenu
                    ? 'bg-teal-100 text-teal-900 border-teal-300 shadow-xs'
                    : 'bg-teal-50 hover:bg-teal-100 text-teal-800 border-teal-200/70'
                }`}
                title="Быстрая смена роли"
              >
                <span>{currentRole.iconEmoji}</span>
                <span className="truncate max-w-[85px] sm:max-w-[110px]">{currentRole.shortName}</span>
                <ChevronDown className="w-3 h-3 text-teal-600 shrink-0" />
              </button>

              {/* Quick Role Dropdown */}
              {showRoleMenu && (
                <div className="absolute top-full left-0 mt-1 w-60 bg-white rounded-xl shadow-xl border border-slate-200 py-1 z-40 animate-in fade-in slide-in-from-top-1">
                  <div className="px-3 py-1 text-[10px] font-bold text-slate-400 uppercase tracking-wider">
                    Роль ИИ
                  </div>
                  {rolesList.map((r) => {
                    const isSel = selectedRole === r.id;
                    const showInfo = activeInfoRole === r.id;
                    return (
                      <div key={r.id}>
                        <div
                          className={`px-3 py-2 flex items-center justify-between text-xs cursor-pointer hover:bg-slate-50 transition-colors ${
                            isSel ? 'text-teal-700 font-bold bg-teal-50/60' : 'text-slate-700'
                          }`}
                          onClick={() => handleSelectRole(r.id)}
                        >
                          <div className="flex items-center gap-2">
                            <span>{r.iconEmoji}</span>
                            <span>{r.shortName}</span>
                          </div>
                          <div className="flex items-center gap-1.5">
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                setActiveInfoRole(showInfo ? null : r.id);
                              }}
                              className="p-1 rounded-full text-slate-400 hover:text-teal-700 hover:bg-slate-100 transition-colors"
                              title="Информация о роли"
                            >
                              <HelpCircle className="w-3.5 h-3.5" />
                            </button>
                            {isSel && <Check className="w-3.5 h-3.5 text-teal-600 stroke-[3]" />}
                          </div>
                        </div>
                        {showInfo && (
                          <div className="px-3 py-1.5 bg-slate-50 text-[11px] text-slate-600 border-t border-b border-slate-100">
                            <span className="font-semibold text-slate-800">{r.fullName}: </span>
                            {r.desc}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </div>

            <span className="text-slate-300">•</span>

            {/* Quick Tone Switcher Pill */}
            <div className="relative" ref={toneMenuRef}>
              <button
                type="button"
                onClick={() => {
                  setShowToneMenu((prev) => !prev);
                  setShowRoleMenu(false);
                }}
                className={`flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-semibold transition-all border ${
                  showToneMenu
                    ? 'bg-slate-200 text-slate-900 border-slate-300 shadow-xs'
                    : 'bg-slate-100 hover:bg-slate-200 text-slate-700 border-slate-200'
                }`}
                title="Быстрая смена тональности"
              >
                <span>{currentTone.iconEmoji}</span>
                <span className="truncate max-w-[80px] sm:max-w-[100px]">{currentTone.shortName}</span>
                <ChevronDown className="w-3 h-3 text-slate-500 shrink-0" />
              </button>

              {/* Quick Tone Dropdown */}
              {showToneMenu && (
                <div className="absolute top-full left-0 mt-1 w-52 bg-white rounded-xl shadow-xl border border-slate-200 py-1 z-40 animate-in fade-in slide-in-from-top-1">
                  <div className="px-3 py-1 text-[10px] font-bold text-slate-400 uppercase tracking-wider">
                    Тональность
                  </div>
                  {tonesList.map((t) => {
                    const isSel = selectedTone === t.id;
                    return (
                      <div
                        key={t.id}
                        className={`px-3 py-2 flex items-center justify-between text-xs cursor-pointer hover:bg-slate-50 transition-colors ${
                          isSel ? 'text-teal-700 font-bold bg-teal-50/60' : 'text-slate-700'
                        }`}
                        onClick={() => handleSelectTone(t.id)}
                      >
                        <div className="flex items-center gap-2">
                          <span>{t.iconEmoji}</span>
                          <span>{t.shortName}</span>
                        </div>
                        {isSel && <Check className="w-3.5 h-3.5 text-teal-600 stroke-[3]" />}
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>

          {/* Settings Gear Button */}
          <button
            type="button"
            onClick={() => {
              setShowSettingsModal(true);
              setShowRoleMenu(false);
              setShowToneMenu(false);
            }}
            className="p-1.5 rounded-lg text-slate-500 hover:text-teal-700 hover:bg-slate-200/60 transition-colors shrink-0 flex items-center gap-1"
            title="Все настройки ИИ"
          >
            <Settings className="w-4 h-4" />
          </button>
        </div>

        {/* Messages Thread */}
        <div className="flex-1 overflow-y-auto p-3.5 sm:p-4 space-y-3">
          {messages.map((msg, i) => (
            <div
              key={i}
              className={`flex gap-2.5 ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
            >
              {msg.role === 'assistant' && (
                <div className="w-7 h-7 rounded-full bg-teal-100 text-teal-700 flex items-center justify-center shrink-0 mt-0.5 text-xs font-bold shadow-xs">
                  {currentRole.iconEmoji}
                </div>
              )}
              <div
                className={`max-w-[82%] rounded-2xl px-3.5 py-2.5 text-xs sm:text-sm leading-relaxed ${
                  msg.role === 'user'
                    ? 'bg-[#0d9488] text-white rounded-tr-sm shadow-xs'
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
              <span className="italic">ИИ печатает ответ...</span>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* Input Bar */}
        <form
          onSubmit={handleSendMessage}
          className="p-2.5 sm:p-3 border-t border-slate-100 flex items-center gap-2 bg-slate-50"
        >
          <input
            type="text"
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            placeholder="Напишите сообщение..."
            className="flex-1 px-3.5 py-2 rounded-xl bg-white border border-slate-200 text-xs sm:text-sm text-slate-900 placeholder:text-slate-400 focus:outline-none focus:border-teal-500 shadow-xs"
          />
          <button
            type="submit"
            disabled={isSending || !inputText.trim()}
            className="p-2.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] disabled:opacity-40 text-white shadow-xs transition-all shrink-0"
          >
            <Send className="w-4 h-4" />
          </button>
        </form>
      </div>

      {/* --- POPUP SETTINGS MODAL --- */}
      {showSettingsModal && (
        <div className="fixed inset-0 z-50 bg-slate-900/50 backdrop-blur-xs flex items-end sm:items-center justify-center p-0 sm:p-4 animate-in fade-in duration-200">
          <div className="w-full max-w-md bg-white rounded-t-3xl sm:rounded-2xl border border-slate-200 shadow-2xl max-h-[85vh] flex flex-col overflow-hidden animate-in slide-in-from-bottom-4 duration-200">
            {/* Modal Header */}
            <div className="px-5 py-4 border-b border-slate-100 flex items-center justify-between bg-slate-50/80">
              <div className="flex items-center gap-2">
                <Settings className="w-4 h-4 text-teal-600" />
                <h3 className="font-bold text-sm text-slate-900">Настройки ИИ</h3>
              </div>
              <button
                type="button"
                onClick={() => setShowSettingsModal(false)}
                className="p-1.5 rounded-full text-slate-400 hover:text-slate-700 hover:bg-slate-200/60 transition-colors"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            {/* Modal Body */}
            <div className="p-5 overflow-y-auto space-y-4">
              {/* Section 1: Role */}
              <div>
                <h4 className="font-bold text-xs text-slate-500 uppercase tracking-wider mb-2">
                  Роль виртуального собеседника
                </h4>
                <div className="space-y-1.5">
                  {rolesList.map((r) => {
                    const isSel = selectedRole === r.id;
                    const showInfo = activeInfoRole === r.id;
                    return (
                      <div key={r.id} className="rounded-xl border border-slate-200 overflow-hidden">
                        <div
                          onClick={() => handleSelectRole(r.id)}
                          className={`p-2.5 flex items-center justify-between cursor-pointer transition-colors text-xs ${
                            isSel
                              ? 'bg-teal-50/80 text-teal-900 font-semibold'
                              : 'bg-white text-slate-700 hover:bg-slate-50'
                          }`}
                        >
                          <div className="flex items-center gap-2">
                            <span>{r.iconEmoji}</span>
                            <span className="font-semibold">{r.shortName}</span>
                            <span className="text-slate-400 text-[11px]">— {r.fullName}</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                setActiveInfoRole(showInfo ? null : r.id);
                              }}
                              className="p-1 rounded-full text-slate-400 hover:text-teal-700 hover:bg-slate-100"
                              title="Подробнее о роли"
                            >
                              <HelpCircle className="w-3.5 h-3.5" />
                            </button>
                            {isSel && <Check className="w-3.5 h-3.5 text-teal-600 stroke-[3]" />}
                          </div>
                        </div>
                        {showInfo && (
                          <div className="px-3 py-2 bg-slate-50 border-t border-slate-100 text-[11px] text-slate-600">
                            <p className="font-semibold text-slate-800">{r.fullName}</p>
                            <p className="mt-0.5">{r.desc}</p>
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>

              {/* Section 2: Tone */}
              <div>
                <h4 className="font-bold text-xs text-slate-500 uppercase tracking-wider mb-2">
                  Тональность ответов
                </h4>
                <div className="grid grid-cols-2 gap-2">
                  {tonesList.map((t) => {
                    const isSel = selectedTone === t.id;
                    return (
                      <button
                        key={t.id}
                        type="button"
                        onClick={() => handleSelectTone(t.id)}
                        className={`p-2.5 rounded-xl border text-xs font-semibold transition-all flex items-center gap-2 ${
                          isSel
                            ? 'border-teal-500 bg-teal-50 text-teal-900 shadow-xs'
                            : 'border-slate-200 bg-white text-slate-700 hover:bg-slate-50'
                        }`}
                      >
                        <span>{t.iconEmoji}</span>
                        <span>{t.shortName}</span>
                        {isSel && <Check className="w-3 h-3 ml-auto text-teal-600 stroke-[3]" />}
                      </button>
                    );
                  })}
                </div>
              </div>

              {/* Section 3: Context Memory */}
              <div className="p-3.5 rounded-xl bg-slate-50 border border-slate-200/80 space-y-2.5">
                <div className="flex items-center justify-between text-xs text-slate-700 font-medium">
                  <span>Память контекста</span>
                  <span className="font-semibold text-teal-800">{memoryCount} / {maxMemory} пар</span>
                </div>
                <div className="w-full h-1.5 rounded-full bg-slate-200 overflow-hidden">
                  <div
                    className="h-full bg-teal-600 transition-all duration-300"
                    style={{ width: `${Math.min(100, (memoryCount / maxMemory) * 100)}%` }}
                  />
                </div>
                <p className="text-[11px] text-slate-400">
                  Смена роли сохраняет контекст беседы
                </p>
                <button
                  type="button"
                  onClick={handleClearHistory}
                  className="w-full py-2 rounded-lg border border-slate-200 bg-white hover:bg-slate-50 text-slate-700 font-medium text-xs transition-colors flex items-center justify-center gap-1.5"
                >
                  <Trash2 className="w-3.5 h-3.5 text-slate-400" />
                  <span>Очистить память диалога</span>
                </button>
              </div>
            </div>

            {/* Modal Footer */}
            <div className="p-4 border-t border-slate-100 bg-slate-50">
              <button
                type="button"
                onClick={() => setShowSettingsModal(false)}
                className="w-full py-2.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] text-white text-xs font-semibold shadow-xs transition-colors"
              >
                Готово
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
