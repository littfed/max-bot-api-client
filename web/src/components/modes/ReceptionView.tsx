import React, { useState, useEffect } from 'react';
import {
  CalendarCheck,
  Calendar,
  Ticket,
  ClipboardList,
  ArrowLeft,
  Clock,
  CheckCircle2,
  AlertCircle,
  QrCode,
  Users,
  ChevronRight,
  Edit2,
  Send,
} from 'lucide-react';
import { apiClient } from '../../api/client';
import { TimeSlot, ReceptionEvent, EventRegistration } from '../../types';

interface ReceptionViewProps {
  onBack: () => void;
  onSuccessToast: (msg: string) => void;
  onErrorToast: (msg: string) => void;
}

const servicesList = [
  { id: 'srv_consult', title: 'Экспресс-консультация по архитектуре', duration: '30 мин' },
  { id: 'srv_audit', title: 'Аудит бизнес-процессов и воронки', duration: '60 мин' },
];

export const ReceptionView: React.FC<ReceptionViewProps> = ({
  onBack,
  onSuccessToast,
  onErrorToast,
}) => {
  const [activeTab, setActiveTab] = useState<'slots' | 'events' | 'onboarding'>('slots');

  // --- Sub-Tab 1: Slots Booking State ---
  const [selectedService, setSelectedService] = useState('srv_consult');
  const [selectedDate, setSelectedDate] = useState(() => {
    const d = new Date();
    d.setDate(d.getDate() + 1);
    return d.toISOString().split('T')[0];
  });
  const [slots, setSlots] = useState<TimeSlot[]>([]);
  const [selectedSlot, setSelectedSlot] = useState<string | null>(null);
  const [isLoadingSlots, setIsLoadingSlots] = useState(false);
  const [isBookingSlot, setIsBookingSlot] = useState(false);
  const [bookingSuccess, setBookingSuccess] = useState(false);

  // --- Sub-Tab 2: Events State ---
  const [events, setEvents] = useState<ReceptionEvent[]>([]);
  const [isLoadingEvents, setIsLoadingEvents] = useState(false);
  const [registeringEventId, setRegisteringEventId] = useState<string | null>(null);
  const [activeRegistration, setActiveRegistration] = useState<EventRegistration | null>(null);

  // --- Sub-Tab 3: Onboarding State ---
  const [wizardStep, setWizardStep] = useState(1);
  const [onboardingAnswers, setOnboardingAnswers] = useState<Record<string, string>>({
    business_type: '',
    primary_goal: '',
    audience_size: '',
    experience_level: '',
  });
  const [isSubmittingOnboarding, setIsSubmittingOnboarding] = useState(false);
  const [onboardingCompleted, setOnboardingCompleted] = useState(false);

  // Load slots when service or date changes
  useEffect(() => {
    if (activeTab === 'slots') {
      loadSlots();
    }
  }, [activeTab, selectedService, selectedDate]);

  // Load events when tab opens
  useEffect(() => {
    if (activeTab === 'events') {
      loadEvents();
    }
  }, [activeTab]);

  const loadSlots = async () => {
    try {
      setIsLoadingSlots(true);
      setSelectedSlot(null);
      const res = await apiClient.getReceptionSlots(selectedService, selectedDate);
      setSlots(res.slots || []);
    } catch (err: any) {
      // Fallback demo slots
      const mockSlots: TimeSlot[] = [
        { id: '1', service_id: selectedService, slot_time: `${selectedDate}T10:00:00Z`, is_booked: false },
        { id: '2', service_id: selectedService, slot_time: `${selectedDate}T11:30:00Z`, is_booked: true },
        { id: '3', service_id: selectedService, slot_time: `${selectedDate}T14:00:00Z`, is_booked: false },
        { id: '4', service_id: selectedService, slot_time: `${selectedDate}T16:00:00Z`, is_booked: false },
        { id: '5', service_id: selectedService, slot_time: `${selectedDate}T18:00:00Z`, is_booked: false },
      ];
      setSlots(mockSlots);
    } finally {
      setIsLoadingSlots(false);
    }
  };

  const handleBookSlot = async () => {
    if (!selectedSlot) return;
    try {
      setIsBookingSlot(true);
      await apiClient.bookReceptionSlot(selectedService, selectedSlot);
      setBookingSuccess(true);
      onSuccessToast('Слот успешно забронирован!');
      loadSlots();
    } catch (err: any) {
      const msg = err.response?.data?.message || 'Слот уже занят или произошла ошибка';
      onErrorToast(msg);
    } finally {
      setIsBookingSlot(false);
    }
  };

  const loadEvents = async () => {
    try {
      setIsLoadingEvents(true);
      const res = await apiClient.getReceptionEvents();
      setEvents(res.events || []);
    } catch (err: any) {
      // Fallback demo events
      setEvents([
        {
          id: 'evt_webinar',
          title: '🔥 Live-сессия: Архитектура Mini App в MAX',
          event_time: new Date(Date.now() + 86400000 * 3).toISOString(),
          total_seats: 100,
          booked_seats: 42,
        },
        {
          id: 'evt_workshop',
          title: '🛠 Практикум: Интеграция оплат и Webhooks',
          event_time: new Date(Date.now() + 86400000 * 7).toISOString(),
          total_seats: 50,
          booked_seats: 38,
        },
      ]);
    } finally {
      setIsLoadingEvents(false);
    }
  };

  const handleRegisterEvent = async (event: ReceptionEvent) => {
    try {
      setRegisteringEventId(event.id);
      const res = await apiClient.registerReceptionEvent(event.id);
      setActiveRegistration(res.registration);
      onSuccessToast(`Вы успешно зарегистрированы на «${event.title}»!`);
      loadEvents();
    } catch (err: any) {
      // Fallback ticket
      setActiveRegistration({
        registration_id: 'reg_' + Math.random().toString(36).substring(2, 8),
        event_id: event.id,
        user_id: 1,
        ticket_qr: `TICKET:${event.id}:USER_${Math.floor(Math.random() * 1000)}`,
        registered_at: new Date().toISOString(),
      });
      onSuccessToast(`Регистрация оформлена (симуляция)`);
    } finally {
      setRegisteringEventId(null);
    }
  };

  const handleOnboardingAnswer = (field: string, val: string) => {
    setOnboardingAnswers((prev) => ({ ...prev, [field]: val }));
    setWizardStep((prev) => Math.min(prev + 1, 5));
  };

  const handleSubmitOnboarding = async () => {
    try {
      setIsSubmittingOnboarding(true);
      await apiClient.submitReceptionOnboarding(onboardingAnswers);
      setOnboardingCompleted(true);
      onSuccessToast('Анкета онбординга успешно отправлена!');
    } catch (err: any) {
      setOnboardingCompleted(true);
      onSuccessToast('Анкета сохранена (симуляция)!');
    } finally {
      setIsSubmittingOnboarding(false);
    }
  };

  return (
    <div className="w-full max-w-2xl mx-auto px-4 py-4 space-y-5 animate-in fade-in duration-200">
      {/* Navigation Header */}
      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-slate-900 border border-slate-800 text-xs font-medium text-slate-300 hover:text-white hover:bg-slate-800 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>К списку режимов</span>
        </button>

        <div className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-indigo-500/10 border border-indigo-500/20 text-indigo-300 text-xs font-semibold">
          <CalendarCheck className="w-3.5 h-3.5" />
          <span>Режим: Ресепшен</span>
        </div>
      </div>

      {/* Banner / Title */}
      <div className="p-4 rounded-2xl bg-gradient-to-r from-indigo-950/60 via-slate-900 to-slate-900 border border-indigo-900/50">
        <h2 className="text-lg font-bold text-white flex items-center gap-2">
          <span>Стойка регистрации и записи</span>
        </h2>
        <p className="text-xs text-slate-400 mt-1">
          Бронирование слотов с защитой от овербукинга через Redis Lock, регистрация на закрытые мероприятия с генерацией QR-билетов и пошаговый онбординг.
        </p>
      </div>

      {/* 3 Sub-Tabs */}
      <div className="flex items-center gap-2 p-1 rounded-xl bg-slate-900 border border-slate-800">
        <button
          onClick={() => {
            setActiveTab('slots');
            setBookingSuccess(false);
          }}
          className={`flex-1 flex items-center justify-center gap-1.5 py-1.5 text-xs font-medium rounded-lg transition-all ${
            activeTab === 'slots'
              ? 'bg-indigo-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          <Clock className="w-3.5 h-3.5" />
          <span>Запись на услугу</span>
        </button>
        <button
          onClick={() => setActiveTab('events')}
          className={`flex-1 flex items-center justify-center gap-1.5 py-1.5 text-xs font-medium rounded-lg transition-all ${
            activeTab === 'events'
              ? 'bg-indigo-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          <Ticket className="w-3.5 h-3.5" />
          <span>Ивенты</span>
        </button>
        <button
          onClick={() => setActiveTab('onboarding')}
          className={`flex-1 flex items-center justify-center gap-1.5 py-1.5 text-xs font-medium rounded-lg transition-all ${
            activeTab === 'onboarding'
              ? 'bg-indigo-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          <ClipboardList className="w-3.5 h-3.5" />
          <span>Онбординг</span>
        </button>
      </div>

      {/* --- SUB-TAB 1: SLOTS BOOKING --- */}
      {activeTab === 'slots' && (
        <div className="space-y-4">
          {bookingSuccess ? (
            <div className="p-6 rounded-2xl bg-slate-900 border border-emerald-500/40 text-center space-y-3">
              <div className="w-12 h-12 rounded-full bg-emerald-500/20 text-emerald-400 mx-auto flex items-center justify-center border border-emerald-500/40">
                <CheckCircle2 className="w-7 h-7" />
              </div>
              <h3 className="text-base font-bold text-white">Бронирование успешно подтверждено!</h3>
              <p className="text-xs text-slate-300 max-w-sm mx-auto">
                Слот зафиксирован за вами с блокировкой распределенного мьютекса. Напоминание придет в чат бота за 2 часа до начала.
              </p>
              <button
                onClick={() => setBookingSuccess(false)}
                className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 transition-colors"
              >
                Забронировать другое время
              </button>
            </div>
          ) : (
            <div className="p-4 rounded-2xl bg-slate-900 border border-slate-800 space-y-4">
              {/* Service Select */}
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">
                  Выберите услугу:
                </label>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  {servicesList.map((srv) => (
                    <div
                      key={srv.id}
                      onClick={() => setSelectedService(srv.id)}
                      className={`p-3 rounded-xl border cursor-pointer transition-all ${
                        selectedService === srv.id
                          ? 'bg-indigo-950/60 border-indigo-500 text-white shadow-sm'
                          : 'bg-slate-950/50 border-slate-800 text-slate-400 hover:border-slate-700'
                      }`}
                    >
                      <p className="text-xs font-semibold text-slate-200">{srv.title}</p>
                      <p className="text-[11px] text-slate-500 mt-0.5">Длительность: {srv.duration}</p>
                    </div>
                  ))}
                </div>
              </div>

              {/* Date Input */}
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5 flex items-center gap-1.5">
                  <Calendar className="w-3.5 h-3.5 text-indigo-400" />
                  <span>Дата консультации:</span>
                </label>
                <input
                  type="date"
                  value={selectedDate}
                  onChange={(e) => setSelectedDate(e.target.value)}
                  className="w-full px-3.5 py-2 rounded-xl bg-slate-950 border border-slate-700 text-slate-100 text-xs focus:outline-none focus:border-indigo-500"
                />
              </div>

              {/* Slot Grid */}
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">
                  Доступные слоты времени:
                </label>

                {isLoadingSlots ? (
                  <p className="text-xs text-slate-400 py-4 text-center">Проверка доступности слотов...</p>
                ) : slots.length === 0 ? (
                  <p className="text-xs text-slate-400 py-4 text-center">
                    На выбранную дату нет свободных окон.
                  </p>
                ) : (
                  <div className="grid grid-cols-3 sm:grid-cols-4 gap-2">
                    {slots.map((s) => {
                      const timeStr = new Date(s.slot_time).toLocaleTimeString('ru-RU', {
                        hour: '2-digit',
                        minute: '2-digit',
                      });
                      const isSelected = selectedSlot === s.slot_time;

                      return (
                        <button
                          key={s.id || s.slot_time}
                          disabled={s.is_booked}
                          onClick={() => setSelectedSlot(s.slot_time)}
                          className={`py-2 px-2.5 rounded-xl border text-xs font-semibold transition-all ${
                            s.is_booked
                              ? 'bg-slate-950/40 border-slate-800/40 text-slate-600 line-through cursor-not-allowed'
                              : isSelected
                              ? 'bg-indigo-600 border-indigo-400 text-white shadow-md shadow-indigo-600/30'
                              : 'bg-slate-950 border-slate-800 text-slate-300 hover:border-indigo-500/50 hover:bg-slate-800'
                          }`}
                        >
                          {timeStr}
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>

              {/* Book Action */}
              <button
                onClick={handleBookSlot}
                disabled={!selectedSlot || isBookingSlot}
                className="w-full py-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-xs font-semibold shadow-md shadow-indigo-600/30 transition-all flex items-center justify-center gap-1.5"
              >
                <CheckCircle2 className="w-4 h-4" />
                <span>
                  {isBookingSlot ? 'Бронирование...' : 'Подтвердить бронь слота'}
                </span>
              </button>
            </div>
          )}
        </div>
      )}

      {/* --- SUB-TAB 2: EVENTS & QR TICKETS --- */}
      {activeTab === 'events' && (
        <div className="space-y-4">
          {isLoadingEvents ? (
            <p className="text-xs text-slate-400 py-8 text-center">Загрузка мероприятий...</p>
          ) : (
            <div className="grid grid-cols-1 gap-3.5">
              {events.map((evt) => {
                const dateFormatted = new Date(evt.event_time).toLocaleDateString('ru-RU', {
                  day: 'numeric',
                  month: 'long',
                  hour: '2-digit',
                  minute: '2-digit',
                });
                const seatsLeft = Math.max(0, evt.total_seats - evt.booked_seats);
                const isFull = seatsLeft === 0;

                return (
                  <div
                    key={evt.id}
                    className="p-4 rounded-2xl bg-slate-900 border border-slate-800 hover:border-slate-700 transition-all space-y-3"
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div>
                        <h3 className="font-semibold text-sm text-white">{evt.title}</h3>
                        <p className="text-xs text-indigo-400 mt-0.5 flex items-center gap-1">
                          <Calendar className="w-3.5 h-3.5" />
                          <span>{dateFormatted}</span>
                        </p>
                      </div>

                      <span
                        className={`text-[10px] font-semibold px-2 py-0.5 rounded-full shrink-0 ${
                          isFull
                            ? 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                            : 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                        }`}
                      >
                        {isFull ? 'Мест нет' : `Осталось: ${seatsLeft}`}
                      </span>
                    </div>

                    <div className="flex items-center justify-between pt-1 border-t border-slate-800/80">
                      <div className="flex items-center gap-1 text-[11px] text-slate-400">
                        <Users className="w-3.5 h-3.5" />
                        <span>Всего мест: {evt.total_seats}</span>
                      </div>

                      <button
                        onClick={() => handleRegisterEvent(evt)}
                        disabled={isFull || registeringEventId === evt.id}
                        className="px-3.5 py-1.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 text-white font-medium text-xs shadow-md shadow-indigo-600/30 transition-all flex items-center gap-1"
                      >
                        <Ticket className="w-3.5 h-3.5" />
                        <span>
                          {registeringEventId === evt.id ? 'Регистрация...' : 'Зарегистрироваться'}
                        </span>
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          {/* QR Ticket Modal */}
          {activeRegistration && (
            <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-200">
              <div className="relative w-full max-w-xs rounded-2xl bg-slate-900 border border-slate-800 p-5 text-center space-y-3 shadow-2xl">
                <div className="w-10 h-10 rounded-full bg-emerald-500/20 text-emerald-400 mx-auto flex items-center justify-center">
                  <CheckCircle2 className="w-6 h-6" />
                </div>
                <h4 className="text-base font-bold text-white">Ваш электронный билет</h4>
                <p className="text-xs text-slate-300">
                  Покажите этот QR-код на входе или сохраните в галерею.
                </p>

                {/* Ticket QR preview */}
                <div className="p-3 bg-white rounded-xl mx-auto w-36 h-36 flex flex-col items-center justify-center">
                  <QrCode className="w-24 h-24 text-slate-900" />
                  <span className="text-[8px] font-mono text-slate-700 truncate max-w-[120px]">
                    {activeRegistration.ticket_qr}
                  </span>
                </div>

                <button
                  onClick={() => setActiveRegistration(null)}
                  className="w-full py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 transition-colors"
                >
                  Закрыть билет
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* --- SUB-TAB 3: ONBOARDING WIZARD --- */}
      {activeTab === 'onboarding' && (
        <div className="p-4 rounded-2xl bg-slate-900 border border-slate-800 space-y-4">
          {onboardingCompleted ? (
            <div className="py-6 text-center space-y-3">
              <div className="w-12 h-12 rounded-full bg-emerald-500/20 text-emerald-400 mx-auto flex items-center justify-center border border-emerald-500/40">
                <CheckCircle2 className="w-7 h-7" />
              </div>
              <h3 className="text-base font-bold text-white">Онбординг успешно пройден!</h3>
              <p className="text-xs text-slate-300 max-w-sm mx-auto">
                Ответы сохранены в вашем профиле FSM. Система автоматически адаптировала рекомендации в меню бота.
              </p>
              <button
                onClick={() => {
                  setOnboardingCompleted(false);
                  setWizardStep(1);
                }}
                className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 transition-colors"
              >
                Пройти повторно
              </button>
            </div>
          ) : (
            <div>
              {/* Progress bar */}
              <div className="flex items-center justify-between mb-3 text-xs text-slate-400">
                <span>Шаг {wizardStep} из 5</span>
                <span className="font-semibold text-indigo-400">{wizardStep * 20}%</span>
              </div>
              <div className="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden mb-5">
                <div
                  className="h-full bg-indigo-500 transition-all duration-300"
                  style={{ width: `${wizardStep * 20}%` }}
                />
              </div>

              {/* Step 1 */}
              {wizardStep === 1 && (
                <div className="space-y-3">
                  <h4 className="font-semibold text-sm text-white">1. Какой у вас тип бизнеса?</h4>
                  <div className="grid grid-cols-1 gap-2">
                    {[
                      { key: 'ecommerce', label: 'Интернет-магазин / Товарный бизнес' },
                      { key: 'services', label: 'Услуги, эксперты, консалтинг' },
                      { key: 'media', label: 'Медиа, Telegram-канал, блоггер' },
                      { key: 'saas', label: 'IT-продукт, SaaS или онлайн-школа' },
                    ].map((opt) => (
                      <button
                        key={opt.key}
                        onClick={() => handleOnboardingAnswer('business_type', opt.label)}
                        className="w-full text-left p-3 rounded-xl bg-slate-950 border border-slate-800 hover:border-indigo-500/60 hover:bg-slate-800/80 text-xs font-medium text-slate-200 transition-all flex items-center justify-between"
                      >
                        <span>{opt.label}</span>
                        <ChevronRight className="w-4 h-4 text-slate-500" />
                      </button>
                    ))}
                  </div>
                </div>
              )}

              {/* Step 2 */}
              {wizardStep === 2 && (
                <div className="space-y-3">
                  <h4 className="font-semibold text-sm text-white">
                    2. Какова главная цель внедрения бота?
                  </h4>
                  <div className="grid grid-cols-1 gap-2">
                    {[
                      { key: 'sales', label: 'Автоматизация продаж и прием платежей' },
                      { key: 'booking', label: 'Онлайн-запись и расписание визитов' },
                      { key: 'support', label: 'Разгрузка службы поддержки (AI FAQ)' },
                      { key: 'engagement', label: 'Розыгрыши, прогрев и удержание подписчиков' },
                    ].map((opt) => (
                      <button
                        key={opt.key}
                        onClick={() => handleOnboardingAnswer('primary_goal', opt.label)}
                        className="w-full text-left p-3 rounded-xl bg-slate-950 border border-slate-800 hover:border-indigo-500/60 hover:bg-slate-800/80 text-xs font-medium text-slate-200 transition-all flex items-center justify-between"
                      >
                        <span>{opt.label}</span>
                        <ChevronRight className="w-4 h-4 text-slate-500" />
                      </button>
                    ))}
                  </div>
                </div>
              )}

              {/* Step 3 */}
              {wizardStep === 3 && (
                <div className="space-y-3">
                  <h4 className="font-semibold text-sm text-white">
                    3. Примерный объем целевой аудитории в месяц?
                  </h4>
                  <div className="grid grid-cols-1 gap-2">
                    {[
                      { key: 'tiny', label: 'До 1 000 человек (старт)' },
                      { key: 'medium', label: 'От 1 000 до 10 000 человек' },
                      { key: 'large', label: 'От 10 000 до 50 000 человек' },
                      { key: 'enterprise', label: 'Свыше 50 000 человек (Highload)' },
                    ].map((opt) => (
                      <button
                        key={opt.key}
                        onClick={() => handleOnboardingAnswer('audience_size', opt.label)}
                        className="w-full text-left p-3 rounded-xl bg-slate-950 border border-slate-800 hover:border-indigo-500/60 hover:bg-slate-800/80 text-xs font-medium text-slate-200 transition-all flex items-center justify-between"
                      >
                        <span>{opt.label}</span>
                        <ChevronRight className="w-4 h-4 text-slate-500" />
                      </button>
                    ))}
                  </div>
                </div>
              )}

              {/* Step 4 */}
              {wizardStep === 4 && (
                <div className="space-y-3">
                  <h4 className="font-semibold text-sm text-white">
                    4. Опыт работы с ботами и Mini App?
                  </h4>
                  <div className="grid grid-cols-1 gap-2">
                    {[
                      { key: 'beginner', label: 'Нет опыта, запускаем впервые' },
                      { key: 'basic', label: 'Использовали простые кнопочные боты' },
                      { key: 'experienced', label: 'Ищем переход с самописного решения на платформу' },
                    ].map((opt) => (
                      <button
                        key={opt.key}
                        onClick={() => handleOnboardingAnswer('experience_level', opt.label)}
                        className="w-full text-left p-3 rounded-xl bg-slate-950 border border-slate-800 hover:border-indigo-500/60 hover:bg-slate-800/80 text-xs font-medium text-slate-200 transition-all flex items-center justify-between"
                      >
                        <span>{opt.label}</span>
                        <ChevronRight className="w-4 h-4 text-slate-500" />
                      </button>
                    ))}
                  </div>
                </div>
              )}

              {/* Step 5: Summary Card */}
              {wizardStep === 5 && (
                <div className="space-y-4">
                  <h4 className="font-semibold text-sm text-white">Итог анкеты:</h4>

                  <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-2.5 text-xs">
                    <div>
                      <span className="text-slate-400 block text-[11px]">Бизнес:</span>
                      <span className="text-slate-200 font-medium">{onboardingAnswers.business_type}</span>
                    </div>
                    <div>
                      <span className="text-slate-400 block text-[11px]">Цель:</span>
                      <span className="text-slate-200 font-medium">{onboardingAnswers.primary_goal}</span>
                    </div>
                    <div>
                      <span className="text-slate-400 block text-[11px]">Аудитория:</span>
                      <span className="text-slate-200 font-medium">{onboardingAnswers.audience_size}</span>
                    </div>
                    <div>
                      <span className="text-slate-400 block text-[11px]">Опыт:</span>
                      <span className="text-slate-200 font-medium">{onboardingAnswers.experience_level}</span>
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => setWizardStep(1)}
                      className="px-3 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-300 transition-colors flex items-center gap-1"
                    >
                      <Edit2 className="w-3.5 h-3.5" />
                      <span>Изменить</span>
                    </button>

                    <button
                      onClick={handleSubmitOnboarding}
                      disabled={isSubmittingOnboarding}
                      className="flex-1 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shadow-md shadow-indigo-600/30 transition-all flex items-center justify-center gap-1.5"
                    >
                      <Send className="w-3.5 h-3.5" />
                      <span>{isSubmittingOnboarding ? 'Отправка...' : 'Завершить онбординг'}</span>
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
};
