package guard

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestCaptchaGateVerificationFlow(t *testing.T) {
	var verifiedUser int64
	var kickedUser int64

	gate := NewCaptchaGate(nil,
		WithTimeout(100*time.Millisecond),
		WithOnVerified(func(ctx context.Context, chatID, userID int64) {
			verifiedUser = userID
		}),
		WithOnKicked(func(ctx context.Context, chatID, userID int64) {
			kickedUser = userID
		}),
	)

	ctx := context.Background()
	chatID := int64(1000)
	userID := int64(2000)

	// 1. Issue challenge
	if err := gate.HandleNewMember(ctx, chatID, userID, "Алексей"); err != nil {
		t.Fatalf("HandleNewMember failed: %v", err)
	}

	// 2. Someone else attempts to click the button
	strangerContext := bot.NewContext(ctx, nil, &maxbot.Update{
		Callback: &maxbot.Callback{
			CallbackID: "cb_stranger",
			Payload:    "guard:v:1000:2000",
			User:       &maxbot.User{UserID: 9999}, // stranger
		},
	})

	handled, err := gate.VerifyCallback(strangerContext)
	if err != nil || !handled {
		t.Fatalf("expected handled=true without error, got handled=%v err=%v", handled, err)
	}
	if verifiedUser != 0 {
		t.Fatalf("stranger should not be verified")
	}

	// 3. Right user clicks the button
	ownerContext := bot.NewContext(ctx, nil, &maxbot.Update{
		Callback: &maxbot.Callback{
			CallbackID: "cb_owner",
			Payload:    "guard:v:1000:2000",
			User:       &maxbot.User{UserID: 2000}, // authorized owner
		},
	})

	handled, err = gate.VerifyCallback(ownerContext)
	if err != nil || !handled {
		t.Fatalf("expected handled=true, got handled=%v err=%v", handled, err)
	}

	if verifiedUser != userID {
		t.Fatalf("expected verifiedUser=%d, got %d", userID, verifiedUser)
	}
	if kickedUser != 0 {
		t.Fatalf("user should not have been kicked")
	}
}

func TestCaptchaGateTimeoutKick(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	var kickedUser int64

	gate := NewCaptchaGate(nil,
		WithTimeout(25*time.Millisecond),
		WithOnKicked(func(ctx context.Context, chatID, userID int64) {
			kickedUser = userID
			wg.Done()
		}),
	)

	ctx := context.Background()
	if err := gate.HandleNewMember(ctx, 500, 600, "SpamBot"); err != nil {
		t.Fatalf("HandleNewMember failed: %v", err)
	}

	// Wait for kick timeout
	wg.Wait()

	if kickedUser != 600 {
		t.Fatalf("expected kickedUser=600, got %d", kickedUser)
	}
}

func TestPostSchedulerLifecycle(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	var publishedMID string

	scheduler := NewPostScheduler(nil,
		WithOnPostSuccess(func(ctx context.Context, post *ScheduledPost, mid string) {
			publishedMID = mid
			wg.Done()
		}),
	)
	defer scheduler.Stop()

	// 1. Queue a post scheduled for 20ms in the future
	post := ScheduledPost{
		ID:        "post_promo_1",
		ChatID:    -1001234567,
		Text:      "🌟 Специальное предложение только сегодня!",
		Format:    "markdown",
		PublishAt: time.Now().Add(20 * time.Millisecond),
	}

	if err := scheduler.Schedule(post); err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	pending := scheduler.ListPending()
	if len(pending) != 1 || pending[0].ID != "post_promo_1" {
		t.Fatalf("expected 1 pending post, got %v", pending)
	}

	// 2. Wait for dispatch
	wg.Wait()

	if publishedMID != "mock_mid_post_promo_1" {
		t.Fatalf("expected mock_mid_post_promo_1, got %s", publishedMID)
	}

	// ListPending should now be empty
	if len(scheduler.ListPending()) != 0 {
		t.Fatalf("expected empty pending list after publication")
	}

	// 3. Test cancellation
	post2 := ScheduledPost{
		ID:        "post_cancel",
		ChatID:    111,
		Text:      "Cancelled",
		PublishAt: time.Now().Add(1 * time.Hour),
	}
	_ = scheduler.Schedule(post2)
	cancelled := scheduler.Cancel("post_cancel")
	if !cancelled {
		t.Fatalf("expected post cancellation to succeed")
	}
}
