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
  variants: PostVariant[];
  created_at: string;
  updated_at: string;
}

export interface StudioState {
  active_draft?: PostDraft | null;
  drafts: PostDraft[];
  profiles: BrandProfile[];
}
