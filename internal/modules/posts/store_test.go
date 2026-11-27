package posts

import (
	"context"
	"errors"
	"testing"
)

func TestInMemoryStore_DraftOwnershipAndProfile(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryStore()

	// 1. Upsert brand profile for user 1
	profile := &BrandProfile{
		UserID:           101,
		Name:             "Acme Tech",
		Niche:            "SaaS",
		TargetAudience:   "Developers",
		ToneOfVoice:      "Friendly and concise",
		ForbiddenWords:   "synergy",
		DefaultPlatforms: []string{"telegram", "vk"},
		IsDefault:        true,
	}
	if err := repo.UpsertProfile(ctx, profile); err != nil {
		t.Fatalf("UpsertProfile failed: %v", err)
	}
	if profile.ID == 0 {
		t.Fatal("expected UpsertProfile to assign non-zero ID")
	}

	defProf, err := repo.GetDefaultProfile(ctx, 101)
	if err != nil {
		t.Fatalf("GetDefaultProfile failed: %v", err)
	}
	if defProf.Name != "Acme Tech" {
		t.Fatalf("expected profile name 'Acme Tech', got %q", defProf.Name)
	}

	// Update existing profile
	profile.ToneOfVoice = "Bold"
	if err := repo.UpsertProfile(ctx, profile); err != nil {
		t.Fatalf("UpsertProfile update failed: %v", err)
	}
	profiles, err := repo.ListProfiles(ctx, 101)
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}
	if len(profiles) != 1 || profiles[0].ToneOfVoice != "Bold" {
		t.Fatalf("expected 1 updated profile with ToneOfVoice 'Bold', got %+v", profiles)
	}

	// 2. Create draft for user 101
	draft := &PostDraft{
		UserID:          101,
		ProfileID:       &profile.ID,
		Status:          DraftStatusIdea,
		RawIdea:         "Launch announcement for v2.0",
		TargetPlatforms: []string{"telegram"},
	}
	if err := repo.CreateDraft(ctx, draft); err != nil {
		t.Fatalf("CreateDraft failed: %v", err)
	}
	if draft.ID == 0 {
		t.Fatal("expected CreateDraft to assign non-zero ID")
	}

	// 3. Ownership isolation: user 202 cannot read or update user 101's draft
	if _, err := repo.GetDraft(ctx, 202, draft.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for another user's GetDraft, got %v", err)
	}

	otherUpdate := *draft
	otherUpdate.UserID = 202
	otherUpdate.RawIdea = "Hijacked"
	if err := repo.UpdateDraft(ctx, &otherUpdate); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for another user's UpdateDraft, got %v", err)
	}

	// Owner can read and update
	got, err := repo.GetDraft(ctx, 101, draft.ID)
	if err != nil {
		t.Fatalf("owner GetDraft failed: %v", err)
	}
	if got.RawIdea != "Launch announcement for v2.0" {
		t.Fatalf("unexpected RawIdea: %q", got.RawIdea)
	}

	got.Status = DraftStatusReady
	got.TextModel = "claude-sonnet-5-5"
	got.ImageHistory = []ImageRevision{
		{Version: 1, URL: "https://img.example/1.png", Prompt: "prompt 1"},
	}
	got.TextHistory = []TextRevision{
		{Version: 1, Platform: "telegram", Text: "Hello v1", Source: "generate", Model: "claude-sonnet-5-5"},
	}
	if err := repo.UpdateDraft(ctx, got); err != nil {
		t.Fatalf("owner UpdateDraft failed: %v", err)
	}
	reloaded, err := repo.GetDraft(ctx, 101, got.ID)
	if err != nil {
		t.Fatalf("reload GetDraft failed: %v", err)
	}
	if reloaded.TextModel != "claude-sonnet-5-5" || len(reloaded.ImageHistory) != 1 || len(reloaded.TextHistory) != 1 {
		t.Fatalf("expected history and text_model persisted, got %+v", reloaded)
	}

	// 4. ListDrafts returns only owner's drafts
	user101Drafts, err := repo.ListDrafts(ctx, 101)
	if err != nil {
		t.Fatalf("ListDrafts(101) failed: %v", err)
	}
	if len(user101Drafts) != 1 || user101Drafts[0].Status != DraftStatusReady {
		t.Fatalf("expected 1 ready draft for user 101, got %+v", user101Drafts)
	}

	user202Drafts, err := repo.ListDrafts(ctx, 202)
	if err != nil {
		t.Fatalf("ListDrafts(202) failed: %v", err)
	}
	if len(user202Drafts) != 0 {
		t.Fatalf("expected 0 drafts for user 202, got %+v", user202Drafts)
	}
}
