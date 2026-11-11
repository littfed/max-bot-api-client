package maxbot

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError represents an error returned by the MAX platform API.
type APIError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	RawBody    string `json:"-"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("max api error (HTTP %d, code %s): %s", e.StatusCode, e.Code, e.Message)
	}
	if e.Message != "" {
		return fmt.Sprintf("max api error (HTTP %d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("max api error (HTTP %d): %s", e.StatusCode, e.RawBody)
}

// IsAPIError returns true if err is or wraps an *APIError.
func IsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

// IsUnauthorized returns true if err indicates an authentication failure (HTTP 401).
func IsUnauthorized(err error) bool {
	if apiErr, ok := IsAPIError(err); ok {
		return apiErr.StatusCode == http.StatusUnauthorized
	}
	return false
}

// IsTooManyRequests returns true if err indicates rate limiting (HTTP 429).
func IsTooManyRequests(err error) bool {
	if apiErr, ok := IsAPIError(err); ok {
		return apiErr.StatusCode == http.StatusTooManyRequests
	}
	return false
}

// IsNotFound returns true if err indicates an entity was not found (HTTP 404).
func IsNotFound(err error) bool {
	if apiErr, ok := IsAPIError(err); ok {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}
