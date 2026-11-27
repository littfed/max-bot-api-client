import { api } from './client';
import {
  ImageMode,
  ImageModelOption,
  ImagePreset,
  ImageProject,
} from '../types/images';

const aiRequestConfig = { timeout: 90000 };

export interface CreateImageProjectInput {
  mode: ImageMode;
  preset_id?: string;
  prompt: string;
  aspect_ratio?: string;
  image_model?: string;
  use_example_source?: boolean;
  sourceFile?: File | null;
}

export const imagesApi = {
  async getPresets(): Promise<ImagePreset[]> {
    const res = await api.get<{ presets: ImagePreset[] }>('/images/presets');
    return res.data.presets || [];
  },

  async getModels(): Promise<ImageModelOption[]> {
    const res = await api.get<{ models: ImageModelOption[] }>('/images/models');
    return res.data.models || [];
  },

  async listProjects(): Promise<ImageProject[]> {
    const res = await api.get<{ projects: ImageProject[] }>('/images/projects');
    return res.data.projects || [];
  },

  async getProject(id: number): Promise<ImageProject> {
    const res = await api.get<ImageProject>(`/images/projects/${id}`);
    return res.data;
  },

  async createProject(input: CreateImageProjectInput): Promise<ImageProject> {
    if (input.sourceFile) {
      const formData = new FormData();
      formData.append('mode', input.mode);
      if (input.preset_id) formData.append('preset_id', input.preset_id);
      formData.append('prompt', input.prompt);
      if (input.aspect_ratio) formData.append('aspect_ratio', input.aspect_ratio);
      if (input.image_model) formData.append('image_model', input.image_model);
      if (input.use_example_source) formData.append('use_example_source', 'true');
      formData.append('source_image', input.sourceFile);

      const res = await api.post<ImageProject>('/images/projects', formData, {
        ...aiRequestConfig,
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      return res.data;
    }

    const res = await api.post<ImageProject>(
      '/images/projects',
      {
        mode: input.mode,
        preset_id: input.preset_id || undefined,
        prompt: input.prompt,
        aspect_ratio: input.aspect_ratio || undefined,
        image_model: input.image_model || undefined,
        use_example_source: input.use_example_source || undefined,
      },
      aiRequestConfig
    );
    return res.data;
  },

  async refineProject(
    id: number,
    prompt: string,
    imageModel?: string
  ): Promise<ImageProject> {
    const res = await api.post<ImageProject>(
      `/images/projects/${id}/refine`,
      {
        prompt,
        image_model: imageModel || undefined,
      },
      aiRequestConfig
    );
    return res.data;
  },

  async restoreProject(id: number, version: number): Promise<ImageProject> {
    const res = await api.post<ImageProject>(`/images/projects/${id}/restore`, {
      version,
    });
    return res.data;
  },

  async deleteProject(id: number): Promise<{ deleted: boolean; id: number }> {
    const res = await api.delete<{ deleted: boolean; id: number }>(
      `/images/projects/${id}`
    );
    return res.data;
  },
};
