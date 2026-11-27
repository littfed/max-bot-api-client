package posts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

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

// UpdateBrief updates the draft's goal, tone, and target platforms and moves it to brief status.
func (s *Service) UpdateBrief(ctx context.Context, userID, draftID int64, goal, tone string, platforms []string) (*PostDraft, error) {
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
func (s *Service) GenerateHooks(ctx context.Context, userID, draftID int64) (*PostDraft, error) {
	if s.repo == nil || s.pipeline == nil {
		return nil, errors.New("posts: service dependencies not configured")
	}

	draft, err := s.repo.GetDraft(ctx, userID, draftID)
	if err != nil {
		return nil, err
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
func (s *Service) GeneratePackage(ctx context.Context, userID, draftID int64, hookID string) (*PostDraft, error) {
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

	prof := s.loadProfileForDraft(ctx, userID, draft)
	specs := resolvePlatformSpecs(draft.TargetPlatforms)

	variants, imagePrompt, err := s.pipeline.GeneratePackage(ctx, draft, prof, specs)
	if err != nil {
		return nil, err
	}

	draft.Variants = variants
	draft.ImagePrompt = imagePrompt

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
		}
	}

	draft.Status = DraftStatusReady
	if err := s.repo.UpdateDraft(ctx, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

// Refine updates either a single platform variant or all variants according to the user's instruction.
func (s *Service) Refine(ctx context.Context, userID, draftID int64, platform, instruction string) (*PostDraft, error) {
	if s.repo == nil || s.pipeline == nil {
		return nil, errors.New("posts: service dependencies not configured")
	}

	draft, err := s.repo.GetDraft(ctx, userID, draftID)
	if err != nil {
		return nil, err
	}

	prof := s.loadProfileForDraft(ctx, userID, draft)
	specs := resolvePlatformSpecs(draft.TargetPlatforms)

	variants, err := s.pipeline.RefineVariant(ctx, draft, prof, specs, platform, instruction)
	if err != nil {
		return nil, err
	}

	draft.Variants = variants
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
		return nil, errors.New("posts: platform is required")
	}
	v.Platform = target

	updated := false
	for i := range draft.Variants {
		if strings.ToLower(strings.TrimSpace(draft.Variants[i].Platform)) == target {
			draft.Variants[i] = v
			updated = true
			break
		}
	}
	if !updated {
		draft.Variants = append(draft.Variants, v)
	}

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
		return nil, errors.New("posts: image prompt cannot be empty")
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
