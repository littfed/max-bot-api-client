package main

import (
	"context"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/gma"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestBuildQuickActionsKeyboard(t *testing.T) {
	kb := BuildQuickActionsKeyboard()
	if kb == nil {
		t.Fatal("expected non-nil keyboard")
	}
	if len(kb.Rows()) != 2 {
		t.Fatalf("expected 2 keyboard rows, got %d", len(kb.Rows()))
	}
}

func TestRegisterMenuHandlers(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		expectedChat  string
		expectedVis   string
		expectedImage string
	}{
		{
			name:          "switch chat model to garda-core",
			payload:       "set:chat:garda-core",
			expectedChat:  "garda-core",
			expectedVis:   gma.DefaultVisionModel,
			expectedImage: gma.DefaultImageModel,
		},
		{
			name:          "switch vision model to glm-4.6-plus",
			payload:       "set:vision:glm-4.6-plus",
			expectedChat:  gma.DefaultChatModel,
			expectedVis:   "glm-4.6-plus",
			expectedImage: gma.DefaultImageModel,
		},
		{
			name:          "switch image model to sdxl",
			payload:       "set:image:sdxl",
			expectedChat:  gma.DefaultChatModel,
			expectedVis:   gma.DefaultVisionModel,
			expectedImage: "sdxl",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sm := gma.NewSessionManager(10, 1*time.Hour)
			b := bot.NewWithClient(nil)
			RegisterMenuHandlers(b, sm)

			userID := int64(12345)
			update := &maxbot.Update{
				Callback: &maxbot.Callback{
					Payload: tt.payload,
					User:    &maxbot.User{UserID: userID},
				},
			}

			ctx := bot.NewContext(context.Background(), nil, update)
			err := b.Router().Handle(ctx)
			if err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			chat, vis, img := sm.GetModels(userID)
			if chat != tt.expectedChat {
				t.Errorf("chat model = %q, want %q", chat, tt.expectedChat)
			}
			if vis != tt.expectedVis {
				t.Errorf("vision model = %q, want %q", vis, tt.expectedVis)
			}
			if img != tt.expectedImage {
				t.Errorf("image model = %q, want %q", img, tt.expectedImage)
			}
		})
	}
}
