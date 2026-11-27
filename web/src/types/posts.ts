export interface BrandProfile {
  id: number;
  user_id: number;
  name: string;
  niche: string;
  target_audience: string;
  tone_of_voice: string;
  forbidden_words: string;
  default_platforms: string[];
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

export interface PostHook {
  id: string;
  title: string;
  text: string;
  angle: string;
}

export interface PostVariant {
  platform: string;
  text: string;
  hashtags?: string[];
  cta?: string;
}

export interface ImageRevision {
  version: number;
  url: string;
  prompt?: string;
  created_at: string;
}

export interface TextRevision {
  version: number;
  platform: string;
  text: string;
  hashtags?: string[];
  cta?: string;
  source: 'generate' | 'refine' | 'manual' | 'restore' | string;
  instruction?: string;
  model?: string;
  created_at: string;
}

export interface ModelOption {
  id: string;
  label: string;
  tier: 'auto' | 'balanced' | 'pro' | 'other' | string;
  description?: string;
}

export interface ModelsCatalog {
  default_model: string;
  curated: ModelOption[];
  all: string[];
}

export interface PostDraft {
  id: number;
  user_id: number;
  profile_id?: number | null;
  status: 'idea' | 'brief' | 'hooks' | 'generating' | 'ready' | 'archived' | string;
  raw_idea: string;
  source_image_url?: string;
  vision_summary?: string;
  goal?: string;
  tone?: string;
  target_platforms: string[];
  hooks: PostHook[];
  selected_hook_id?: string;
  image_prompt?: string;
  generated_image_url?: string;
  text_model?: string;
  image_history?: ImageRevision[];
  text_history?: TextRevision[];
  variants: PostVariant[];
  created_at: string;
  updated_at: string;
}

export interface StudioState {
  active_draft?: PostDraft | null;
  drafts: PostDraft[];
  profiles: BrandProfile[];
}
