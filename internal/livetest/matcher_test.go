package livetest

import (
	"testing"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestCallbackPayloadMatcher(t *testing.T) {
	pred := CallbackPayloadMatcher("test_click", 12345)

	// Matching update
	matchUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCallback,
		Callback: &maxbot.Callback{
			Payload: "test_click",
			User:    &maxbot.User{UserID: 12345},
		},
	}
	if !pred(matchUpdate) {
		t.Errorf("expected update to match")
	}

	// Mismatched payload
	wrongPayload := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCallback,
		Callback: &maxbot.Callback{
			Payload: "other_click",
			User:    &maxbot.User{UserID: 12345},
		},
	}
	if pred(wrongPayload) {
		t.Errorf("expected update with wrong payload to not match")
	}

	// Mismatched user
	wrongUser := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCallback,
		Callback: &maxbot.Callback{
			Payload: "test_click",
			User:    &maxbot.User{UserID: 99999},
		},
	}
	if pred(wrongUser) {
		t.Errorf("expected update with wrong user to not match")
	}

	// Nil callback
	nilCB := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCallback,
	}
	if pred(nilCB) {
		t.Errorf("expected update with nil callback to not match")
	}
}

func TestTextMessageMatcher(t *testing.T) {
	pred := TextMessageMatcher("hello", 54321)

	// Matching update
	matchUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender: &maxbot.User{UserID: 54321},
			Body:   maxbot.MessageBody{Text: "hello"},
		},
	}
	if !pred(matchUpdate) {
		t.Errorf("expected text update to match")
	}

	// Mismatched text
	wrongText := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender: &maxbot.User{UserID: 54321},
			Body:   maxbot.MessageBody{Text: "bye"},
		},
	}
	if pred(wrongText) {
		t.Errorf("expected update with wrong text to not match")
	}
}
