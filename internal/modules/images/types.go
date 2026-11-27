package images

import "time"

// Mode constants for image studio projects and presets.
const (
	ModeGenerate = "generate"
	ModeEdit     = "edit"
)

// Status constants for image_projects.
const (
	StatusDraft      = "draft"
	StatusProcessing = "processing"
	StatusReady      = "ready"
	StatusFailed     = "failed"
)

// Action constants for image_revisions.action_type.
const (
	ActionGenerate = "generate"
	ActionEdit     = "edit"
	ActionRefine   = "refine"
	ActionRestore  = "restore"
)

// Pipeline constants for image_revisions.pipeline_used.
const (
	PipelineGenerations    = "images_generations"
	PipelineEdits          = "images_edits"
	PipelineVisionFallback = "vision_fallback"
	PipelineRestore        = "restore"
)

// Preset describes a predefined image generation or editing scenario.
type Preset struct {
	ID                 string `json:"id"`
	Mode               string `json:"mode"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	PromptTemplate     string `json:"prompt_template"`
	DefaultAspectRatio string `json:"default_aspect_ratio"`
	ExamplePrompt      string `json:"example_prompt"`
	ExampleBeforeURL   string `json:"example_before_url,omitempty"`
	ExampleAfterURL    string `json:"example_after_url"`
}

// Project represents a row in image_projects.
type Project struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"user_id"`
	Mode            string     `json:"mode"`
	PresetID        string     `json:"preset_id,omitempty"`
	Title           string     `json:"title"`
	SourceImageKey  string     `json:"source_image_key,omitempty"`
	SourceImageURL  string     `json:"source_image_url,omitempty"`
	CurrentImageKey string     `json:"current_image_key,omitempty"`
	CurrentImageURL string     `json:"current_image_url,omitempty"`
	CurrentVersion  int        `json:"current_version"`
	ImageModel      string     `json:"image_model"`
	AspectRatio     string     `json:"aspect_ratio"`
	Status          string     `json:"status"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Revisions       []Revision `json:"revisions,omitempty"`
}

// Revision represents a row in image_revisions.
type Revision struct {
	ID           int64     `json:"id"`
	ProjectID    int64     `json:"project_id"`
	Version      int       `json:"version"`
	Prompt       string    `json:"prompt"`
	ActionType   string    `json:"action_type"`
	ImageKey     string    `json:"image_key"`
	ImageURL     string    `json:"image_url"`
	Model        string    `json:"model"`
	PipelineUsed string    `json:"pipeline_used"`
	CreatedAt    time.Time `json:"created_at"`
}

// ModelOption describes an available image model for the studio UI.
type ModelOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}
