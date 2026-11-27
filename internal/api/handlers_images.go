package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/littfed/max-bot-api-client/internal/modules/images"
	"github.com/littfed/max-bot-api-client/internal/storage"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

type imagesHandler struct {
	cfg RouterConfig
}

func newImagesHandler(cfg RouterConfig) *imagesHandler {
	return &imagesHandler{cfg: cfg}
}

func (h *imagesHandler) getService() *images.Service {
	if h.cfg.ImagesModule != nil {
		return h.cfg.ImagesModule.Service()
	}
	return nil
}

func (h *imagesHandler) requireContext(w http.ResponseWriter, r *http.Request) (*store.User, *images.Service, bool) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil, nil, false
	}
	svc := h.getService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "images module not configured")
		return nil, nil, false
	}
	return user, svc, true
}

func (h *imagesHandler) checkModeActive(w http.ResponseWriter, r *http.Request) bool {
	if h.cfg.Guard == nil {
		return true
	}
	active, maintMsg, err := h.cfg.Guard.IsActive(r.Context(), "images")
	if err == nil && !active {
		if maintMsg == "" {
			maintMsg = "Фотостудия временно недоступна (техническое обслуживание)"
		}
		writeError(w, http.StatusForbidden, maintMsg)
		return false
	}
	return true
}

func (h *imagesHandler) consumeQuota(w http.ResponseWriter, r *http.Request, userID int64) bool {
	if h.cfg.Orchestrator == nil {
		return true
	}
	allowed, err := h.cfg.Orchestrator.ConsumeQuota(r.Context(), userID, "images")
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("quota check failed: %v", err))
		return false
	}
	if !allowed {
		writeError(w, http.StatusTooManyRequests, "Суточный лимит операций в Фотостудии исчерпан")
		return false
	}
	return true
}

func parseImageProjectID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return 0, false
	}
	return id, true
}

func writeImagesServiceError(w http.ResponseWriter, prefix string, err error) {
	switch {
	case errors.Is(err, images.ErrProjectNotFound):
		writeError(w, http.StatusNotFound, "project not found")
	case errors.Is(err, images.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, images.ErrImageTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, images.ErrInvalidImage), errors.Is(err, images.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("%s: %v", prefix, err))
	}
}

func (h *imagesHandler) handleListPresets(w http.ResponseWriter, r *http.Request) {
	_, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"presets": svc.ListPresets(),
	})
}

func (h *imagesHandler) handleListModels(w http.ResponseWriter, r *http.Request) {
	_, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	models, err := svc.ListModels(r.Context())
	if err != nil {
		writeImagesServiceError(w, "failed to list models", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"models": models,
	})
}

func (h *imagesHandler) handleListProjects(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	projects, err := svc.ListProjects(r.Context(), user.ID)
	if err != nil {
		writeImagesServiceError(w, "failed to list projects", err)
		return
	}
	if projects == nil {
		projects = []images.Project{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"projects": projects,
	})
}

func (h *imagesHandler) handleGetProject(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	projectID, ok := parseImageProjectID(w, r)
	if !ok {
		return
	}
	proj, err := svc.GetProject(r.Context(), user.ID, projectID)
	if err != nil {
		writeImagesServiceError(w, "failed to get project", err)
		return
	}
	writeJSON(w, http.StatusOK, proj)
}

func (h *imagesHandler) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	if !h.checkModeActive(w, r) {
		return
	}

	input := images.CreateProjectInput{
		UserID: user.ID,
	}

	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(images.MaxUploadBytes + (1 << 20)); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				writeError(w, http.StatusRequestEntityTooLarge, "uploaded file exceeds maximum size")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid multipart form")
			return
		}

		input.Mode = r.FormValue("mode")
		input.PresetID = r.FormValue("preset_id")
		input.Prompt = r.FormValue("prompt")
		input.AspectRatio = r.FormValue("aspect_ratio")
		input.ImageModel = r.FormValue("image_model")
		if rawExample := strings.TrimSpace(r.FormValue("use_example_source")); rawExample != "" {
			parsed, err := strconv.ParseBool(rawExample)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid use_example_source")
				return
			}
			input.UseExampleSource = parsed
		}

		file, _, err := r.FormFile("source_image")
		if err == nil {
			defer file.Close()
			data, readErr := io.ReadAll(io.LimitReader(file, images.MaxUploadBytes+1))
			if readErr != nil {
				writeError(w, http.StatusBadRequest, "failed to read source_image")
				return
			}
			if len(data) > images.MaxUploadBytes {
				writeError(w, http.StatusRequestEntityTooLarge, images.ErrImageTooLarge.Error())
				return
			}
			input.SourceImage = data
		} else if !errors.Is(err, http.ErrMissingFile) {
			writeError(w, http.StatusBadRequest, "invalid source_image field")
			return
		}
	} else {
		var req struct {
			Mode             string `json:"mode"`
			PresetID         string `json:"preset_id"`
			Prompt           string `json:"prompt"`
			AspectRatio      string `json:"aspect_ratio"`
			ImageModel       string `json:"image_model"`
			UseExampleSource bool   `json:"use_example_source"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		input.Mode = req.Mode
		input.PresetID = req.PresetID
		input.Prompt = req.Prompt
		input.AspectRatio = req.AspectRatio
		input.ImageModel = req.ImageModel
		input.UseExampleSource = req.UseExampleSource
	}

	if err := validateCreateProjectInput(input); err != nil {
		writeImagesServiceError(w, "failed to create project", err)
		return
	}
	if !h.consumeQuota(w, r, user.ID) {
		return
	}

	proj, err := svc.CreateAndRun(r.Context(), input)
	if err != nil {
		writeImagesServiceError(w, "failed to create project", err)
		return
	}
	writeJSON(w, http.StatusCreated, proj)
}

func validateCreateProjectInput(input images.CreateProjectInput) error {
	var preset images.Preset
	hasPreset := false
	if presetID := strings.TrimSpace(input.PresetID); presetID != "" {
		p, ok := images.GetPreset(presetID)
		if !ok {
			return fmt.Errorf("%w: unknown preset %q", images.ErrInvalidInput, presetID)
		}
		preset = p
		hasPreset = true
	}

	mode := strings.TrimSpace(input.Mode)
	if mode == "" && hasPreset {
		mode = preset.Mode
	}
	if mode == "" {
		mode = images.ModeGenerate
	}
	if mode != images.ModeGenerate && mode != images.ModeEdit {
		return fmt.Errorf("%w: unsupported mode %q", images.ErrInvalidInput, mode)
	}

	if strings.TrimSpace(input.Prompt) == "" {
		return fmt.Errorf("%w: prompt is required", images.ErrInvalidInput)
	}

	if mode == images.ModeEdit {
		switch {
		case len(input.SourceImage) > 0:
			if _, _, err := images.ValidateImageBytes(input.SourceImage); err != nil {
				return err
			}
		case !input.UseExampleSource:
			return fmt.Errorf("%w: source image is required for edit mode", images.ErrInvalidInput)
		}
	}
	return nil
}

func (h *imagesHandler) handleRefineProject(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	if !h.checkModeActive(w, r) {
		return
	}
	projectID, ok := parseImageProjectID(w, r)
	if !ok {
		return
	}

	var req struct {
		Prompt     string `json:"prompt"`
		ImageModel string `json:"image_model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		writeImagesServiceError(w, "failed to refine project", fmt.Errorf("%w: prompt is required", images.ErrInvalidInput))
		return
	}
	if _, err := svc.GetProject(r.Context(), user.ID, projectID); err != nil {
		writeImagesServiceError(w, "failed to refine project", err)
		return
	}
	if !h.consumeQuota(w, r, user.ID) {
		return
	}

	proj, err := svc.Refine(r.Context(), user.ID, projectID, req.Prompt, req.ImageModel)
	if err != nil {
		writeImagesServiceError(w, "failed to refine project", err)
		return
	}
	writeJSON(w, http.StatusOK, proj)
}

func (h *imagesHandler) handleRestoreProject(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	projectID, ok := parseImageProjectID(w, r)
	if !ok {
		return
	}

	var req struct {
		Version int `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	proj, err := svc.Restore(r.Context(), user.ID, projectID, req.Version)
	if err != nil {
		writeImagesServiceError(w, "failed to restore project", err)
		return
	}
	writeJSON(w, http.StatusOK, proj)
}

func (h *imagesHandler) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	projectID, ok := parseImageProjectID(w, r)
	if !ok {
		return
	}

	if err := svc.DeleteProject(r.Context(), user.ID, projectID); err != nil {
		writeImagesServiceError(w, "failed to delete project", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": true,
		"id":      projectID,
	})
}

func (h *imagesHandler) handleGetAsset(w http.ResponseWriter, r *http.Request) {
	svc := h.getService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "images module not configured")
		return
	}

	key := r.PathValue("key")
	if strings.TrimSpace(key) == "" {
		writeError(w, http.StatusBadRequest, "asset key is required")
		return
	}

	data, contentType, err := svc.GetAsset(r.Context(), key)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			writeError(w, http.StatusNotFound, "asset not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load asset: %v", err))
		return
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
