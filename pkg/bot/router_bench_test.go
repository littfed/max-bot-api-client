package bot

import (
	"context"
	"testing"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func BenchmarkRouter_Handle(b *testing.B) {
	r := NewRouter()
	r.OnCommand("ping", func(c *Context) error {
		return nil
	})
	r.OnCommand("start", func(c *Context) error {
		return nil
	})
	r.OnMessage(func(c *Context) error {
		return nil
	})

	update := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		User:       &maxbot.User{UserID: 100},
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Text: "/ping",
			},
		},
	}
	ctx := NewContext(context.Background(), nil, update)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Handle(ctx)
	}
}
