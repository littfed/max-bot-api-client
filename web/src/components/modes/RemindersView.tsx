import React, { useState, useEffect } from 'react';
import {
  Bell,
  Clock,
  Calendar,
  CheckCircle2,
  Trash2,
  Plus,
  Zap,
  Repeat,
  ArrowLeft,
  X,
  AlertCircle,
  Tag,
} from 'lucide-react';
import { apiClient } from '../../api/client';
import { ReminderItem } from '../../types';

interface RemindersViewProps {
  onBack: () => void;
  onSuccessToast: (msg: string) => void;
  onErrorToast: (msg: string) => void;
}

export const RemindersView: React.FC<RemindersViewProps> = ({
  onBack,
  onSuccessToast,
  onErrorToast,
}) => {
  const [reminders, setReminders] = useState<ReminderItem[]>([]);
  const [activeTab, setActiveTab] = useState<'active' | 'history'>('active');
  const [isLoading, setIsLoading] = useState(false);
  const [isModalOpen, setIsModalOpen] = useState(false);

  // Demo simulation countdown state
  const [simCountdown, setSimCountdown] = useState<number | null>(null);

  // Create form state
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [remindAtInput, setRemindAtInput] = useState('');
  const [recurrenceRule, setRecurrenceRule] = useState<'none' | 'daily' | 'weekdays' | 'weekly'>('none');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const fetchReminders = async () => {
    setIsLoading(true);
    try {
      const data = await apiClient.getReminders(activeTab === 'active' ? 'active' : 'all');
      setReminders(data.reminders || []);
    } catch (err: any) {
      onErrorToast('Не удалось загрузить напоминания');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchReminders();
  }, [activeTab]);

  // Countdown timer effect for demo simulation
  useEffect(() => {
    if (simCountdown === null) return;
    if (simCountdown <= 0) {
      setSimCountdown(null);
      fetchReminders();
      onSuccessToast('⏰ Демо-напоминание отправлено вам в чат бота!');
      return;
    }
    const timer = setTimeout(() => {
      setSimCountdown(simCountdown - 1);
    }, 1000);
    return () => clearTimeout(timer);
  }, [simCountdown]);

  const handleApplyPreset = (minutes: number) => {
    const target = new Date(Date.now() + minutes * 60 * 1000);
    setRemindAtInput(target.toISOString().slice(0, 16));
  };

  const handleApplyTomorrow = (hour: number, minute: number) => {
    const target = new Date();
    target.setDate(target.getDate() + 1);
    target.setHours(hour, minute, 0, 0);
    setRemindAtInput(target.toISOString().slice(0, 16));
  };

  const handleCreateReminder = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) {
      onErrorToast('Введите текст задачи');
      return;
    }
    if (!remindAtInput) {
      onErrorToast('Выберите дату и время напоминания');
      return;
    }

    setIsSubmitting(true);
    try {
      const dateObj = new Date(remindAtInput);
      await apiClient.createReminder({
        title: title.trim(),
        description: description.trim(),
        remind_at: dateObj.toISOString(),
        recurrence_rule: recurrenceRule,
      });
      onSuccessToast('Напоминание успешно создано');
      setIsModalOpen(false);
      setTitle('');
      setDescription('');
      setRemindAtInput('');
      setRecurrenceRule('none');
      fetchReminders();
    } catch (err: any) {
      onErrorToast('Ошибка при создании напоминания');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleComplete = async (id: number) => {
    try {
      const res = await apiClient.reminderAction(id, 'complete');
      if (res.next) {
        onSuccessToast('Задача выполнена! Следующая запланирована автоматически.');
      } else {
        onSuccessToast('Задача отмечена как выполненная');
      }
      fetchReminders();
    } catch (err) {
      onErrorToast('Не удалось отметить задачу выполненной');
    }
  };

  const handleSnooze = async (id: number, minutes: number) => {
    try {
      await apiClient.reminderAction(id, 'snooze', minutes);
      onSuccessToast(`Отложено на ${minutes} минут`);
      fetchReminders();
    } catch (err) {
      onErrorToast('Не удалось отложить задачу');
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await apiClient.deleteReminder(id);
      onSuccessToast('Напоминание удалено');
      fetchReminders();
    } catch (err) {
      onErrorToast('Не удалось удалить напоминание');
    }
  };

  const handleTriggerDemo = async () => {
    try {
      await apiClient.simulateReminder();
      setSimCountdown(10);
      onSuccessToast('Тестовый триггер запущен! Сообщение поступит через 10 секунд.');
    } catch (err) {
      onErrorToast('Не удалось запустить симуляцию');
    }
  };

  const activeReminders = reminders.filter((r) => r.status === 'pending' || r.status === 'sent');
  const historyReminders = reminders.filter((r) => r.status === 'completed' || r.status === 'cancelled');
  const displayedReminders = activeTab === 'active' ? activeReminders : historyReminders;

  return (
    <div className="flex flex-col min-h-screen bg-slate-900 text-slate-100 pb-20">
      {/* Top Header */}
      <header className="sticky top-0 z-20 flex items-center justify-between px-4 py-3 bg-slate-900/95 backdrop-blur border-b border-slate-800">
        <div className="flex items-center gap-3">
          <button
            onClick={onBack}
            className="p-2 -ml-2 rounded-xl text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
          >
            <ArrowLeft className="w-5 h-5" />
          </button>
          <div>
            <h1 className="text-lg font-bold text-white flex items-center gap-2">
              <Bell className="w-5 h-5 text-amber-400" />
              Напоминания
            </h1>
            <p className="text-xs text-slate-400">Режим 3: Тайм-менеджер & Триггеры</p>
          </div>
        </div>

        <button
          onClick={() => setIsModalOpen(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 bg-amber-500 hover:bg-amber-600 active:bg-amber-700 text-slate-950 font-semibold rounded-xl text-xs transition-colors shadow-sm"
        >
          <Plus className="w-4 h-4" />
          Создать
        </button>
      </header>

      {/* Main Content */}
      <main className="flex-1 p-4 max-w-lg mx-auto w-full space-y-4">
        {/* Banner with Simulation Trigger */}
        <section className="bg-gradient-to-br from-amber-500/10 via-slate-800 to-slate-800/80 border border-amber-500/30 rounded-2xl p-4 shadow-sm relative overflow-hidden">
          <div className="flex items-start justify-between gap-3">
            <div>
              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium bg-amber-500/20 text-amber-300 mb-1.5">
                <Zap className="w-3 h-3" />
                Live Demo
              </span>
              <h2 className="text-sm font-semibold text-white">Проверьте триггеры в действии</h2>
              <p className="text-xs text-slate-300 mt-1">
                Запустите мгновенное демо-напоминание на 10 сек с A/B шаблоном и кнопками откладывания в чате.
              </p>
            </div>
          </div>

          <div className="mt-3 flex items-center gap-2">
            <button
              onClick={handleTriggerDemo}
              disabled={simCountdown !== null}
              className="flex-1 flex items-center justify-center gap-2 px-3 py-2 bg-amber-500 hover:bg-amber-400 text-slate-950 font-semibold text-xs rounded-xl transition-colors disabled:opacity-50"
            >
              <Zap className="w-3.5 h-3.5 fill-current" />
              {simCountdown !== null ? `Отправка через ${simCountdown} сек...` : '⚡ Тест пуша (10 сек)'}
            </button>
          </div>
        </section>

        {/* Tab switchers */}
        <div className="flex bg-slate-800/80 p-1 rounded-xl border border-slate-700/60">
          <button
            onClick={() => setActiveTab('active')}
            className={`flex-1 py-1.5 text-xs font-semibold rounded-lg transition-all ${
              activeTab === 'active'
                ? 'bg-amber-500 text-slate-950 shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            В планах ({activeReminders.length})
          </button>
          <button
            onClick={() => setActiveTab('history')}
            className={`flex-1 py-1.5 text-xs font-semibold rounded-lg transition-all ${
              activeTab === 'history'
                ? 'bg-amber-500 text-slate-950 shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            История
          </button>
        </div>

        {/* Reminders List */}
        {isLoading ? (
          <div className="flex flex-col items-center justify-center py-12 text-slate-500 gap-2">
            <div className="w-6 h-6 border-2 border-amber-500 border-t-transparent rounded-full animate-spin" />
            <span className="text-xs">Загрузка задач...</span>
          </div>
        ) : displayedReminders.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 px-4 text-center rounded-2xl border border-dashed border-slate-800 bg-slate-850/40">
            <Clock className="w-10 h-10 text-slate-600 mb-2" />
            <h3 className="text-sm font-semibold text-slate-300">
              {activeTab === 'active' ? 'Нет активных напоминаний' : 'История пуста'}
            </h3>
            <p className="text-xs text-slate-500 max-w-xs mt-1">
              {activeTab === 'active'
                ? 'Запланируйте разовую задачу или повторяющуюся привычку прямо сейчас.'
                : 'Выполненные и отмененные задачи будут сохраняться здесь.'}
            </p>
            {activeTab === 'active' && (
              <button
                onClick={() => setIsModalOpen(true)}
                className="mt-4 px-4 py-2 bg-slate-800 hover:bg-slate-700 text-amber-400 text-xs font-medium rounded-xl border border-slate-700 transition-colors"
              >
                + Создать задачу
              </button>
            )}
          </div>
        ) : (
          <div className="space-y-3">
            {displayedReminders.map((rem) => {
              const remindDate = new Date(rem.remind_at);
              const isOverdue = rem.status === 'pending' && remindDate.getTime() < Date.now();

              return (
                <div
                  key={rem.id}
                  className={`bg-slate-800/90 border rounded-2xl p-4 transition-all ${
                    isOverdue
                      ? 'border-red-500/40 bg-red-950/10'
                      : 'border-slate-700/60 hover:border-slate-600'
                  }`}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-1.5 flex-wrap mb-1">
                        {rem.source_type === 'reception' ? (
                          <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-blue-500/20 text-blue-300 border border-blue-500/30">
                            🛎️ Ресепшен
                          </span>
                        ) : rem.source_type === 'demo_simulation' ? (
                          <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/20 text-amber-300 border border-amber-500/30">
                            ⚡ Демо
                          </span>
                        ) : (
                          <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-slate-700 text-slate-300">
                            📌 Личное
                          </span>
                        )}

                        {rem.recurrence_rule && rem.recurrence_rule !== 'none' && (
                          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium bg-purple-500/20 text-purple-300 border border-purple-500/30">
                            <Repeat className="w-2.5 h-2.5" />
                            {rem.recurrence_rule === 'daily'
                              ? 'Ежедневно'
                              : rem.recurrence_rule === 'weekdays'
                              ? 'По будням'
                              : 'Еженедельно'}
                          </span>
                        )}

                        {rem.status === 'completed' && (
                          <span className="px-2 py-0.5 rounded-full text-[10px] font-medium bg-emerald-500/20 text-emerald-300">
                            Выполнено
                          </span>
                        )}
                      </div>

                      <h4 className="text-sm font-semibold text-white truncate">{rem.title}</h4>
                      {rem.description && (
                        <p className="text-xs text-slate-400 mt-0.5 line-clamp-2">{rem.description}</p>
                      )}

                      <div className="flex items-center gap-1.5 mt-2 text-xs text-slate-400">
                        <Clock className={`w-3.5 h-3.5 ${isOverdue ? 'text-red-400' : 'text-slate-400'}`} />
                        <span className={isOverdue ? 'text-red-400 font-medium' : ''}>
                          {remindDate.toLocaleString('ru-RU', {
                            day: 'numeric',
                            month: 'short',
                            hour: '2-digit',
                            minute: '2-digit',
                          })}
                        </span>
                        {isOverdue && <span className="text-[10px] text-red-400 font-semibold">(срок)</span>}
                      </div>
                    </div>

                    {/* Actions */}
                    {rem.status !== 'completed' && rem.status !== 'cancelled' ? (
                      <div className="flex items-center gap-1">
                        <button
                          onClick={() => handleComplete(rem.id)}
                          title="Отметить выполненным"
                          className="p-2 text-emerald-400 hover:bg-emerald-500/20 rounded-xl transition-colors"
                        >
                          <CheckCircle2 className="w-5 h-5" />
                        </button>
                        <button
                          onClick={() => handleSnooze(rem.id, 15)}
                          title="Отложить на 15 мин"
                          className="px-2 py-1 text-[11px] font-medium text-amber-300 bg-amber-500/10 hover:bg-amber-500/20 border border-amber-500/30 rounded-lg transition-colors"
                        >
                          +15м
                        </button>
                        <button
                          onClick={() => handleDelete(rem.id)}
                          title="Удалить"
                          className="p-2 text-slate-500 hover:text-red-400 hover:bg-red-500/10 rounded-xl transition-colors"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    ) : (
                      <button
                        onClick={() => handleDelete(rem.id)}
                        className="p-1.5 text-slate-600 hover:text-slate-400"
                        title="Удалить из истории"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </main>

      {/* Creation Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 bg-slate-950/80 backdrop-blur-sm animate-fade-in">
          <div className="w-full max-w-md bg-slate-900 border border-slate-800 rounded-t-3xl sm:rounded-3xl p-5 space-y-4 shadow-2xl max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between pb-2 border-b border-slate-800">
              <h3 className="text-base font-bold text-white flex items-center gap-2">
                <Plus className="w-5 h-5 text-amber-400" />
                Новое напоминание
              </h3>
              <button
                onClick={() => setIsModalOpen(false)}
                className="p-1.5 text-slate-400 hover:text-white rounded-xl hover:bg-slate-800 transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleCreateReminder} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">
                  Что вам напомнить? *
                </label>
                <input
                  type="text"
                  required
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder="Например: Позвонить врачу, Оплатить счет"
                  className="w-full px-3.5 py-2.5 bg-slate-800 border border-slate-700 rounded-xl text-sm text-white placeholder-slate-500 focus:outline-none focus:border-amber-500 transition-colors"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">
                  Заметка или описание
                </label>
                <textarea
                  rows={2}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Дополнительные детали..."
                  className="w-full px-3.5 py-2 bg-slate-800 border border-slate-700 rounded-xl text-xs text-white placeholder-slate-500 focus:outline-none focus:border-amber-500 transition-colors resize-none"
                />
              </div>

              {/* Quick time presets */}
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">
                  Быстрый выбор времени:
                </label>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => handleApplyPreset(15)}
                    className="py-1.5 px-2 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-xs rounded-xl text-slate-200 transition-colors text-center"
                  >
                    ⏰ Через 15 минут
                  </button>
                  <button
                    type="button"
                    onClick={() => handleApplyPreset(60)}
                    className="py-1.5 px-2 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-xs rounded-xl text-slate-200 transition-colors text-center"
                  >
                    ⏰ Через 1 час
                  </button>
                  <button
                    type="button"
                    onClick={() => handleApplyPreset(180)}
                    className="py-1.5 px-2 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-xs rounded-xl text-slate-200 transition-colors text-center"
                  >
                    🌙 Через 3 часа
                  </button>
                  <button
                    type="button"
                    onClick={() => handleApplyTomorrow(10, 0)}
                    className="py-1.5 px-2 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-xs rounded-xl text-slate-200 transition-colors text-center"
                  >
                    ☀️ Завтра в 10:00
                  </button>
                </div>
              </div>

              {/* Custom Date/Time input */}
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">
                  Точная дата и время *
                </label>
                <input
                  type="datetime-local"
                  required
                  value={remindAtInput}
                  onChange={(e) => setRemindAtInput(e.target.value)}
                  className="w-full px-3.5 py-2.5 bg-slate-800 border border-slate-700 rounded-xl text-sm text-white focus:outline-none focus:border-amber-500 transition-colors"
                />
              </div>

              {/* Recurrence Rule */}
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">
                  Повтор задачи:
                </label>
                <div className="grid grid-cols-2 gap-2 text-xs">
                  {[
                    { id: 'none', label: '1️⃣ Однократно' },
                    { id: 'daily', label: '🔁 Каждый день' },
                    { id: 'weekdays', label: '💼 По будням' },
                    { id: 'weekly', label: '📅 Раз в неделю' },
                  ].map((item) => (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => setRecurrenceRule(item.id as any)}
                      className={`py-2 px-2.5 rounded-xl border text-center transition-all ${
                        recurrenceRule === item.id
                          ? 'border-amber-500 bg-amber-500/20 text-amber-300 font-semibold'
                          : 'border-slate-700 bg-slate-800/80 text-slate-400 hover:text-white'
                      }`}
                    >
                      {item.label}
                    </button>
                  ))}
                </div>
              </div>

              <div className="pt-2 flex gap-3">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="flex-1 py-2.5 bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium text-xs rounded-xl transition-colors"
                >
                  Отмена
                </button>
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="flex-1 py-2.5 bg-amber-500 hover:bg-amber-400 active:bg-amber-600 text-slate-950 font-bold text-xs rounded-xl transition-colors disabled:opacity-50"
                >
                  {isSubmitting ? 'Сохранение...' : 'Запланировать'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
