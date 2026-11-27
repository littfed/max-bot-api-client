package posts

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type mockAIGateway struct {
	chatResponses []string
	chatErrors    []error
	chatCalls     []mockChatCall

	visionResp  string
	visionErr   error
	visionCalls []mockVisionCall

	imageResp  string
	imageErr   error
	imageCalls []mockImageCall
}

type mockChatCall struct {
	systemPrompt string
	userPrompt   string
}

type mockVisionCall struct {
	prompt   string
	imageURL string
}

type mockImageCall struct {
	prompt      string
	aspectRatio string
}

func (m *mockAIGateway) Chat(_ context.Context, systemPrompt, userPrompt string) (string, error) {
	idx := len(m.chatCalls)
	m.chatCalls = append(m.chatCalls, mockChatCall{
		systemPrompt: systemPrompt,
		userPrompt:   userPrompt,
	})
	if idx < len(m.chatErrors) && m.chatErrors[idx] != nil {
		return "", m.chatErrors[idx]
	}
	if idx < len(m.chatResponses) {
		return m.chatResponses[idx], nil
	}
	return "", errors.New("unexpected chat call")
}

func (m *mockAIGateway) Vision(_ context.Context, prompt, imageURL string) (string, error) {
	m.visionCalls = append(m.visionCalls, mockVisionCall{
		prompt:   prompt,
		imageURL: imageURL,
	})
	return m.visionResp, m.visionErr
}

func (m *mockAIGateway) GenerateImage(_ context.Context, prompt, aspectRatio string) (string, error) {
	m.imageCalls = append(m.imageCalls, mockImageCall{
		prompt:      prompt,
		aspectRatio: aspectRatio,
	})
	return m.imageResp, m.imageErr
}

func TestPipeline_GenerateHooks_RetryOnInvalidJSON(t *testing.T) {
	gw := &mockAIGateway{
		chatResponses: []string{
			"not valid json at all {",
			`{"hooks":[
				{"id":"h1","title":"Боль","text":"Почему охваты падают?","angle":"problem"},
				{"id":"h2","title":"Кейс","text":"Как мы выросли в 3 раза","angle":"case"},
				{"id":"h3","title":"Провокация","text":"Перестаньте писать посты так","angle":"provocation"}
			]}`,
		},
	}

	p := NewPipeline(gw)
	draft := &PostDraft{
		RawIdea: "Запуск нового курса по контент-маркетингу",
		Goal:    "engagement",
		Tone:    "expert",
	}
	profile := &BrandProfile{
		Name:           "ContentLab",
		Niche:          "Marketing",
		TargetAudience: "SMM specialists",
		ToneOfVoice:    "friendly expert",
	}

	hooks, err := p.GenerateHooks(context.Background(), draft, profile)
	if err != nil {
		t.Fatalf("GenerateHooks() error = %v", err)
	}
	if len(gw.chatCalls) != 2 {
		t.Fatalf("expected 2 chat calls (1 initial + 1 retry), got %d", len(gw.chatCalls))
	}
	if len(hooks) != 3 {
		t.Fatalf("expected 3 hooks, got %d", len(hooks))
	}
	if hooks[0].ID != "h1" || hooks[1].ID != "h2" || hooks[2].ID != "h3" {
		t.Errorf("unexpected hook IDs: %+v", hooks)
	}
}

func TestPipeline_GeneratePackage_Success(t *testing.T) {
	gw := &mockAIGateway{
		chatResponses: []string{
			"```json\n" + `{
				"variants": [
					{
						"platform": "max",
						"text": "Структурированный пост для MAX",
						"hashtags": ["#маркетинг", "#контент"],
						"cta": "Подписывайтесь на канал"
					},
					{
						"platform": "vk",
						"text": "Развёрнутый пост для ВКонтакте",
						"hashtags": ["#маркетинг", "#smm", "#кейсы"],
						"cta": "Пишите в комментарии"
					}
				],
				"image_prompt": "Minimalist flat-lay workspace with notebook and coffee, warm lighting"
			}` + "\n```",
		},
	}

	p := NewPipeline(gw)
	draft := &PostDraft{
		RawIdea:        "Запуск нового курса по контент-маркетингу",
		SelectedHookID: "h1",
		Hooks: []PostHook{
			{ID: "h1", Title: "Боль", Text: "Почему охваты падают?", Angle: "problem"},
		},
		TargetPlatforms: []string{"max", "vk"},
	}

	maxSpec, _ := GetPlatformSpec("max")
	vkSpec, _ := GetPlatformSpec("vk")

	variants, imagePrompt, err := p.GeneratePackage(context.Background(), draft, nil, []PlatformSpec{maxSpec, vkSpec})
	if err != nil {
		t.Fatalf("GeneratePackage() error = %v", err)
	}
	if len(variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(variants))
	}
	if variants[0].Platform != "max" || variants[1].Platform != "vk" {
		t.Errorf("unexpected platforms: %+v", variants)
	}
	if imagePrompt != "Minimalist flat-lay workspace with notebook and coffee, warm lighting" {
		t.Errorf("unexpected imagePrompt: %q", imagePrompt)
	}
}

func TestPipeline_AnalyzeImage_And_GenerateVisual(t *testing.T) {
	gw := &mockAIGateway{
		visionResp: "На фото современный офис и ноутбук с графиками роста",
		imageResp:  "https://cdn.example.com/generated.png",
	}

	p := NewPipeline(gw)

	summary, err := p.AnalyzeImage(context.Background(), "https://cdn.example.com/source.jpg")
	if err != nil {
		t.Fatalf("AnalyzeImage() error = %v", err)
	}
	if len(gw.visionCalls) != 1 {
		t.Fatalf("expected 1 vision call, got %d", len(gw.visionCalls))
	}
	if gw.visionCalls[0].imageURL != "https://cdn.example.com/source.jpg" {
		t.Errorf("unexpected vision imageURL: %q", gw.visionCalls[0].imageURL)
	}
	if summary != gw.visionResp {
		t.Errorf("unexpected summary: %q", summary)
	}

	imgURL, err := p.GenerateVisual(context.Background(), "Minimalist office desk", "16:9")
	if err != nil {
		t.Fatalf("GenerateVisual() error = %v", err)
	}
	if len(gw.imageCalls) != 1 {
		t.Fatalf("expected 1 image call, got %d", len(gw.imageCalls))
	}
	if gw.imageCalls[0].prompt != "Minimalist office desk" || gw.imageCalls[0].aspectRatio != "16:9" {
		t.Errorf("unexpected image call: %+v", gw.imageCalls[0])
	}
	if imgURL != "https://cdn.example.com/generated.png" {
		t.Errorf("unexpected imgURL: %q", imgURL)
	}
}

func TestPipeline_RefineVariant(t *testing.T) {
	gw := &mockAIGateway{
		chatResponses: []string{
			`{
				"variants": [
					{
						"platform": "vk",
						"text": "Короткий и энергичный пост для ВКонтакте",
						"hashtags": ["#smm"],
						"cta": "Жми лайк!"
					}
				]
			}`,
			`{
				"variants": [
					{
						"platform": "max",
						"text": "Обновлённый пост для MAX",
						"hashtags": ["#max"],
						"cta": "Читайте далее"
					},
					{
						"platform": "vk",
						"text": "Обновлённый пост для ВКонтакте",
						"hashtags": ["#vk"],
						"cta": "Делитесь постом"
					}
				]
			}`,
		},
	}

	p := NewPipeline(gw)
	draft := &PostDraft{
		RawIdea: "Запуск нового курса",
		Variants: []PostVariant{
			{Platform: "max", Text: "Исходный пост для MAX", CTA: "Подпишись"},
			{Platform: "vk", Text: "Исходный длинный пост для ВКонтакте", CTA: "Комментируй"},
		},
	}
	maxSpec, _ := GetPlatformSpec("max")
	vkSpec, _ := GetPlatformSpec("vk")
	specs := []PlatformSpec{maxSpec, vkSpec}

	// Targeted refinement of "vk" should preserve "max" and update "vk".
	updated, err := p.RefineVariant(context.Background(), draft, nil, specs, "vk", "Сделай короче и энергичнее")
	if err != nil {
		t.Fatalf("RefineVariant(vk) error = %v", err)
	}
	if len(updated) != 2 {
		t.Fatalf("expected 2 variants after targeted refine, got %d", len(updated))
	}
	if updated[0].Platform != "max" || updated[0].Text != "Исходный пост для MAX" {
		t.Errorf("expected max variant to remain unchanged, got %+v", updated[0])
	}
	if updated[1].Platform != "vk" || !strings.Contains(updated[1].Text, "Короткий") {
		t.Errorf("expected vk variant to be updated, got %+v", updated[1])
	}

	// Refining all platforms when targetPlatform is empty.
	updatedAll, err := p.RefineVariant(context.Background(), draft, nil, specs, "", "Сделай тон более официальным")
	if err != nil {
		t.Fatalf("RefineVariant(all) error = %v", err)
	}
	if len(updatedAll) != 2 {
		t.Fatalf("expected 2 variants after full refine, got %d", len(updatedAll))
	}
	if updatedAll[0].Text != "Обновлённый пост для MAX" || updatedAll[1].Text != "Обновлённый пост для ВКонтакте" {
		t.Errorf("expected all variants to be updated, got %+v", updatedAll)
	}
}
