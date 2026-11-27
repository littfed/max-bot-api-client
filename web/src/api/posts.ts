import { api } from './client';
import {
  BrandProfile,
  ModelsCatalog,
  PostDraft,
  PostVariant,
  StudioState,
} from '../types/posts';

const modulesConfig = {};
const aiModulesConfig = { timeout: 90000 };

export const postsApi = {
  async getModels(): Promise<ModelsCatalog> {
    const res = await api.get<ModelsCatalog>('/posts/models', modulesConfig);
    return res.data;
  },

  async getState(draftId?: number): Promise<StudioState> {
    const res = await api.get<StudioState>('/posts/state', {
      ...modulesConfig,
      params: draftId ? { draft_id: draftId } : undefined,
    });
    return res.data;
  },

  async saveProfile(profile: Partial<BrandProfile>): Promise<BrandProfile> {
    const res = await api.post<{ profile: BrandProfile }>(
      '/posts/profile',
      profile,
      modulesConfig
    );
    return res.data.profile;
  },

  async createDraft(rawIdea: string, sourceImageUrl?: string): Promise<PostDraft> {
    const res = await api.post<{ draft: PostDraft }>(
      '/posts/drafts',
      {
        raw_idea: rawIdea,
        source_image_url: sourceImageUrl || undefined,
      },
      aiModulesConfig
    );
    return res.data.draft;
  },

  async updateBrief(
    draftId: number,
    goal: string,
    tone: string,
    targetPlatforms: string[],
    textModel?: string
  ): Promise<PostDraft> {
    const res = await api.patch<{ draft: PostDraft }>(
      `/posts/drafts/${draftId}/brief`,
      {
        goal,
        tone,
        target_platforms: targetPlatforms,
        text_model: textModel || undefined,
      },
      modulesConfig
    );
    return res.data.draft;
  },

  async generateHooks(draftId: number, textModel?: string): Promise<PostDraft> {
    const res = await api.post<{ draft: PostDraft }>(
      `/posts/drafts/${draftId}/hooks`,
      textModel ? { text_model: textModel } : {},
      aiModulesConfig
    );
    return res.data.draft;
  },

  async generatePackage(
    draftId: number,
    hookId: string,
    textModel?: string
  ): Promise<PostDraft> {
    const res = await api.post<{ draft: PostDraft }>(
      `/posts/drafts/${draftId}/generate`,
      {
        hook_id: hookId,
        text_model: textModel || undefined,
      },
      aiModulesConfig
    );
    return res.data.draft;
  },

  async refineDraft(
    draftId: number,
    platform: string,
    instruction: string,
    textModel?: string
  ): Promise<PostDraft> {
    const res = await api.post<{ draft: PostDraft }>(
      `/posts/drafts/${draftId}/refine`,
      {
        platform,
        instruction,
        text_model: textModel || undefined,
      },
      aiModulesConfig
    );
    return res.data.draft;
  },

  async updateVariant(
    draftId: number,
    platform: string,
    variant: PostVariant
  ): Promise<PostDraft> {
    const res = await api.patch<{ draft: PostDraft }>(
      `/posts/drafts/${draftId}/variants/${encodeURIComponent(platform)}`,
      variant,
      modulesConfig
    );
    return res.data.draft;
  },

  async regenerateImage(draftId: number, customPrompt?: string): Promise<PostDraft> {
    const res = await api.post<{ draft: PostDraft }>(
      `/posts/drafts/${draftId}/image`,
      customPrompt ? { prompt: customPrompt } : {},
      aiModulesConfig
    );
    return res.data.draft;
  },

  async restoreRevision(
    draftId: number,
    type: 'image' | 'text',
    version: number,
    platform?: string
  ): Promise<PostDraft> {
    const res = await api.post<{ draft: PostDraft }>(
      `/posts/drafts/${draftId}/restore`,
      {
        type,
        version,
        platform: platform || undefined,
      },
      modulesConfig
    );
    return res.data.draft;
  },
};
