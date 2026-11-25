package bot

import (
	"context"
	"regexp"
	"testing"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

type stubFSMState string

func (s stubFSMState) GetState() (string, error) {
	return string(s), nil
}

func TestRouter_AllMatchers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		register func(r *Router, hit *bool)
		makeCtx  func() *Context
		wantHit  bool
	}{
		{
			name:     "OnCommand_matches_with_slash_and_case",
			register: func(r *Router, hit *bool) { r.OnCommand("/Help", func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCreated,
					Message:    &maxbot.Message{Body: maxbot.MessageBody{Text: "/help arg"}},
				})
			},
			wantHit: true,
		},
		{
			name:     "OnText_case_insensitive_trimmed",
			register: func(r *Router, hit *bool) { r.OnText("  Привет  ", func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCreated,
					Message:    &maxbot.Message{Body: maxbot.MessageBody{Text: "привет"}},
				})
			},
			wantHit: true,
		},
		{
			name:     "OnTextRegex_matches_pattern",
			register: func(r *Router, hit *bool) { r.OnTextRegex(regexp.MustCompile(`^order-\d+$`), func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCreated,
					Message:    &maxbot.Message{Body: maxbot.MessageBody{Text: "order-42"}},
				})
			},
			wantHit: true,
		},
		{
			name:     "OnStart_matches_bot_started",
			register: func(r *Router, hit *bool) { r.OnStart(func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{UpdateType: maxbot.UpdateTypeBotStarted})
			},
			wantHit: true,
		},
		{
			name:     "OnStart_matches_slash_start",
			register: func(r *Router, hit *bool) { r.OnStart(func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCreated,
					Message:    &maxbot.Message{Body: maxbot.MessageBody{Text: "/start ref1"}},
				})
			},
			wantHit: true,
		},
		{
			name:     "OnIdleMessage_matches_plain_text_without_state",
			register: func(r *Router, hit *bool) { r.OnIdleMessage(func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCreated,
					Message:    &maxbot.Message{Body: maxbot.MessageBody{Text: "просто текст"}},
				})
			},
			wantHit: true,
		},
		{
			name:     "OnIdleMessage_skips_when_in_fsm_state",
			register: func(r *Router, hit *bool) { r.OnIdleMessage(func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				c := NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCreated,
					Message:    &maxbot.Message{Body: maxbot.MessageBody{Text: "просто текст"}},
				})
				c.Set("fsm_context", stubFSMState("AWAITING_NAME"))
				return c
			},
			wantHit: false,
		},
		{
			name:     "OnCallback_matches_any_callback",
			register: func(r *Router, hit *bool) { r.OnCallback(func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCallback,
					Callback:   &maxbot.Callback{Payload: "any"},
				})
			},
			wantHit: true,
		},
		{
			name:     "OnCallbackPayload_exact_match",
			register: func(r *Router, hit *bool) { r.OnCallbackPayload("menu:main", func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCallback,
					Callback:   &maxbot.Callback{Payload: "menu:main"},
				})
			},
			wantHit: true,
		},
		{
			name:     "OnCallbackPrefix_prefix_match",
			register: func(r *Router, hit *bool) { r.OnCallbackPrefix("shop:", func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCallback,
					Callback:   &maxbot.Callback{Payload: "shop:buy:1"},
				})
			},
			wantHit: true,
		},
		{
			name:     "OnAction_matches_tracked_action",
			register: func(r *Router, hit *bool) { r.OnAction("buy_item", func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCallback,
					Callback:   &maxbot.Callback{Payload: "trk:buy_item|screen=shop"},
				})
			},
			wantHit: true,
		},
		{
			name:     "OnState_matches_active_state",
			register: func(r *Router, hit *bool) { r.OnState("STEP_1", func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				c := NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCreated,
					Message:    &maxbot.Message{Body: maxbot.MessageBody{Text: "input"}},
				})
				c.Set("fsm_context", stubFSMState("STEP_1"))
				return c
			},
			wantHit: true,
		},
		{
			name:     "OnStateCallback_matches_state_and_payload",
			register: func(r *Router, hit *bool) { r.OnStateCallback("STEP_1", "confirm", func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				c := NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCallback,
					Callback:   &maxbot.Callback{Payload: "confirm"},
				})
				c.Set("fsm_context", stubFSMState("STEP_1"))
				return c
			},
			wantHit: true,
		},
		{
			name:     "OnBotAdded",
			register: func(r *Router, hit *bool) { r.OnBotAdded(func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{UpdateType: maxbot.UpdateTypeBotAdded})
			},
			wantHit: true,
		},
		{
			name:     "OnBotRemoved",
			register: func(r *Router, hit *bool) { r.OnBotRemoved(func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{UpdateType: maxbot.UpdateTypeBotRemoved})
			},
			wantHit: true,
		},
		{
			name:     "OnUserAdded",
			register: func(r *Router, hit *bool) { r.OnUserAdded(func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{UpdateType: maxbot.UpdateTypeUserAdded})
			},
			wantHit: true,
		},
		{
			name:     "OnUserRemoved",
			register: func(r *Router, hit *bool) { r.OnUserRemoved(func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{UpdateType: maxbot.UpdateTypeUserRemoved})
			},
			wantHit: true,
		},
		{
			name:     "OnAudio",
			register: func(r *Router, hit *bool) { r.OnAudio(func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCreated,
					Message: &maxbot.Message{
						Body: maxbot.MessageBody{
							Attachments: []maxbot.Attachment{{Type: maxbot.UploadTypeAudio, Payload: map[string]any{"url": "https://ex.com/a.ogg"}}},
						},
					},
				})
			},
			wantHit: true,
		},
		{
			name:     "OnFile",
			register: func(r *Router, hit *bool) { r.OnFile(func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCreated,
					Message: &maxbot.Message{
						Body: maxbot.MessageBody{
							Attachments: []maxbot.Attachment{{Type: maxbot.UploadTypeFile, Payload: map[string]any{"name": "doc.pdf"}}},
						},
					},
				})
			},
			wantHit: true,
		},
		{
			name:     "OnAny",
			register: func(r *Router, hit *bool) { r.OnAny(func(c *Context) error { *hit = true; return nil }) },
			makeCtx: func() *Context {
				return NewContext(context.Background(), nil, &maxbot.Update{UpdateType: "unknown_event"})
			},
			wantHit: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := NewRouter()
			var hit bool
			tc.register(r, &hit)
			if err := r.Handle(tc.makeCtx()); err != nil {
				t.Fatalf("Handle failed: %v", err)
			}
			if hit != tc.wantHit {
				t.Errorf("hit = %v, want %v", hit, tc.wantHit)
			}
		})
	}
}

func TestRouter_FirstMatchPriority(t *testing.T) {
	t.Parallel()

	r := NewRouter()
	var matched string
	r.OnCommand("ping", func(c *Context) error {
		matched = "command"
		return nil
	})
	r.OnAny(func(c *Context) error {
		matched = "fallback"
		return nil
	})

	ctx := NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message:    &maxbot.Message{Body: maxbot.MessageBody{Text: "/ping"}},
	})
	if err := r.Handle(ctx); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if matched != "command" {
		t.Fatalf("expected first matching route 'command', got %q", matched)
	}
}
