package livetest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GenerateMarkdownReport formats the suite summary and records into GitHub-flavored Markdown.
func GenerateMarkdownReport(summary SuiteSummary, records []*TestRecord) string {
	var sb strings.Builder

	sb.WriteString("# 🧪 MAX Bot API Live Test Report\n\n")
	sb.WriteString(fmt.Sprintf("**Executed:** %s  \n", summary.StartTime.Format("2006-01-02 15:04:05 MST")))
	sb.WriteString(fmt.Sprintf("**Duration:** `%s`  \n", summary.Duration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("**Status:** Total: `%d` | Passed: `%d` | Failed: `%d` | Skipped: `%d` | Timeout: `%d`\n\n",
		summary.Total, summary.Passed, summary.Failed, summary.Skipped, summary.Timeout))

	sb.WriteString("## 📊 Test Results\n\n")
	sb.WriteString("| Status | Category | Test Case | Duration | RTT | Details |\n")
	sb.WriteString("| :---: | :--- | :--- | :---: | :---: | :--- |\n")

	for _, r := range records {
		var icon string
		switch r.Status {
		case StatusPassed:
			icon = "✅ PASS"
		case StatusFailed:
			icon = "❌ FAIL"
		case StatusTimeout:
			icon = "⏰ TIMEOUT"
		case StatusSkipped:
			icon = "⏭️ SKIP"
		default:
			icon = string(r.Status)
		}

		rttStr := "-"
		if r.RTT > 0 {
			rttStr = fmt.Sprintf("`%s`", r.RTT.Round(time.Millisecond))
		}

		detailStr := r.Description
		if r.ErrorMsg != "" {
			detailStr = fmt.Sprintf("**Error:** %s", r.ErrorMsg)
		}

		sb.WriteString(fmt.Sprintf("| %s | `%s` | **%s** | `%s` | %s | %s |\n",
			icon, r.Category, r.Name, r.Duration.Round(time.Millisecond), rttStr, detailStr))
	}

	sb.WriteString("\n## 📝 Event Transcripts\n\n")
	for _, r := range records {
		if len(r.Events) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s (`%s`)\n\n", r.Name, r.Status))
		for _, ev := range r.Events {
			dirBadge := "📤 Outbound"
			if ev.Direction == DirectionInbound {
				dirBadge = "📥 Inbound"
			} else if ev.Direction == DirectionAction {
				dirBadge = "⚡ Action"
			} else if ev.Direction == DirectionAssert {
				dirBadge = "🔍 Assert"
			}

			statusBadge := "✅"
			if !ev.Passed {
				statusBadge = "❌"
			}

			sb.WriteString(fmt.Sprintf("- **%s** `[%s]` %s **%s**: %s\n",
				ev.Timestamp.Format("15:04:05.000"), dirBadge, statusBadge, ev.Title, ev.Text))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// SaveMarkdownReport writes the Markdown report to the given file path, creating directories as needed.
func SaveMarkdownReport(filePath string, summary SuiteSummary, records []*TestRecord) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	content := GenerateMarkdownReport(summary, records)
	return os.WriteFile(filePath, []byte(content), 0644)
}
