export type ImageMode = 'generate' | 'edit';

export type ImageProjectStatus = 'draft' | 'processing' | 'ready' | 'failed';

export type ImageActionType = 'generate' | 'edit' | 'refine' | 'restore';

export interface ImagePreset {
  id: string;
  mode: ImageMode;
  title: string;
  description: string;
  prompt_template: string;
  default_aspect_ratio: string;
  example_prompt: string;
  example_before_url?: string;
  example_after_url: string;
}

export interface ImageRevision {
  id: number;
  project_id: number;
  version: number;
  prompt: string;
  action_type: ImageActionType;
  image_key: string;
  image_url: string;
  model: string;
  pipeline_used: string;
  created_at: string;
}

export interface ImageProject {
  id: number;
  user_id: number;
  mode: ImageMode;
  preset_id?: string;
  title: string;
  source_image_key?: string;
  source_image_url?: string;
  current_image_key?: string;
  current_image_url?: string;
  current_version: number;
  image_model: string;
  aspect_ratio: string;
  status: ImageProjectStatus;
  error_message?: string;
  created_at: string;
  updated_at: string;
  revisions?: ImageRevision[];
}

export interface ImageModelOption {
  id: string;
  label: string;
  description?: string;
}
