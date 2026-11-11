package livetest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestReporterAndHTMLViewer(t *testing.T) {
	summary := SuiteSummary{
		Total:     2,
		Passed:    1,
		Failed:    1,
		Duration:  1234 * time.Millisecond,
		StartTime: time.Now().Add(-1234 * time.Millisecond),
		EndTime:   time.Now(),
	}

	records := []*TestRecord{
		{
			Name:        "Test_SamplePass",
			Category:    "api",
			Description: "Sample pass test",
			Status:      StatusPassed,
			Duration:    200 * time.Millisecond,
			RTT:         180 * time.Millisecond,
			Events: []TranscriptEvent{
				{
					Timestamp: time.Now(),
					Direction: DirectionOutbound,
					Title:     "POST /messages",
					Text:      "Hello world",
					Passed:    true,
				},
			},
		},
		{
			Name:        "Test_SampleFail",
			Category:    "lifecycle",
			Description: "Sample fail test",
			Status:      StatusFailed,
			Duration:    100 * time.Millisecond,
			ErrorMsg:    "network timeout",
		},
	}

	bot := &maxbot.User{
		UserID:   536754869,
		Name:     "КомБот",
		Username: "id452401070481_bot",
		IsBot:    true,
	}

	// 1. Markdown generation
	md := GenerateMarkdownReport(summary, records)
	if !strings.Contains(md, "MAX Bot API Live Test Report") {
		t.Errorf("markdown missing title")
	}
	if !strings.Contains(md, "Test_SamplePass") || !strings.Contains(md, "Test_SampleFail") {
		t.Errorf("markdown missing test records")
	}

	// 2. HTML generation
	html := GenerateHTMLReport(summary, records, bot)
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Errorf("HTML missing doctype")
	}
	if !strings.Contains(html, "КомБот") {
		t.Errorf("HTML missing bot name")
	}
	if !strings.Contains(html, "network timeout") {
		t.Errorf("HTML missing error message")
	}

	// 3. File saving
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "reports", "test_report.md")
	htmlPath := filepath.Join(tmpDir, "reports", "test_report.html")

	if err := SaveMarkdownReport(mdPath, summary, records); err != nil {
		t.Fatalf("SaveMarkdownReport failed: %v", err)
	}
	if _, err := os.Stat(mdPath); err != nil {
		t.Errorf("markdown file was not created: %v", err)
	}

	if err := SaveHTMLReport(htmlPath, summary, records, bot); err != nil {
		t.Fatalf("SaveHTMLReport failed: %v", err)
	}
	if _, err := os.Stat(htmlPath); err != nil {
		t.Errorf("HTML file was not created: %v", err)
	}
}
