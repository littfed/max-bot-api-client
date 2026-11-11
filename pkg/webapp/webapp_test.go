package webapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func buildValidInitData(botToken string, userID int64, username string, age time.Duration) string {
	authDate := time.Now().Add(-age).Unix()
	userJSON := fmt.Sprintf(`{"id":%d,"first_name":"Иван","username":"%s"}`, userID, username)

	vals := url.Values{}
	vals.Set("query_id", "AAG_TEST_123")
	vals.Set("user", userJSON)
	vals.Set("auth_date", fmt.Sprintf("%d", authDate))

	checkStr, _, _, _ := ComputeDataCheckString(vals.Encode() + "&hash=placeholder")
	hash := ComputeInitDataHash(checkStr, botToken)

	vals.Set("hash", hash)
	return vals.Encode()
}

func TestInitDataValidation(t *testing.T) {
	botToken := "secret_bot_token_xyz"
	validQuery := buildValidInitData(botToken, 100500, "ivan_test", 5*time.Minute)

	// 1. Successful verification
	data, err := ValidateInitData(validQuery, botToken, 1*time.Hour)
	if err != nil {
		t.Fatalf("ValidateInitData failed: %v", err)
	}

	if data.User == nil || data.User.ID != 100500 || data.User.Username != "ivan_test" {
		t.Fatalf("unexpected user payload: %+v", data.User)
	}
	if data.QueryID != "AAG_TEST_123" {
		t.Fatalf("unexpected query_id: %s", data.QueryID)
	}

	// 2. Tampered signature
	tamperedQuery := validQuery + "extra"
	_, err = ValidateInitData(tamperedQuery, botToken, 1*time.Hour)
	if err == nil {
		t.Fatal("expected failure on tampered initData")
	}

	// 3. Expired initData
	expiredQuery := buildValidInitData(botToken, 100500, "ivan_test", 2*time.Hour)
	_, err = ValidateInitData(expiredQuery, botToken, 1*time.Hour)
	if err != ErrInitDataExpired {
		t.Fatalf("expected ErrInitDataExpired, got %v", err)
	}
}

func TestOpenAppButtonAndReturnLink(t *testing.T) {
	// 1. Valid button with slug
	btn, err := OpenAppButton("🚀 Открыть магазин", "shop_bot", "promo_2026")
	if err != nil {
		t.Fatalf("OpenAppButton failed: %v", err)
	}
	if btn.Type != "open_app" || btn.WebApp != "shop_bot?start=promo_2026" {
		t.Fatalf("unexpected button: %+v", btn)
	}

	// 2. Erroneous URL passed to OpenAppButton
	_, err = OpenAppButton("Открыть", "https://mysite.ru/app")
	if err != ErrURLInWebAppButton {
		t.Fatalf("expected ErrURLInWebAppButton, got %v", err)
	}

	// 3. Return Link
	returnLink := GenerateReturnLink("my_super_bot", "order_paid_999")
	expectedLink := "https://max.ru/my_super_bot?start=order_paid_999"
	if returnLink != expectedLink {
		t.Fatalf("expected %s, got %s", expectedLink, returnLink)
	}
}

func TestWebBridgeHTTPHandler(t *testing.T) {
	botToken := "bridge_token_123"
	bridge := NewBridge(botToken)

	var actionReceived bool
	var orderID string

	bridge.OnAction("checkout", func(ctx context.Context, data *InitData, payload json.RawMessage) (any, error) {
		actionReceived = true
		var cart map[string]string
		_ = json.Unmarshal(payload, &cart)
		orderID = cart["order_id"]
		return map[string]string{"status": "accepted"}, nil
	})

	initData := buildValidInitData(botToken, 777, "shopper", 10*time.Second)

	reqBody, _ := json.Marshal(ActionRequest{
		InitData: initData,
		Action:   "checkout",
		Payload:  json.RawMessage(`{"order_id":"ORD-42"}`),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/webapp/action", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	bridge.HTTPHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !actionReceived || orderID != "ORD-42" {
		t.Fatalf("action was not executed properly: actionReceived=%v, orderID=%s", actionReceived, orderID)
	}
}
