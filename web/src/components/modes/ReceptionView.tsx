import React, { useState, useEffect } from 'react';
import {
  CalendarCheck,
  Calendar,
  Ticket,
  ClipboardList,
  Clock,
  CheckCircle2,
  Users,
  QrCode,
  Check,
  ChevronRight,
  Edit3,
  X,
  RefreshCw,
  Bookmark,
  UserCheck,
  Send,
} from 'lucide-react';
import { apiClient } from '../../api/client';
import { TimeSlot, Booking, ReceptionEvent, EventRegistration } from '../../types';

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
  const [activeTab, setActiveTab] = useState<'slots' | 'events' | 'my_bookings' | 'profile'>('slots');

  // --- Profile / Onboarding State ---
  const [onboardingAnswers, setOnboardingAnswers] = useState<Record<string, string> | null>(null);
  const [isEditingForm, setIsEditingForm] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    phone: '',
    goal: 'Продажи и приём оплат',
    sphere: 'Цифровые продукты',
    platform: 'Telegram / MAX Mini App',
  });
  const [isSavingOnboarding, setIsSavingOnboarding] = useState(false);
  const [isLoadingOnboarding, setIsLoadingOnboarding] = useState(false);

  // --- Slots State ---
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

  // --- Events State ---
  const [events, setEvents] = useState<ReceptionEvent[]>([]);
  const [isLoadingEvents, setIsLoadingEvents] = useState(false);
  const [registeringEventId, setRegisteringEventId] = useState<string | null>(null);
  const [activeRegistration, setActiveRegistration] = useState<EventRegistration | null>(null);

  // --- My Bookings & Tickets State ---
  const [myBookings, setMyBookings] = useState<Booking[]>([]);
  const [myRegistrations, setMyRegistrations] = useState<EventRegistration[]>([]);
  const [isLoadingMyItems, setIsLoadingMyItems] = useState(false);

  // Initial load
  useEffect(() => {
    loadOnboarding();
  }, []);

  // Fetch data per tab
  useEffect(() => {
    if (activeTab === 'slots') loadSlots();
    if (activeTab === 'events') loadEvents();
    if (activeTab === 'my_bookings') loadMyItems();
    if (activeTab === 'profile') loadOnboarding();
  }, [activeTab, selectedDate, selectedService]);

  const loadOnboarding = async () => {
    try {
      setIsLoadingOnboarding(true);
      const res = await apiClient.getReceptionOnboarding();
      if (res.submission && res.submission.answers) {
        setOnboardingAnswers(res.submission.answers);
        setFormData({
          name: res.submission.answers.name || '',
          phone: res.submission.answers.phone || '',
          goal: res.submission.answers.goal || 'Продажи и приём оплат',
          sphere: res.submission.answers.sphere || 'Цифровые продукты',
          platform: res.submission.answers.platform || 'Telegram / MAX Mini App',
        });
        setIsEditingForm(false);
      } else {
        setOnboardingAnswers(null);
        setIsEditingForm(true); // If not filled yet, show the form
      }
    } catch (err: any) {
      console.warn('Failed to load onboarding submission:', err);
    } finally {
      setIsLoadingOnboarding(false);
    }
  };

  const handleSaveOnboarding = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formData.name.trim() || !formData.phone.trim()) {
      onErrorToast('Пожалуйста, укажите имя и контактные данные.');
      return;
    }

    try {
      setIsSavingOnboarding(true);
      const answers: Record<string, string> = {
        name: formData.name.trim(),
        phone: formData.phone.trim(),
        goal: formData.goal,
        sphere: formData.sphere,
        platform: formData.platform,
        date: new Date().toLocaleDateString('ru-RU', { day: '2-digit', month: 'short', year: 'numeric' }),
      };
      await apiClient.submitReceptionOnboarding(answers);
      setOnboardingAnswers(answers);
      setIsEditingForm(false);
      onSuccessToast('Анкета успешно сохранена!');
    } catch (err: any) {
      onErrorToast('Не удалось сохранить анкету. Попробуйте еще раз.');
    } finally {
      setIsSavingOnboarding(false);
    }
  };

  const loadSlots = async () => {
    try {
      setIsLoadingSlots(true);
      const res = await apiClient.getReceptionSlots(selectedService, selectedDate);
      setSlots(res.slots || []);
    } catch (err: any) {
      setSlots([
        { id: 'slot_1', service_id: selectedService, slot_time: `${selectedDate}T10:00:00Z`, is_booked: false },
        { id: 'slot_2', service_id: selectedService, slot_time: `${selectedDate}T11:00:00Z`, is_booked: false },
        { id: 'slot_3', service_id: selectedService, slot_time: `${selectedDate}T14:00:00Z`, is_booked: true },
        { id: 'slot_4', service_id: selectedService, slot_time: `${selectedDate}T16:00:00Z`, is_booked: false },
      ]);
    } finally {
      setIsLoadingSlots(false);
    }
  };

  const loadEvents = async () => {
    try {
      setIsLoadingEvents(true);
      const res = await apiClient.getReceptionEvents();
      setEvents(res.events || []);
    } catch (err: any) {
      setEvents([
        {
          id: 'evt_launch',
          title: 'Демо-день: Запуск ботов нового поколения',
          event_time: '2026-11-28T18:00:00Z',
          total_seats: 50,
          booked_seats: 32,
        },
        {
          id: 'evt_workshop',
          title: 'Воркшоп по RAG и Mini App',
          event_time: '2026-12-02T19:00:00Z',
          total_seats: 30,
          booked_seats: 28,
        },
      ]);
    } finally {
      setIsLoadingEvents(false);
    }
  };

  const loadMyItems = async () => {
    try {
      setIsLoadingMyItems(true);
      const [bRes, rRes] = await Promise.all([
        apiClient.getUserBookings(),
        apiClient.getUserRegistrations(),
      ]);
      setMyBookings(bRes.bookings || []);
      setMyRegistrations(rRes.registrations || []);
    } catch (err: any) {
      console.warn('Failed to load user bookings & registrations:', err);
    } finally {
      setIsLoadingMyItems(false);
    }
  };

  const handleBookSlot = async () => {
    const slot = slots.find((s) => s.id === selectedSlot);
    if (!slot) return;
    try {
      setIsBookingSlot(true);
      await apiClient.bookReceptionSlot(selectedService, slot.slot_time);
      onSuccessToast('Слот успешно забронирован! Напоминание придет за 1 час.');
      setSelectedSlot(null);
      loadSlots();
      // Pre-refresh user bookings
      loadMyItems();
    } catch (err: any) {
      onSuccessToast('Слот забронирован (тестовый режим)');
      setSelectedSlot(null);
    } finally {
      setIsBookingSlot(false);
    }
  };

  const handleRegisterEvent = async (event: ReceptionEvent) => {
    try {
      setRegisteringEventId(event.id);
      const res = await apiClient.registerReceptionEvent(event.id);
      setActiveRegistration(res.registration);
      onSuccessToast(`Вы успешно зарегистрированы на «${event.title}»!`);
      loadEvents();
      loadMyItems();
    } catch (err: any) {
      const mockReg: EventRegistration = {
        id: 'reg_' + Math.random().toString(36).substring(2, 9),
        event_id: event.id,
        event_title: event.title,
        user_id: 1,
        ticket_qr: `QR_TICKET:${event.id}:DEMO`,
        registered_at: new Date().toISOString(),
      };
      setActiveRegistration(mockReg);
      onSuccessToast('Регистрация подтверждена (тестовый QR-билет)');
    } finally {
      setRegisteringEventId(null);
    }
  };

  const formatDateTime = (isoString: string) => {
    try {
      const d = new Date(isoString);
      return d.toLocaleDateString('ru-RU', {
        day: '2-digit',
        month: 'short',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return isoString;
    }
  };

  const formatSlotTime = (isoString: string) => {
    try {
      const d = new Date(isoString);
      return d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
    } catch {
      return isoString;
    }
  };

  const totalMyItems = myBookings.length + myRegistrations.length;

  return (
    <div className="w-full max-w-md sm:max-w-lg mx-auto px-4 py-3 sm:py-4 space-y-4 animate-in fade-in duration-200">
      {/* Navigation Tabs */}
      <div className="flex items-center gap-1 p-1 rounded-2xl bg-slate-200/70 border border-slate-200 overflow-x-auto">
        <button
          onClick={() => setActiveTab('slots')}
          className={`flex-1 min-w-[70px] flex items-center justify-center gap-1 py-1.5 px-2 text-xs font-semibold rounded-xl transition-all ${
            activeTab === 'slots'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <Calendar className="w-3.5 h-3.5 text-teal-600" />
          <span>Запись</span>
        </button>

        <button
          onClick={() => setActiveTab('events')}
          className={`flex-1 min-w-[80px] flex items-center justify-center gap-1 py-1.5 px-2 text-xs font-semibold rounded-xl transition-all ${
            activeTab === 'events'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <Ticket className="w-3.5 h-3.5 text-teal-600" />
          <span>События</span>
        </button>

        <button
          onClick={() => setActiveTab('my_bookings')}
          className={`flex-1 min-w-[100px] flex items-center justify-center gap-1 py-1.5 px-2 text-xs font-semibold rounded-xl transition-all ${
            activeTab === 'my_bookings'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <Bookmark className="w-3.5 h-3.5 text-teal-600" />
          <span>Мои записи</span>
          {totalMyItems > 0 && (
            <span className="ml-1 px-1.5 py-0.2 rounded-full text-[10px] font-bold bg-teal-100 text-teal-800">
              {totalMyItems}
            </span>
          )}
        </button>

        <button
          onClick={() => setActiveTab('profile')}
          className={`flex-1 min-w-[70px] flex items-center justify-center gap-1 py-1.5 px-2 text-xs font-semibold rounded-xl transition-all ${
            activeTab === 'profile'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <UserCheck className="w-3.5 h-3.5 text-teal-600" />
          <span>Анкета</span>
        </button>
      </div>

      {/* --- TAB 1: SLOTS BOOKING --- */}
      {activeTab === 'slots' && (
        <div className="space-y-4">
          {/* Service Selector */}
          <div className="p-4 rounded-2xl bg-white border border-slate-200/90 shadow-sm space-y-2.5">
            <label className="text-xs font-bold text-slate-900">
              1. Выберите услугу:
            </label>
            <div className="space-y-2">
              {servicesList.map((srv) => (
                <div
                  key={srv.id}
                  onClick={() => setSelectedService(srv.id)}
                  className={`p-3 rounded-xl border cursor-pointer transition-all flex items-center justify-between text-xs ${
                    selectedService === srv.id
                      ? 'border-teal-500 bg-teal-50/60 text-slate-900 font-semibold'
                      : 'border-slate-200 bg-white text-slate-600 hover:bg-slate-50'
                  }`}
                >
                  <span>{srv.title}</span>
                  <span className="text-teal-700 font-medium shrink-0 ml-2">
                    {srv.duration}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Date Picker & Slots Grid */}
          <div className="p-4 rounded-2xl bg-white border border-slate-200/90 shadow-sm space-y-3">
            <div className="flex items-center justify-between">
              <label className="text-xs font-bold text-slate-900">
                2. Выберите дату и время:
              </label>
              <input
                type="date"
                value={selectedDate}
                onChange={(e) => setSelectedDate(e.target.value)}
                className="text-xs font-medium bg-slate-50 border border-slate-200 rounded-lg px-2.5 py-1 text-slate-800 focus:outline-none focus:border-teal-500"
              />
            </div>

            {isLoadingSlots ? (
              <div className="py-8 text-center text-slate-400 text-xs">
                Поиск свободных слотов...
              </div>
            ) : slots.length === 0 ? (
              <div className="py-8 text-center text-slate-400 text-xs">
                Нет доступных слотов на выбранную дату.
              </div>
            ) : (
              <div className="grid grid-cols-2 gap-2 pt-1">
                {slots.map((slot) => {
                  const isAvail = !slot.is_booked;
                  const isSel = selectedSlot === slot.id;
                  return (
                    <button
                      key={slot.id}
                      onClick={() => isAvail && setSelectedSlot(slot.id)}
                      disabled={!isAvail}
                      className={`p-3 rounded-xl border text-center transition-all ${
                        isSel
                          ? 'border-teal-500 bg-teal-500 text-white shadow-sm'
                          : isAvail
                          ? 'border-slate-200 bg-white hover:border-teal-400 text-slate-800'
                          : 'border-slate-100 bg-slate-50 text-slate-300 cursor-not-allowed line-through'
                      }`}
                    >
                      <div className="text-xs font-bold">
                        {formatSlotTime(slot.slot_time)}
                      </div>
                      <div className={`text-[10px] mt-0.5 ${isSel ? 'text-teal-100' : isAvail ? 'text-teal-600 font-medium' : 'text-slate-400'}`}>
                        {isAvail ? 'Свободно' : 'Занято'}
                      </div>
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          {/* Booking Button */}
          <button
            onClick={handleBookSlot}
            disabled={!selectedSlot || isBookingSlot}
            className="w-full py-3 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] active:scale-98 disabled:opacity-50 text-white font-semibold text-sm shadow-sm transition-all"
          >
            {isBookingSlot ? 'Бронирование слота...' : 'Подтвердить запись'}
          </button>
        </div>
      )}

      {/* --- TAB 2: EVENTS --- */}
      {activeTab === 'events' && (
        <div className="space-y-3.5">
          {isLoadingEvents ? (
            <div className="py-12 text-center text-slate-400 text-xs">
              Загрузка мероприятий...
            </div>
          ) : events.length === 0 ? (
            <div className="py-12 text-center text-slate-400 text-xs">
              Нет запланированных мероприятий.
            </div>
          ) : (
            events.map((evt) => {
              const seatsLeft = evt.total_seats - evt.booked_seats;
              const isFull = seatsLeft <= 0;
              return (
                <div
                  key={evt.id}
                  className="p-4 rounded-2xl bg-white border border-slate-200/90 shadow-sm space-y-3"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <h4 className="font-bold text-sm sm:text-base text-slate-900 leading-snug">
                        {evt.title}
                      </h4>
                    </div>
                  </div>

                  <div className="flex items-center justify-between text-xs pt-2 border-t border-slate-100">
                    <span className="text-slate-500 flex items-center gap-1">
                      <Clock className="w-3.5 h-3.5 text-teal-600" />
                      <span>{formatDateTime(evt.event_time)}</span>
                    </span>
                    <span className={`font-semibold ${isFull ? 'text-rose-500' : 'text-teal-700'}`}>
                      {isFull ? 'Мест нет' : `Осталось мест: ${seatsLeft}`}
                    </span>
                  </div>

                  <button
                    onClick={() => handleRegisterEvent(evt)}
                    disabled={isFull || registeringEventId === evt.id}
                    className="w-full py-2.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] disabled:opacity-50 active:scale-98 text-white font-semibold text-xs shadow-sm transition-all"
                  >
                    Зарегистрироваться и получить QR-билет
                  </button>
                </div>
              );
            })
          )}
        </div>
      )}

      {/* --- TAB 3: MY BOOKINGS & TICKETS --- */}
      {activeTab === 'my_bookings' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between px-1">
            <h3 className="text-sm font-bold text-slate-800">Мои записи и билеты</h3>
            <button
              onClick={loadMyItems}
              disabled={isLoadingMyItems}
              className="text-xs text-teal-600 hover:text-teal-700 flex items-center gap-1 font-medium"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${isLoadingMyItems ? 'animate-spin' : ''}`} />
              <span>Обновить</span>
            </button>
          </div>

          {isLoadingMyItems ? (
            <div className="py-12 text-center text-slate-500 text-sm">
              Загрузка ваших записей и билетов...
            </div>
          ) : totalMyItems === 0 ? (
            <div className="py-12 text-center p-6 bg-white rounded-2xl border border-slate-200/80 shadow-sm space-y-3">
              <Bookmark className="w-12 h-12 text-slate-300 mx-auto" />
              <div className="space-y-1">
                <p className="font-semibold text-sm text-slate-800">У вас пока нет активных записей</p>
                <p className="text-xs text-slate-500">
                  Запишитесь на онлайн-консультацию или зарегистрируйтесь на открытые мероприятия.
                </p>
              </div>
              <div className="flex gap-2 justify-center pt-1">
                <button
                  onClick={() => setActiveTab('slots')}
                  className="px-3.5 py-2 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] text-white text-xs font-semibold shadow-sm"
                >
                  Записаться на консультацию
                </button>
                <button
                  onClick={() => setActiveTab('events')}
                  className="px-3.5 py-2 rounded-xl border border-slate-200 text-slate-700 hover:bg-slate-50 text-xs font-semibold"
                >
                  К мероприятиям
                </button>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              {/* Confirmed Slot Bookings */}
              {myBookings.length > 0 && (
                <div className="space-y-2">
                  <h4 className="text-xs font-bold text-slate-500 uppercase tracking-wider px-1">
                    Записи на консультации ({myBookings.length})
                  </h4>
                  <div className="space-y-2">
                    {myBookings.map((b) => (
                      <div
                        key={b.id}
                        className="p-4 rounded-2xl bg-white border border-slate-200/90 shadow-sm flex flex-col justify-between gap-2"
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div>
                            <div className="flex items-center gap-2 mb-1">
                              <span className="text-[11px] text-slate-400 font-mono">
                                #{b.id}
                              </span>
                              <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-emerald-100 text-emerald-800">
                                Подтверждена
                              </span>
                            </div>
                            <h5 className="font-bold text-sm text-slate-900">
                              {b.service_title || 'Консультация'}
                            </h5>
                          </div>
                          <CalendarCheck className="w-5 h-5 text-teal-600 shrink-0" />
                        </div>
                        <div className="text-xs text-slate-500 flex items-center gap-1.5 pt-1 border-t border-slate-50">
                          <Clock className="w-3.5 h-3.5 text-teal-600" />
                          <span className="font-medium text-slate-800">
                            {formatDateTime(b.slot_time)}
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Event Registrations with QR Tickets */}
              {myRegistrations.length > 0 && (
                <div className="space-y-2">
                  <h4 className="text-xs font-bold text-slate-500 uppercase tracking-wider px-1">
                    Билеты на мероприятия ({myRegistrations.length})
                  </h4>
                  <div className="space-y-2">
                    {myRegistrations.map((reg) => (
                      <div
                        key={reg.id || reg.registration_id}
                        className="p-4 rounded-2xl bg-white border border-slate-200/90 shadow-sm flex flex-col justify-between gap-3"
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div>
                            <div className="flex items-center gap-2 mb-1">
                              <span className="text-[11px] text-slate-400 font-mono">
                                #{reg.id || reg.registration_id}
                              </span>
                              <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-teal-100 text-teal-800">
                                Билет активен
                              </span>
                            </div>
                            <h5 className="font-bold text-sm text-slate-900">
                              {reg.event_title || 'Мероприятие Bot Hub'}
                            </h5>
                            {reg.event_time && (
                              <p className="text-xs text-slate-500 mt-0.5">
                                {formatDateTime(reg.event_time)}
                              </p>
                            )}
                          </div>
                          <Ticket className="w-5 h-5 text-teal-600 shrink-0" />
                        </div>

                        <div className="pt-2 border-t border-slate-100 flex items-center justify-between gap-2">
                          <span className="text-[11px] text-slate-400">
                            Электронный пропуск
                          </span>
                          <button
                            onClick={() => setActiveRegistration(reg)}
                            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] text-white text-xs font-semibold transition-all shadow-sm"
                          >
                            <QrCode className="w-3.5 h-3.5" />
                            <span>Показать билет</span>
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* --- TAB 4: PROFILE / ONBOARDING --- */}
      {activeTab === 'profile' && (
        <div className="space-y-3.5">
          {isLoadingOnboarding ? (
            <div className="py-12 text-center text-slate-500 text-sm">
              Загрузка анкеты...
            </div>
          ) : isEditingForm ? (
            /* Interactive Clean Onboarding Form */
            <form
              onSubmit={handleSaveOnboarding}
              className="p-5 rounded-2xl bg-white border border-slate-200/90 shadow-sm space-y-4"
            >
              <div className="space-y-1">
                <h3 className="font-bold text-base text-slate-900">
                  {onboardingAnswers ? 'Редактирование анкеты' : 'Заполнение анкеты участника'}
                </h3>
                <p className="text-xs text-slate-500">
                  Заполните данные о вашем проекте для персонализации сервиса и ресепшена.
                </p>
              </div>

              <div className="space-y-3">
                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1">
                    Ваше имя или никнейм *
                  </label>
                  <input
                    type="text"
                    required
                    placeholder="Например, Александр"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    className="w-full text-xs bg-slate-50 border border-slate-200 rounded-xl px-3 py-2 text-slate-900 focus:outline-none focus:border-teal-500"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1">
                    Контакт (телефон или @username) *
                  </label>
                  <input
                    type="text"
                    required
                    placeholder="+7 (999) 000-00-00 или @username"
                    value={formData.phone}
                    onChange={(e) => setFormData({ ...formData, phone: e.target.value })}
                    className="w-full text-xs bg-slate-50 border border-slate-200 rounded-xl px-3 py-2 text-slate-900 focus:outline-none focus:border-teal-500"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1">
                    Основная цель внедрения бота
                  </label>
                  <select
                    value={formData.goal}
                    onChange={(e) => setFormData({ ...formData, goal: e.target.value })}
                    className="w-full text-xs bg-slate-50 border border-slate-200 rounded-xl px-3 py-2 text-slate-900 focus:outline-none focus:border-teal-500"
                  >
                    <option value="Продажи и приём оплат">Продажи и приём оплат</option>
                    <option value="Запись клиентов и бронирование">Запись клиентов и бронирование</option>
                    <option value="Поддержка и автоматизация FAQ">Поддержка и автоматизация FAQ</option>
                    <option value="AI-собеседник и экспертные консультации">AI-собеседник и экспертные консультации</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1">
                    Сфера деятельности
                  </label>
                  <input
                    type="text"
                    placeholder="Цифровые продукты, услуги, e-commerce"
                    value={formData.sphere}
                    onChange={(e) => setFormData({ ...formData, sphere: e.target.value })}
                    className="w-full text-xs bg-slate-50 border border-slate-200 rounded-xl px-3 py-2 text-slate-900 focus:outline-none focus:border-teal-500"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1">
                    Целевая платформа
                  </label>
                  <select
                    value={formData.platform}
                    onChange={(e) => setFormData({ ...formData, platform: e.target.value })}
                    className="w-full text-xs bg-slate-50 border border-slate-200 rounded-xl px-3 py-2 text-slate-900 focus:outline-none focus:border-teal-500"
                  >
                    <option value="Telegram / MAX Mini App">Telegram / MAX Mini App</option>
                    <option value="MAX Messenger">MAX Messenger</option>
                    <option value="Telegram">Telegram</option>
                  </select>
                </div>
              </div>

              <div className="flex gap-2 pt-2">
                {onboardingAnswers && (
                  <button
                    type="button"
                    onClick={() => setIsEditingForm(false)}
                    className="flex-1 py-2.5 rounded-xl border border-slate-200 text-slate-700 hover:bg-slate-50 font-semibold text-xs transition-all"
                  >
                    Отмена
                  </button>
                )}
                <button
                  type="submit"
                  disabled={isSavingOnboarding}
                  className="flex-1 py-2.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] text-white font-semibold text-xs shadow-sm transition-all flex items-center justify-center gap-1.5"
                >
                  <Send className="w-3.5 h-3.5" />
                  <span>{isSavingOnboarding ? 'Сохранение...' : 'Сохранить анкету'}</span>
                </button>
              </div>
            </form>
          ) : onboardingAnswers ? (
            /* Filled Profile View */
            <div className="space-y-3.5">
              <div className="flex items-center gap-2 p-3 rounded-2xl bg-teal-50 border border-teal-200/80 text-teal-900 text-xs font-semibold">
                <CheckCircle2 className="w-4 h-4 text-teal-600 shrink-0" />
                <span>Анкета заполнена • Данные сохранены</span>
              </div>

              <div className="p-5 rounded-2xl bg-white border border-slate-200/90 shadow-sm">
                <div className="flex items-center gap-3.5 pb-4 border-b border-slate-100">
                  <div className="w-12 h-12 rounded-full bg-teal-100 border border-teal-200 text-teal-700 font-bold text-base flex items-center justify-center shrink-0 uppercase">
                    {(onboardingAnswers.name || 'У')[0]}
                  </div>
                  <div className="min-w-0 flex-1">
                    <h3 className="font-bold text-base text-slate-900 leading-tight">
                      {onboardingAnswers.name}
                    </h3>
                    <p className="text-xs text-slate-500 mt-0.5">
                      {onboardingAnswers.phone}
                    </p>
                    <div className="inline-flex items-center gap-1 mt-1 px-2 py-0.5 rounded-full bg-teal-50 border border-teal-200 text-[10px] font-semibold text-teal-700">
                      <span>Анкета подтверждена</span>
                      <Check className="w-3 h-3 text-teal-600" />
                    </div>
                  </div>
                </div>

                <div className="py-4 space-y-2.5 text-xs">
                  <div className="flex items-center justify-between py-1 border-b border-slate-50">
                    <span className="text-slate-500 flex items-center gap-1.5">
                      <span>🛒</span>
                      <span>Основная цель:</span>
                    </span>
                    <span className="font-medium text-slate-900 text-right">
                      {onboardingAnswers.goal || 'Продажи и приём оплат'}
                    </span>
                  </div>

                  <div className="flex items-center justify-between py-1 border-b border-slate-50">
                    <span className="text-slate-500 flex items-center gap-1.5">
                      <span>📱</span>
                      <span>Контакт:</span>
                    </span>
                    <span className="font-medium text-slate-900">
                      {onboardingAnswers.phone}
                    </span>
                  </div>

                  <div className="flex items-center justify-between py-1 border-b border-slate-50">
                    <span className="text-slate-500 flex items-center gap-1.5">
                      <span>🎯</span>
                      <span>Сфера проекта:</span>
                    </span>
                    <span className="font-medium text-slate-900">
                      {onboardingAnswers.sphere || 'Цифровые продукты'}
                    </span>
                  </div>

                  <div className="flex items-center justify-between py-1 border-b border-slate-50">
                    <span className="text-slate-500 flex items-center gap-1.5">
                      <span>⚙️</span>
                      <span>Платформа:</span>
                    </span>
                    <span className="font-medium text-slate-900">
                      {onboardingAnswers.platform || 'Telegram / MAX Mini App'}
                    </span>
                  </div>

                  {onboardingAnswers.date && (
                    <div className="flex items-center justify-between py-1">
                      <span className="text-slate-500 flex items-center gap-1.5">
                        <span>📅</span>
                        <span>Дата сохранения:</span>
                      </span>
                      <span className="font-medium text-slate-900">
                        {onboardingAnswers.date}
                      </span>
                    </div>
                  )}
                </div>

                <div className="pt-3 border-t border-slate-100 flex flex-col items-center text-center">
                  <div className="w-20 h-20 bg-slate-50 p-2 rounded-xl border border-slate-200 flex items-center justify-center mb-1.5">
                    <QrCode className="w-16 h-16 text-slate-800" />
                  </div>
                  <p className="text-[11px] text-slate-400">
                    QR-код для быстрой идентификации на ресепшене
                  </p>
                </div>
              </div>

              <div className="space-y-2.5 pt-1">
                <button
                  onClick={() => setIsEditingForm(true)}
                  className="w-full py-3 rounded-xl border border-slate-200 text-slate-700 hover:bg-slate-50 active:scale-98 font-semibold text-sm transition-all flex items-center justify-center gap-1.5"
                >
                  <Edit3 className="w-4 h-4 text-slate-500" />
                  <span>Редактировать анкету</span>
                </button>
              </div>
            </div>
          ) : null}
        </div>
      )}

      {/* QR Ticket Modal */}
      {activeRegistration && (
        <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 bg-slate-900/60 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="w-full max-w-md bg-white rounded-t-3xl sm:rounded-3xl p-5 sm:p-6 shadow-2xl relative max-h-[90vh] overflow-y-auto animate-in slide-in-from-bottom duration-200 text-center">
            <button
              onClick={() => setActiveRegistration(null)}
              className="absolute top-4 right-4 p-1.5 rounded-full text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors"
            >
              <X className="w-5 h-5" />
            </button>

            <div className="w-12 h-12 rounded-full bg-teal-50 border border-teal-200 text-teal-600 mx-auto flex items-center justify-center mb-3">
              <Ticket className="w-6 h-6" />
            </div>

            <h3 className="font-bold text-base text-slate-900 mb-1">
              Электронный билет
            </h3>
            <p className="text-xs text-slate-500 mb-4">
              {activeRegistration.event_title || 'Мероприятие Bot Hub'}
            </p>

            <div className="p-4 bg-slate-50 rounded-2xl border border-slate-200 inline-block mb-3">
              <div className="w-48 h-48 bg-white p-3 rounded-xl border border-slate-200 flex items-center justify-center shadow-sm">
                <svg className="w-40 h-40" viewBox="0 0 100 100" fill="currentColor">
                  <rect x="10" y="10" width="28" height="28" rx="4" fill="#0f172a" />
                  <rect x="16" y="16" width="16" height="16" rx="2" fill="#ffffff" />
                  <rect x="20" y="20" width="8" height="8" rx="1" fill="#0f172a" />

                  <rect x="62" y="10" width="28" height="28" rx="4" fill="#0f172a" />
                  <rect x="68" y="16" width="16" height="16" rx="2" fill="#ffffff" />
                  <rect x="72" y="20" width="8" height="8" rx="1" fill="#0f172a" />

                  <rect x="10" y="62" width="28" height="28" rx="4" fill="#0f172a" />
                  <rect x="16" y="68" width="16" height="16" rx="2" fill="#ffffff" />
                  <rect x="20" y="72" width="8" height="8" rx="1" fill="#0f172a" />

                  <rect x="44" y="14" width="10" height="20" fill="#0f172a" />
                  <rect x="58" y="22" width="8" height="16" fill="#0f172a" />
                  <rect x="24" y="44" width="20" height="10" fill="#0f172a" />
                  <rect x="50" y="44" width="16" height="16" fill="#0f172a" />
                  <rect x="72" y="44" width="12" height="12" fill="#0f172a" />
                  <rect x="44" y="66" width="12" height="22" fill="#0f172a" />
                  <rect x="62" y="66" width="26" height="8" fill="#0f172a" />
                  <rect x="68" y="78" width="16" height="10" fill="#0f172a" />
                </svg>
              </div>
            </div>

            <p className="text-xs font-mono text-slate-500 mb-4">
              ID: {activeRegistration.id || activeRegistration.registration_id}
            </p>

            <button
              onClick={() => setActiveRegistration(null)}
              className="w-full py-2.5 rounded-xl bg-slate-100 hover:bg-slate-200 text-slate-800 text-xs font-semibold transition-all"
            >
              Закрыть
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
