package posts

import "time"

// FSM step constants for the Content Studio module.
const (
	StepIdle        = "idle"
	StepIdeaInput   = "idea_input"
	StepBriefSetup  = "brief_setup"
	StepHooksSelect = "hooks_select"
	StepGenerating  = "generating"
	StepReady       = "ready"
)

// Draft status constants for post_drafts.
const (
	DraftStatusIdea       = "idea"
	DraftStatusBrief      = "brief"
	DraftStatusHooks      = "hooks"
	DraftStatusGenerating = "generating"
	DraftStatusReady      = "ready"
	DraftStatusArchived   = "archived"
)

// BrandProfile stores a user's brand voice and default publishing settings.
type BrandProfile struct {
	ID               int64     `json:"id"`
	UserID           int64     `json:"user_id"`
	Name             string    `json:"name"`
	Niche            string    `json:"niche"`
	TargetAudience   string    `json:"target_audience"`
	ToneOfVoice      string    `json:"tone_of_voice"`
	ForbiddenWords   string    `json:"forbidden_words"`
	DefaultPlatforms []string  `json:"default_platforms"`
	IsDefault        bool      `json:"is_default"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// PostHook represents a generated hook option for a post draft.
type PostHook struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
	Angle string `json:"angle"`
}

// PostVariant represents a platform-adapted generated post variant.
type PostVariant struct {
	Platform string   `json:"platform"`
	Text     string   `json:"text"`
	Hashtags []string `json:"hashtags,omitempty"`
	CTA      string   `json:"cta,omitempty"`
}

// ImageRevision records a generated or uploaded visual version for a draft.
type ImageRevision struct {
	Version   int       `json:"version"`
	URL       string    `json:"url"`
	Prompt    string    `json:"prompt,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// TextRevision records a historical snapshot of a platform variant.
type TextRevision struct {
	Version     int       `json:"version"`
	Platform    string    `json:"platform"`
	Text        string    `json:"text"`
	Hashtags    []string  `json:"hashtags,omitempty"`
	CTA         string    `json:"cta,omitempty"`
	Source      string    `json:"source"` // "generate" | "refine" | "manual" | "restore"
	Instruction string    `json:"instruction,omitempty"`
	Model       string    `json:"model,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ModelOption describes an available text model from GMA.
type ModelOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Tier        string `json:"tier"` // "auto" | "balanced" | "pro" | "other"
	Description string `json:"description,omitempty"`
}

// ModelsCatalog holds curated and full lists of text models available in GMA.
type ModelsCatalog struct {
	DefaultModel string        `json:"default_model"`
	Curated      []ModelOption `json:"curated"`
	All          []string      `json:"all"`
}

// PostDraft represents a single content studio post draft.
type PostDraft struct {
	ID                int64         `json:"id"`
	UserID            int64         `json:"user_id"`
	ProfileID         *int64        `json:"profile_id,omitempty"`
	Status            string        `json:"status"`
	RawIdea           string        `json:"raw_idea"`
	SourceImageURL    string        `json:"source_image_url,omitempty"`
	VisionSummary     string        `json:"vision_summary,omitempty"`
	Goal              string        `json:"goal,omitempty"`
	Tone              string        `json:"tone,omitempty"`
	TargetPlatforms   []string      `json:"target_platforms"`
	Hooks             []PostHook    `json:"hooks"`
	SelectedHookID    string        `json:"selected_hook_id,omitempty"`
	ImagePrompt       string          `json:"image_prompt,omitempty"`
	GeneratedImageURL string          `json:"generated_image_url,omitempty"`
	TextModel         string          `json:"text_model,omitempty"`
	ImageHistory      []ImageRevision `json:"image_history,omitempty"`
	TextHistory       []TextRevision  `json:"text_history,omitempty"`
	Variants          []PostVariant   `json:"variants"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

// FSMStatePayload is persisted in user_mode_states for the posts module.
type FSMStatePayload struct {
	ActiveDraftID int64 `json:"active_draft_id"`
}

// StudioState represents the full Content Studio state returned to clients.
type StudioState struct {
	ActiveDraft *PostDraft     `json:"active_draft,omitempty"`
	Drafts      []PostDraft    `json:"drafts"`
	Profiles    []BrandProfile `json:"profiles"`
}
