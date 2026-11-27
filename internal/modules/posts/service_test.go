package posts

import (
	"context"
	"errors"
	"testing"
)

func TestService_FullPipelineAndGracefulImageFallback(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	gw := &mockAIGateway{
		visionResp: "Минималистичный рабочий стол с ноутбуком и чашкой кофе",
		chatResponses: []string{
			// 1. GenerateHooks for draftWithImage
			`{"hooks":[
				{"id":"h1","title":"Боль","text":"Почему выгорают авторы?","angle":"problem"},
				{"id":"h2","title":"Кейс","text":"Как мы сократили время на посты в 3 раза","angle":"case"}
			]}`,
			// 2. GeneratePackage for draftWithImage
			`{"variants":[
				{"platform":"max","text":"Пост для MAX про продуктивность","hashtags":["#продуктивность"],"cta":"Попробуйте сегодня"},
				{"platform":"telegram","text":"Пост для Telegram про продуктивность","hashtags":["#smm"],"cta":"Подписывайтесь"}
			],"image_prompt":"Minimalist desk with laptop and coffee, editorial lighting"}`,
			// 3. GenerateHooks for draftTextOnly
			`{"hooks":[
				{"id":"h1","title":"Выгода","text":"5 шагов к сильному контенту","angle":"benefit"}
			]}`,
			// 4. GeneratePackage for draftTextOnly (triggers GenerateImage which fails)
			`{"variants":[
				{"platform":"max","text":"Текстовый пост для MAX","hashtags":["#контент"],"cta":"Сохраните"}
			],"image_prompt":"Clean flat lay notebook and pen"}`,
		},
		imageErr: errors.New("image provider timeout"),
	}

	svc := NewService(store, NewPipeline(gw))
	const userID int64 = 101

	// 1. SaveProfile
	profile, err := svc.SaveProfile(ctx, userID, &BrandProfile{
		Name:             "Studio Brand",
		Niche:            "Marketing",
		TargetAudience:   "Founders",
		ToneOfVoice:      "confident",
		DefaultPlatforms: []string{"MAX", "telegram"},
		IsDefault:        true,
	})
	if err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}
	if profile.ID == 0 {
		t.Fatal("expected non-zero profile ID")
	}

	// 2. StartDraft with sourceImageURL -> Vision called, VisionSummary populated, TargetPlatforms & Tone inherited
	draftWithImage, err := svc.StartDraft(ctx, userID, "Запуск утреннего дайджеста", "https://example.com/desk.jpg")
	if err != nil {
		t.Fatalf("StartDraft(with image) error = %v", err)
	}
	if len(gw.visionCalls) != 1 {
		t.Fatalf("expected 1 Vision call, got %d", len(gw.visionCalls))
	}
	if draftWithImage.VisionSummary != gw.visionResp {
		t.Fatalf("VisionSummary = %q, want %q", draftWithImage.VisionSummary, gw.visionResp)
	}
	if draftWithImage.Tone != "confident" {
		t.Fatalf("Tone = %q, want %q", draftWithImage.Tone, "confident")
	}
	if len(draftWithImage.TargetPlatforms) != 2 || draftWithImage.TargetPlatforms[0] != "max" || draftWithImage.TargetPlatforms[1] != "telegram" {
		t.Fatalf("TargetPlatforms = %v, want [max telegram]", draftWithImage.TargetPlatforms)
	}

	// 3. UpdateBrief
	draftWithImage, err = svc.UpdateBrief(ctx, userID, draftWithImage.ID, "engagement", "friendly", "", []string{"max", "telegram"})
	if err != nil {
		t.Fatalf("UpdateBrief() error = %v", err)
	}
	if draftWithImage.Status != DraftStatusBrief || draftWithImage.Goal != "engagement" || draftWithImage.Tone != "friendly" {
		t.Fatalf("unexpected draft after UpdateBrief: %+v", draftWithImage)
	}

	// 4. GenerateHooks
	draftWithImage, err = svc.GenerateHooks(ctx, userID, draftWithImage.ID, "")
	if err != nil {
		t.Fatalf("GenerateHooks() error = %v", err)
	}
	if draftWithImage.Status != DraftStatusHooks || len(draftWithImage.Hooks) != 2 {
		t.Fatalf("unexpected draft after GenerateHooks: status=%s hooks=%d", draftWithImage.Status, len(draftWithImage.Hooks))
	}

	// 5. GeneratePackage on draft with sourceImageURL
	draftWithImage, err = svc.GeneratePackage(ctx, userID, draftWithImage.ID, "h2", "")
	if err != nil {
		t.Fatalf("GeneratePackage(with image) error = %v", err)
	}
	if draftWithImage.Status != DraftStatusReady {
		t.Fatalf("Status = %q, want %q", draftWithImage.Status, DraftStatusReady)
	}
	if len(draftWithImage.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(draftWithImage.Variants))
	}

	// 6. Text-only draft -> GeneratePackage calls GenerateImage, which fails -> graceful fallback
	draftTextOnly, err := svc.StartDraft(ctx, userID, "Гайд по контент-плану", "")
	if err != nil {
		t.Fatalf("StartDraft(text only) error = %v", err)
	}
	draftTextOnly, err = svc.UpdateBrief(ctx, userID, draftTextOnly.ID, "leads", "", "", []string{"max"})
	if err != nil {
		t.Fatalf("UpdateBrief(text only) error = %v", err)
	}
	draftTextOnly, err = svc.GenerateHooks(ctx, userID, draftTextOnly.ID, "")
	if err != nil {
		t.Fatalf("GenerateHooks(text only) error = %v", err)
	}
	draftTextOnly, err = svc.GeneratePackage(ctx, userID, draftTextOnly.ID, "h1", "")
	if err != nil {
		t.Fatalf("GeneratePackage(text only) unexpected error on image fallback: %v", err)
	}
	if len(gw.imageCalls) != 1 {
		t.Fatalf("expected 1 GenerateImage call, got %d", len(gw.imageCalls))
	}
	if draftTextOnly.Status != DraftStatusReady {
		t.Fatalf("Status = %q, want %q", draftTextOnly.Status, DraftStatusReady)
	}
	if len(draftTextOnly.Variants) != 1 {
		t.Fatalf("expected 1 variant saved, got %d", len(draftTextOnly.Variants))
	}
	if draftTextOnly.GeneratedImageURL != "" {
		t.Fatalf("expected empty GeneratedImageURL on fallback, got %q", draftTextOnly.GeneratedImageURL)
	}
}

func TestService_RefineAndManualUpdate(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	gw := &mockAIGateway{
		chatResponses: []string{
			// Refine only telegram
			`{"variants":[
				{"platform":"telegram","text":"Короткий обновлённый пост для Telegram","hashtags":["#tg"],"cta":"Жми"}
			]}`,
		},
	}

	svc := NewService(store, NewPipeline(gw))
	const userID int64 = 202

	draft := &PostDraft{
		UserID:          userID,
		Status:          DraftStatusReady,
		RawIdea:         "Запуск продукта",
		TargetPlatforms: []string{"max", "telegram"},
		Variants: []PostVariant{
			{Platform: "max", Text: "Исходный MAX", Hashtags: []string{"#max"}, CTA: "Читать"},
			{Platform: "telegram", Text: "Исходный Telegram", Hashtags: []string{"#tg"}, CTA: "Перейти"},
		},
	}
	if err := store.CreateDraft(ctx, draft); err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}

	// Refine targeted platform ("telegram")
	refined, err := svc.Refine(ctx, userID, draft.ID, "telegram", "Сделай короче", "")
	if err != nil {
		t.Fatalf("Refine() error = %v", err)
	}
	if len(refined.Variants) != 2 {
		t.Fatalf("expected 2 variants after refine, got %d", len(refined.Variants))
	}
	if refined.Variants[0].Text != "Исходный MAX" {
		t.Fatalf("expected MAX variant preserved, got %q", refined.Variants[0].Text)
	}
	if refined.Variants[1].Text != "Короткий обновлённый пост для Telegram" {
		t.Fatalf("expected Telegram variant updated, got %q", refined.Variants[1].Text)
	}

	// UpdateVariantManual updates "max" and preserves "telegram"
	updated, err := svc.UpdateVariantManual(ctx, userID, draft.ID, "max", PostVariant{
		Platform: "max",
		Text:     "Отредактированный вручную MAX",
		Hashtags: []string{"#manual"},
		CTA:      "Комментируйте",
	})
	if err != nil {
		t.Fatalf("UpdateVariantManual() error = %v", err)
	}
	if len(updated.Variants) != 2 {
		t.Fatalf("expected 2 variants after manual update, got %d", len(updated.Variants))
	}
	if updated.Variants[0].Text != "Отредактированный вручную MAX" {
		t.Fatalf("expected MAX variant manually updated, got %q", updated.Variants[0].Text)
	}
	if updated.Variants[1].Text != "Короткий обновлённый пост для Telegram" {
		t.Fatalf("expected Telegram variant preserved after manual update, got %q", updated.Variants[1].Text)
	}
}

func TestService_OwnershipCheck(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, NewPipeline(&mockAIGateway{}))

	const ownerID int64 = 1
	const otherUserID int64 = 2

	draft := &PostDraft{
		UserID:          ownerID,
		Status:          DraftStatusIdea,
		RawIdea:         "Приватная идея",
		TargetPlatforms: []string{"max"},
	}
	if err := store.CreateDraft(ctx, draft); err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}

	if _, err := svc.UpdateBrief(ctx, otherUserID, draft.ID, "sales", "formal", "", []string{"max"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateBrief() by other user: got %v, want ErrNotFound", err)
	}
	if _, err := svc.GenerateHooks(ctx, otherUserID, draft.ID, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GenerateHooks() by other user: got %v, want ErrNotFound", err)
	}
	if _, err := svc.GetState(ctx, otherUserID, draft.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetState() by other user: got %v, want ErrNotFound", err)
	}
}

func TestService_ListAvailableModels_FilteredAndFallback(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	gw := &mockAIGateway{
		modelsResp: []string{"auto", "gemini-3-8-flash", "gpt-4o-mini", "flux-dev", "whisper-1", "custom-text-model"},
	}
	svc := NewService(store, NewPipeline(gw))

	catalog, err := svc.ListAvailableModels(ctx)
	if err != nil {
		t.Fatalf("ListAvailableModels() error = %v", err)
	}
	if catalog.DefaultModel != "auto" {
		t.Fatalf("DefaultModel = %q, want %q", catalog.DefaultModel, "auto")
	}
	if len(catalog.Curated) != 3 {
		t.Fatalf("expected 3 curated models matching gateway, got %d (%+v)", len(catalog.Curated), catalog.Curated)
	}
	for _, m := range catalog.All {
		if m == "flux-dev" || m == "whisper-1" {
			t.Fatalf("expected non-text model %q to be excluded from All: %v", m, catalog.All)
		}
	}
	if len(catalog.All) != 4 {
		t.Fatalf("expected 4 text models in All, got %d (%v)", len(catalog.All), catalog.All)
	}

	// Fallback when ListModels fails
	gwErr := &mockAIGateway{modelsErr: errors.New("gateway down")}
	svcFallback := NewService(store, NewPipeline(gwErr))
	fallbackCatalog, err := svcFallback.ListAvailableModels(ctx)
	if err != nil {
		t.Fatalf("ListAvailableModels(fallback) error = %v", err)
	}
	if len(fallbackCatalog.Curated) != len(curatedTextModels) {
		t.Fatalf("expected %d curated fallback models, got %d", len(curatedTextModels), len(fallbackCatalog.Curated))
	}
}

func TestService_TextModelAndRevisionHistoryAndRestore(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	chatResps := []string{
		`{"hooks":[{"id":"h1","title":"Боль","text":"Хук","angle":"problem"}]}`,
		`{"variants":[{"platform":"max","text":"v1 max","hashtags":["#v1"],"cta":"c1"}],"image_prompt":"prompt v1"}`,
		`{"variants":[{"platform":"max","text":"v2 refined","hashtags":["#v2"],"cta":"c2"}]}`,
	}
	gw := &mockAIGateway{
		chatResponses: chatResps,
		imageResp:     "https://img.example/v1.png",
	}
	svc := NewService(store, NewPipeline(gw))
	const userID int64 = 77

	draft, err := svc.StartDraft(ctx, userID, "Тестовая идея", "")
	if err != nil {
		t.Fatalf("StartDraft() error = %v", err)
	}

	draft, err = svc.UpdateBrief(ctx, userID, draft.ID, "engagement", "friendly", "gpt-5.4", []string{"max"})
	if err != nil {
		t.Fatalf("UpdateBrief() error = %v", err)
	}
	if draft.TextModel != "gpt-5.4" {
		t.Fatalf("TextModel = %q, want gpt-5.4", draft.TextModel)
	}

	draft, err = svc.GenerateHooks(ctx, userID, draft.ID, "claude-sonnet-5-5")
	if err != nil {
		t.Fatalf("GenerateHooks() error = %v", err)
	}
	if draft.TextModel != "claude-sonnet-5-5" || gw.chatCalls[0].model != "claude-sonnet-5-5" {
		t.Fatalf("expected override model claude-sonnet-5-5, got draft=%q call=%q", draft.TextModel, gw.chatCalls[0].model)
	}

	draft, err = svc.GeneratePackage(ctx, userID, draft.ID, "h1", "deepseek-4-pro")
	if err != nil {
		t.Fatalf("GeneratePackage() error = %v", err)
	}
	if draft.TextModel != "deepseek-4-pro" || gw.chatCalls[1].model != "deepseek-4-pro" {
		t.Fatalf("expected override model deepseek-4-pro, got draft=%q call=%q", draft.TextModel, gw.chatCalls[1].model)
	}
	if len(draft.ImageHistory) != 1 || draft.ImageHistory[0].Version != 1 || draft.ImageHistory[0].URL != "https://img.example/v1.png" {
		t.Fatalf("unexpected ImageHistory after GeneratePackage: %+v", draft.ImageHistory)
	}
	if len(draft.TextHistory) != 1 || draft.TextHistory[0].Version != 1 || draft.TextHistory[0].Source != "generate" || draft.TextHistory[0].Model != "deepseek-4-pro" {
		t.Fatalf("unexpected TextHistory after GeneratePackage: %+v", draft.TextHistory)
	}

	// Regenerate image -> v2
	gw.imageResp = "https://img.example/v2.png"
	draft, err = svc.RegenerateImage(ctx, userID, draft.ID, "prompt v2")
	if err != nil {
		t.Fatalf("RegenerateImage() error = %v", err)
	}
	if len(draft.ImageHistory) != 2 || draft.ImageHistory[1].Version != 2 {
		t.Fatalf("expected 2 image revisions, got %+v", draft.ImageHistory)
	}

	// Refine -> text v2
	draft, err = svc.Refine(ctx, userID, draft.ID, "max", "Сделай короче", "qwen3.6-plus")
	if err != nil {
		t.Fatalf("Refine() error = %v", err)
	}
	if len(draft.TextHistory) != 2 || draft.TextHistory[1].Version != 2 || draft.TextHistory[1].Source != "refine" || draft.TextHistory[1].Instruction != "Сделай короче" || draft.TextHistory[1].Model != "qwen3.6-plus" {
		t.Fatalf("unexpected TextHistory after Refine: %+v", draft.TextHistory)
	}

	// Manual update -> text v3
	draft, err = svc.UpdateVariantManual(ctx, userID, draft.ID, "max", PostVariant{Platform: "max", Text: "v3 manual"})
	if err != nil {
		t.Fatalf("UpdateVariantManual() error = %v", err)
	}
	if len(draft.TextHistory) != 3 || draft.TextHistory[2].Version != 3 || draft.TextHistory[2].Source != "manual" {
		t.Fatalf("unexpected TextHistory after UpdateVariantManual: %+v", draft.TextHistory)
	}

	// Restore image v1
	draft, err = svc.RestoreRevision(ctx, userID, draft.ID, "image", 1, "")
	if err != nil {
		t.Fatalf("RestoreRevision(image v1) error = %v", err)
	}
	if draft.GeneratedImageURL != "https://img.example/v1.png" || draft.ImagePrompt != "prompt v1" {
		t.Fatalf("unexpected image after restore: url=%q prompt=%q", draft.GeneratedImageURL, draft.ImagePrompt)
	}

	// Restore text v1
	draft, err = svc.RestoreRevision(ctx, userID, draft.ID, "text", 1, "max")
	if err != nil {
		t.Fatalf("RestoreRevision(text v1) error = %v", err)
	}
	if draft.Variants[0].Text != "v1 max" {
		t.Fatalf("expected restored text 'v1 max', got %q", draft.Variants[0].Text)
	}
	if len(draft.TextHistory) != 4 || draft.TextHistory[3].Version != 4 || draft.TextHistory[3].Source != "restore" {
		t.Fatalf("expected restore entry in TextHistory, got %+v", draft.TextHistory)
	}

	// Ownership & validation checks for RestoreRevision
	if _, err := svc.RestoreRevision(ctx, 9999, draft.ID, "text", 1, "max"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for non-owner, got %v", err)
	}
	if _, err := svc.RestoreRevision(ctx, userID, draft.ID, "image", 999, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing version, got %v", err)
	}
	if _, err := svc.RestoreRevision(ctx, userID, draft.ID, "invalid", 1, ""); err == nil {
		t.Fatal("expected error for invalid restore kind")
	}

	// Cap checks: >20 images and >60 text revisions
	for i := 0; i < 22; i++ {
		gw.imageResp = "https://img.example/loop.png"
		draft, err = svc.RegenerateImage(ctx, userID, draft.ID, "loop")
		if err != nil {
			t.Fatalf("RegenerateImage loop %d error = %v", i, err)
		}
	}
	if len(draft.ImageHistory) != 20 {
		t.Fatalf("expected ImageHistory capped at 20, got %d", len(draft.ImageHistory))
	}
	if draft.ImageHistory[len(draft.ImageHistory)-1].Version != 24 {
		t.Fatalf("expected last ImageHistory version 24, got %d", draft.ImageHistory[len(draft.ImageHistory)-1].Version)
	}

	for i := 0; i < 60; i++ {
		draft, err = svc.UpdateVariantManual(ctx, userID, draft.ID, "max", PostVariant{Platform: "max", Text: "loop"})
		if err != nil {
			t.Fatalf("UpdateVariantManual loop %d error = %v", i, err)
		}
	}
	if len(draft.TextHistory) != 60 {
		t.Fatalf("expected TextHistory capped at 60, got %d", len(draft.TextHistory))
	}
	if draft.TextHistory[len(draft.TextHistory)-1].Version != 64 {
		t.Fatalf("expected last TextHistory version 64, got %d", draft.TextHistory[len(draft.TextHistory)-1].Version)
	}
}
