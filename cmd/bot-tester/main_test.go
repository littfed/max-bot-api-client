package main

import (
	"strings"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/livetest"
)

func TestFormatStatusBadge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status livetest.TestStatus
		want   string
	}{
		{"passed", livetest.StatusPassed, "PASS"},
		{"failed", livetest.StatusFailed, "FAIL"},
		{"timeout", livetest.StatusTimeout, "TIMEOUT"},
		{"skipped", livetest.StatusSkipped, "SKIP"},
		{"custom", livetest.TestStatus("CUSTOM"), "CUSTOM"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatStatusBadge(tc.status)
			if !strings.Contains(got, tc.want) {
				t.Errorf("formatStatusBadge(%q) = %q, want substring %q", tc.status, got, tc.want)
			}
		})
	}
}
