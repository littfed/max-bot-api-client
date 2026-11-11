package livetest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/guard"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/payments"
	"github.com/littfed/max-bot-api-client/pkg/webapp"
)

func TestAdvancedTourEndToEndIntegration(t *testing.T) {
	// 1. Setup FSM Memory Storage
	storage := fsm.NewMemoryStorage(15 * time.Minute)
	defer storage.Close()

	router := bot.NewRouter()
	router.Use(fsm.Middleware(storage))

	// 2. Setup E-Commerce Wizard
	var orderCompleted bool
	var selectedProduct string
	var shippingAddress string

	wizard := fsm.NewWizard("ecom_order", fsm.WithTimeout(5*time.Minute))

	wizard.Step("STEP_SELECT_PRODUCT", func(c *bot.Context) error {
		selectedProduct = c.Text()
		_ = fsm.FromContext(c).Set("product", selectedProduct)
		return wizard.Next(c)
	})

	wizard.Step("STEP_SHIPPING", func(c *bot.Context) error {
		shippingAddress = c.Text()
		_ = fsm.FromContext(c).Set("address", shippingAddress)
		return wizard.Next(c)
	})

	wizard.Step("STEP_PAYMENT", func(c *bot.Context) error {
		orderCompleted = true
		return nil
	})

	wizard.Register(router)

	// User initiates wizard
	senderID := int64(888)
	chatID := int64(999)

	makeUpdate := func(text string) *bot.Context {
		return bot.NewContext(context.Background(), nil, &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: senderID, FirstName: "Мария"},
				Recipient: &maxbot.MessageRecipient{ChatID: chatID},
				Body:      maxbot.MessageBody{Text: text},
			},
		})
	}

	// Start wizard
	initCtx := makeUpdate("")
	mw := fsm.Middleware(storage)
	_ = mw(func(c *bot.Context) error {
		return wizard.Start(c)
	})(initCtx)

	// Step 1: User enters product
	if err := router.Handle(makeUpdate("Умная колонка")); err != nil {
		t.Fatalf("wizard step 1 failed: %v", err)
	}
	if selectedProduct != "Умная колонка" {
		t.Fatalf("expected product 'Умная колонка', got %s", selectedProduct)
	}

	// Step 2: User enters shipping address
	if err := router.Handle(makeUpdate("Москва, Тверская 10")); err != nil {
		t.Fatalf("wizard step 2 failed: %v", err)
	}
	if shippingAddress != "Москва, Тверская 10" {
		t.Fatalf("expected address 'Москва, Тверская 10', got %s", shippingAddress)
	}

	// Step 3: Check reached payment step
	checkCtx := makeUpdate("")
	_ = mw(func(c *bot.Context) error { return nil })(checkCtx)
	idx, st, active := wizard.CurrentStep(checkCtx)
	if !active || idx != 2 || st != "STEP_PAYMENT" {
		t.Fatalf("expected step 2 STEP_PAYMENT, got idx=%d st=%s", idx, st)
	}

	// Advance to complete
	_ = router.Handle(makeUpdate("confirm"))
	if !orderCompleted {
		t.Fatalf("expected orderCompleted to be true")
	}

	// 3. SBP & Payment Router Verification
	sbpLink := payments.GenerateSBPLink("ORD-101", payments.SBPOptions{
		Type:   payments.SBPTypeDynamic,
		Amount: 4990.00,
	})
	if sbpLink == "" || !bytes.Contains([]byte(sbpLink), []byte("sum=499000")) {
		t.Fatalf("invalid sbp link: %s", sbpLink)
	}

	paymentRouter := payments.NewRouter()
	paymentRouter.RegisterGateway(payments.NewTBankGateway("TerminalLive", "SuperPassword"))

	var paymentReceived bool
	paymentRouter.OnSuccess(func(ctx context.Context, p *payments.Payment) error {
		paymentReceived = true
		if p.Amount != 4990.00 || p.UserID != senderID {
			t.Errorf("unexpected payment payload: %+v", p)
		}
		return nil
	})

	tbankPayload := map[string]any{
		"TerminalKey": "TerminalLive",
		"OrderId":     "ORD-101",
		"Success":     true,
		"Status":      "CONFIRMED",
		"PaymentId":   "998877",
		"Amount":      int64(499000),
		"DATA": map[string]any{
			"user_id": fmt.Sprintf("%d", senderID),
			"chat_id": fmt.Sprintf("%d", chatID),
		},
	}
	token := payments.CalculateTBankToken(tbankPayload, "SuperPassword")
	tbankPayload["Token"] = token

	reqBody, _ := json.Marshal(tbankPayload)
	req := httptest.NewRequest(http.MethodPost, "/webhook/tbank", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	paymentRouter.HTTPHandler("tbank")(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("payment webhook failed: %d %s", rec.Code, rec.Body.String())
	}
	if !paymentReceived {
		t.Fatal("expected payment fulfillment hook to be triggered")
	}

	// 4. Captcha Gate Verification
	var captchaVerified bool
	captcha := guard.NewCaptchaGate(nil,
		guard.WithTimeout(1*time.Second),
		guard.WithOnVerified(func(ctx context.Context, cID, uID int64) {
			captchaVerified = true
		}),
	)
	_ = captcha.HandleNewMember(context.Background(), chatID, senderID, "Мария")

	cbCtx := bot.NewContext(context.Background(), nil, &maxbot.Update{
		Callback: &maxbot.Callback{
			CallbackID: "cb_verify_1",
			Payload:    fmt.Sprintf("guard:v:%d:%d", chatID, senderID),
			User:       &maxbot.User{UserID: senderID},
		},
	})

	handled, err := captcha.VerifyCallback(cbCtx)
	if err != nil || !handled || !captchaVerified {
		t.Fatalf("captcha verification failed: handled=%v err=%v verified=%v", handled, err, captchaVerified)
	}

	// 5. Mini App Open Button & Return Link
	appBtn, err := webapp.OpenAppButton("Магазин", "ecom_max_bot", "promo_bonus")
	if err != nil || appBtn.Type != "open_app" {
		t.Fatalf("open app button generation failed: %v", err)
	}

	returnLink := webapp.GenerateReturnLink("ecom_max_bot", "order_confirmed")
	if returnLink != "https://max.ru/ecom_max_bot?start=order_confirmed" {
		t.Fatalf("unexpected return link: %s", returnLink)
	}
}
