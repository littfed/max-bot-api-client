package main

import (
	"context"
	"strings"
	"testing"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestHoodEngineAndDeveloperMode(t *testing.T) {
	hood := NewHoodEngine()
	hood.Register("test_action", TechCard{
		Title:       "Тестовая операция",
		Endpoint:    "/chats/123/messages",
		Method:      "POST",
		FSMState:    "STATE_TESTING",
		Description: "Описание внутренней логики для разработчиков",
		Payload:     `{"text":"hello"}`,
	})

	userID := int64(777)

	// 1. Initial state - dev mode is OFF
	if hood.IsDevMode(userID) {
		t.Fatal("expected dev mode to be off initially")
	}

	formatted := hood.Format(userID, "Привет, мир!", "test_action")
	if formatted != "Привет, мир!" {
		t.Fatalf("expected plain text when dev mode is off, got: %s", formatted)
	}

	// 2. Toggle dev mode ON
	enabled := hood.ToggleDevMode(userID)
	if !enabled || !hood.IsDevMode(userID) {
		t.Fatal("expected dev mode to be toggled on")
	}

	formattedWithBadge := hood.Format(userID, "Привет, мир!", "test_action")
	if !strings.Contains(formattedWithBadge, "Режим разработчика: Под капотом") {
		t.Fatalf("expected dev mode badge in formatted message, got:\n%s", formattedWithBadge)
	}
	if !strings.Contains(formattedWithBadge, "POST /chats/123/messages") {
		t.Fatalf("expected API endpoint in formatted message, got:\n%s", formattedWithBadge)
	}

	// 3. Technical Card Callback Handler
	router := bot.NewRouter()
	hood.RegisterRoutes(router)

	var answerToast string
	ctx := bot.NewContext(context.Background(), nil, &maxbot.Update{
		Callback: &maxbot.Callback{
			CallbackID: "cb_hood_1",
			Payload:    "hood:test_action",
			User:       &maxbot.User{UserID: userID},
		},
	})

	// Inject test answer callback hook or mock
	if err := router.Handle(ctx); err != nil {
		t.Fatalf("router.Handle failed: %v", err)
	}
	_ = answerToast
}

func TestMainMenuKeyboard(t *testing.T) {
	hood := NewHoodEngine()
	userID := int64(123)

	kbOff := BuildMainMenuKeyboard(hood, userID)
	if kbOff == nil || len(kbOff.Rows()) < 5 {
		t.Fatalf("unexpected keyboard structure: %+v", kbOff)
	}

	// Dev mode button should say ВЫКЛ
	var devBtnFound bool
	for _, row := range kbOff.Rows() {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Режим разработчика") {
				devBtnFound = true
				if !strings.Contains(btn.Text, "ВЫКЛ") {
					t.Errorf("expected ВЫКЛ in dev button, got %s", btn.Text)
				}
			}
		}
	}
	if !devBtnFound {
		t.Fatal("dev mode toggle button not found in main menu")
	}

	// Flip dev mode ON
	hood.ToggleDevMode(userID)
	kbOn := BuildMainMenuKeyboard(hood, userID)
	for _, row := range kbOn.Rows() {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Режим разработчика") {
				if !strings.Contains(btn.Text, "ВКЛ") {
					t.Errorf("expected ВКЛ in dev button, got %s", btn.Text)
				}
			}
		}
	}
}
