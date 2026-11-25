//go:build integration || live

package livetest

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/ai"
)

// TestLiveAIGatewayConnectivity tests live gateway ping if AI_GATEWAY_URL is provided in environment.
func TestLiveAIGatewayConnectivity(t *testing.T) {
	gatewayURL := strings.TrimSpace(os.Getenv("AI_GATEWAY_URL"))
	if gatewayURL == "" {
		t.Skip("Skipping live AI Gateway connectivity test: AI_GATEWAY_URL is not set")
	}

	apiKey := strings.TrimSpace(os.Getenv("AI_GATEWAY_KEY"))
	client := ai.NewClient(gatewayURL, apiKey, ai.WithTimeout(10*time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("AI Gateway at %s is not reachable: %v", gatewayURL, err)
	}
	t.Logf("AI Gateway at %s successfully reachable.", gatewayURL)
}
