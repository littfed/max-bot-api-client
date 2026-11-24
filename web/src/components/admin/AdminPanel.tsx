import React, { useState, useEffect } from 'react';
import {
  Shield,
  Sliders,
  Users,
  MessageSquare,
  FileText,
  ArrowLeft,
  CheckCircle2,
  AlertTriangle,
  RotateCcw,
  Send,
  Eye,
  Filter,
} from 'lucide-react';
import { apiClient } from '../../api/client';
import { ModeItem, Lead, SupportTicket, AdminAuditLog } from '../../types';

interface AdminPanelProps {
  onBack: () => void;
  onSuccessToast: (msg: string) => void;
  onErrorToast: (msg: string) => void;
}

export const AdminPanel: React.FC<AdminPanelProps> = ({
  onBack,
  onSuccessToast,
  onErrorToast,
}) => {
  const [activeTab, setActiveTab] = useState<'modes' | 'leads' | 'tickets' | 'audit'>('modes');

  // --- Tab 1: Modes Maintenance ---
  const [modes, setModes] = useState<ModeItem[]>([]);
  const [isLoadingModes, setIsLoadingModes] = useState(false);
  const [editingModeId, setEditingModeId] = useState<string | null>(null);
  const [editMsg, setEditMsg] = useState('');

  // --- Tab 2: Leads Feed ---
  const [leads, setLeads] = useState<Lead[]>([]);
  const [leadStatusFilter, setLeadStatusFilter] = useState<string>('');
  const [isLoadingLeads, setIsLoadingLeads] = useState(false);

  // --- Tab 3: Tickets Queue ---
  const [tickets, setTickets] = useState<SupportTicket[]>([]);
  const [isLoadingTickets, setIsLoadingTickets] = useState(false);
  const [replyTextMap, setReplyTextMap] = useState<Record<number, string>>({});
  const [isReplyingId, setIsReplyingId] = useState<number | null>(null);

  // --- Tab 4: Audit Logs ---
  const [auditLogs, setAuditLogs] = useState<AdminAuditLog[]>([]);
  const [isLoadingAudit, setIsLoadingAudit] = useState(false);

  useEffect(() => {
    if (activeTab === 'modes') loadModes();
    if (activeTab === 'leads') loadLeads();
    if (activeTab === 'tickets') loadTickets();
    if (activeTab === 'audit') loadAuditLogs();
  }, [activeTab, leadStatusFilter]);

  const loadModes = async () => {
    try {
      setIsLoadingModes(true);
      const res = await apiClient.getAdminModes();
      setModes(res.modes || []);
    } catch (err: any) {
      setModes([
        { id: 'shop', title: 'Магазин', is_active: true },
        { id: 'reception', title: 'Ресепшен', is_active: true },
        { id: 'reminders', title: 'Напоминания', is_active: false, maintenance_message: 'Модуль на обновлении' },
        { id: 'channels', title: 'Каналы', is_active: false, maintenance_message: 'Модуль на обновлении' },
        { id: 'giveaways', title: 'Конкурсы', is_active: false, maintenance_message: 'Модуль на обновлении' },
        { id: 'support', title: 'Помощь', is_active: true },
        { id: 'docs', title: 'Документы', is_active: false, maintenance_message: 'Модуль на обновлении' },
        { id: 'chat', title: 'Общение', is_active: true },
      ]);
    } finally {
      setIsLoadingModes(false);
    }
  };

  const handleToggleMode = async (mode: ModeItem, newActive: boolean, newMsg?: string) => {
    const targetId = mode.id || (mode as any).mode_id;
    if (!targetId) return;

    try {
      const msg = newMsg !== undefined ? newMsg : (mode.maintenance_message || 'Режим временно недоступен');
      await apiClient.toggleAdminMode(targetId, newActive, msg);
      setModes((prev) =>
        prev.map((m) => {
          const mId = m.id || (m as any).mode_id;
          return mId === targetId
            ? { ...m, id: targetId, is_active: newActive, maintenance_message: msg }
            : m;
        })
      );
      onSuccessToast(`Режим «${mode.title}» переключен: ${newActive ? 'Активен' : 'Профилактика'}`);
      setEditingModeId(null);
    } catch (err: any) {
      onErrorToast('Не удалось обновить статус режима');
    }
  };

  const loadLeads = async () => {
    try {
      setIsLoadingLeads(true);
      const res = await apiClient.getAdminLeads(leadStatusFilter || undefined);
      setLeads(res.leads || []);
    } catch (err: any) {
      setLeads([]);
    } finally {
      setIsLoadingLeads(false);
    }
  };

  const handleUpdateLeadStatus = async (leadId: number, newStatus: 'new' | 'in_progress' | 'closed') => {
    try {
      await apiClient.updateAdminLeadStatus(leadId, newStatus);
      setLeads((prev) =>
        prev.map((l) => (l.id === leadId ? { ...l, status: newStatus } : l))
      );
      onSuccessToast('Статус лида обновлен');
    } catch (err: any) {
      onErrorToast('Ошибка обновления лида');
    }
  };

  const loadTickets = async () => {
    try {
      setIsLoadingTickets(true);
      const res = await apiClient.getAdminTickets();
      setTickets(res.tickets || []);
    } catch (err: any) {
      setTickets([]);
    } finally {
      setIsLoadingTickets(false);
    }
  };

  const handleReplyTicket = async (ticketId: number) => {
    const reply = replyTextMap[ticketId];
    if (!reply || !reply.trim()) return;

    try {
      setIsReplyingId(ticketId);
      await apiClient.replyAdminTicket(ticketId, reply.trim());
      onSuccessToast('Ответ отправлен пользователю в чат бота');
      setTickets((prev) =>
        prev.map((t) => (t.id === ticketId ? { ...t, status: 'answered' as const } : t))
      );
      setReplyTextMap((prev) => ({ ...prev, [ticketId]: '' }));
    } catch (err: any) {
      onErrorToast('Ошибка отправки ответа');
    } finally {
      setIsReplyingId(null);
    }
  };

  const loadAuditLogs = async () => {
    try {
      setIsLoadingAudit(true);
      const res = await apiClient.getAdminAuditLogs();
      setAuditLogs(res.audit_logs || []);
    } catch (err: any) {
      setAuditLogs([]);
    } finally {
      setIsLoadingAudit(false);
    }
  };

  return (
    <div className="w-full max-w-md sm:max-w-lg mx-auto px-4 py-3 sm:py-4 space-y-4 animate-in fade-in duration-200">
      {/* Navigation Tabs */}
      <div className="flex items-center gap-1.5 p-1 rounded-2xl bg-slate-200/60 border border-slate-200 overflow-x-auto no-scrollbar">
        <button
          onClick={() => setActiveTab('modes')}
          className={`flex-1 flex items-center justify-center gap-1 py-1.5 px-2.5 text-xs font-semibold rounded-xl whitespace-nowrap transition-all ${
            activeTab === 'modes'
              ? 'bg-amber-500 text-white shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <Sliders className="w-3.5 h-3.5" />
          <span>Режимы (8)</span>
        </button>

        <button
          onClick={() => setActiveTab('leads')}
          className={`flex-1 flex items-center justify-center gap-1 py-1.5 px-2.5 text-xs font-semibold rounded-xl whitespace-nowrap transition-all ${
            activeTab === 'leads'
              ? 'bg-amber-500 text-white shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <Users className="w-3.5 h-3.5" />
          <span>Лиды</span>
        </button>

        <button
          onClick={() => setActiveTab('tickets')}
          className={`flex-1 flex items-center justify-center gap-1 py-1.5 px-2.5 text-xs font-semibold rounded-xl whitespace-nowrap transition-all ${
            activeTab === 'tickets'
              ? 'bg-amber-500 text-white shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <MessageSquare className="w-3.5 h-3.5" />
          <span>Тикеты</span>
        </button>

        <button
          onClick={() => setActiveTab('audit')}
          className={`flex-1 flex items-center justify-center gap-1 py-1.5 px-2.5 text-xs font-semibold rounded-xl whitespace-nowrap transition-all ${
            activeTab === 'audit'
              ? 'bg-amber-500 text-white shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <FileText className="w-3.5 h-3.5" />
          <span>Аудит</span>
        </button>
      </div>

      {/* --- TAB 1: MODES MAINTENANCE SWITCHES --- */}
      {activeTab === 'modes' && (
        <div className="space-y-3">
          <div className="p-3 rounded-2xl bg-amber-50 border border-amber-200 text-xs text-amber-900">
            Управляйте рубильниками доступности 8 режимов платформы. Изменения мгновенно применяются через Redis и MaintenanceGuard.
          </div>

          {isLoadingModes ? (
            <p className="text-xs text-slate-400 py-6 text-center">Загрузка конфигурации режимов...</p>
          ) : (
            <div className="space-y-2.5">
              {modes.map((m) => {
                const modeId = m.id || (m as any).mode_id;
                return (
                <div
                  key={modeId}
                  className="p-3.5 rounded-2xl bg-white border border-slate-200/90 shadow-sm space-y-2"
                >
                  <div className="flex items-center justify-between">
                    <div>
                      <h4 className="font-bold text-xs sm:text-sm text-slate-900">{m.title}</h4>
                      <span className="text-[10px] text-slate-400 font-mono">id: {modeId}</span>
                    </div>

                    {/* Toggle Switch */}
                    <button
                      onClick={() => handleToggleMode(m, !m.is_active)}
                      className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                        m.is_active ? 'bg-[#0d9488]' : 'bg-slate-300'
                      }`}
                    >
                      <span
                        className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                          m.is_active ? 'translate-x-6' : 'translate-x-1'
                        }`}
                      />
                    </button>
                  </div>

                  {/* Maintenance message editor */}
                  {editingModeId === modeId ? (
                    <div className="space-y-1.5 pt-1">
                      <input
                        type="text"
                        value={editMsg}
                        onChange={(e) => setEditMsg(e.target.value)}
                        placeholder="Текст заглушки при профилактике"
                        className="w-full px-3 py-1.5 rounded-xl bg-slate-50 border border-slate-200 text-slate-900 text-xs focus:outline-none focus:border-amber-500"
                      />
                      <div className="flex items-center justify-end gap-1.5">
                        <button
                          onClick={() => setEditingModeId(null)}
                          className="px-2.5 py-1 rounded-lg bg-slate-100 text-xs text-slate-600 hover:bg-slate-200"
                        >
                          Отмена
                        </button>
                        <button
                          onClick={() => handleToggleMode(m, m.is_active, editMsg)}
                          className="px-2.5 py-1 rounded-lg bg-amber-500 text-xs text-white font-medium hover:bg-amber-600"
                        >
                          Сохранить
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div
                      onClick={() => {
                        setEditingModeId(modeId);
                        setEditMsg(m.maintenance_message || '');
                      }}
                      className="cursor-pointer group p-2 rounded-xl bg-slate-50 border border-slate-100 hover:border-slate-200 transition-colors"
                      title="Нажмите для редактирования текста"
                    >
                      <span className="text-[11px] text-slate-500 block line-clamp-1 group-hover:text-amber-700">
                        💬 {m.maintenance_message || 'Режим временно недоступен'}
                      </span>
                    </div>
                  )}
                </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* --- TAB 2: LEADS FEED --- */}
      {activeTab === 'leads' && (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold text-slate-900">Поток входящих лидов</span>
            <div className="flex items-center gap-1">
              <Filter className="w-3.5 h-3.5 text-slate-400" />
              <select
                value={leadStatusFilter}
                onChange={(e) => setLeadStatusFilter(e.target.value)}
                className="bg-white border border-slate-200 rounded-lg px-2 py-1 text-xs text-slate-700 focus:outline-none"
              >
                <option value="">Все статусы</option>
                <option value="new">Новые</option>
                <option value="in_progress">В работе</option>
                <option value="closed">Закрытые</option>
              </select>
            </div>
          </div>

          {isLoadingLeads ? (
            <p className="text-xs text-slate-400 py-6 text-center">Загрузка лидов...</p>
          ) : leads.length === 0 ? (
            <p className="text-xs text-slate-500 py-6 text-center">Заявок пока нет.</p>
          ) : (
            <div className="space-y-2.5">
              {leads.map((l) => (
                <div
                  key={l.id}
                  className="p-3.5 rounded-2xl bg-white border border-slate-200/90 shadow-sm space-y-2"
                >
                  <div className="flex items-start justify-between">
                    <div>
                      <h4 className="font-bold text-xs text-slate-900">{l.contact}</h4>
                      <p className="text-[11px] text-slate-400">
                        {new Date(l.created_at).toLocaleString('ru-RU')}
                      </p>
                    </div>
                    <select
                      value={l.status}
                      onChange={(e) => handleUpdateLeadStatus(l.id, e.target.value as any)}
                      className="text-[10px] font-semibold rounded-full px-2 py-0.5 bg-slate-100 text-slate-700 border border-slate-200 focus:outline-none"
                    >
                      <option value="new">Новый</option>
                      <option value="in_progress">В работе</option>
                      <option value="closed">Закрыт</option>
                    </select>
                  </div>

                  {l.comment && (
                    <p className="text-xs text-slate-600 bg-slate-50 p-2 rounded-xl">
                      {l.comment}
                    </p>
                  )}

                  <div className="flex items-center gap-2 text-[10px] text-slate-400">
                    <span>Режим: <strong className="text-slate-700">{l.source_mode}</strong></span>
                    {l.tested_modes && l.tested_modes.length > 0 && (
                      <span>Тестировал: {Array.isArray(l.tested_modes) ? l.tested_modes.join(', ') : String(l.tested_modes)}</span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* --- TAB 3: TICKETS --- */}
      {activeTab === 'tickets' && (
        <div className="space-y-3">
          <div className="p-3 rounded-2xl bg-teal-50 border border-teal-200 text-xs text-teal-900">
            Очередь эскалированных тикетов от пользователей. Ответ будет доставлен прямо в чат Telegram / MAX.
          </div>

          {isLoadingTickets ? (
            <p className="text-xs text-slate-400 py-6 text-center">Загрузка очереди тикетов...</p>
          ) : tickets.length === 0 ? (
            <p className="text-xs text-slate-500 py-6 text-center">Все тикеты обработаны.</p>
          ) : (
            <div className="space-y-3">
              {tickets.map((t) => (
                <div
                  key={t.id}
                  className="p-3.5 rounded-2xl bg-white border border-slate-200/90 shadow-sm space-y-2.5"
                >
                  <div className="flex items-start justify-between">
                    <div>
                      <h4 className="font-bold text-xs text-slate-900">#{t.id} {t.subject}</h4>
                      <p className="text-[10px] text-slate-400">
                        Пользователь: {t.user_id} • {new Date(t.created_at).toLocaleString('ru-RU')}
                      </p>
                    </div>
                    <span
                      className={`text-[10px] font-semibold px-2 py-0.5 rounded-full ${
                        t.status === 'open'
                          ? 'bg-rose-50 text-rose-700 border border-rose-200'
                          : 'bg-teal-50 text-teal-700 border border-teal-200'
                      }`}
                    >
                      {t.status === 'open' ? 'Открыт' : 'Отвечен'}
                    </span>
                  </div>

                  <p className="text-xs text-slate-700 bg-slate-50 p-2.5 rounded-xl leading-relaxed">
                    {t.message}
                  </p>

                  {t.status === 'open' && (
                    <div className="space-y-1.5 pt-1">
                      <textarea
                        rows={2}
                        value={replyTextMap[t.id] || ''}
                        onChange={(e) =>
                          setReplyTextMap((prev) => ({ ...prev, [t.id]: e.target.value }))
                        }
                        placeholder="Напишите ответ пользователю..."
                        className="w-full px-3 py-1.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-slate-900 focus:outline-none focus:border-teal-500"
                      />
                      <div className="flex justify-end">
                        <button
                          onClick={() => handleReplyTicket(t.id)}
                          disabled={isReplyingId === t.id || !replyTextMap[t.id]?.trim()}
                          className="px-3 py-1.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] disabled:opacity-40 text-xs text-white font-medium flex items-center gap-1.5"
                        >
                          <Send className="w-3.5 h-3.5" />
                          <span>{isReplyingId === t.id ? 'Отправка...' : 'Ответить в чат'}</span>
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* --- TAB 4: AUDIT LOGS --- */}
      {activeTab === 'audit' && (
        <div className="space-y-3">
          {isLoadingAudit ? (
            <p className="text-xs text-slate-400 py-6 text-center">Загрузка журнала аудита...</p>
          ) : auditLogs.length === 0 ? (
            <p className="text-xs text-slate-500 py-6 text-center">Журнал действий пока пуст.</p>
          ) : (
            <div className="space-y-2">
              {auditLogs.map((log) => (
                <div
                  key={log.id}
                  className="p-3 rounded-xl bg-white border border-slate-200/90 text-xs flex items-center justify-between"
                >
                  <div>
                    <span className="font-semibold text-slate-900 block">{log.action}</span>
                    <span className="text-[10px] text-slate-400 font-mono">
                      Админ: #{log.admin_id} • IP: {log.ip || 'local'}
                    </span>
                  </div>
                  <span className="text-[10px] text-slate-400">
                    {new Date(log.created_at).toLocaleTimeString('ru-RU')}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
};
