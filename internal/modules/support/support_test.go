package support

import (
	"context"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

func TestSupport_AskAndTickets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := New(nil)

	if m.ID() != "support" || m.Title() == "" {
		t.Fatalf("unexpected module metadata: id=%q title=%q", m.ID(), m.Title())
	}

	tests := []struct {
		name     string
		question string
		scope    string
		wantMin  float64
	}{
		{"shop_subscription_match", "Как оформить подписку в магазине?", "shop", 0.1},
		{"global_fallback_match", "Как заказать коммерческого бота?", "global", 0.1},
		{"no_match_low_score", "квантовая физика черных дыр xyz", "shop", 0.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ans, score, err := m.Ask(ctx, tc.question, tc.scope)
			if err != nil {
				t.Fatalf("Ask failed: %v", err)
			}
			if tc.wantMin > 0 && ans == "" {
				t.Fatal("expected non-empty answer for matching query")
			}
			if score < tc.wantMin {
				t.Errorf("score = %f, want >= %f", score, tc.wantMin)
			}
		})
	}

	ticket, err := m.CreateTicket(ctx, 42, "max", "shop", "Оплата", "Не прошел платеж")
	if err != nil || ticket.ID == 0 || ticket.Status != "open" {
		t.Fatalf("CreateTicket failed: ticket=%+v err=%v", ticket, err)
	}

	tickets, err := m.GetTickets(ctx, 42)
	if err != nil || len(tickets) != 1 {
		t.Fatalf("GetTickets failed: len=%d err=%v", len(tickets), err)
	}
}

func TestSupport_Handlers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := New(nil)
	user := &store.User{ID: 88}

	res, err := m.HandleMessage(ctx, user, &core.State{ModeID: "support"}, &core.IncomingMessage{Text: "Как оформить подписку?"})
	if err != nil || res == nil || !res.Handled || res.Text == "" {
		t.Fatalf("HandleMessage failed: res=%+v err=%v", res, err)
	}

	cbRes, err := m.HandleCallback(ctx, user, &core.State{ModeID: "support"}, &core.IncomingCallback{Data: "support:faq"})
	if err != nil || cbRes == nil || !cbRes.Handled || cbRes.Text == "" {
		t.Fatalf("HandleCallback(support:faq) failed: res=%+v err=%v", cbRes, err)
	}
}
