package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSBPLinkGeneration(t *testing.T) {
	// 1. Dynamic SBP link
	link := GenerateSBPLink("AD10004", SBPOptions{
		Type:   SBPTypeDynamic,
		BankID: "100000000004",
		Amount: 350.00,
	})

	expectedPrefix := "https://qr.nspk.ru/AD10004?"
	if len(link) < len(expectedPrefix) || link[:len(expectedPrefix)] != expectedPrefix {
		t.Fatalf("unexpected link: %s", link)
	}

	if !bytes.Contains([]byte(link), []byte("sum=35000")) {
		t.Fatalf("expected sum=35000 in link, got: %s", link)
	}
	if !bytes.Contains([]byte(link), []byte("type=02")) {
		t.Fatalf("expected type=02 in link, got: %s", link)
	}
	if !bytes.Contains([]byte(link), []byte("bank=100000000004")) {
		t.Fatalf("expected bank in link, got: %s", link)
	}
}

func TestTBankTokenAndVerification(t *testing.T) {
	password := "SecretPass123"
	terminalKey := "MerchantTinkoff01"

	payload := map[string]any{
		"TerminalKey": terminalKey,
		"OrderId":     "ORDER-777",
		"Success":     true,
		"Status":      "CONFIRMED",
		"PaymentId":   int64(987654321),
		"ErrorCode":   "0",
		"Amount":      int64(150000), // 1500.00 RUB
		"DATA": map[string]any{
			"user_id": "456",
			"chat_id": "789",
		},
	}

	token := CalculateTBankToken(payload, password)
	if token == "" {
		t.Fatal("generated token cannot be empty")
	}

	payload["Token"] = token

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	gw := NewTBankGateway(terminalKey, password)

	// Valid request
	req := httptest.NewRequest(http.MethodPost, "/webhook/tbank", bytes.NewReader(body))
	payment, err := gw.ParseAndVerify(req)
	if err != nil {
		t.Fatalf("ParseAndVerify failed: %v", err)
	}

	if payment.Status != StatusSucceeded {
		t.Fatalf("expected status succeeded, got %s", payment.Status)
	}
	if payment.Amount != 1500.00 {
		t.Fatalf("expected amount 1500.00, got %f", payment.Amount)
	}
	if payment.UserID != 456 || payment.ChatID != 789 {
		t.Fatalf("unexpected user/chat IDs: %d / %d", payment.UserID, payment.ChatID)
	}

	// Tampered request (invalid signature)
	payload["Token"] = "bad_signature_token"
	tamperedBody, _ := json.Marshal(payload)
	tamperedReq := httptest.NewRequest(http.MethodPost, "/webhook/tbank", bytes.NewReader(tamperedBody))
	_, err = gw.ParseAndVerify(tamperedReq)
	if err == nil {
		t.Fatal("expected signature verification failure for tampered token")
	}
}

func TestYooKassaAndRouterIntegration(t *testing.T) {
	router := NewRouter()
	router.RegisterGateway(NewYooKassaGateway("test_key"))

	var fulfillmentHandled bool
	var paidAmount float64

	router.OnSuccess(func(ctx context.Context, p *Payment) error {
		fulfillmentHandled = true
		paidAmount = p.Amount
		return nil
	})

	yooPayload := `{
		"type": "notification",
		"event": "payment.succeeded",
		"object": {
			"id": "2d8f990a-000f-5000-8000-1c888d123456",
			"status": "succeeded",
			"amount": {
				"value": "299.90",
				"currency": "RUB"
			},
			"description": "Monthly VIP Subscription",
			"metadata": {
				"user_id": "111",
				"chat_id": "222"
			},
			"created_at": "2026-11-10T12:00:00.000Z"
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhook/yookassa", bytes.NewReader([]byte(yooPayload)))
	rec := httptest.NewRecorder()

	router.HTTPHandler("yookassa")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !fulfillmentHandled {
		t.Fatal("expected OnSuccess fulfillment handler to be invoked")
	}
	if paidAmount != 299.90 {
		t.Fatalf("expected paid amount 299.90, got %f", paidAmount)
	}
}
