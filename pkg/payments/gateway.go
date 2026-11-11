package payments

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Gateway defines the interface for verifying and parsing webhook notifications from payment providers.
type Gateway interface {
	Name() string
	ParseAndVerify(r *http.Request) (*Payment, error)
}

// --- YooKassa Gateway ---

// YooKassaGateway implements the Gateway interface for YooKassa (ЮKassa).
type YooKassaGateway struct {
	secretKey string
}

// NewYooKassaGateway creates a new YooKassa webhook processor.
func NewYooKassaGateway(secretKey string) *YooKassaGateway {
	return &YooKassaGateway{secretKey: secretKey}
}

func (g *YooKassaGateway) Name() string {
	return "yookassa"
}

type yooKassaNotification struct {
	Type   string `json:"type"`
	Event  string `json:"event"`
	Object struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Amount struct {
			Value    string `json:"value"`
			Currency string `json:"currency"`
		} `json:"amount"`
		Description string            `json:"description"`
		Metadata    map[string]string `json:"metadata"`
		CreatedAt   string            `json:"created_at"`
	} `json:"object"`
}

func (g *YooKassaGateway) ParseAndVerify(r *http.Request) (*Payment, error) {
	if r.Method != http.MethodPost {
		return nil, fmt.Errorf("payments: expected POST request, got %s", r.Method)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("payments: failed to read request body: %w", err)
	}

	var notif yooKassaNotification
	if err := json.Unmarshal(body, &notif); err != nil {
		return nil, fmt.Errorf("payments: failed to decode yookassa json: %w", err)
	}

	var status PaymentStatus
	switch notif.Event {
	case "payment.succeeded":
		status = StatusSucceeded
	case "payment.canceled":
		status = StatusCanceled
	case "payment.waiting_for_capture":
		status = StatusPending
	case "refund.succeeded":
		status = StatusRefunded
	default:
		status = PaymentStatus(notif.Object.Status)
	}

	amountVal, _ := strconv.ParseFloat(notif.Object.Amount.Value, 64)

	var userID, chatID int64
	if uid, ok := notif.Object.Metadata["user_id"]; ok {
		userID, _ = strconv.ParseInt(uid, 10, 64)
	}
	if cid, ok := notif.Object.Metadata["chat_id"]; ok {
		chatID, _ = strconv.ParseInt(cid, 10, 64)
	}

	createdAt, _ := time.Parse(time.RFC3339, notif.Object.CreatedAt)
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	return &Payment{
		ID:          notif.Object.ID,
		Gateway:     g.Name(),
		Amount:      amountVal,
		Currency:    notif.Object.Amount.Currency,
		Status:      status,
		UserID:      userID,
		ChatID:      chatID,
		Description: notif.Object.Description,
		Metadata:    notif.Object.Metadata,
		CreatedAt:   createdAt,
	}, nil
}

// --- T-Bank Gateway ---

// TBankGateway implements the Gateway interface for T-Bank (formerly Tinkoff) Acquiring.
type TBankGateway struct {
	terminalKey string
	password    string
}

// NewTBankGateway creates a new T-Bank webhook processor with SHA-256 signature verification.
func NewTBankGateway(terminalKey string, password string) *TBankGateway {
	return &TBankGateway{
		terminalKey: terminalKey,
		password:    password,
	}
}

func (g *TBankGateway) Name() string {
	return "tbank"
}

// CalculateTBankToken computes the official SHA-256 signature token according to T-Bank documentation.
func CalculateTBankToken(params map[string]any, password string) string {
	keys := make([]string, 0, len(params)+1)
	for k, v := range params {
		if k == "Token" {
			continue
		}
		// Skip nested objects or arrays (DATA, Receipt, etc.)
		switch v.(type) {
		case map[string]any, []any:
			continue
		default:
			keys = append(keys, k)
		}
	}
	keys = append(keys, "Password")
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		if k == "Password" {
			sb.WriteString(password)
		} else {
			sb.WriteString(fmt.Sprintf("%v", params[k]))
		}
	}

	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:])
}

func (g *TBankGateway) ParseAndVerify(r *http.Request) (*Payment, error) {
	if r.Method != http.MethodPost {
		return nil, fmt.Errorf("payments: expected POST request, got %s", r.Method)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("payments: failed to read request body: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("payments: invalid json: %w", err)
	}

	receivedToken, _ := raw["Token"].(string)
	if receivedToken == "" {
		return nil, errors.New("payments: missing Token in T-Bank notification")
	}

	expectedToken := CalculateTBankToken(raw, g.password)
	if subtle.ConstantTimeCompare([]byte(receivedToken), []byte(expectedToken)) != 1 {
		return nil, fmt.Errorf("payments: invalid signature token in T-Bank notification (expected %s, got %s)", expectedToken, receivedToken)
	}

	orderID, _ := raw["OrderId"].(string)
	paymentID := fmt.Sprintf("%v", raw["PaymentId"])
	if paymentID == "" {
		paymentID = orderID
	}

	// Amount is sent in kopecks
	var amount float64
	switch v := raw["Amount"].(type) {
	case json.Number:
		kop, _ := v.Float64()
		amount = kop / 100.0
	case float64:
		amount = v / 100.0
	case int64:
		amount = float64(v) / 100.0
	case string:
		kop, _ := strconv.ParseFloat(v, 64)
		amount = kop / 100.0
	}

	statusStr, _ := raw["Status"].(string)
	var status PaymentStatus
	switch statusStr {
	case "CONFIRMED":
		status = StatusSucceeded
	case "CANCELED", "REJECTED":
		status = StatusCanceled
	case "REFUNDED", "PARTIAL_REFUNDED":
		status = StatusRefunded
	default:
		status = StatusPending
	}

	metadata := make(map[string]string)
	var userID, chatID int64
	if dataMap, ok := raw["DATA"].(map[string]any); ok {
		for k, v := range dataMap {
			strVal := fmt.Sprintf("%v", v)
			metadata[k] = strVal
			if k == "user_id" {
				userID, _ = strconv.ParseInt(strVal, 10, 64)
			}
			if k == "chat_id" {
				chatID, _ = strconv.ParseInt(strVal, 10, 64)
			}
		}
	}

	return &Payment{
		ID:          paymentID,
		Gateway:     g.Name(),
		Amount:      amount,
		Currency:    "RUB",
		Status:      status,
		UserID:      userID,
		ChatID:      chatID,
		Description: fmt.Sprintf("Order #%s", orderID),
		Metadata:    metadata,
		CreatedAt:   time.Now(),
	}, nil
}
