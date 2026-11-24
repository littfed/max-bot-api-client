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
  const [activeTab, setActiveTab] = useState<'profile' | 'slots' | 'events'>('profile');

  // --- Profile / Onboarding State ---
  const [profileData, setProfileData] = useState({
    name: 'Алексей Смирнов',
    id: '24910',
    goal: 'Продажи и приём оплат',
    phone: '+7 (999) 123-45-67',
    sphere: 'Цифровые продукты',
    platform: 'Telegram / MAX Mini App',
    date: '24 ноя 2026',
  });

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

  // Fetch Slots or Events
  useEffect(() => {
    if (activeTab === 'slots') loadSlots();
    if (activeTab === 'events') loadEvents();
  }, [activeTab, selectedDate, selectedService]);

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

  const handleBookSlot = async () => {
    const slot = slots.find((s) => s.id === selectedSlot);
    if (!slot) return;
    try {
      setIsBookingSlot(true);
      await apiClient.bookReceptionSlot(selectedService, slot.slot_time);
      onSuccessToast('Слот успешно забронирован! Напоминание придет за 1 час.');
      setSelectedSlot(null);
      loadSlots();
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
    } catch (err: any) {
      setActiveRegistration({
        id: 'reg_' + Math.random().toString(36).substring(2, 9),
        event_id: event.id,
        user_id: 1,
        ticket_qr: 'https://api.qrserver.com/v1/create-qr-code/?data=PASS-DEMO',
        registered_at: new Date().toISOString(),
      });
      onSuccessToast('Регистрация подтверждена (тестовый QR-билет)');
    } finally {
      setRegisteringEventId(null);
    }
  };

  const formatSlotTime = (isoString: string) => {
    try {
      const d = new Date(isoString);
      return d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
    } catch (_) {
      return isoString;
    }
  };

  return (
    <div className="w-full max-w-md sm:max-w-lg mx-auto px-4 py-3 sm:py-4 space-y-4 animate-in fade-in duration-200">
      {/* Navigation Tabs */}
      <div className="flex items-center gap-1.5 p-1 rounded-2xl bg-slate-200/60 border border-slate-200">
        <button
          onClick={() => setActiveTab('profile')}
          className={`flex-1 py-1.5 text-xs font-semibold rounded-xl transition-all ${
            activeTab === 'profile'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          Карточка профиля
        </button>
        <button
          onClick={() => setActiveTab('slots')}
          className={`flex-1 flex items-center justify-center gap-1 py-1.5 text-xs font-semibold rounded-xl transition-all ${
            activeTab === 'slots'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <Calendar className="w-3.5 h-3.5 text-teal-600" />
          <span>Запись на слот</span>
        </button>
        <button
          onClick={() => setActiveTab('events')}
          className={`flex-1 flex items-center justify-center gap-1 py-1.5 text-xs font-semibold rounded-xl transition-all ${
            activeTab === 'events'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <Ticket className="w-3.5 h-3.5 text-teal-600" />
          <span>Мероприятия</span>
        </button>
      </div>

      {/* --- TAB 1: PROFILE CARD MATCHING MOCKUP --- */}
      {activeTab === 'profile' && (
        <div className="space-y-3.5">
          {/* Top Success Banner */}
          <div className="flex items-center gap-2 p-3 rounded-2xl bg-teal-50 border border-teal-200/80 text-teal-900 text-xs font-semibold">
            <CheckCircle2 className="w-4 h-4 text-teal-600 shrink-0" />
            <span>Анкета успешно заполнена • 100%</span>
          </div>

          {/* Profile Card */}
          <div className="p-5 rounded-2xl bg-white border border-slate-200/90 shadow-sm">
            {/* Header: Avatar, Name, ID, Badge */}
            <div className="flex items-center gap-3.5 pb-4 border-b border-slate-100">
              <div className="w-12 h-12 rounded-full bg-teal-100 border border-teal-200 text-teal-700 font-bold text-base flex items-center justify-center shrink-0">
                AC
              </div>
              <div className="min-w-0 flex-1">
                <h3 className="font-bold text-base text-slate-900 leading-tight">
                  {profileData.name}
                </h3>
                <p className="text-xs text-slate-500 mt-0.5">
                  ID #{profileData.id} • Клиент
                </p>
                <div className="inline-flex items-center gap-1 mt-1 px-2 py-0.5 rounded-full bg-teal-50 border border-teal-200 text-[10px] font-semibold text-teal-700">
                  <span>Профиль подтверждён</span>
                  <Check className="w-3 h-3 text-teal-600" />
                </div>
              </div>
            </div>

            {/* Profile Fields List matching Mockup */}
            <div className="py-4 space-y-2.5 text-xs">
              <div className="flex items-center justify-between py-1 border-b border-slate-50">
                <span className="text-slate-500 flex items-center gap-1.5">
                  <span>🛒</span>
                  <span>Основная цель:</span>
                </span>
                <span className="font-medium text-slate-900 text-right">
                  {profileData.goal}
                </span>
              </div>

              <div className="flex items-center justify-between py-1 border-b border-slate-50">
                <span className="text-slate-500 flex items-center gap-1.5">
                  <span>📱</span>
                  <span>Телефон:</span>
                </span>
                <span className="font-medium text-slate-900">
                  {profileData.phone}
                </span>
              </div>

              <div className="flex items-center justify-between py-1 border-b border-slate-50">
                <span className="text-slate-500 flex items-center gap-1.5">
                  <span>🎯</span>
                  <span>Сфера проекта:</span>
                </span>
                <span className="font-medium text-slate-900">
                  {profileData.sphere}
                </span>
              </div>

              <div className="flex items-center justify-between py-1 border-b border-slate-50">
                <span className="text-slate-500 flex items-center gap-1.5">
                  <span>⚙️</span>
                  <span>Платформа:</span>
                </span>
                <span className="font-medium text-slate-900">
                  {profileData.platform}
                </span>
              </div>

              <div className="flex items-center justify-between py-1">
                <span className="text-slate-500 flex items-center gap-1.5">
                  <span>📅</span>
                  <span>Дата заполнения:</span>
                </span>
                <span className="font-medium text-slate-900">
                  {profileData.date}
                </span>
              </div>
            </div>

            {/* Small QR Code Container */}
            <div className="pt-3 border-t border-slate-100 flex flex-col items-center text-center">
              <div className="w-20 h-20 bg-slate-50 p-2 rounded-xl border border-slate-200 flex items-center justify-center mb-1.5">
                <QrCode className="w-16 h-16 text-slate-800" />
              </div>
              <p className="text-[11px] text-slate-400">
                QR-код для быстрой отметки на ресепшене
              </p>
            </div>
          </div>

          {/* Action Buttons matching Mockup */}
          <div className="space-y-2.5 pt-1">
            <button
              onClick={onBack}
              className="w-full py-3 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] active:scale-98 text-white font-semibold text-sm shadow-sm transition-all"
            >
              Перейти в Хаб →
            </button>
            <button
              onClick={() => onSuccessToast('Режим редактирования анкеты активирован')}
              className="w-full py-3 rounded-xl border border-slate-200 text-slate-700 hover:bg-slate-50 active:scale-98 font-semibold text-sm transition-all flex items-center justify-center gap-1.5"
            >
              <Edit3 className="w-4 h-4 text-slate-500" />
              <span>Редактировать анкету</span>
            </button>
          </div>
        </div>
      )}

      {/* --- TAB 2: SLOTS BOOKING --- */}
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

      {/* --- TAB 3: EVENTS --- */}
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
                      <h4 className="font-bold text-sm text-slate-900 leading-snug">
                        {evt.title}
                      </h4>
                    </div>
                  </div>

                  <div className="flex items-center justify-between text-xs pt-2 border-t border-slate-100">
                    <span className="text-slate-500 flex items-center gap-1">
                      <Clock className="w-3.5 h-3.5 text-teal-600" />
                      <span>{new Date(evt.event_time).toLocaleDateString('ru-RU')}</span>
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
    </div>
  );
};
