package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/gma"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestRegisterChatHandlers(t *testing.T) {
	gmaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}

		var req gma.ChatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		lastMsg := req.Messages[len(req.Messages)-1].TextContent()
		resp := gma.ChatCompletionResponse{
			Choices: []gma.ChatChoice{
				{
					Message: gma.ChatMessage{
						Role:    gma.RoleAssistant,
						Content: "Ответ на: " + lastMsg,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(gmaMock.Close)

	tests := []struct {
		name            string
		inputText       string
		presetHistory   []gma.ChatMessage
		expectedHistory int
		wantError       bool
	}{
		{
			name:            "plain text query dispatches to model and appends to memory",
			inputText:       "Расскажи про Go",
			expectedHistory: 2, // 1 user + 1 assistant
		},
		{
			name:      "/clear command wipes existing conversation history",
			inputText: "/clear",
			presetHistory: []gma.ChatMessage{
				gma.NewTextMessage(gma.RoleUser, "старый вопрос"),
				gma.NewTextMessage(gma.RoleAssistant, "старый ответ"),
			},
			expectedHistory: 0,
		},
		{
			name:            "/help command does not pollute conversation history",
			inputText:       "/help",
			expectedHistory: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sm := gma.NewSessionManager(10, 1*time.Hour)
			userID := int64(42000)

			for _, m := range tt.presetHistory {
				sm.AddMessage(userID, m)
			}

			gmaClient := gma.NewClient(gmaMock.URL, "test-key")
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			b := bot.NewWithClient(nil)

			RegisterChatHandlers(b, gmaClient, sm, logger)

			update := &maxbot.Update{
				UpdateType: maxbot.UpdateTypeMessageCreated,
				User:       &maxbot.User{UserID: userID},
				Message: &maxbot.Message{
					Body: maxbot.MessageBody{
						Text: tt.inputText,
					},
				},
			}

			ctx := bot.NewContext(context.Background(), nil, update)
			err := b.Router().Handle(ctx)

			if (err != nil) != tt.wantError {
				t.Fatalf("Handle() error = %v, wantError = %v", err, tt.wantError)
			}

			history := sm.GetHistory(userID)
			if len(history) != tt.expectedHistory {
				t.Errorf("history length = %d, want %d", len(history), tt.expectedHistory)
			}
		})
	}
}

func TestHasImageAttachment(t *testing.T) {
	tests := []struct {
		name string
		msg  *maxbot.Message
		want bool
	}{
		{
			name: "nil message returns false",
			msg:  nil,
			want: false,
		},
		{
			name: "text only message returns false",
			msg: &maxbot.Message{
				Body: maxbot.MessageBody{Text: "Привет"},
			},
			want: false,
		},
		{
			name: "photo attachment returns true",
			msg: &maxbot.Message{
				Body: maxbot.MessageBody{
					Attachments: []maxbot.Attachment{
						{Type: "photo"},
					},
				},
			},
			want: true,
		},
		{
			name: "image attachment returns true",
			msg: &maxbot.Message{
				Body: maxbot.MessageBody{
					Attachments: []maxbot.Attachment{
						{Type: maxbot.UploadTypeImage},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasImageAttachment(tt.msg); got != tt.want {
				t.Errorf("hasImageAttachment() = %v, want %v", got, tt.want)
			}
		})
	}
}
