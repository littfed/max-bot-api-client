import axios, { AxiosInstance } from 'axios';
import {
  BootstrapResponse,
  Product,
  ShopOrder,
  PaymentSimulationResult,
  TimeSlot,
  Booking,
  ReceptionEvent,
  EventRegistration,
  SupportAskResponse,
  SupportTicket,
  ChatConfigResponse,
  ChatMessageResponse,
  ChatSessionResponse,
  ChatSettingsResponse,
  Lead,
  AdminAuditLog,
  ModeItem,
} from '../types';

declare global {
  interface Window {
    Telegram?: {
      WebApp?: {
        initData?: string;
        initDataUnsafe?: Record<string, any>;
        close?: () => void;
        expand?: () => void;
        ready?: () => void;
      };
    };
    MaxWebApp?: {
      initData?: string;
      close?: () => void;
    };
  }
}

// Safe LocalStorage helpers (prevent exceptions in restricted WebViews)
function safeGetStorage(key: string): string | null {
  try {
    return typeof localStorage !== 'undefined' ? localStorage.getItem(key) : null;
  } catch (_) {
    return null;
  }
}

function safeSetStorage(key: string, value: string): void {
  try {
    if (typeof localStorage !== 'undefined') localStorage.setItem(key, value);
  } catch (_) {}
}

function safeRemoveStorage(key: string): void {
  try {
    if (typeof localStorage !== 'undefined') localStorage.removeItem(key);
  } catch (_) {}
}

// Detect client platform
export function getClientPlatform(): 'max' | 'telegram' | 'web' {
  if (typeof window !== 'undefined') {
    const win = window as any;
    if (win.WebViewHandler || win.max || win.MaxWebApp || (win.WebApp && !win.Telegram?.WebApp?.initData)) {
      return 'max';
    }
    if (win.Telegram?.WebApp?.initData) return 'telegram';

    const params = new URLSearchParams(window.location.search);
    const p = params.get('platform');
    if (p === 'max' || p === 'telegram') return p;
    if (params.get('query_id')?.startsWith('MAX_') || params.get('WebAppData')) return 'max';

    if (window.location.hash) {
      const hashParams = new URLSearchParams(window.location.hash.replace(/^#/, ''));
      if (hashParams.get('WebAppData') || hashParams.get('query_id')?.startsWith('MAX_')) return 'max';
    }

    if (/telegram/i.test(navigator.userAgent)) return 'telegram';
  }
  return 'max';
}

// Extract initData from webapp environment or URL query
export function getInitData(): string {
  if (typeof window === 'undefined') return '';

  const win = window as any;
  try {
    if (win.WebApp?.initData) return win.WebApp.initData;
    if (win.max?.initData) return win.max.initData;
    if (win.MaxWebApp?.initData) return win.MaxWebApp.initData;
    if (win.Telegram?.WebApp?.initData) return win.Telegram.WebApp.initData;
  } catch (_) {}

  // Check URL query parameters
  if (window.location.search) {
    const search = window.location.search.startsWith('?')
      ? window.location.search.slice(1)
      : window.location.search;
    const params = new URLSearchParams(search);

    if (params.get('WebAppData')) return params.get('WebAppData')!;
    if (params.get('init_data')) return params.get('init_data')!;
    if (params.get('tgWebAppData')) return params.get('tgWebAppData')!;

    // If query contains 'hash' and ('user' or 'auth_date'), the search query string itself IS the signed initData
    if (params.has('hash') && (params.has('user') || params.has('auth_date'))) {
      return search;
    }
  }

  // Check URL hash fragment (Telegram/MAX may place data in hash fragment)
  if (window.location.hash) {
    const hashStr = window.location.hash.startsWith('#')
      ? window.location.hash.slice(1)
      : window.location.hash;
    const hashParams = new URLSearchParams(hashStr);

    if (hashParams.get('WebAppData')) return hashParams.get('WebAppData')!;
    if (hashParams.get('tgWebAppData')) return hashParams.get('tgWebAppData')!;
    if (hashParams.get('init_data')) return hashParams.get('init_data')!;
    if (hashParams.has('hash') && (hashParams.has('user') || hashParams.has('auth_date'))) {
      return hashStr;
    }
  }

  return '';
}

// Base Axios instance
export const api: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 30000,
});

// Attach auth headers to every outgoing request
api.interceptors.request.use((config) => {
  const initData = getInitData();
  if (initData) {
    config.headers['X-Init-Data'] = initData;
  }

  const platform = getClientPlatform();
  config.headers['X-Platform'] = platform;

  const storedToken = safeGetStorage('bothub_jwt_token');
  if (storedToken) {
    config.headers['Authorization'] = `Bearer ${storedToken}`;
  }

  // Support local dev header if query param dev_user_id is present
  if (typeof window !== 'undefined') {
    const params = new URLSearchParams(window.location.search);
    const devUserId = params.get('dev_user_id');
    if (devUserId) {
      config.headers['X-Dev-User-ID'] = devUserId;
    }
  }

  return config;
});

// Interceptor to automatically persist token from bootstrap response
api.interceptors.response.use(
  (response) => {
    if (response.data && response.data.token) {
      safeSetStorage('bothub_jwt_token', response.data.token);
    }
    return response;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Typed API methods
export const apiClient = {
  // Bootstrap & Core Hub
  async getBootstrap(): Promise<BootstrapResponse> {
    const res = await api.get<BootstrapResponse>('/bootstrap');
    return res.data;
  },

  async setUserMode(modeId: string): Promise<{ success: boolean; active_mode: string }> {
    const res = await api.post<{ success: boolean; active_mode: string }>('/user/mode', {
      mode_id: modeId,
    });
    return res.data;
  },

  async tapMascot(): Promise<{ phrase: string; priority: number; emotion: string }> {
    const res = await api.get<{ phrase: string; priority: number; emotion: string }>('/mascot/tap');
    return res.data;
  },

  async submitLead(data: {
    contact: string;
    comment: string;
    source_mode: string;
    tested_modes: string[];
  }): Promise<{ success: boolean; lead_id: number }> {
    const res = await api.post<{ success: boolean; lead_id: number }>('/lead', data);
    return res.data;
  },

  // Shop Module
  async getShopCatalog(): Promise<{ products: Product[] }> {
    const res = await api.get<{ products: Product[] }>('/shop/catalog');
    return res.data;
  },

  async createShopOrder(productId: string): Promise<{ order: ShopOrder }> {
    const res = await api.post<{ order: ShopOrder }>('/shop/orders', {
      product_id: productId,
    });
    return res.data;
  },

  async simulateShopPayment(
    orderId: string,
    action: 'paid' | 'abandoned'
  ): Promise<{ result: PaymentSimulationResult }> {
    const res = await api.post<{ result: PaymentSimulationResult }>(
      `/shop/orders/${encodeURIComponent(orderId)}/simulate`,
      { action }
    );
    return res.data;
  },

  async getUserOrders(): Promise<{ orders: ShopOrder[] }> {
    const res = await api.get<{ orders: ShopOrder[] }>('/shop/orders');
    return res.data;
  },

  // Reception Module
  async getReceptionSlots(serviceId: string, date?: string): Promise<{ service_id: string; slots: TimeSlot[] }> {
    const res = await api.get<{ service_id: string; slots: TimeSlot[] }>('/reception/slots', {
      params: { service_id: serviceId, date },
    });
    return res.data;
  },

  async bookReceptionSlot(serviceId: string, slotTime: string): Promise<{ success: boolean; message: string }> {
    const res = await api.post<{ success: boolean; message: string }>('/reception/book', {
      service_id: serviceId,
      slot_time: slotTime,
    });
    return res.data;
  },

  async getUserBookings(): Promise<{ bookings: Booking[] }> {
    const res = await api.get<{ bookings: Booking[] }>('/reception/bookings');
    return res.data;
  },

  async getReceptionEvents(): Promise<{ events: ReceptionEvent[] }> {
    const res = await api.get<{ events: ReceptionEvent[] }>('/reception/events');
    return res.data;
  },

  async registerReceptionEvent(eventId: string): Promise<{ registration: EventRegistration }> {
    const res = await api.post<{ registration: EventRegistration }>(
      `/reception/events/${encodeURIComponent(eventId)}/register`
    );
    return res.data;
  },

  async getUserRegistrations(): Promise<{ registrations: EventRegistration[] }> {
    const res = await api.get<{ registrations: EventRegistration[] }>('/reception/registrations');
    return res.data;
  },

  async submitReceptionOnboarding(answers: Record<string, string>): Promise<{ success: boolean }> {
    const res = await api.post<{ success: boolean }>('/reception/onboarding', { answers });
    return res.data;
  },

  async getReceptionOnboarding(): Promise<{ submission: { id: number; user_id: number; answers: Record<string, string>; created_at: string } | null }> {
    const res = await api.get<{ submission: { id: number; user_id: number; answers: Record<string, string>; created_at: string } | null }>('/reception/onboarding');
    return res.data;
  },

  // Support Module
  async askSupport(question: string, modeScope: string = 'global'): Promise<SupportAskResponse> {
    const res = await api.post<SupportAskResponse>('/support/ask', {
      question,
      mode_scope: modeScope,
    });
    return res.data;
  },

  async createSupportTicket(
    subject: string,
    message: string,
    modeScope: string = 'global'
  ): Promise<{ ticket: SupportTicket }> {
    const res = await api.post<{ ticket: SupportTicket }>('/support/tickets', {
      subject,
      message,
      mode_scope: modeScope,
    });
    return res.data;
  },

  async getSupportTickets(): Promise<{ tickets: SupportTicket[] }> {
    const res = await api.get<{ tickets: SupportTicket[] }>('/support/tickets');
    return res.data;
  },

  // Chat Module
  async getChatConfig(): Promise<ChatConfigResponse> {
    const res = await api.get<ChatConfigResponse>('/chat/config');
    return res.data;
  },

  async getChatSession(): Promise<ChatSessionResponse> {
    const res = await api.get<ChatSessionResponse>('/chat/session');
    return res.data;
  },

  async setChatSettings(roleId: number, toneId: number): Promise<ChatSettingsResponse> {
    const res = await api.post<ChatSettingsResponse>('/chat/settings', {
      role_id: roleId,
      tone_id: toneId,
    });
    return res.data;
  },

  async sendChatMessage(roleId: number, toneId: number, message: string): Promise<ChatMessageResponse> {
    const res = await api.post<ChatMessageResponse>('/chat/message', {
      role_id: roleId,
      tone_id: toneId,
      message,
    }, {
      timeout: 90000,
    });
    return res.data;
  },

  async clearChatSession(): Promise<{ success: boolean }> {
    const res = await api.post<{ success: boolean }>('/chat/clear');
    return res.data;
  },

  // Admin Endpoints
  async getAdminModes(): Promise<{ modes: ModeItem[] }> {
    const res = await api.get<{ modes: ModeItem[] }>('/admin/modes');
    return res.data;
  },

  async toggleAdminMode(
    modeId: string,
    isActive: boolean,
    maintenanceMessage: string
  ): Promise<{ success: boolean }> {
    const res = await api.post<{ success: boolean }>(`/admin/modes/${encodeURIComponent(modeId)}/toggle`, {
      is_active: isActive,
      maintenance_message: maintenanceMessage,
    });
    return res.data;
  },

  async getAdminLeads(status?: string): Promise<{ leads: Lead[] }> {
    const res = await api.get<{ leads: Lead[] }>('/admin/leads', {
      params: status ? { status } : undefined,
    });
    return res.data;
  },

  async updateAdminLeadStatus(id: number, status: string): Promise<{ success: boolean }> {
    const res = await api.post<{ success: boolean }>(`/admin/leads/${id}/status`, { status });
    return res.data;
  },

  async getAdminTickets(status?: string): Promise<{ tickets: SupportTicket[] }> {
    const res = await api.get<{ tickets: SupportTicket[] }>('/admin/tickets', {
      params: status ? { status } : undefined,
    });
    return res.data;
  },

  async replyAdminTicket(id: number, message: string): Promise<{ success: boolean }> {
    const res = await api.post<{ success: boolean }>(`/admin/tickets/${id}/reply`, { message });
    return res.data;
  },

  async getAdminAuditLogs(limit?: number): Promise<{ audit_logs: AdminAuditLog[] }> {
    const res = await api.get<{ audit_logs: AdminAuditLog[] }>('/admin/audit-logs', {
      params: limit ? { limit } : undefined,
    });
    return res.data;
  },
};
