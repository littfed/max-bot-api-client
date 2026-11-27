package images

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/littfed/max-bot-api-client/pkg/ai"
)

const (
	chatTimeout    = 60 * time.Second
	visionTimeout  = 30 * time.Second
	imageTimeout   = 90 * time.Second
	modelsTimeout  = 10 * time.Second
	modelsCacheTTL = 5 * time.Minute
	chatMaxTokens  = 4096
)

var defaultImageModels = []ModelOption{
	{ID: "flux", Label: "FLUX", Description: "Universal high-quality image generation"},
	{ID: "gpt-image-1", Label: "GPT Image 1", Description: "OpenAI image generation and editing"},
}

// AIGateway abstracts chat, model listing, vision, image generation, and image editing.
type AIGateway interface {
	Chat(ctx context.Context, model, systemPrompt, userPrompt string) (string, error)
	ListModels(ctx context.Context) ([]string, error)
	Vision(ctx context.Context, prompt, imageDataURI string) (string, error)
	GenerateImage(ctx context.Context, prompt, model, size string) ([]byte, string, error)
	EditImage(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error)
}

type aiClientGateway struct {
	client     *ai.Client
	httpClient *http.Client

	modelsMu       sync.RWMutex
	cachedModels   []string
	modelsCachedAt time.Time
}

// NewAIClientGateway adapts *ai.Client to AIGateway with per-operation timeouts and model caching.
func NewAIClientGateway(client *ai.Client) AIGateway {
	return &aiClientGateway{
		client:     client,
		httpClient: &http.Client{Timeout: imageTimeout},
	}
}

func (g *aiClientGateway) Chat(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	if g.client == nil {
		return "", errors.New("images: ai client is nil")
	}
	reqCtx, cancel := context.WithTimeout(ctx, chatTimeout)
	defer cancel()

	var messages []ai.ChatMessage
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, ai.NewTextMessage(ai.RoleSystem, systemPrompt))
	}
	messages = append(messages, ai.NewTextMessage(ai.RoleUser, userPrompt))

	maxTokens := chatMaxTokens
	req := &ai.ChatRequest{
		Messages:  messages,
		MaxTokens: &maxTokens,
	}
	if m := strings.TrimSpace(model); m != "" {
		req.Model = m
	}

	resp, err := g.client.Chat(reqCtx, req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (g *aiClientGateway) ListModels(ctx context.Context) ([]string, error) {
	if g.client == nil {
		return nil, errors.New("images: ai client is nil")
	}

	g.modelsMu.RLock()
	if len(g.cachedModels) > 0 && time.Since(g.modelsCachedAt) < modelsCacheTTL {
		out := make([]string, len(g.cachedModels))
		copy(out, g.cachedModels)
		g.modelsMu.RUnlock()
		return out, nil
	}
	g.modelsMu.RUnlock()

	reqCtx, cancel := context.WithTimeout(ctx, modelsTimeout)
	defer cancel()

	rawModels, err := g.client.ListModels(reqCtx)
	if err != nil {
		return nil, fmt.Errorf("images: %w", err)
	}

	models := make([]string, 0, len(rawModels))
	seen := make(map[string]struct{}, len(rawModels))
	for _, rawID := range rawModels {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}

	g.modelsMu.Lock()
	g.cachedModels = make([]string, len(models))
	copy(g.cachedModels, models)
	g.modelsCachedAt = time.Now()
	g.modelsMu.Unlock()

	return models, nil
}

func (g *aiClientGateway) Vision(ctx context.Context, prompt, imageDataURI string) (string, error) {
	if g.client == nil {
		return "", errors.New("images: ai client is nil")
	}
	reqCtx, cancel := context.WithTimeout(ctx, visionTimeout)
	defer cancel()

	resp, err := g.client.Vision(reqCtx, &ai.VisionRequest{
		Prompt:   prompt,
		ImageURL: imageDataURI,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (g *aiClientGateway) GenerateImage(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
	if g.client == nil {
		return nil, "", errors.New("images: ai client is nil")
	}
	reqCtx, cancel := context.WithTimeout(ctx, imageTimeout)
	defer cancel()

	resp, err := g.client.GenerateImage(reqCtx, &ai.ImageRequest{
		Prompt:         prompt,
		Model:          strings.TrimSpace(model),
		Size:           size,
		ResponseFormat: "b64_json",
	})
	if err != nil {
		return nil, "", err
	}
	return g.unwrapImageResponse(reqCtx, resp)
}

func (g *aiClientGateway) EditImage(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error) {
	if g.client == nil {
		return nil, "", errors.New("images: ai client is nil")
	}
	reqCtx, cancel := context.WithTimeout(ctx, imageTimeout)
	defer cancel()

	resp, err := g.client.EditImage(reqCtx, &ai.ImageEditRequest{
		ImageBytes:     imageBytes,
		Filename:       "source.png",
		Prompt:         prompt,
		Model:          strings.TrimSpace(model),
		Size:           size,
		ResponseFormat: "b64_json",
	})
	if err != nil {
		return nil, "", err
	}
	return g.unwrapImageResponse(reqCtx, resp)
}

func (g *aiClientGateway) unwrapImageResponse(ctx context.Context, resp *ai.ImageResponse) ([]byte, string, error) {
	if resp == nil {
		return nil, "", errors.New("images: empty image response from AI gateway")
	}
	if len(resp.Data) > 0 {
		return resp.Data, resp.URL, nil
	}
	if resp.URL != "" {
		data, err := downloadImageURL(ctx, g.httpClient, resp.URL)
		if err == nil && len(data) > 0 {
			return data, resp.URL, nil
		}
		return nil, resp.URL, nil
	}
	return nil, "", errors.New("images: empty image response from AI gateway")
}

// Pipeline orchestrates image model discovery, generation, and hybrid editing.
type Pipeline struct {
	gw         AIGateway
	httpClient *http.Client
}

// NewPipeline creates a Pipeline backed by the provided AIGateway.
func NewPipeline(gw AIGateway) *Pipeline {
	return &Pipeline{
		gw:         gw,
		httpClient: &http.Client{Timeout: imageTimeout},
	}
}

// ListImageModels returns curated image models available from the gateway,
// falling back to default models if the gateway is unavailable or returns none.
func (p *Pipeline) ListImageModels(ctx context.Context) ([]ModelOption, error) {
	if p.gw == nil {
		return append([]ModelOption(nil), defaultImageModels...), nil
	}
	rawModels, err := p.gw.ListModels(ctx)
	if err != nil || len(rawModels) == 0 {
		return append([]ModelOption(nil), defaultImageModels...), nil
	}

	known := make(map[string]ModelOption, len(defaultImageModels))
	for _, opt := range defaultImageModels {
		known[strings.ToLower(opt.ID)] = opt
	}

	var result []ModelOption
	seen := make(map[string]struct{})
	for _, id := range rawModels {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" || !isImageModelID(trimmed) {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if opt, ok := known[key]; ok {
			result = append(result, opt)
			continue
		}
		result = append(result, ModelOption{
			ID:    trimmed,
			Label: trimmed,
		})
	}

	if len(result) == 0 {
		return append([]ModelOption(nil), defaultImageModels...), nil
	}
	return result, nil
}

// Generate creates a new image from a text prompt. If the prompt contains Cyrillic
// characters, it attempts a fast Chat translation/enrichment to English, falling back
// to the raw prompt if Chat fails.
func (p *Pipeline) Generate(ctx context.Context, prompt, model, aspectRatio string) ([]byte, string, error) {
	if p.gw == nil {
		return nil, "", errors.New("images: ai gateway is nil")
	}
	cleanPrompt := strings.TrimSpace(prompt)
	if cleanPrompt == "" {
		return nil, "", errors.New("images: prompt is required")
	}

	genPrompt := p.prepareEnglishPrompt(ctx, cleanPrompt)
	size := aspectRatioToSize(aspectRatio)

	reqCtx, cancel := context.WithTimeout(ctx, imageTimeout)
	defer cancel()

	data, imgURL, err := p.gw.GenerateImage(reqCtx, genPrompt, model, size)
	if err != nil {
		return nil, "", err
	}
	resolved, err := resolveImageBytes(reqCtx, p.httpClient, data, imgURL)
	if err != nil {
		return nil, "", err
	}
	return resolved, PipelineGenerations, nil
}

// Edit modifies an existing image. It tries EditImage first and automatically falls
// back to Vision + Chat + GenerateImage when the gateway does not support /images/edits.
func (p *Pipeline) Edit(ctx context.Context, srcBytes []byte, prompt, model, aspectRatio string) ([]byte, string, error) {
	if p.gw == nil {
		return nil, "", errors.New("images: ai gateway is nil")
	}
	if len(srcBytes) == 0 {
		return nil, "", errors.New("images: source image bytes are required")
	}
	cleanPrompt := strings.TrimSpace(prompt)
	if cleanPrompt == "" {
		return nil, "", errors.New("images: edit prompt is required")
	}

	size := aspectRatioToSize(aspectRatio)

	editCtx, editCancel := context.WithTimeout(ctx, imageTimeout)
	defer editCancel()

	data, imgURL, err := p.gw.EditImage(editCtx, srcBytes, cleanPrompt, model, size)
	if err == nil {
		resolved, resolveErr := resolveImageBytes(editCtx, p.httpClient, data, imgURL)
		if resolveErr != nil {
			return nil, "", resolveErr
		}
		return resolved, PipelineEdits, nil
	}

	if !isEditUnsupportedError(err) {
		return nil, "", err
	}

	return p.editViaVisionFallback(ctx, srcBytes, cleanPrompt, model, size)
}

func (p *Pipeline) editViaVisionFallback(ctx context.Context, srcBytes []byte, editPrompt, model, size string) ([]byte, string, error) {
	dataURI := encodeImageDataURI(srcBytes)

	visionPrompt := "Describe this image in rich visual detail: main subject, pose or layout, composition, lighting, color palette, background, and artistic style. Be concise and factual."
	description, err := p.gw.Vision(ctx, visionPrompt, dataURI)
	if err != nil {
		return nil, "", fmt.Errorf("images: vision fallback failed: %w", err)
	}

	systemPrompt := "You are an expert AI image prompt engineer. Merge the original image description with the requested edit into a single cohesive, detailed English image-generation prompt. Output only the final prompt text."
	userPrompt := fmt.Sprintf("Original image description:\n%s\n\nRequested edit:\n%s", strings.TrimSpace(description), editPrompt)

	mergedPrompt, err := p.gw.Chat(ctx, "", systemPrompt, userPrompt)
	if err != nil || strings.TrimSpace(mergedPrompt) == "" {
		// Graceful fallback if Chat fails during merge.
		mergedPrompt = strings.TrimSpace(description) + ", " + p.prepareEnglishPrompt(ctx, editPrompt)
	} else {
		mergedPrompt = strings.TrimSpace(mergedPrompt)
	}

	genCtx, genCancel := context.WithTimeout(ctx, imageTimeout)
	defer genCancel()

	data, imgURL, err := p.gw.GenerateImage(genCtx, mergedPrompt, model, size)
	if err != nil {
		return nil, "", fmt.Errorf("images: vision fallback generation failed: %w", err)
	}
	resolved, err := resolveImageBytes(genCtx, p.httpClient, data, imgURL)
	if err != nil {
		return nil, "", err
	}
	return resolved, PipelineVisionFallback, nil
}

func (p *Pipeline) prepareEnglishPrompt(ctx context.Context, prompt string) string {
	if !containsCyrillic(prompt) {
		return prompt
	}
	systemPrompt := "Translate and slightly enrich the following image generation prompt into concise, vivid English. Output only the English prompt."
	translated, err := p.gw.Chat(ctx, "", systemPrompt, prompt)
	if err != nil || strings.TrimSpace(translated) == "" {
		return prompt
	}
	return strings.TrimSpace(translated)
}

func aspectRatioToSize(aspectRatio string) string {
	switch strings.TrimSpace(aspectRatio) {
	case "1:1":
		return "1024x1024"
	case "16:9":
		return "1344x768"
	case "9:16":
		return "768x1344"
	case "4:3":
		return "1152x896"
	default:
		return "1024x1024"
	}
}

func isEditUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	msg := strings.ToLower(err.Error())
	markers := []string{
		"status 404",
		"status 405",
		"status 501",
		"404",
		"405",
		"501",
		"not supported",
		"unsupported",
		"unknown endpoint",
		"invalid model",
	}
	for _, m := range markers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}

func isImageModelID(id string) bool {
	lower := strings.ToLower(id)
	keywords := []string{
		"flux",
		"image",
		"dall-e",
		"sdxl",
		"sd-",
		"stable-diffusion",
		"imagen",
		"midjourney",
		"recraft",
		"ideogram",
		"seedream",
		"kandinsky",
		"wanx",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func containsCyrillic(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Cyrillic) {
			return true
		}
	}
	return false
}

func encodeImageDataURI(data []byte) string {
	mimeType := http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = "image/jpeg"
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
}

func resolveImageBytes(ctx context.Context, client *http.Client, data []byte, imgURL string) ([]byte, error) {
	if len(data) > 0 {
		return data, nil
	}
	if imgURL == "" {
		return nil, errors.New("images: empty image data and URL")
	}
	if strings.HasPrefix(imgURL, "data:") {
		idx := strings.Index(imgURL, ",")
		if idx == -1 {
			return nil, errors.New("images: invalid data URI")
		}
		decoded, err := base64.StdEncoding.DecodeString(imgURL[idx+1:])
		if err != nil {
			return nil, fmt.Errorf("images: decode data URI failed: %w", err)
		}
		return decoded, nil
	}
	return downloadImageURL(ctx, client, imgURL)
}

func downloadImageURL(ctx context.Context, client *http.Client, imgURL string) ([]byte, error) {
	if client == nil {
		client = &http.Client{Timeout: imageTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imgURL, nil)
	if err != nil {
		return nil, fmt.Errorf("images: build download request failed: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("images: download image failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("images: download image status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("images: read downloaded image failed: %w", err)
	}
	if len(body) == 0 {
		return nil, errors.New("images: downloaded image is empty")
	}
	return body, nil
}
