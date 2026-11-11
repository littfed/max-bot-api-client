package gma

import (
	"testing"
	"time"
)

func TestSessionManager_RingBuffer(t *testing.T) {
	sm := NewSessionManager(3, 1*time.Hour)
	userID := int64(1001)

	sm.AddUserMessage(userID, "msg 1")
	sm.AddAssistantMessage(userID, "resp 1")
	sm.AddUserMessage(userID, "msg 2")

	hist := sm.GetHistory(userID)
	if len(hist) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(hist))
	}
	if hist[0].TextContent() != "msg 1" {
		t.Errorf("expected msg 1 at 0, got %s", hist[0].TextContent())
	}

	// Adding 4th message should slide window and drop msg 1
	sm.AddAssistantMessage(userID, "resp 2")
	hist = sm.GetHistory(userID)
	if len(hist) != 3 {
		t.Fatalf("expected 3 messages after trimming, got %d", len(hist))
	}
	if hist[0].TextContent() != "resp 1" {
		t.Errorf("expected resp 1 at index 0 after drop, got %s", hist[0].TextContent())
	}
	if hist[2].TextContent() != "resp 2" {
		t.Errorf("expected resp 2 at index 2, got %s", hist[2].TextContent())
	}

	// Test Clear
	sm.Clear(userID)
	hist = sm.GetHistory(userID)
	if len(hist) != 0 {
		t.Errorf("expected empty history after Clear, got %d messages", len(hist))
	}
}

func TestSessionManager_ModelPreferences(t *testing.T) {
	sm := NewSessionManager(5, 1*time.Hour)
	userID := int64(2002)

	chat, vis, img := sm.GetModels(userID)
	if chat != DefaultChatModel || vis != DefaultVisionModel || img != DefaultImageModel {
		t.Errorf("expected default models, got %s, %s, %s", chat, vis, img)
	}

	sm.SetChatModel(userID, "custom-llm")
	sm.SetVisionModel(userID, "custom-vision")
	sm.SetImageModel(userID, "custom-img")

	chat, vis, img = sm.GetModels(userID)
	if chat != "custom-llm" || vis != "custom-vision" || img != "custom-img" {
		t.Errorf("updated models mismatch: %s, %s, %s", chat, vis, img)
	}
}

func TestSessionManager_TTLExpiration(t *testing.T) {
	sm := NewSessionManager(5, 10*time.Millisecond)
	userID := int64(3003)

	sm.AddUserMessage(userID, "hello")
	if len(sm.GetHistory(userID)) != 1 {
		t.Fatalf("expected 1 message")
	}

	time.Sleep(20 * time.Millisecond)
	if len(sm.GetHistory(userID)) != 0 {
		t.Errorf("expected history to expire after TTL")
	}
}
