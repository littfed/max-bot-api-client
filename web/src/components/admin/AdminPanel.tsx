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

  // Load Modes
  const loadModes = async () => {
    try {
      setIsLoadingModes(true);
      const res = await apiClient.getAdminModes();
      setModes(res.modes || []);
    } catch (err: any) {
      onErrorToast('Не удалось загрузить режимы админки');
    } finally {
      setIsLoadingModes(false);
    }
  };

  const handleToggleMode = async (mode: ModeItem, newActive: boolean, msg?: string) => {
    try {
      const messageToSave = msg !== undefined ? msg : (mode.maintenance_message || 'Режим временно недоступен');
      await apiClient.toggleAdminMode(mode.id, newActive, messageToSave);
      onSuccessToast(`Режим ${mode.title}: ${newActive ? 'АКТИВИРОВАН' : 'НА ПРОФИЛАКТИКЕ'}`);
      setEditingModeId(null);
      loadModes();
    } catch (err: any) {
      onErrorToast('Ошибка переключения рубильника режима');
    }
  };

  // Load Leads
  const loadLeads = async () => {
    try {
      setIsLoadingLeads(true);
      const res = await apiClient.getAdminLeads(leadStatusFilter || undefined);
      setLeads(res.leads || []);
    } catch (err: any) {
      onErrorToast('Не удалось загрузить лиды');
    } finally {
      setIsLoadingLeads(false);
    }
  };

  const handleUpdateLeadStatus = async (leadId: number, newStatus: string) => {
    try {
      await apiClient.updateAdminLeadStatus(leadId, newStatus);
      onSuccessToast(`Статус лида #${leadId} обновлен на «${newStatus}»`);
      loadLeads();
    } catch (err: any) {
      onErrorToast('Ошибка обновления статуса лида');
    }
  };

  // Load Tickets
  const loadTickets = async () => {
    try {
      setIsLoadingTickets(true);
      const res = await apiClient.getAdminTickets();
      setTickets(res.tickets || []);
    } catch (err: any) {
      onErrorToast('Не удалось загрузить тикеты поддержки');
    } finally {
      setIsLoadingTickets(false);
    }
  };

  const handleReplyTicket = async (ticketId: number) => {
    const reply = replyTextMap[ticketId]?.trim();
    if (!reply) return;

    try {
      setIsReplyingId(ticketId);
      await apiClient.replyAdminTicket(ticketId, reply);
      onSuccessToast(`Ответ отправлен пользователю по тикету #${ticketId}`);
      setReplyTextMap((prev) => ({ ...prev, [ticketId]: '' }));
      loadTickets();
    } catch (err: any) {
      onErrorToast('Ошибка отправки ответа на тикет');
    } finally {
      setIsReplyingId(null);
    }
  };

  // Load Audit Logs
  const loadAuditLogs = async () => {
    try {
      setIsLoadingAudit(true);
      const res = await apiClient.getAdminAuditLogs(50);
      setAuditLogs(res.audit_logs || []);
    } catch (err: any) {
      onErrorToast('Не удалось загрузить журнал аудита');
    } finally {
      setIsLoadingAudit(false);
    }
  };

  return (
    <div className="w-full max-w-3xl mx-auto px-4 py-4 space-y-4 animate-in fade-in duration-200">
      {/* Header */}
      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-slate-900 border border-slate-800 text-xs font-medium text-slate-300 hover:text-white hover:bg-slate-800 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>К Хабу</span>
        </button>

        <div className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs font-semibold">
          <Shield className="w-3.5 h-3.5" />
          <span>Скрытая панель администратора</span>
        </div>
      </div>

      {/* 4 Tabs */}
      <div className="flex items-center gap-1.5 p-1 rounded-xl bg-slate-900 border border-slate-800 overflow-x-auto">
        <button
          onClick={() => setActiveTab('modes')}
          className={`flex items-center gap-1.5 py-1.5 px-3 text-xs font-medium rounded-lg whitespace-nowrap transition-all ${
            activeTab === 'modes'
              ? 'bg-amber-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          <Sliders className="w-3.5 h-3.5" />
          <span>Рубильники (8)</span>
        </button>

        <button
          onClick={() => setActiveTab('leads')}
          className={`flex items-center gap-1.5 py-1.5 px-3 text-xs font-medium rounded-lg whitespace-nowrap transition-all ${
            activeTab === 'leads'
              ? 'bg-amber-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          <Users className="w-3.5 h-3.5" />
          <span>Лиды</span>
        </button>

        <button
          onClick={() => setActiveTab('tickets')}
          className={`flex items-center gap-1.5 py-1.5 px-3 text-xs font-medium rounded-lg whitespace-nowrap transition-all ${
            activeTab === 'tickets'
              ? 'bg-amber-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          <MessageSquare className="w-3.5 h-3.5" />
          <span>Тикеты</span>
        </button>

        <button
          onClick={() => setActiveTab('audit')}
          className={`flex items-center gap-1.5 py-1.5 px-3 text-xs font-medium rounded-lg whitespace-nowrap transition-all ${
            activeTab === 'audit'
              ? 'bg-amber-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          <FileText className="w-3.5 h-3.5" />
          <span>Аудит</span>
        </button>
      </div>

      {/* --- TAB 1: MODES MAINTENANCE SWITCHES --- */}
      {activeTab === 'modes' && (
        <div className="space-y-3">
          <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800 text-xs text-slate-400">
            Управляйте рубильниками доступности 8 режимов платформы. Изменения мгновенно транслируются через Redis и синхронизируются с MaintenanceGuard.
          </div>

          {isLoadingModes ? (
            <p className="text-xs text-slate-400 py-6 text-center">Загрузка конфигурации режимов...</p>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {modes.map((m) => (
                <div
                  key={m.id}
                  className="p-3.5 rounded-xl bg-slate-900 border border-slate-800 space-y-2.5"
                >
                  <div className="flex items-center justify-between">
                    <div>
                      <h4 className="font-semibold text-xs text-white">{m.title}</h4>
                      <span className="text-[10px] text-slate-500 font-mono">id: {m.id}</span>
                    </div>

                    {/* Toggle Switch */}
                    <button
                      onClick={() => handleToggleMode(m, !m.is_active)}
                      className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                        m.is_active ? 'bg-emerald-600' : 'bg-slate-700'
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
                  {editingModeId === m.id ? (
                    <div className="space-y-1.5 pt-1">
                      <input
                        type="text"
                        value={editMsg}
                        onChange={(e) => setEditMsg(e.target.value)}
                        placeholder="Текст заглушки при профилактике"
                        className="w-full px-2.5 py-1.5 rounded-lg bg-slate-950 border border-slate-700 text-slate-100 text-[11px] focus:outline-none focus:border-amber-500"
                      />
                      <div className="flex items-center justify-end gap-1.5">
                        <button
                          onClick={() => setEditingModeId(null)}
                          className="px-2 py-1 rounded bg-slate-800 text-[10px] text-slate-400"
                        >
                          Отмена
                        </button>
                        <button
                          onClick={() => handleToggleMode(m, m.is_active, editMsg)}
                          className="px-2 py-1 rounded bg-amber-600 text-[10px] text-white font-medium"
                        >
                          Сохранить
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div
                      onClick={() => {
                        setEditingModeId(m.id);
                        setEditMsg(m.maintenance_message || '');
                      }}
                      className="cursor-pointer group p-2 rounded-lg bg-slate-950/40 border border-slate-800/80 hover:border-slate-700 transition-colors"
                      title="Нажмите для редактирования текста"
                    >
                      <span className="text-[10px] text-slate-400 block line-clamp-1 group-hover:text-amber-300">
                        💬 {m.maintenance_message || 'Режим временно недоступен'}
                      </span>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* --- TAB 2: LEADS FEED --- */}
      {activeTab === 'leads' && (
        <div className="space-y-3">
          {/* Status filter buttons */}
          <div className="flex items-center gap-1.5 text-xs">
            <span className="text-slate-400 flex items-center gap-1">
              <Filter className="w-3.5 h-3.5" />
              <span>Фильтр:</span>
            </span>
            {['', 'new', 'in_progress', 'closed'].map((st) => (
              <button
                key={st}
                onClick={() => setLeadStatusFilter(st)}
                className={`px-2.5 py-1 rounded-lg border text-[11px] transition-colors ${
                  leadStatusFilter === st
                    ? 'bg-amber-600 border-amber-500 text-white'
                    : 'bg-slate-900 border-slate-800 text-slate-400 hover:text-slate-200'
                }`}
              >
                {st === '' ? 'Все' : st === 'new' ? 'Новые' : st === 'in_progress' ? 'В работе' : 'Закрытые'}
              </button>
            ))}
          </div>

          {isLoadingLeads ? (
            <p className="text-xs text-slate-400 py-6 text-center">Загрузка лидов...</p>
          ) : leads.length === 0 ? (
            <p className="text-xs text-slate-500 py-6 text-center">Заявок с выбранным фильтром нет.</p>
          ) : (
            <div className="space-y-2.5">
              {leads.map((l) => (
                <div
                  key={l.id}
                  className="p-3.5 rounded-xl bg-slate-900 border border-slate-800 space-y-2 text-xs"
                >
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <span className="font-semibold text-white">#{l.id} {l.contact}</span>
                      <div className="flex items-center gap-2 text-[10px] text-slate-400 mt-0.5">
                        <span>Платформа: {l.platform}</span>
                        <span>•</span>
                        <span>Режим: {l.source_mode}</span>
                        <span>•</span>
                        <span>{new Date(l.created_at).toLocaleString('ru-RU')}</span>
                      </div>
                    </div>

                    <div className="flex items-center gap-1">
                      {['new', 'in_progress', 'closed'].map((statusOption) => (
                        <button
                          key={statusOption}
                          onClick={() => handleUpdateLeadStatus(l.id, statusOption)}
                          className={`px-2 py-0.5 rounded text-[10px] font-medium border transition-colors ${
                            l.status === statusOption
                              ? statusOption === 'new'
                                ? 'bg-amber-500/20 text-amber-400 border-amber-500/40'
                                : statusOption === 'in_progress'
                                ? 'bg-sky-500/20 text-sky-400 border-sky-500/40'
                                : 'bg-emerald-500/20 text-emerald-400 border-emerald-500/40'
                              : 'bg-slate-950 border-slate-800 text-slate-500 hover:text-slate-300'
                          }`}
                        >
                          {statusOption === 'new' ? 'New' : statusOption === 'in_progress' ? 'In Progress' : 'Closed'}
                        </button>
                      ))}
                    </div>
                  </div>

                  {l.comment && (
                    <p className="p-2 rounded-lg bg-slate-950 border border-slate-800 text-slate-300 text-xs">
                      {l.comment}
                    </p>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* --- TAB 3: TICKETS QUEUE --- */}
      {activeTab === 'tickets' && (
        <div className="space-y-3">
          {isLoadingTickets ? (
            <p className="text-xs text-slate-400 py-6 text-center">Загрузка очереди тикетов...</p>
          ) : tickets.length === 0 ? (
            <p className="text-xs text-slate-500 py-6 text-center">Открытых тикетов поддержки нет.</p>
          ) : (
            <div className="space-y-3">
              {tickets.map((t) => (
                <div
                  key={t.id}
                  className="p-3.5 rounded-xl bg-slate-900 border border-slate-800 space-y-2 text-xs"
                >
                  <div className="flex items-center justify-between">
                    <div>
                      <span className="font-semibold text-white">#{t.id} {t.subject}</span>
                      <p className="text-[10px] text-slate-400">
                        Пользователь #{t.user_id} ({t.platform}) • Скоуп: {t.mode_scope}
                      </p>
                    </div>

                    <span
                      className={`text-[10px] font-semibold px-2 py-0.5 rounded-full ${
                        t.status === 'open'
                          ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                          : 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                      }`}
                    >
                      {t.status === 'open' ? 'Ожидает ответа' : 'Отвечен'}
                    </span>
                  </div>

                  {t.message && (
                    <p className="p-2.5 rounded-lg bg-slate-950 border border-slate-800 text-slate-200">
                      {t.message}
                    </p>
                  )}

                  {/* Reply Form */}
                  <div className="flex items-center gap-2 pt-1">
                    <input
                      type="text"
                      value={replyTextMap[t.id] || ''}
                      onChange={(e) =>
                        setReplyTextMap((prev) => ({ ...prev, [t.id]: e.target.value }))
                      }
                      placeholder="Напишите ответ оператора..."
                      className="flex-1 px-3 py-1.5 rounded-xl bg-slate-950 border border-slate-700 text-slate-100 text-xs focus:outline-none focus:border-amber-500"
                    />
                    <button
                      onClick={() => handleReplyTicket(t.id)}
                      disabled={isReplyingId === t.id || !replyTextMap[t.id]?.trim()}
                      className="px-3 py-1.5 rounded-xl bg-amber-600 hover:bg-amber-500 disabled:opacity-40 text-white font-medium text-xs shadow-md shadow-amber-600/30 transition-all flex items-center gap-1"
                    >
                      <Send className="w-3.5 h-3.5" />
                      <span>{isReplyingId === t.id ? 'Отправка...' : 'Ответить'}</span>
                    </button>
                  </div>
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
            <p className="text-xs text-slate-500 py-6 text-center">Записей аудита пока нет.</p>
          ) : (
            <div className="space-y-2">
              {auditLogs.map((log) => (
                <div
                  key={log.id}
                  className="p-3 rounded-xl bg-slate-900 border border-slate-800 text-xs space-y-1 font-mono"
                >
                  <div className="flex items-center justify-between text-slate-400">
                    <span className="font-semibold text-amber-400">
                      {log.action} {log.target_mode ? `-> ${log.target_mode}` : ''}
                    </span>
                    <span className="text-[10px] text-slate-500">
                      {new Date(log.created_at).toLocaleString('ru-RU')}
                    </span>
                  </div>
                  <div className="text-[11px] text-slate-400 flex items-center gap-3">
                    <span>Admin #{log.admin_id}</span>
                    <span>IP: {log.ip || '127.0.0.1'}</span>
                  </div>
                  {log.payload_diff && (
                    <pre className="p-1.5 rounded bg-slate-950 text-[10px] text-slate-300 overflow-x-auto">
                      {JSON.stringify(log.payload_diff, null, 2)}
                    </pre>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
};
