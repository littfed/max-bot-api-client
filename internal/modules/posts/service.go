package posts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ErrInvalidInput is returned when caller-supplied arguments fail validation.
var ErrInvalidInput = errors.New("posts: invalid input")

const (
	defaultTextModel = "auto"
	maxImageHistory  = 20
	maxTextHistory   = 60
)

var curatedTextModels = []ModelOption{
	{ID: "auto", Label: "Auto (Smart Router)", Tier: "auto", Description: "Автоматический выбор оптимальной модели"},
	{ID: "gemini-3.8-flash", Label: "Gemini 3.8 Flash", Tier: "balanced", Description: "Быстрая и сбалансированная модель Google"},
	{ID: "deepseek-4-1-flash", Label: "DeepSeek V4.1 Flash", Tier: "balanced", Description: "Быстрая модель DeepSeek для текстов"},
	{ID: "gpt-6-luna", Label: "GPT-6 Luna", Tier: "balanced", Description: "Универсальная модель нового поколения"},
	{ID: "claude-sonnet-5-5", Label: "Claude Sonnet 5.5", Tier: "balanced", Description: "Выразительный копирайтинг и структура"},
	{ID: "gpt-5.4", Label: "GPT-5.4", Tier: "balanced", Description: "Универсальная модель OpenAI"},
	{ID: "deepseek-4-pro", Label: "DeepSeek 4 Pro", Tier: "pro", Description: "Глубокая логика и аналитика"},
	{ID: "qwen3.6-plus", Label: "Qwen 3.6 Plus", Tier: "balanced", Description: "Сильный мультиязычный текст"},
	{ID: "claude-opus-5-5", Label: "Claude Opus 5.5", Tier: "pro", Description: "Максимальное качество и нюансы стиля"},
}

// Service orchestrates Content Studio workflows across persistence and the AI pipeline.
type Service struct {
	repo     Repository
	pipeline *Pipeline
}

// NewService creates a new Content Studio Service.
func NewService(repo Repository, pipeline *Pipeline) *Service {
	return &Service{
		repo:     repo,
		pipeline: pipeline,
	}
}

// GetState returns the user's drafts, brand profiles, and optionally the active draft.
func (s *Service) GetState(ctx context.Context, userID int64, activeDraftID int64) (*StudioState, error) {
	if s.repo == nil {
		return nil, errors.New("posts: repository is nil")
	}

	var activeDraft *PostDraft
	if activeDraftID > 0 {
		d, err := s.repo.GetDraft(ctx, userID, activeDraftID)
		if err != nil {
			return nil, err
		}
		activeDraft = d
	}

	drafts, err := s.repo.ListDrafts(ctx, userID)
	if err != nil {
		return nil, err
	}
	profiles, err := s.repo.ListProfiles(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &StudioState{
		ActiveDraft: activeDraft,
		Drafts:      drafts,
		Profiles:    profiles,
	}, nil
}

// SaveProfile creates or updates a user's brand profile with normalized default platforms.
func (s *Service) SaveProfile(ctx context.Context, userID int64, p *BrandProfile) (*BrandProfile, error) {
	if s.repo == nil {
		return nil, errors.New("posts: repository is nil")
	}
	if p == nil {
		return nil, errors.New("posts: profile cannot be nil")
	}

	p.UserID = userID
	p.DefaultPlatforms = NormalizePlatforms(p.DefaultPlatforms)
	if err := s.repo.UpsertProfile(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// StartDraft initializes a new post draft from a raw idea and/or source image URL.
func (s *Service) StartDraft(ctx context.Context, userID int64, rawIdea, sourceImageURL string) (*PostDraft, error) {
	if s.repo == nil {
		return nil, errors.New("posts: repository is nil")
	}

	idea := strings.TrimSpace(rawIdea)
	imgURL := strings.TrimSpace(sourceImageURL)
	if idea == "" && imgURL == "" {
		return nil, errors.New("posts: rawIdea or sourceImageURL is required")
	}

	draft := &PostDraft{
		UserID:         userID,
		Status:         DraftStatusIdea,
		RawIdea:        idea,
		SourceImageURL: imgURL,
	}

	prof, err := s.repo.GetDefaultProfile(ctx, userID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if prof != nil {
		profID := prof.ID
		draft.ProfileID = &profID
		draft.Tone = prof.ToneOfVoice
		draft.TargetPlatforms = NormalizePlatforms(prof.DefaultPlatforms)
	} else {
		draft.TargetPlatforms = NormalizePlatforms(nil)
	}

	if imgURL != "" {
		if s.pipeline == nil {
			if idea == "" {
				return nil, errors.New("posts: pipeline is nil")
			}
			slog.Warn("posts: pipeline is nil, skipping vision analysis", "user_id", userID)
		} else {
			summary, err := s.pipeline.AnalyzeImage(ctx, imgURL)
			if err != nil {
				slog.Warn("posts: vision analysis failed, continuing with raw idea", "user_id", userID, "error", err)
				if idea == "" {
					return nil, fmt.Errorf("posts: vision analysis failed: %w", err)
				}
			} else {
				draft.VisionSummary = summary
			}
		}
	}

	if err := s.repo.CreateDraft(ctx, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

// ListAvailableModels returns curated and full text model lists from the AI gateway,
// falling back to the curated catalog if the gateway is unavailable.
func (s *Service) ListAvailableModels(ctx context.Context) (*ModelsCatalog, error) {
	catalog := &ModelsCatalog{
		DefaultModel: defaultTextModel,
	}

	var rawModels []string
	var err error
	if s.pipeline != nil && s.pipeline.gw != nil {
		rawModels, err = s.pipeline.ListModels(ctx)
	} else {
		err = errors.New("posts: pipeline not configured")
	}

	if err != nil || len(rawModels) == 0 {
		catalog.Curated = append([]ModelOption(nil), curatedTextModels...)
		catalog.All = make([]string, 0, len(curatedTextModels))
		for _, opt := range curatedTextModels {
			catalog.All = append(catalog.All, opt.ID)
		}
		return catalog, nil
	}

	availSet := make(map[string]struct{}, len(rawModels))
	allText := make([]string, 0, len(rawModels))
	for _, id := range rawModels {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		availSet[trimmed] = struct{}{}
		if isTextModel(trimmed) {
			allText = append(allText, trimmed)
		}
	}

	curated := make([]ModelOption, 0, len(curatedTextModels))
	for _, opt := range curatedTextModels {
		if _, ok := availSet[opt.ID]; ok {
			curated = append(curated, opt)
		}
	}
	catalog.Curated = curated
	catalog.All = allText
	return catalog, nil
}

// UpdateBrief updates the draft's goal, tone, optional text model, and target platforms and moves it to brief status.
func (s *Service) UpdateBrief(ctx context.Context, userID, draftID int64, goal, tone, textModel string, platforms []string) (*PostDraft, error) {
	if s.repo == nil {
		return nil, errors.New("posts: repository is nil")
	}

	draft, err := s.repo.GetDraft(ctx, userID, draftID)
	if err != nil {
		return nil, err
	}

	draft.Goal = strings.TrimSpace(goal)
	if trimmedTone := strings.TrimSpace(tone); trimmedTone != "" {
		draft.Tone = trimmedTone
	}
	if trimmedModel := strings.TrimSpace(textModel); trimmedModel != "" {
		draft.TextModel = trimmedModel
	}
	if len(platforms) > 0 || len(draft.TargetPlatforms) == 0 {
		draft.TargetPlatforms = NormalizePlatforms(platforms)
	}
	draft.Status = DraftStatusBrief

	if err := s.repo.UpdateDraft(ctx, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

// GenerateHooks generates hook options for a draft using the AI pipeline and persists them.
func (s *Service) GenerateHooks(ctx context.Context, userID, draftID int64, textModel string) (*PostDraft, error) {
	if s.repo == nil || s.pipeline == nil {
		return nil, errors.New("posts: service dependencies not configured")
	}

	draft, err := s.repo.GetDraft(ctx, userID, draftID)
	if err != nil {
		return nil, err
	}

	if trimmedModel := strings.TrimSpace(textModel); trimmedModel != "" {
		draft.TextModel = trimmedModel
	}

	prof := s.loadProfileForDraft(ctx, userID, draft)
	hooks, err := s.pipeline.GenerateHooks(ctx, draft, prof)
	if err != nil {
		return nil, err
	}

	draft.Hooks = hooks
	draft.Status = DraftStatusHooks
	if err := s.repo.UpdateDraft(ctx, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

// GeneratePackage generates platform-adapted post variants and an optional visual illustration.
func (s *Service) GeneratePackage(ctx context.Context, userID, draftID int64, hookID, textModel string) (*PostDraft, error) {
	if s.repo == nil || s.pipeline == nil {
		return nil, errors.New("posts: service dependencies not configured")
	}

	draft, err := s.repo.GetDraft(ctx, userID, draftID)
	if err != nil {
		return nil, err
	}

	if trimmedHook := strings.TrimSpace(hookID); trimmedHook != "" {
		draft.SelectedHookID = trimmedHook
	}
	if trimmedModel := strings.TrimSpace(textModel); trimmedModel != "" {
		draft.TextModel = trimmedModel
	}

	prof := s.loadProfileForDraft(ctx, userID, draft)
	specs := resolvePlatformSpecs(draft.TargetPlatforms)

	variants, imagePrompt, err := s.pipeline.GeneratePackage(ctx, draft, prof, specs)
	if err != nil {
		return nil, err
	}

	draft.Variants = variants
	draft.ImagePrompt = imagePrompt
	for _, v := range variants {
		appendTextRevision(draft, v, "generate", "", draft.TextModel)
	}

	if strings.TrimSpace(draft.SourceImageURL) == "" && imagePrompt != "" {
		aspectRatio := specs[0].AspectRatio
		if aspectRatio == "" {
			aspectRatio = "16:9"
		}
		imgURL, imgErr := s.pipeline.GenerateVisual(ctx, imagePrompt, aspectRatio)
		if imgErr != nil {
			slog.Warn("posts: visual generation failed, saving text variants without image",
				"user_id", userID, "draft_id", draftID, "error", imgErr)
		} else {
			draft.GeneratedImageURL = imgURL
			appendImageRevision(draft, imgURL, imagePrompt)
		}
	}

	draft.Status = DraftStatusReady
	if err := s.repo.UpdateDraft(ctx, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

// Refine updates either a single platform variant or all variants according to the user's instruction.
func (s *Service) Refine(ctx context.Context, userID, draftID int64, platform, instruction, textModel string) (*PostDraft, error) {
	if s.repo == nil || s.pipeline == nil {
		return nil, errors.New("posts: service dependencies not configured")
	}

	draft, err := s.repo.GetDraft(ctx, userID, draftID)
	if err != nil {
		return nil, err
	}

	if trimmedModel := strings.TrimSpace(textModel); trimmedModel != "" {
		draft.TextModel = trimmedModel
	}

	prof := s.loadProfileForDraft(ctx, userID, draft)
	specs := resolvePlatformSpecs(draft.TargetPlatforms)

	variants, err := s.pipeline.RefineVariant(ctx, draft, prof, specs, platform, instruction)
	if err != nil {
		return nil, err
	}

	draft.Variants = variants
	target := strings.ToLower(strings.TrimSpace(platform))
	for _, v := range variants {
		if target != "" && target != "all" && strings.ToLower(strings.TrimSpace(v.Platform)) != target {
			continue
		}
		appendTextRevision(draft, v, "refine", instruction, draft.TextModel)
	}

	draft.Status = DraftStatusReady
	if err := s.repo.UpdateDraft(ctx, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

// UpdateVariantManual replaces or appends a single platform's variant with user-edited content.
func (s *Service) UpdateVariantManual(ctx context.Context, userID, draftID int64, platform string, v PostVariant) (*PostDraft, error) {
	if s.repo == nil {
		return nil, errors.New("posts: repository is nil")
	}

	draft, err := s.repo.GetDraft(ctx, userID, draftID)
	if err != nil {
		return nil, err
	}

	target := strings.ToLower(strings.TrimSpace(platform))
	if target == "" {
		target = strings.ToLower(strings.TrimSpace(v.Platform))
	}
	if target == "" {
		return nil, fmt.Errorf("%w: platform is required", ErrInvalidInput)
	}
	v.Platform = target

	draft.Variants = upsertVariant(draft.Variants, v)
	appendTextRevision(draft, v, "manual", "", "")

	if err := s.repo.UpdateDraft(ctx, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

// RegenerateImage generates a new illustration for a draft using customPrompt or the draft's ImagePrompt.
func (s *Service) RegenerateImage(ctx context.Context, userID, draftID int64, customPrompt string) (*PostDraft, error) {
	if s.repo == nil || s.pipeline == nil {
		return nil, errors.New("posts: service dependencies not configured")
	}

	draft, err := s.repo.GetDraft(ctx, userID, draftID)
	if err != nil {
		return nil, err
	}

	prompt := strings.TrimSpace(customPrompt)
	if prompt == "" {
		prompt = strings.TrimSpace(draft.ImagePrompt)
	}
	if prompt == "" {
		return nil, fmt.Errorf("%w: image prompt cannot be empty", ErrInvalidInput)
	}

	specs := resolvePlatformSpecs(draft.TargetPlatforms)
	aspectRatio := specs[0].AspectRatio
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}

	imgURL, err := s.pipeline.GenerateVisual(ctx, prompt, aspectRatio)
	if err != nil {
		return nil, err
	}

	draft.ImagePrompt = prompt
	draft.GeneratedImageURL = imgURL
	appendImageRevision(draft, imgURL, prompt)

	if err := s.repo.UpdateDraft(ctx, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

// RestoreRevision restores a historical image or text revision on a draft.
func (s *Service) RestoreRevision(ctx context.Context, userID, draftID int64, kind string, version int, platform string) (*PostDraft, error) {
	if s.repo == nil {
		return nil, errors.New("posts: repository is nil")
	}

	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	if normalizedKind != "image" && normalizedKind != "text" {
		return nil, fmt.Errorf("%w: type must be 'image' or 'text'", ErrInvalidInput)
	}
	if version <= 0 {
		return nil, fmt.Errorf("%w: version must be positive", ErrInvalidInput)
	}

	draft, err := s.repo.GetDraft(ctx, userID, draftID)
	if err != nil {
		return nil, err
	}

	switch normalizedKind {
	case "image":
		var found *ImageRevision
		for i := range draft.ImageHistory {
			if draft.ImageHistory[i].Version == version {
				found = &draft.ImageHistory[i]
				break
			}
		}
		if found == nil {
			return nil, fmt.Errorf("%w: image revision %d", ErrNotFound, version)
		}
		draft.GeneratedImageURL = found.URL
		if found.Prompt != "" {
			draft.ImagePrompt = found.Prompt
		}

	case "text":
		targetPlatform := strings.ToLower(strings.TrimSpace(platform))
		var found *TextRevision
		for i := range draft.TextHistory {
			if draft.TextHistory[i].Version != version {
				continue
			}
			if targetPlatform != "" && strings.ToLower(strings.TrimSpace(draft.TextHistory[i].Platform)) != targetPlatform {
				continue
			}
			found = &draft.TextHistory[i]
			break
		}
		if found == nil {
			return nil, fmt.Errorf("%w: text revision %d", ErrNotFound, version)
		}
		restoredVariant := PostVariant{
			Platform: found.Platform,
			Text:     found.Text,
			Hashtags: append([]string(nil), found.Hashtags...),
			CTA:      found.CTA,
		}
		draft.Variants = upsertVariant(draft.Variants, restoredVariant)
		appendTextRevision(draft, restoredVariant, "restore", fmt.Sprintf("restore v%d", version), found.Model)
	}

	if err := s.repo.UpdateDraft(ctx, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

func (s *Service) loadProfileForDraft(ctx context.Context, userID int64, draft *PostDraft) *BrandProfile {
	if draft != nil && draft.ProfileID != nil {
		profiles, err := s.repo.ListProfiles(ctx, userID)
		if err == nil {
			for i := range profiles {
				if profiles[i].ID == *draft.ProfileID {
					p := profiles[i]
					return &p
				}
			}
		}
	}
	prof, err := s.repo.GetDefaultProfile(ctx, userID)
	if err != nil {
		return nil
	}
	return prof
}

func resolvePlatformSpecs(platformIDs []string) []PlatformSpec {
	normalized := NormalizePlatforms(platformIDs)
	specs := make([]PlatformSpec, 0, len(normalized))
	for _, id := range normalized {
		if spec, ok := GetPlatformSpec(id); ok {
			specs = append(specs, spec)
		}
	}
	return specs
}

func appendImageRevision(draft *PostDraft, url, prompt string) {
	nextVer := 1
	if n := len(draft.ImageHistory); n > 0 {
		nextVer = draft.ImageHistory[n-1].Version + 1
	}
	draft.ImageHistory = append(draft.ImageHistory, ImageRevision{
		Version:   nextVer,
		URL:       url,
		Prompt:    prompt,
		CreatedAt: time.Now().UTC(),
	})
	if len(draft.ImageHistory) > maxImageHistory {
		draft.ImageHistory = draft.ImageHistory[len(draft.ImageHistory)-maxImageHistory:]
	}
}

func appendTextRevision(draft *PostDraft, v PostVariant, source, instruction, model string) {
	nextVer := 1
	if n := len(draft.TextHistory); n > 0 {
		nextVer = draft.TextHistory[n-1].Version + 1
	}
	draft.TextHistory = append(draft.TextHistory, TextRevision{
		Version:     nextVer,
		Platform:    v.Platform,
		Text:        v.Text,
		Hashtags:    append([]string(nil), v.Hashtags...),
		CTA:         v.CTA,
		Source:      source,
		Instruction: instruction,
		Model:       model,
		CreatedAt:   time.Now().UTC(),
	})
	if len(draft.TextHistory) > maxTextHistory {
		draft.TextHistory = draft.TextHistory[len(draft.TextHistory)-maxTextHistory:]
	}
}

func upsertVariant(variants []PostVariant, v PostVariant) []PostVariant {
	target := strings.ToLower(strings.TrimSpace(v.Platform))
	for i := range variants {
		if strings.ToLower(strings.TrimSpace(variants[i].Platform)) == target {
			variants[i] = v
			return variants
		}
	}
	return append(variants, v)
}

func isTextModel(id string) bool {
	lower := strings.ToLower(strings.TrimSpace(id))
	if lower == "" {
		return false
	}
	excluded := []string{
		"flux", "dall-e", "imagen", "midjourney", "stable-diffusion",
		"sdxl", "sd-", "whisper", "tts", "embedding", "rerank", "moderation",
		"-image", "image-",
	}
	for _, pat := range excluded {
		if strings.Contains(lower, pat) {
			return false
		}
	}
	return true
}
