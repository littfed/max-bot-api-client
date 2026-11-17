package webapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrMissingHash is returned when initData does not contain the required hash parameter.
	ErrMissingHash = errors.New("webapp: missing hash parameter in initData")
	// ErrInvalidSignature is returned when HMAC-SHA256 signature verification fails.
	ErrInvalidSignature = errors.New("webapp: invalid initData cryptographic signature")
	// ErrInitDataExpired is returned when auth_date is older than the configured maxAge.
	ErrInitDataExpired = errors.New("webapp: initData has expired")
)

// ComputeDataCheckString extracts parameters from raw initData query, sorts them alphabetically, and joins with newline.
func ComputeDataCheckString(rawQuery string) (checkString string, receivedHash string, params url.Values, err error) {
	vals, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", "", nil, fmt.Errorf("webapp: invalid query string: %w", err)
	}

	receivedHash = vals.Get("hash")
	if receivedHash == "" {
		return "", "", nil, ErrMissingHash
	}

	keys := make([]string, 0, len(vals))
	for k := range vals {
		if k == "hash" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, vals.Get(k)))
	}

	return strings.Join(lines, "\n"), receivedHash, vals, nil
}

// ComputeInitDataHash generates the HMAC-SHA256 signature for raw initData using botToken.
func ComputeInitDataHash(dataCheckString string, botToken string) string {
	macSecret := hmac.New(sha256.New, []byte("WebAppData"))
	macSecret.Write([]byte(botToken))
	secretKey := macSecret.Sum(nil)

	macData := hmac.New(sha256.New, secretKey)
	macData.Write([]byte(dataCheckString))
	return hex.EncodeToString(macData.Sum(nil))
}

// SignInitData generates a cryptographically signed initData query string for a user and bot token.
func SignInitData(botToken string, userID int64, firstName string, username string, queryID ...string) string {
	authDate := time.Now().Unix()
	userMap := map[string]any{
		"id": userID,
	}
	if firstName != "" {
		userMap["first_name"] = firstName
	}
	if username != "" {
		userMap["username"] = username
	}
	userJSON, _ := json.Marshal(userMap)

	qID := fmt.Sprintf("MAX_%d", authDate%100000)
	if len(queryID) > 0 && queryID[0] != "" {
		qID = queryID[0]
	}

	vals := url.Values{}
	vals.Set("query_id", qID)
	vals.Set("user", string(userJSON))
	vals.Set("auth_date", strconv.FormatInt(authDate, 10))

	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, vals.Get(k)))
	}
	checkStr := strings.Join(lines, "\n")
	hash := ComputeInitDataHash(checkStr, botToken)
	vals.Set("hash", hash)

	return vals.Encode()
}

// ValidateInitData verifies the authenticity and freshness of Mini App initData.
// If maxAge is 0, a default expiration limit of 24 hours is applied.
func ValidateInitData(rawQuery string, botToken string, maxAge ...time.Duration) (*InitData, error) {
	if botToken == "" {
		return nil, errors.New("webapp: botToken cannot be empty")
	}

	checkString, receivedHash, vals, err := ComputeDataCheckString(rawQuery)
	if err != nil {
		return nil, err
	}

	expectedHash := ComputeInitDataHash(checkString, botToken)
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(receivedHash)), []byte(strings.ToLower(expectedHash))) != 1 {
		return nil, ErrInvalidSignature
	}

	// Validate auth_date
	authDateSec, err := strconv.ParseInt(vals.Get("auth_date"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("webapp: invalid auth_date: %w", err)
	}
	authTime := time.Unix(authDateSec, 0)

	allowedAge := 24 * time.Hour
	if len(maxAge) > 0 && maxAge[0] > 0 {
		allowedAge = maxAge[0]
	}

	if time.Since(authTime) > allowedAge {
		return nil, ErrInitDataExpired
	}

	data := &InitData{
		QueryID:      vals.Get("query_id"),
		AuthDate:     authTime,
		Hash:         receivedHash,
		StartParam:   vals.Get("start_param"),
		ChatType:     vals.Get("chat_type"),
		ChatInstance: vals.Get("chat_instance"),
		Raw:          rawQuery,
	}

	if userRaw := vals.Get("user"); userRaw != "" {
		var user InitDataUser
		if err := json.Unmarshal([]byte(userRaw), &user); err == nil {
			data.User = &user
		}
	}

	return data, nil
}
