package payments

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// SBPType specifies whether the SBP QR code is static or dynamic.
type SBPType string

const (
	// SBPTypeStatic represents a reusable merchant QR code.
	SBPTypeStatic SBPType = "01"
	// SBPTypeDynamic represents a one-time order QR code with fixed amount.
	SBPTypeDynamic SBPType = "02"
)

// SBPOptions configures SBP deep link generation.
type SBPOptions struct {
	Type   SBPType
	BankID string
	Amount float64 // In Rubles, e.g. 250.50
	CRC    string
}

// GenerateSBPLink constructs a standard NSPK Fast Payment System deep link.
// Format: https://qr.nspk.ru/{qr_id}?type=02&bank={bank_id}&sum={kopecks}
func GenerateSBPLink(qrID string, opts ...SBPOptions) string {
	qrID = strings.TrimPrefix(qrID, "/")
	baseURL := fmt.Sprintf("https://qr.nspk.ru/%s", qrID)

	if len(opts) == 0 {
		return baseURL
	}
	opt := opts[0]

	q := url.Values{}
	if opt.Type != "" {
		q.Set("type", string(opt.Type))
	} else {
		q.Set("type", string(SBPTypeDynamic))
	}

	if opt.BankID != "" {
		q.Set("bank", opt.BankID)
	}

	if opt.Amount > 0 {
		kopecks := int64(opt.Amount * 100)
		q.Set("sum", fmt.Sprintf("%d", kopecks))
		q.Set("cur", "RUB")
	}

	if opt.CRC != "" {
		q.Set("crc", opt.CRC)
	}

	queryString := q.Encode()
	if queryString != "" {
		return fmt.Sprintf("%s?%s", baseURL, queryString)
	}
	return baseURL
}

// UploadSBPQRCode uploads a generated QR code image (PNG bytes) to the MAX Bot API uploads endpoint.
// It returns an Attachment ready to be included in SendMessageRequest.Attachments.
func UploadSBPQRCode(ctx context.Context, client *maxbot.Client, qrImagePNG []byte, filename string) (*maxbot.Attachment, error) {
	if client == nil {
		return nil, fmt.Errorf("payments: maxbot client cannot be nil")
	}
	if len(qrImagePNG) == 0 {
		return nil, fmt.Errorf("payments: qr image bytes cannot be empty")
	}
	if filename == "" {
		filename = "sbp_qr.png"
	}

	uploadResp, err := client.UploadMedia(ctx, "image", filename, bytes.NewReader(qrImagePNG))
	if err != nil {
		return nil, fmt.Errorf("payments: failed to upload sbp qr: %w", err)
	}

	return &maxbot.Attachment{
		Type: "image",
		Payload: map[string]any{
			"token": uploadResp.Token,
		},
	}, nil
}
