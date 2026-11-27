package images

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"net/http"
	"strings"

	"github.com/littfed/max-bot-api-client/internal/storage"
)

// Sentinel errors returned by the images Service.
var (
	ErrForbidden     = errors.New("images: forbidden")
	ErrInvalidImage  = errors.New("images: invalid image")
	ErrInvalidInput  = errors.New("images: invalid input")
	ErrImageTooLarge = errors.New("images: image too large")
)

// MaxUploadBytes is the maximum allowed size for uploaded source images (10 MiB).
const MaxUploadBytes = 10 * 1024 * 1024

const defaultImageModel = "flux"

// ValidateImageBytes checks that data is non-empty, within MaxUploadBytes, and has a
// supported image MIME type (JPEG, PNG, or WebP).
func ValidateImageBytes(data []byte) (mimeType string, ext string, err error) {
	if len(data) == 0 {
		return "", "", ErrInvalidImage
	}
	if len(data) > MaxUploadBytes {
		return "", "", ErrImageTooLarge
	}

	detected := http.DetectContentType(data)
	switch detected {
	case "image/jpeg":
		return detected, ".jpg", nil
	case "image/png":
		return detected, ".png", nil
	case "image/webp":
		return detected, ".webp", nil
	default:
		return "", "", ErrInvalidImage
	}
}

// CreateProjectInput holds parameters for creating and running a new image project.
type CreateProjectInput struct {
	UserID           int64
	Mode             string
	PresetID         string
	Prompt           string
	AspectRatio      string
	ImageModel       string
	SourceImage      []byte
	UseExampleSource bool
}

// Service orchestrates image studio projects, revisions, AI pipeline calls, and S3 storage.
type Service struct {
	store           Store
	pipeline        *Pipeline
	storage         storage.ObjectStorage
	exampleAssetsFS fs.FS
}

// NewService creates a new image studio Service.
func NewService(store Store, pipeline *Pipeline, objStorage storage.ObjectStorage) *Service {
	return &Service{
		store:    store,
		pipeline: pipeline,
		storage:  objStorage,
	}
}

// SetExampleAssetsFS configures the filesystem used to load preset example source images.
func (s *Service) SetExampleAssetsFS(fsys fs.FS) {
	s.exampleAssetsFS = fsys
}

// SaveTempSourceImage validates and stores an uploaded chat photo for a pending edit flow.
func (s *Service) SaveTempSourceImage(ctx context.Context, userID int64, data []byte) (string, error) {
	if s.storage == nil {
		return "", errors.New("images: storage is nil")
	}
	if userID <= 0 {
		return "", ErrInvalidInput
	}
	mimeType, ext, err := ValidateImageBytes(data)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("images/%d/pending/source_%s%s", userID, newRandomHexUUID(), ext)
	if _, err := s.storage.PutObject(ctx, key, mimeType, data); err != nil {
		return "", fmt.Errorf("images: save pending source image: %w", err)
	}
	return key, nil
}

// ValidateCreateProjectInput validates input parameters for creating a new image project.
func ValidateCreateProjectInput(input CreateProjectInput) error {
	var preset Preset
	hasPreset := false
	if presetID := strings.TrimSpace(input.PresetID); presetID != "" {
		p, ok := GetPreset(presetID)
		if !ok {
			return fmt.Errorf("%w: unknown preset %q", ErrInvalidInput, presetID)
		}
		preset = p
		hasPreset = true
	}

	rawMode := strings.TrimSpace(input.Mode)
	if rawMode != "" && hasPreset && rawMode != preset.Mode {
		return fmt.Errorf("%w: mode %q does not match preset %q mode %q", ErrInvalidInput, rawMode, preset.ID, preset.Mode)
	}

	mode := rawMode
	if mode == "" && hasPreset {
		mode = preset.Mode
	}
	if mode == "" {
		mode = ModeGenerate
	}
	if mode != ModeGenerate && mode != ModeEdit {
		return fmt.Errorf("%w: unsupported mode %q", ErrInvalidInput, mode)
	}

	if strings.TrimSpace(input.Prompt) == "" {
		return fmt.Errorf("%w: prompt is required", ErrInvalidInput)
	}

	switch strings.TrimSpace(input.AspectRatio) {
	case "", "1:1", "16:9", "9:16", "4:3":
	default:
		return fmt.Errorf("%w: unsupported aspect_ratio %q", ErrInvalidInput, input.AspectRatio)
	}

	if len(strings.TrimSpace(input.ImageModel)) > 128 {
		return fmt.Errorf("%w: image_model exceeds 128 characters", ErrInvalidInput)
	}

	if mode == ModeEdit {
		switch {
		case len(input.SourceImage) > 0:
			if _, _, err := ValidateImageBytes(input.SourceImage); err != nil {
				return err
			}
		case !input.UseExampleSource:
			return fmt.Errorf("%w: source image is required for edit mode", ErrInvalidInput)
		}
	}
	return nil
}

// ListPresets returns all available studio presets.
func (s *Service) ListPresets() []Preset {
	return ListPresets()
}

// ListModels returns available image models from the AI pipeline.
func (s *Service) ListModels(ctx context.Context) ([]ModelOption, error) {
	if s.pipeline == nil {
		return append([]ModelOption(nil), defaultImageModels...), nil
	}
	return s.pipeline.ListImageModels(ctx)
}

// ListProjects returns all image projects belonging to userID.
func (s *Service) ListProjects(ctx context.Context, userID int64) ([]Project, error) {
	if s.store == nil {
		return nil, errors.New("images: store is nil")
	}
	if userID <= 0 {
		return nil, ErrInvalidInput
	}
	return s.store.ListProjectsByUser(ctx, userID)
}

// GetProject loads a project with its revisions after verifying ownership.
func (s *Service) GetProject(ctx context.Context, userID, projectID int64) (*Project, error) {
	if s.store == nil {
		return nil, errors.New("images: store is nil")
	}
	if userID <= 0 || projectID <= 0 {
		return nil, ErrInvalidInput
	}

	proj, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if proj.UserID != userID {
		return nil, ErrForbidden
	}

	revisions, err := s.store.ListRevisions(ctx, projectID)
	if err != nil {
		return nil, err
	}
	proj.Revisions = revisions
	return proj, nil
}

// CreateAndRun creates a new project, optionally uploads its source image, runs the AI
// pipeline, stores the v1 result in object storage, and records the initial revision.
func (s *Service) CreateAndRun(ctx context.Context, input CreateProjectInput) (*Project, error) {
	if err := s.ensureReady(); err != nil {
		return nil, err
	}
	if input.UserID <= 0 {
		return nil, ErrInvalidInput
	}
	if err := ValidateCreateProjectInput(input); err != nil {
		return nil, err
	}

	var preset Preset
	hasPreset := false
	presetID := strings.TrimSpace(input.PresetID)
	if presetID != "" {
		p, ok := GetPreset(presetID)
		if !ok {
			return nil, fmt.Errorf("%w: unknown preset %q", ErrInvalidInput, presetID)
		}
		preset = p
		hasPreset = true
	}

	mode := strings.TrimSpace(input.Mode)
	if mode == "" && hasPreset {
		mode = preset.Mode
	}
	if mode == "" {
		mode = ModeGenerate
	}
	if mode != ModeGenerate && mode != ModeEdit {
		return nil, fmt.Errorf("%w: unsupported mode %q", ErrInvalidInput, mode)
	}

	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("%w: prompt is required", ErrInvalidInput)
	}

	aspectRatio := strings.TrimSpace(input.AspectRatio)
	if aspectRatio == "" {
		if hasPreset && preset.DefaultAspectRatio != "" {
			aspectRatio = preset.DefaultAspectRatio
		} else {
			aspectRatio = "1:1"
		}
	}

	imageModel := strings.TrimSpace(input.ImageModel)
	if imageModel == "" {
		imageModel = defaultImageModel
	}

	var srcBytes []byte
	var srcMIME, srcExt string
	if mode == ModeEdit {
		switch {
		case len(input.SourceImage) > 0:
			var err error
			srcMIME, srcExt, err = ValidateImageBytes(input.SourceImage)
			if err != nil {
				return nil, err
			}
			srcBytes = input.SourceImage
		case input.UseExampleSource:
			if s.exampleAssetsFS != nil {
				relPath := strings.TrimPrefix(preset.ExampleBeforeURL, "/")
				if relPath == "" {
					return nil, fmt.Errorf("%w: preset %q has no example source", ErrInvalidInput, preset.ID)
				}
				data, err := fs.ReadFile(s.exampleAssetsFS, relPath)
				if err != nil {
					return nil, fmt.Errorf("images: load example source %q: %w", relPath, err)
				}
				mimeType, ext, err := ValidateImageBytes(data)
				if err != nil {
					return nil, err
				}
				srcBytes = data
				srcMIME = mimeType
				srcExt = ext
			} else {
				srcBytes = generateExampleSourcePNG()
				srcMIME = "image/png"
				srcExt = ".png"
			}
		default:
			return nil, fmt.Errorf("%w: source image is required for edit mode", ErrInvalidInput)
		}
	}

	title := buildProjectTitle(prompt, preset.Title)
	proj := &Project{
		UserID:         input.UserID,
		Mode:           mode,
		PresetID:       presetID,
		Title:          title,
		CurrentVersion: 0,
		ImageModel:     imageModel,
		AspectRatio:    aspectRatio,
		Status:         StatusProcessing,
	}
	if err := s.store.CreateProject(ctx, proj); err != nil {
		return nil, err
	}

	if mode == ModeEdit {
		srcKey := fmt.Sprintf("images/%d/%d/source_%s%s", input.UserID, proj.ID, newRandomHexUUID(), srcExt)
		srcURL, err := s.storage.PutObject(ctx, srcKey, srcMIME, srcBytes)
		if err != nil {
			s.markProjectFailed(ctx, proj, err)
			return nil, fmt.Errorf("images: upload source image: %w", err)
		}
		proj.SourceImageKey = srcKey
		proj.SourceImageURL = srcURL
		if err := s.store.UpdateProject(ctx, proj); err != nil {
			return nil, err
		}
	}

	effectivePrompt := prompt
	if hasPreset && preset.PromptTemplate != "" {
		effectivePrompt = preset.PromptTemplate + "\n\nUser instruction: " + prompt
	}

	var (
		resBytes     []byte
		pipelineUsed string
		aiErr        error
		actionType   string
	)
	if mode == ModeEdit {
		actionType = ActionEdit
		resBytes, pipelineUsed, aiErr = s.pipeline.Edit(ctx, srcBytes, effectivePrompt, imageModel, aspectRatio)
	} else {
		actionType = ActionGenerate
		resBytes, pipelineUsed, aiErr = s.pipeline.Generate(ctx, effectivePrompt, imageModel, aspectRatio)
	}
	if aiErr != nil {
		s.markProjectFailed(ctx, proj, aiErr)
		return nil, aiErr
	}

	resMIME, resExt := detectOutputImageFormat(resBytes)
	resKey := fmt.Sprintf("images/%d/%d/v1_%s%s", input.UserID, proj.ID, newRandomHexUUID(), resExt)
	resURL, err := s.storage.PutObject(ctx, resKey, resMIME, resBytes)
	if err != nil {
		s.markProjectFailed(ctx, proj, err)
		return nil, fmt.Errorf("images: upload generated image: %w", err)
	}

	rev := &Revision{
		ProjectID:    proj.ID,
		Version:      1,
		Prompt:       prompt,
		ActionType:   actionType,
		ImageKey:     resKey,
		ImageURL:     resURL,
		Model:        imageModel,
		PipelineUsed: pipelineUsed,
	}
	if err := s.store.AddRevision(ctx, rev); err != nil {
		s.markProjectFailed(ctx, proj, err)
		return nil, err
	}

	proj.CurrentImageKey = resKey
	proj.CurrentImageURL = resURL
	proj.CurrentVersion = 1
	proj.Status = StatusReady
	proj.ErrorMessage = ""
	if err := s.store.UpdateProject(ctx, proj); err != nil {
		return nil, err
	}
	proj.Revisions = []Revision{*rev}
	return proj, nil
}

// Refine applies an iterative edit prompt to the project's current image, creating a new revision.
// On AI failure, the previous CurrentImageURL and CurrentVersion remain intact.
func (s *Service) Refine(ctx context.Context, userID, projectID int64, prompt, model string) (*Project, error) {
	if err := s.ensureReady(); err != nil {
		return nil, err
	}
	cleanPrompt := strings.TrimSpace(prompt)
	if userID <= 0 || projectID <= 0 || cleanPrompt == "" {
		return nil, ErrInvalidInput
	}

	proj, err := s.GetProject(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	if proj.CurrentImageKey == "" {
		return nil, fmt.Errorf("%w: project has no current image", ErrInvalidInput)
	}

	currentBytes, _, err := s.storage.GetObject(ctx, proj.CurrentImageKey)
	if err != nil {
		return nil, fmt.Errorf("images: load current image: %w", err)
	}

	useModel := strings.TrimSpace(model)
	if useModel == "" {
		useModel = proj.ImageModel
	}
	if useModel == "" {
		useModel = defaultImageModel
	}

	resBytes, pipelineUsed, err := s.pipeline.Edit(ctx, currentBytes, cleanPrompt, useModel, proj.AspectRatio)
	if err != nil {
		s.markProjectFailed(ctx, proj, err)
		return nil, err
	}

	nextVersion := nextRevisionVersion(proj)
	resMIME, resExt := detectOutputImageFormat(resBytes)
	resKey := fmt.Sprintf("images/%d/%d/v%d_%s%s", userID, proj.ID, nextVersion, newRandomHexUUID(), resExt)
	resURL, err := s.storage.PutObject(ctx, resKey, resMIME, resBytes)
	if err != nil {
		s.markProjectFailed(ctx, proj, err)
		return nil, fmt.Errorf("images: upload refined image: %w", err)
	}

	rev := &Revision{
		ProjectID:    proj.ID,
		Version:      nextVersion,
		Prompt:       cleanPrompt,
		ActionType:   ActionRefine,
		ImageKey:     resKey,
		ImageURL:     resURL,
		Model:        useModel,
		PipelineUsed: pipelineUsed,
	}
	if err := s.store.AddRevision(ctx, rev); err != nil {
		return nil, err
	}

	proj.CurrentImageKey = resKey
	proj.CurrentImageURL = resURL
	proj.CurrentVersion = nextVersion
	proj.ImageModel = useModel
	proj.Status = StatusReady
	proj.ErrorMessage = ""
	if err := s.store.UpdateProject(ctx, proj); err != nil {
		return nil, err
	}

	proj.Revisions = append(proj.Revisions, *rev)
	return proj, nil
}

// Restore creates a new revision pointing to an earlier version's image key and URL.
func (s *Service) Restore(ctx context.Context, userID, projectID int64, version int) (*Project, error) {
	if s.store == nil {
		return nil, errors.New("images: store is nil")
	}
	if userID <= 0 || projectID <= 0 || version <= 0 {
		return nil, ErrInvalidInput
	}

	proj, err := s.GetProject(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}

	var target *Revision
	for i := range proj.Revisions {
		if proj.Revisions[i].Version == version {
			target = &proj.Revisions[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("%w: version %d not found", ErrInvalidInput, version)
	}

	nextVersion := nextRevisionVersion(proj)
	rev := &Revision{
		ProjectID:    proj.ID,
		Version:      nextVersion,
		Prompt:       fmt.Sprintf("Восстановлена версия v%d", version),
		ActionType:   ActionRestore,
		ImageKey:     target.ImageKey,
		ImageURL:     target.ImageURL,
		Model:        target.Model,
		PipelineUsed: PipelineRestore,
	}
	if err := s.store.AddRevision(ctx, rev); err != nil {
		return nil, err
	}

	proj.CurrentImageKey = target.ImageKey
	proj.CurrentImageURL = target.ImageURL
	proj.CurrentVersion = nextVersion
	proj.Status = StatusReady
	proj.ErrorMessage = ""
	if err := s.store.UpdateProject(ctx, proj); err != nil {
		return nil, err
	}

	proj.Revisions = append(proj.Revisions, *rev)
	return proj, nil
}

// DeleteProject verifies ownership, removes the project from persistence, and best-effort deletes its S3 objects.
func (s *Service) DeleteProject(ctx context.Context, userID, projectID int64) error {
	if s.store == nil {
		return errors.New("images: store is nil")
	}
	if userID <= 0 || projectID <= 0 {
		return ErrInvalidInput
	}

	proj, err := s.GetProject(ctx, userID, projectID)
	if err != nil {
		return err
	}

	keys := collectProjectKeys(proj)
	if err := s.store.DeleteProject(ctx, projectID); err != nil {
		return err
	}

	if s.storage != nil {
		for _, key := range keys {
			_ = s.storage.DeleteObject(ctx, key)
		}
	}
	return nil
}

// GetAsset retrieves raw object bytes and content type from storage.
func (s *Service) GetAsset(ctx context.Context, key string) ([]byte, string, error) {
	if s.storage == nil {
		return nil, "", errors.New("images: storage is nil")
	}
	return s.storage.GetObject(ctx, key)
}

func (s *Service) ensureReady() error {
	if s.store == nil {
		return errors.New("images: store is nil")
	}
	if s.pipeline == nil {
		return errors.New("images: pipeline is nil")
	}
	if s.storage == nil {
		return errors.New("images: storage is nil")
	}
	return nil
}

func (s *Service) markProjectFailed(ctx context.Context, proj *Project, err error) {
	if proj == nil || err == nil {
		return
	}
	proj.Status = StatusFailed
	proj.ErrorMessage = err.Error()
	_ = s.store.UpdateProject(ctx, proj)
}

func nextRevisionVersion(proj *Project) int {
	maxVer := proj.CurrentVersion
	for _, r := range proj.Revisions {
		if r.Version > maxVer {
			maxVer = r.Version
		}
	}
	return maxVer + 1
}

func collectProjectKeys(proj *Project) []string {
	seen := make(map[string]struct{})
	var keys []string
	add := func(k string) {
		if k == "" {
			return
		}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	add(proj.SourceImageKey)
	add(proj.CurrentImageKey)
	for _, r := range proj.Revisions {
		add(r.ImageKey)
	}
	return keys
}

func detectOutputImageFormat(data []byte) (string, string) {
	mimeType, ext, err := ValidateImageBytes(data)
	if err == nil {
		return mimeType, ext
	}
	return "image/png", ".png"
}

func buildProjectTitle(prompt, fallback string) string {
	clean := strings.TrimSpace(prompt)
	if clean == "" {
		clean = strings.TrimSpace(fallback)
	}
	if clean == "" {
		return "Изображение"
	}
	runes := []rune(clean)
	if len(runes) <= 60 {
		return clean
	}
	return string(runes[:57]) + "..."
}

func newRandomHexUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func generateExampleSourcePNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	bg := color.RGBA{R: 235, G: 240, B: 248, A: 255}
	fg := color.RGBA{R: 72, G: 112, B: 204, A: 255}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if x >= 4 && x < 12 && y >= 4 && y < 12 {
				img.Set(x, y, fg)
			} else {
				img.Set(x, y, bg)
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
