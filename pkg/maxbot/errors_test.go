package maxbot

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *APIError
		want string
	}{
		{
			name: "with_code_and_message",
			err: &APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "invalid_param",
				Message:    "chat_id is required",
			},
			want: "max api error (HTTP 400, code invalid_param): chat_id is required",
		},
		{
			name: "with_message_only",
			err: &APIError{
				StatusCode: http.StatusNotFound,
				Message:    "message not found",
			},
			want: "max api error (HTTP 404): message not found",
		},
		{
			name: "with_raw_body_fallback",
			err: &APIError{
				StatusCode: http.StatusBadGateway,
				RawBody:    "upstream timeout",
			},
			want: "max api error (HTTP 502): upstream timeout",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAPIError_Helpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		err          error
		wantAPI      bool
		wantUnauth   bool
		wantTooMany  bool
		wantNotFound bool
	}{
		{
			name: "401_unauthorized",
			err:  &APIError{StatusCode: http.StatusUnauthorized, Message: "bad token"},
			wantAPI: true, wantUnauth: true,
		},
		{
			name: "429_too_many_requests",
			err:  &APIError{StatusCode: http.StatusTooManyRequests, Message: "slow down"},
			wantAPI: true, wantTooMany: true,
		},
		{
			name: "404_not_found",
			err:  &APIError{StatusCode: http.StatusNotFound, Message: "missing"},
			wantAPI: true, wantNotFound: true,
		},
		{
			name: "wrapped_429_error",
			err:  fmt.Errorf("send failed: %w", &APIError{StatusCode: http.StatusTooManyRequests, Message: "rate limited"}),
			wantAPI: true, wantTooMany: true,
		},
		{
			name: "non_api_error",
			err:  errors.New("network timeout"),
		},
		{
			name: "nil_error",
			err:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, gotAPI := IsAPIError(tc.err)
			if gotAPI != tc.wantAPI {
				t.Errorf("IsAPIError() = %v, want %v", gotAPI, tc.wantAPI)
			}
			if got := IsUnauthorized(tc.err); got != tc.wantUnauth {
				t.Errorf("IsUnauthorized() = %v, want %v", got, tc.wantUnauth)
			}
			if got := IsTooManyRequests(tc.err); got != tc.wantTooMany {
				t.Errorf("IsTooManyRequests() = %v, want %v", got, tc.wantTooMany)
			}
			if got := IsNotFound(tc.err); got != tc.wantNotFound {
				t.Errorf("IsNotFound() = %v, want %v", got, tc.wantNotFound)
			}
		})
	}
}
