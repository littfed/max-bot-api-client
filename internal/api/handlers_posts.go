package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/littfed/max-bot-api-client/internal/modules/posts"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

type postsHandler struct {
	cfg RouterConfig
}

func newPostsHandler(cfg RouterConfig) *postsHandler {
	return &postsHandler{cfg: cfg}
}

func (h *postsHandler) getService() *posts.Service {
	if h.cfg.PostsModule != nil {
		return h.cfg.PostsModule.Service()
	}
	return nil
}

func (h *postsHandler) requireContext(w http.ResponseWriter, r *http.Request) (*store.User, *posts.Service, bool) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil, nil, false
	}
	svc := h.getService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "posts module not configured")
		return nil, nil, false
	}
	return user, svc, true
}

func parseDraftID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid draft id")
		return 0, false
	}
	return id, true
}

func (h *postsHandler) syncActiveDraft(ctx context.Context, userID int64, step string, draftID int64) {
	if h.cfg.FSM == nil || draftID <= 0 {
		return
	}
	payload, err := json.Marshal(posts.FSMStatePayload{ActiveDraftID: draftID})
	if err != nil {
		slog.Warn("posts api: failed to marshal fsm payload", "user_id", userID, "draft_id", draftID, "error", err)
		return
	}
	if err := h.cfg.FSM.Set(ctx, userID, "posts", step, payload); err != nil {
		slog.Warn("posts api: failed to sync fsm active_draft_id", "user_id", userID, "draft_id", draftID, "error", err)
	}
}

func writePostsServiceError(w http.ResponseWriter, prefix string, err error) {
	if errors.Is(err, posts.ErrNotFound) {
		writeError(w, http.StatusNotFound, "draft not found")
		return
	}
	if errors.Is(err, posts.ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, fmt.Sprintf("%s: %v", prefix, err))
}

func decodeOptionalJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	err := json.NewDecoder(r.Body).Decode(dst)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func (h *postsHandler) handleListModels(w http.ResponseWriter, r *http.Request) {
	_, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}

	catalog, err := svc.ListAvailableModels(r.Context())
	if err != nil {
		writePostsServiceError(w, "failed to list models", err)
		return
	}

	writeJSON(w, http.StatusOK, catalog)
}

func (h *postsHandler) handleGetState(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}

	var activeDraftID int64
	explicitDraftID := false
	if rawID := strings.TrimSpace(r.URL.Query().Get("draft_id")); rawID != "" {
		parsed, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "invalid draft_id")
			return
		}
		activeDraftID = parsed
		explicitDraftID = true
	} else if h.cfg.FSM != nil {
		if st, err := h.cfg.FSM.Get(r.Context(), user.ID, "posts"); err == nil && st != nil && len(st.Payload) > 0 {
			var payload posts.FSMStatePayload
			if err := json.Unmarshal(st.Payload, &payload); err == nil && payload.ActiveDraftID > 0 {
				activeDraftID = payload.ActiveDraftID
			}
		}
	}

	state, err := svc.GetState(r.Context(), user.ID, activeDraftID)
	if err != nil {
		if errors.Is(err, posts.ErrNotFound) && !explicitDraftID && activeDraftID > 0 {
			state, err = svc.GetState(r.Context(), user.ID, 0)
		}
		if err != nil {
			writePostsServiceError(w, "failed to get posts state", err)
			return
		}
	}

	writeJSON(w, http.StatusOK, state)
}

func (h *postsHandler) handleSaveProfile(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}

	var req posts.BrandProfile
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	saved, err := svc.SaveProfile(r.Context(), user.ID, &req)
	if err != nil {
		writePostsServiceError(w, "failed to save profile", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"profile": saved,
	})
}

type startDraftRequest struct {
	RawIdea        string `json:"raw_idea"`
	SourceImageURL string `json:"source_image_url"`
}

func (h *postsHandler) handleStartDraft(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}

	var req startDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.RawIdea) == "" && strings.TrimSpace(req.SourceImageURL) == "" {
		writeError(w, http.StatusBadRequest, "raw_idea or source_image_url is required")
		return
	}

	draft, err := svc.StartDraft(r.Context(), user.ID, req.RawIdea, req.SourceImageURL)
	if err != nil {
		writePostsServiceError(w, "failed to start draft", err)
		return
	}

	h.syncActiveDraft(r.Context(), user.ID, posts.StepBriefSetup, draft.ID)

	writeJSON(w, http.StatusCreated, map[string]any{
		"draft": draft,
	})
}

type updateBriefRequest struct {
	Goal            string   `json:"goal"`
	Tone            string   `json:"tone"`
	TextModel       string   `json:"text_model"`
	TargetPlatforms []string `json:"target_platforms"`
}

func (h *postsHandler) handleUpdateBrief(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	draftID, ok := parseDraftID(w, r)
	if !ok {
		return
	}

	var req updateBriefRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	draft, err := svc.UpdateBrief(r.Context(), user.ID, draftID, req.Goal, req.Tone, req.TextModel, req.TargetPlatforms)
	if err != nil {
		writePostsServiceError(w, "failed to update brief", err)
		return
	}

	h.syncActiveDraft(r.Context(), user.ID, posts.StepBriefSetup, draft.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"draft": draft,
	})
}

type generateHooksRequest struct {
	TextModel string `json:"text_model"`
}

func (h *postsHandler) handleGenerateHooks(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	draftID, ok := parseDraftID(w, r)
	if !ok {
		return
	}

	var req generateHooksRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	draft, err := svc.GenerateHooks(r.Context(), user.ID, draftID, req.TextModel)
	if err != nil {
		writePostsServiceError(w, "failed to generate hooks", err)
		return
	}

	h.syncActiveDraft(r.Context(), user.ID, posts.StepHooksSelect, draft.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"draft": draft,
	})
}

type generatePackageRequest struct {
	HookID    string `json:"hook_id"`
	TextModel string `json:"text_model"`
}

func (h *postsHandler) handleGeneratePackage(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	draftID, ok := parseDraftID(w, r)
	if !ok {
		return
	}

	var req generatePackageRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	draft, err := svc.GeneratePackage(r.Context(), user.ID, draftID, req.HookID, req.TextModel)
	if err != nil {
		writePostsServiceError(w, "failed to generate package", err)
		return
	}

	h.syncActiveDraft(r.Context(), user.ID, posts.StepReady, draft.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"draft": draft,
	})
}

type refineRequest struct {
	Platform    string `json:"platform"`
	Instruction string `json:"instruction"`
	TextModel   string `json:"text_model"`
}

func (h *postsHandler) handleRefine(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	draftID, ok := parseDraftID(w, r)
	if !ok {
		return
	}

	var req refineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Instruction) == "" {
		writeError(w, http.StatusBadRequest, "instruction is required")
		return
	}

	draft, err := svc.Refine(r.Context(), user.ID, draftID, req.Platform, req.Instruction, req.TextModel)
	if err != nil {
		writePostsServiceError(w, "failed to refine draft", err)
		return
	}

	h.syncActiveDraft(r.Context(), user.ID, posts.StepReady, draft.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"draft": draft,
	})
}

func (h *postsHandler) handleUpdateVariant(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	draftID, ok := parseDraftID(w, r)
	if !ok {
		return
	}

	platformName := strings.TrimSpace(r.PathValue("platform"))
	if platformName == "" {
		writeError(w, http.StatusBadRequest, "platform is required")
		return
	}

	var req posts.PostVariant
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	draft, err := svc.UpdateVariantManual(r.Context(), user.ID, draftID, platformName, req)
	if err != nil {
		writePostsServiceError(w, "failed to update variant", err)
		return
	}

	h.syncActiveDraft(r.Context(), user.ID, posts.StepReady, draft.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"draft": draft,
	})
}

type regenerateImageRequest struct {
	Prompt string `json:"prompt"`
}

func (h *postsHandler) handleRegenerateImage(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	draftID, ok := parseDraftID(w, r)
	if !ok {
		return
	}

	var req regenerateImageRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	draft, err := svc.RegenerateImage(r.Context(), user.ID, draftID, req.Prompt)
	if err != nil {
		writePostsServiceError(w, "failed to regenerate image", err)
		return
	}

	h.syncActiveDraft(r.Context(), user.ID, posts.StepReady, draft.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"draft": draft,
	})
}

type restoreRevisionRequest struct {
	Type     string `json:"type"`
	Version  int    `json:"version"`
	Platform string `json:"platform"`
}

func (h *postsHandler) handleRestoreRevision(w http.ResponseWriter, r *http.Request) {
	user, svc, ok := h.requireContext(w, r)
	if !ok {
		return
	}
	draftID, ok := parseDraftID(w, r)
	if !ok {
		return
	}

	var req restoreRevisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	kind := strings.ToLower(strings.TrimSpace(req.Type))
	if kind != "image" && kind != "text" {
		writeError(w, http.StatusBadRequest, "type must be 'image' or 'text'")
		return
	}
	if req.Version <= 0 {
		writeError(w, http.StatusBadRequest, "version must be positive")
		return
	}

	draft, err := svc.RestoreRevision(r.Context(), user.ID, draftID, kind, req.Version, req.Platform)
	if err != nil {
		writePostsServiceError(w, "failed to restore revision", err)
		return
	}

	h.syncActiveDraft(r.Context(), user.ID, posts.StepReady, draft.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"draft": draft,
	})
}
