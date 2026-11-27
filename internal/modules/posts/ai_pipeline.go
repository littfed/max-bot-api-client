package posts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/ai"
)

const (
	chatTimeout    = 60 * time.Second
	visionTimeout  = 30 * time.Second
	imageTimeout   = 60 * time.Second
	modelsTimeout  = 10 * time.Second
	modelsCacheTTL = 5 * time.Minute
	chatMaxTokens  = 4096
)

// AIGateway abstracts text chat, model listing, vision analysis, and image generation for the posts pipeline.
type AIGateway interface {
	Chat(ctx context.Context, model, systemPrompt, userPrompt string) (string, error)
	ListModels(ctx context.Context) ([]string, error)
	Vision(ctx context.Context, prompt, imageURL string) (string, error)
	GenerateImage(ctx context.Context, prompt, aspectRatio string) (string, error)
}

type aiClientGateway struct {
	client *ai.Client

	modelsMu       sync.RWMutex
	cachedModels   []string
	modelsCachedAt time.Time
}

// NewAIGateway adapts an *ai.Client to the AIGateway interface with per-operation timeouts.
func NewAIGateway(client *ai.Client) AIGateway {
	return &aiClientGateway{client: client}
}

func (g *aiClientGateway) Chat(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	if g.client == nil {
		return "", errors.New("posts: ai client is nil")
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
		return nil, errors.New("posts: ai client is nil")
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
		return nil, fmt.Errorf("posts: %w", err)
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

func (g *aiClientGateway) Vision(ctx context.Context, prompt, imageURL string) (string, error) {
	if g.client == nil {
		return "", errors.New("posts: ai client is nil")
	}
	reqCtx, cancel := context.WithTimeout(ctx, visionTimeout)
	defer cancel()

	resp, err := g.client.Vision(reqCtx, &ai.VisionRequest{
		Prompt:   prompt,
		ImageURL: imageURL,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (g *aiClientGateway) GenerateImage(ctx context.Context, prompt, aspectRatio string) (string, error) {
	if g.client == nil {
		return "", errors.New("posts: ai client is nil")
	}
	reqCtx, cancel := context.WithTimeout(ctx, imageTimeout)
	defer cancel()

	resp, err := g.client.GenerateImage(reqCtx, &ai.ImageRequest{
		Prompt:         prompt,
		Size:           aspectRatioToSize(aspectRatio),
		ResponseFormat: "url",
	})
	if err != nil {
		return "", err
	}
	if resp.URL != "" {
		return resp.URL, nil
	}
	if len(resp.Data) > 0 {
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(resp.Data), nil
	}
	return "", errors.New("posts: empty image response from AI gateway")
}

func aspectRatioToSize(aspectRatio string) string {
	switch strings.TrimSpace(aspectRatio) {
	case "16:9":
		return "1280x720"
	case "4:5":
		return "896x1120"
	case "9:16":
		return "720x1280"
	default:
		return "1024x1024"
	}
}

// Pipeline orchestrates AI-assisted content generation for the Content Studio.
type Pipeline struct {
	gw AIGateway
}

// NewPipeline creates a new content generation Pipeline backed by the given AIGateway.
func NewPipeline(gw AIGateway) *Pipeline {
	return &Pipeline{gw: gw}
}

// ListModels returns available model IDs from the underlying AI gateway.
func (p *Pipeline) ListModels(ctx context.Context) ([]string, error) {
	if p.gw == nil {
		return nil, errors.New("posts: ai gateway is nil")
	}
	return p.gw.ListModels(ctx)
}

// AnalyzeImage inspects a source image URL and returns a concise visual summary for post ideation.
func (p *Pipeline) AnalyzeImage(ctx context.Context, imageURL string) (string, error) {
	if p.gw == nil {
		return "", errors.New("posts: ai gateway is nil")
	}
	if strings.TrimSpace(imageURL) == "" {
		return "", errors.New("posts: imageURL cannot be empty")
	}
	prompt := "Опиши ключевые объекты, сюжет, настроение и детали на изображении, которые можно использовать как основу для поста в соцсетях. Если на картинке есть текст, кратко передай его смысл."
	return p.gw.Vision(ctx, prompt, imageURL)
}

type hooksResponse struct {
	Hooks []PostHook `json:"hooks"`
}

// GenerateHooks generates 3-5 distinct hook angles for a draft and optional brand profile.
func (p *Pipeline) GenerateHooks(ctx context.Context, draft *PostDraft, profile *BrandProfile) ([]PostHook, error) {
	if p.gw == nil {
		return nil, errors.New("posts: ai gateway is nil")
	}
	if draft == nil {
		return nil, errors.New("posts: draft cannot be nil")
	}

	systemPrompt := `Ты — опытный контент-стратег и копирайтер для социальных сетей.
Твоя задача — предложить 3–5 сильных хуков (заходов) для поста под разными углами.
Отвечай строго валидным JSON без markdown-разметки и пояснений в формате:
{"hooks":[{"id":"h1","title":"Короткое название угла","text":"Текст цепляющего первого абзаца","angle":"problem|case|provocation|story|benefit"}]}`

	var b strings.Builder
	b.WriteString("Сгенерируй хуки для поста.\n")
	appendDraftContext(&b, draft)
	appendBrandContext(&b, profile)

	resp, err := chatJSON[hooksResponse](ctx, p.gw, draft.TextModel, systemPrompt, b.String())
	if err != nil {
		return nil, err
	}
	if len(resp.Hooks) == 0 {
		return nil, errors.New("posts: AI returned no hooks")
	}
	return resp.Hooks, nil
}

type packageResponse struct {
	Variants    []PostVariant `json:"variants"`
	ImagePrompt string        `json:"image_prompt"`
}

// GeneratePackage generates platform-adapted post variants and an image generation prompt.
func (p *Pipeline) GeneratePackage(ctx context.Context, draft *PostDraft, profile *BrandProfile, specs []PlatformSpec) ([]PostVariant, string, error) {
	if p.gw == nil {
		return nil, "", errors.New("posts: ai gateway is nil")
	}
	if draft == nil {
		return nil, "", errors.New("posts: draft cannot be nil")
	}
	if len(specs) == 0 {
		return nil, "", errors.New("posts: at least one platform spec is required")
	}

	systemPrompt := `Ты — профессиональный SMM-редактор. Адаптируй пост под каждую указанную платформу с учётом её ограничений, стиля и разметки, а также предложи промпт на английском языке для генерации иллюстрации.
Отвечай строго валидным JSON без markdown-разметки и пояснений. Внутри строковых значений JSON не используй неэкранированные двойные кавычки " (используй кавычки-ёлочки «»), переносы строк кодируй как \n.
Формат ответа:
{"image_prompt":"Detailed English prompt for image generation","variants":[{"platform":"id","text":"Текст поста","hashtags":["#тег"],"cta":"Призыв к действию"}]}`

	var b strings.Builder
	b.WriteString("Сгенерируй пакет постов для выбранных платформ.\n")
	appendDraftContext(&b, draft)
	if hook := findSelectedHook(draft); hook != nil {
		fmt.Fprintf(&b, "Выбранный хук (%s — %s): %s\n", hook.Angle, hook.Title, hook.Text)
	}
	appendBrandContext(&b, profile)
	appendPlatformSpecs(&b, specs)

	resp, err := chatJSON[packageResponse](ctx, p.gw, draft.TextModel, systemPrompt, b.String())
	if err == nil && len(resp.Variants) > 0 {
		return resp.Variants, strings.TrimSpace(resp.ImagePrompt), nil
	}
	if len(specs) == 1 {
		if err != nil {
			return nil, "", err
		}
		return nil, "", errors.New("posts: AI returned no variants")
	}

	// Fallback for multi-platform packages when combined JSON output is truncated:
	// generate each platform variant in parallel so every response fits comfortably within token limits.
	return p.generatePackageParallel(ctx, draft, profile, specs)
}

func (p *Pipeline) generatePackageParallel(ctx context.Context, draft *PostDraft, profile *BrandProfile, specs []PlatformSpec) ([]PostVariant, string, error) {
	type item struct {
		variant     PostVariant
		imagePrompt string
		err         error
	}
	results := make([]item, len(specs))
	var wg sync.WaitGroup

	for i, spec := range specs {
		wg.Add(1)
		go func(idx int, sp PlatformSpec) {
			defer wg.Done()
			needImg := idx == 0
			sysPrompt := `Ты — профессиональный SMM-редактор. Адаптируй пост под указанную платформу.
Отвечай строго валидным JSON без markdown-разметки. Внутри строк не используй неэкранированные двойные кавычки " (используй «»), переносы строк кодируй как \n.
Формат ответа:
{"image_prompt":"Detailed English prompt for image generation","variants":[{"platform":"id","text":"Текст поста","hashtags":["#тег"],"cta":"Призыв к действию"}]}`
			if !needImg {
				sysPrompt = `Ты — профессиональный SMM-редактор. Адаптируй пост под указанную платформу.
Отвечай строго валидным JSON без markdown-разметки. Внутри строк не используй неэкранированные двойные кавычки " (используй «»), переносы строк кодируй как \n.
Формат ответа:
{"variants":[{"platform":"id","text":"Текст поста","hashtags":["#тег"],"cta":"Призыв к действию"}]}`
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "Сгенерируй пост для одной платформы: %s.\n", sp.ID)
			appendDraftContext(&sb, draft)
			if hook := findSelectedHook(draft); hook != nil {
				fmt.Fprintf(&sb, "Выбранный хук (%s — %s): %s\n", hook.Angle, hook.Title, hook.Text)
			}
			appendBrandContext(&sb, profile)
			appendPlatformSpecs(&sb, []PlatformSpec{sp})

			r, err := chatJSON[packageResponse](ctx, p.gw, draft.TextModel, sysPrompt, sb.String())
			if err != nil {
				results[idx].err = err
				return
			}
			if len(r.Variants) == 0 {
				results[idx].err = fmt.Errorf("posts: empty variant for %s", sp.ID)
				return
			}
			v := r.Variants[0]
			if strings.TrimSpace(v.Platform) == "" {
				v.Platform = sp.ID
			}
			results[idx].variant = v
			results[idx].imagePrompt = strings.TrimSpace(r.ImagePrompt)
		}(i, spec)
	}
	wg.Wait()

	variants := make([]PostVariant, 0, len(specs))
	var imgPrompt string
	var firstErr error
	for _, res := range results {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			continue
		}
		variants = append(variants, res.variant)
		if imgPrompt == "" && res.imagePrompt != "" {
			imgPrompt = res.imagePrompt
		}
	}
	if len(variants) == 0 {
		if firstErr != nil {
			return nil, "", firstErr
		}
		return nil, "", errors.New("posts: AI returned no variants")
	}
	return variants, imgPrompt, nil
}

// GenerateVisual generates an image for the given prompt and aspect ratio, returning its URL or data URI.
func (p *Pipeline) GenerateVisual(ctx context.Context, imagePrompt, aspectRatio string) (string, error) {
	if p.gw == nil {
		return "", errors.New("posts: ai gateway is nil")
	}
	if strings.TrimSpace(imagePrompt) == "" {
		return "", errors.New("posts: imagePrompt cannot be empty")
	}
	return p.gw.GenerateImage(ctx, imagePrompt, aspectRatio)
}

type refineResponse struct {
	Variants []PostVariant `json:"variants"`
}

// RefineVariant refines either a single targetPlatform variant (preserving others) or all variants.
func (p *Pipeline) RefineVariant(ctx context.Context, draft *PostDraft, profile *BrandProfile, specs []PlatformSpec, targetPlatform, instruction string) ([]PostVariant, error) {
	if p.gw == nil {
		return nil, errors.New("posts: ai gateway is nil")
	}
	if draft == nil {
		return nil, errors.New("posts: draft cannot be nil")
	}
	if strings.TrimSpace(instruction) == "" {
		return nil, errors.New("posts: instruction cannot be empty")
	}

	target := strings.ToLower(strings.TrimSpace(targetPlatform))
	isSingleTarget := target != "" && target != "all"

	systemPrompt := `Ты — профессиональный SMM-редактор. Доработай указанные варианты постов строго по инструкции пользователя, сохраняя требования платформ и голос бренда.
Отвечай строго валидным JSON без markdown-разметки и пояснений в формате:
{"variants":[{"platform":"id","text":"Обновлённый текст","hashtags":["#тег"],"cta":"Призыв к действию"}]}`

	var b strings.Builder
	if isSingleTarget {
		fmt.Fprintf(&b, "Доработай вариант только для платформы %q.\n", target)
	} else {
		b.WriteString("Доработай все варианты постов.\n")
	}
	fmt.Fprintf(&b, "Инструкция: %s\n", instruction)
	appendDraftContext(&b, draft)
	appendBrandContext(&b, profile)
	appendPlatformSpecs(&b, filterSpecsForTarget(specs, target, isSingleTarget))
	if len(draft.Variants) > 0 {
		b.WriteString("Текущие варианты:\n")
		for _, v := range draft.Variants {
			if isSingleTarget && strings.ToLower(v.Platform) != target {
				continue
			}
			fmt.Fprintf(&b, "- [%s] text=%q hashtags=%v cta=%q\n", v.Platform, v.Text, v.Hashtags, v.CTA)
		}
	}

	resp, err := chatJSON[refineResponse](ctx, p.gw, draft.TextModel, systemPrompt, b.String())
	if err != nil {
		return nil, err
	}
	if len(resp.Variants) == 0 {
		return nil, errors.New("posts: AI returned no refined variants")
	}

	if !isSingleTarget || len(draft.Variants) == 0 {
		return resp.Variants, nil
	}

	return mergeRefinedVariants(draft.Variants, resp.Variants, target), nil
}

func mergeRefinedVariants(existing, refined []PostVariant, target string) []PostVariant {
	byPlatform := make(map[string]PostVariant, len(refined))
	for _, v := range refined {
		byPlatform[strings.ToLower(strings.TrimSpace(v.Platform))] = v
	}

	out := make([]PostVariant, 0, len(existing))
	replaced := false
	for _, v := range existing {
		key := strings.ToLower(strings.TrimSpace(v.Platform))
		if key == target {
			if updated, ok := byPlatform[key]; ok {
				out = append(out, updated)
				replaced = true
				continue
			}
		}
		out = append(out, v)
	}
	if !replaced {
		if updated, ok := byPlatform[target]; ok {
			out = append(out, updated)
		}
	}
	return out
}

func chatJSON[T any](ctx context.Context, gw AIGateway, model, systemPrompt, userPrompt string) (T, error) {
	var zero T

	raw, err := gw.Chat(ctx, model, systemPrompt, userPrompt)
	if err != nil {
		return zero, err
	}

	var result T
	cleaned := stripCodeFences(raw)
	if err := json.Unmarshal([]byte(cleaned), &result); err == nil {
		return result, nil
	} else {
		correctivePrompt := fmt.Sprintf(
			"%s\n\nПредыдущий ответ не удалось разобрать как валидный JSON (ошибка: %v).\n"+
				"Верни ТОЛЬКО валидный JSON без markdown-блоков ```json и без какого-либо текста до или после JSON.",
			userPrompt, err,
		)
		retryRaw, retryErr := gw.Chat(ctx, model, systemPrompt, correctivePrompt)
		if retryErr != nil {
			return zero, retryErr
		}
		retryCleaned := stripCodeFences(retryRaw)
		if err2 := json.Unmarshal([]byte(retryCleaned), &result); err2 != nil {
			return zero, fmt.Errorf("posts: invalid JSON from AI after retry: %w", err2)
		}
		return result, nil
	}
}

func stripCodeFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "```") {
		if idx := strings.Index(trimmed, "\n"); idx != -1 {
			trimmed = trimmed[idx+1:]
		} else {
			trimmed = strings.TrimPrefix(trimmed, "```")
		}
		trimmed = strings.TrimSpace(trimmed)
		if strings.HasSuffix(trimmed, "```") {
			trimmed = strings.TrimSuffix(trimmed, "```")
		}
		trimmed = strings.TrimSpace(trimmed)
	}
	if start := strings.IndexByte(trimmed, '{'); start != -1 {
		if end := strings.LastIndexByte(trimmed, '}'); end > start {
			return trimmed[start : end+1]
		}
	}
	return trimmed
}

func appendDraftContext(b *strings.Builder, draft *PostDraft) {
	if draft.RawIdea != "" {
		fmt.Fprintf(b, "Идея: %s\n", draft.RawIdea)
	}
	if draft.VisionSummary != "" {
		fmt.Fprintf(b, "Описание исходного изображения: %s\n", draft.VisionSummary)
	}
	if draft.Goal != "" {
		fmt.Fprintf(b, "Цель: %s\n", draft.Goal)
	}
	if draft.Tone != "" {
		fmt.Fprintf(b, "Тон: %s\n", draft.Tone)
	}
}

func appendBrandContext(b *strings.Builder, profile *BrandProfile) {
	if profile == nil {
		return
	}
	b.WriteString("Профиль бренда:\n")
	if profile.Name != "" {
		fmt.Fprintf(b, "- Название: %s\n", profile.Name)
	}
	if profile.Niche != "" {
		fmt.Fprintf(b, "- Ниша: %s\n", profile.Niche)
	}
	if profile.TargetAudience != "" {
		fmt.Fprintf(b, "- Целевая аудитория: %s\n", profile.TargetAudience)
	}
	if profile.ToneOfVoice != "" {
		fmt.Fprintf(b, "- Tone of voice: %s\n", profile.ToneOfVoice)
	}
	if profile.ForbiddenWords != "" {
		fmt.Fprintf(b, "- Запрещённые слова (не использовать!): %s\n", profile.ForbiddenWords)
	}
}

func appendPlatformSpecs(b *strings.Builder, specs []PlatformSpec) {
	if len(specs) == 0 {
		return
	}
	b.WriteString("Требования платформ:\n")
	for _, spec := range specs {
		fmt.Fprintf(
			b,
			"- %s (%s): max_chars=%d, markup=%s, hashtags=%s, style=%s\n",
			spec.ID, spec.DisplayName, spec.MaxChars, spec.MarkupStyle, spec.HashtagHint, spec.StylePrompt,
		)
	}
}

func filterSpecsForTarget(specs []PlatformSpec, target string, isSingleTarget bool) []PlatformSpec {
	if !isSingleTarget {
		return specs
	}
	var filtered []PlatformSpec
	for _, s := range specs {
		if strings.ToLower(s.ID) == target {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	return specs
}

func findSelectedHook(draft *PostDraft) *PostHook {
	if draft.SelectedHookID == "" {
		return nil
	}
	for i := range draft.Hooks {
		if draft.Hooks[i].ID == draft.SelectedHookID {
			return &draft.Hooks[i]
		}
	}
	return nil
}
