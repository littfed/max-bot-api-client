//go:build live

package livetest_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/internal/livetest"
)

func TestLiveAPISuite(t *testing.T) {
	cfg := livetest.LoadConfig()
	if cfg.Token == "" {
		t.Skip("Skipping live test: MAX_BOT_TOKEN is not set")
	}

	suite, err := livetest.NewSuite(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize live test suite: %v", err)
	}

	suite.Register(
		&livetest.TestCaseGetMe{},
		&livetest.TestCaseSetCommands{},
		&livetest.TestCaseUploadMedia{},
		&livetest.TestCaseMessageLifecycle{},
		&livetest.TestCaseInteractiveCallback{Timeout: 5 * time.Second},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	records, summary := suite.RunAll(ctx)

	t.Logf("Summary: %s", summary.String())

	if cfg.RenderHTML {
		timestamp := time.Now().Format("20060102_150405")
		mdPath := filepath.Join(cfg.ReportDir, fmt.Sprintf("live_test_%s.md", timestamp))
		htmlPath := filepath.Join(cfg.ReportDir, fmt.Sprintf("live_test_%s.html", timestamp))

		if err := livetest.SaveMarkdownReport(mdPath, summary, records); err == nil {
			t.Logf("Saved Markdown report to: %s", mdPath)
		}
		if err := livetest.SaveHTMLReport(htmlPath, summary, records, suite.BotUser); err == nil {
			t.Logf("Saved HTML report to: %s", htmlPath)
		}
	}

	for _, rec := range records {
		t.Run(rec.Name, func(t *testing.T) {
			if rec.Status == livetest.StatusSkipped {
				t.Skip(rec.ErrorMsg)
			}
			if rec.Status != livetest.StatusPassed {
				t.Fatalf("Test %s failed (RTT=%s): %v", rec.Name, rec.RTT, rec.Error)
			}
			t.Logf("PASS [%s] in %s (RTT: %s)", rec.Name, rec.Duration, rec.RTT)
		})
	}
}
