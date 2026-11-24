export interface User {
  id: number;
  active_mode_id: string;
  role: 'user' | 'admin' | 'superadmin' | 'support_operator';
  timezone: string;
  created_at: string;
  updated_at: string;
}

export interface ModeItem {
  id: string;
  title: string;
  is_active: boolean;
  maintenance_message?: string;
  daily_quota?: number;
}

export interface MascotState {
  phrase: string;
  priority: number;
  emotion: 'friendly' | 'excited' | 'alert' | 'happy';
}

export interface Capabilities {
  can_switch_mode: boolean;
  has_admin_access: boolean;
  available_modes_count: number;
  total_modes_count: number;
}

export interface BootstrapResponse {
  user: User;
  token: string;
  active_mode: string;
  modes: ModeItem[];
  mascot: MascotState;
  capabilities: Capabilities;
}

// Shop
export interface Product {
  id: string;
  title: string;
  description: string;
  price: number;
  category: string;
  item_type: 'digital' | 'subscription';
  content_payload: string;
  is_active: boolean;
}

export interface ShopOrder {
  id: string;
  user_id: number;
  product_id: string;
  status: 'pending' | 'paid' | 'abandoned';
  qr_code: string;
  created_at: string;
}

export interface PaymentSimulationResult {
  order_id: string;
  status: 'paid' | 'abandoned';
  message: string;
  delivery_payload?: string;
  reminder_scheduled?: boolean;
}

// Reception
export interface Service {
  id: string;
  title: string;
  duration_minutes: number;
  price: number;
  description: string;
}

export interface TimeSlot {
  id: string;
  service_id: string;
  slot_time: string;
  is_booked: boolean;
  booked_by_user_id?: number;
}

export interface ReceptionEvent {
  id: string;
  title: string;
  event_time: string;
  total_seats: number;
  booked_seats: number;
}

export interface EventRegistration {
  registration_id?: string;
  id?: string;
  event_id: string;
  user_id: number;
  ticket_qr: string;
  registered_at?: string;
}

// Support
export interface SupportAskResponse {
  answer: string;
  score: number;
  can_escalate: boolean;
}

export interface SupportTicket {
  id: number;
  user_id: number;
  platform: string;
  mode_scope: string;
  subject: string;
  message?: string;
  status: 'open' | 'answered' | 'closed';
  created_at: string;
  updated_at?: string;
}

// Chat
export interface ChatRole {
  id: number;
  name: string;
  description: string;
  system_prompt?: string;
}

export interface ChatTone {
  id: number;
  name: string;
  description: string;
  instruction?: string;
}

export interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
  created_at?: string;
}

export interface ChatConfigResponse {
  roles: ChatRole[];
  tones: ChatTone[];
}

export interface ChatMessageResponse {
  reply: string;
}

// Admin
export interface Lead {
  id: number;
  user_id: number;
  platform: string;
  source_mode: string;
  tested_modes: string[] | any;
  contact: string;
  comment: string;
  status: 'new' | 'in_progress' | 'closed';
  created_at: string;
}

export interface AdminAuditLog {
  id: number;
  admin_id: number;
  action: string;
  target_mode?: string;
  payload_diff?: any;
  ip?: string;
  created_at: string;
}

export interface ToastMessage {
  id: string;
  type: 'info' | 'success' | 'warning' | 'error';
  text: string;
}
