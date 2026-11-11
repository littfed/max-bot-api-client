package livetest

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// GenerateHTMLReport produces a standalone, self-contained HTML report with a MAX-themed chat transcript.
func GenerateHTMLReport(summary SuiteSummary, records []*TestRecord, bot *maxbot.User) string {
	botName := "MAX Bot"
	botUsername := "bot"
	var botID int64
	if bot != nil {
		if bot.Name != "" {
			botName = bot.Name
		}
		if bot.Username != "" {
			botUsername = bot.Username
		}
		botID = bot.UserID
	}

	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>MAX Bot API Live Test Report</title>
<style>
  :root {
    --bg-dark: #0f172a;
    --surface: #1e293b;
    --surface-light: #334155;
    --text-primary: #f8fafc;
    --text-muted: #94a3b8;
    --primary: #3b82f6;
    --success: #10b981;
    --danger: #ef4444;
    --warning: #f59e0b;
    --bubble-bot: #1e293b;
    --bubble-user: #2563eb;
    --border: #334155;
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    background: var(--bg-dark);
    color: var(--text-primary);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    line-height: 1.5;
    padding: 24px;
  }
  .container { max-width: 1000px; margin: 0 auto; }
  .header {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 20px 24px;
    margin-bottom: 24px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 16px;
  }
  .bot-info { display: flex; align-items: center; gap: 14px; }
  .bot-avatar {
    width: 48px;
    height: 48px;
    border-radius: 50%;
    background: linear-gradient(135deg, #3b82f6, #8b5cf6);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 22px;
  }
  .bot-meta h1 { font-size: 20px; font-weight: 700; color: #fff; }
  .bot-meta p { font-size: 13px; color: var(--text-muted); }
  .stats-badges { display: flex; gap: 10px; flex-wrap: wrap; }
  .badge {
    padding: 6px 14px;
    border-radius: 9999px;
    font-size: 13px;
    font-weight: 600;
  }
  .badge-pass { background: rgba(16, 185, 129, 0.2); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.4); }
  .badge-fail { background: rgba(239, 68, 68, 0.2); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.4); }
  .badge-skip { background: rgba(148, 163, 184, 0.2); color: #cbd5e1; border: 1px solid rgba(148, 163, 184, 0.4); }
  .badge-time { background: rgba(59, 130, 246, 0.2); color: #60a5fa; border: 1px solid rgba(59, 130, 246, 0.4); }

  .card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 24px;
    margin-bottom: 24px;
  }
  .card-title {
    font-size: 18px;
    font-weight: 700;
    margin-bottom: 16px;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  table { width: 100%; border-collapse: collapse; }
  th, td { text-align: left; padding: 12px 14px; border-bottom: 1px solid var(--border); font-size: 14px; }
  th { color: var(--text-muted); font-size: 12px; text-transform: uppercase; letter-spacing: 0.05em; }
  tr:hover { background: rgba(255, 255, 255, 0.02); }

  /* Chat transcript view */
  .chat-view {
    background: #0b1120;
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .bubble-wrap { display: flex; flex-direction: column; }
  .bubble-wrap.outbound { align-items: flex-start; }
  .bubble-wrap.inbound { align-items: flex-end; }
  .bubble-wrap.action { align-items: center; }

  .bubble {
    max-width: 80%;
    padding: 12px 16px;
    border-radius: 14px;
    font-size: 14px;
    position: relative;
  }
  .bubble.outbound {
    background: var(--bubble-bot);
    border: 1px solid var(--border);
    border-bottom-left-radius: 2px;
  }
  .bubble.inbound {
    background: var(--bubble-user);
    color: #fff;
    border-bottom-right-radius: 2px;
  }
  .system-pill {
    background: rgba(51, 65, 85, 0.6);
    color: var(--text-muted);
    font-size: 12px;
    padding: 4px 14px;
    border-radius: 9999px;
  }
  .bubble-header {
    font-size: 11px;
    font-weight: 600;
    color: var(--text-muted);
    margin-bottom: 4px;
    display: flex;
    justify-content: space-between;
    gap: 12px;
  }
  .bubble-time { font-size: 10px; color: var(--text-muted); margin-top: 6px; text-align: right; }

  /* Button intents */
  .button-row { display: flex; gap: 8px; margin-top: 10px; flex-wrap: wrap; }
  .btn {
    padding: 8px 16px;
    border-radius: 8px;
    font-size: 13px;
    font-weight: 600;
    cursor: default;
    border: none;
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .btn-positive { background: #059669; color: #fff; }
  .btn-negative { background: #dc2626; color: #fff; }
  .btn-primary { background: #2563eb; color: #fff; }
  .btn-default { background: #475569; color: #f1f5f9; }

  details {
    margin-top: 8px;
    font-size: 11px;
    background: rgba(0,0,0,0.25);
    border-radius: 6px;
    padding: 6px 10px;
  }
  summary { cursor: pointer; color: var(--text-muted); font-weight: 600; }
  pre {
    margin-top: 6px;
    white-space: pre-wrap;
    word-break: break-all;
    font-family: monospace;
    color: #38bdf8;
  }
</style>
</head>
<body>
<div class="container">
  <div class="header">
    <div class="bot-info">
      <div class="bot-avatar">🤖</div>
      <div class="bot-meta">
        <h1>` + html.EscapeString(botName) + `</h1>
        <p>@` + html.EscapeString(botUsername) + ` &bull; ID: ` + fmt.Sprintf("%d", botID) + `</p>
      </div>
    </div>
    <div class="stats-badges">
      <span class="badge badge-pass">✅ ` + fmt.Sprintf("%d Passed", summary.Passed) + `</span>
      <span class="badge badge-fail">❌ ` + fmt.Sprintf("%d Failed", summary.Failed) + `</span>
      <span class="badge badge-skip">⏭️ ` + fmt.Sprintf("%d Skipped", summary.Skipped) + `</span>
      <span class="badge badge-time">⏱️ ` + fmt.Sprintf("%s", summary.Duration.Round(time.Millisecond)) + `</span>
    </div>
  </div>

  <div class="card">
    <div class="card-title">
      <span>📊 Сводная таблица проверок</span>
      <span style="font-size:12px; color:var(--text-muted);">` + summary.StartTime.Format("02.01.2006 15:04:05") + `</span>
    </div>
    <table>
      <thead>
        <tr>
          <th>Статус</th>
          <th>Категория</th>
          <th>Тест</th>
          <th>Время</th>
          <th>RTT</th>
          <th>Результат</th>
        </tr>
      </thead>
      <tbody>`)

	for _, r := range records {
		statusClass := "badge-pass"
		statusIcon := "✅ PASS"
		if r.Status == StatusFailed {
			statusClass = "badge-fail"
			statusIcon = "❌ FAIL"
		} else if r.Status == StatusSkipped {
			statusClass = "badge-skip"
			statusIcon = "⏭️ SKIP"
		} else if r.Status == StatusTimeout {
			statusClass = "badge-fail"
			statusIcon = "⏰ TIMEOUT"
		}

		rttStr := "-"
		if r.RTT > 0 {
			rttStr = r.RTT.Round(time.Millisecond).String()
		}

		detail := r.Description
		if r.ErrorMsg != "" {
			detail = `<span style="color:#f87171;">` + html.EscapeString(r.ErrorMsg) + `</span>`
		} else {
			detail = html.EscapeString(detail)
		}

		sb.WriteString(fmt.Sprintf(`
        <tr>
          <td><span class="badge %s" style="padding:3px 10px; font-size:11px;">%s</span></td>
          <td><code>%s</code></td>
          <td><strong>%s</strong></td>
          <td>%s</td>
          <td><code>%s</code></td>
          <td>%s</td>
        </tr>`, statusClass, statusIcon, html.EscapeString(r.Category), html.EscapeString(r.Name),
			r.Duration.Round(time.Millisecond), rttStr, detail))
	}

	sb.WriteString(`
      </tbody>
    </table>
  </div>

  <div class="card">
    <div class="card-title">
      <span>💬 Визуальный журнал диалога (Web-Chat View)</span>
    </div>
    <div class="chat-view">`)

	var hasEvents bool
	for _, r := range records {
		for _, ev := range r.Events {
			hasEvents = true
			timeStr := ev.Timestamp.Format("15:04:05.000")

			if ev.Direction == DirectionAction || ev.Direction == DirectionAssert {
				sb.WriteString(fmt.Sprintf(`
      <div class="bubble-wrap action">
        <span class="system-pill">⚡ %s &bull; %s: %s</span>
      </div>`, timeStr, html.EscapeString(ev.Title), html.EscapeString(ev.Text)))
				continue
			}

			isOutbound := ev.Direction == DirectionOutbound
			wrapClass := "outbound"
			bubbleClass := "outbound"
			senderLabel := botName
			if !isOutbound {
				wrapClass = "inbound"
				bubbleClass = "inbound"
				senderLabel = "Тестировщик"
			}

			payloadJSON := ""
			if ev.Payload != nil {
				if b, err := json.MarshalIndent(ev.Payload, "", "  "); err == nil {
					payloadJSON = string(b)
				}
			}

			sb.WriteString(fmt.Sprintf(`
      <div class="bubble-wrap %s">
        <div class="bubble %s">
          <div class="bubble-header">
            <span>%s</span>
            <span>%s</span>
          </div>
          <div>%s</div>`, wrapClass, bubbleClass, html.EscapeString(senderLabel), html.EscapeString(ev.Title),
				nl2br(html.EscapeString(ev.Text))))

			// Render mock buttons if payload has attachments with inline_keyboard
			renderMockButtons(&sb, ev.Payload)

			if payloadJSON != "" {
				sb.WriteString(fmt.Sprintf(`
          <details>
            <summary>JSON Payload</summary>
            <pre>%s</pre>
          </details>`, html.EscapeString(payloadJSON)))
			}

			sb.WriteString(fmt.Sprintf(`
          <div class="bubble-time">%s</div>
        </div>
      </div>`, timeStr))
		}
	}

	if !hasEvents {
		sb.WriteString(`
      <div class="bubble-wrap action">
        <span class="system-pill">Нет событий транскрипта для отображения</span>
      </div>`)
	}

	sb.WriteString(`
    </div>
  </div>
</div>
</body>
</html>`)

	return sb.String()
}

func nl2br(s string) string {
	return strings.ReplaceAll(s, "\n", "<br>")
}

func renderMockButtons(sb *strings.Builder, payload any) {
	if payload == nil {
		return
	}
	msg, ok := payload.(*maxbot.Message)
	if !ok || msg == nil {
		return
	}
	for _, att := range msg.Body.Attachments {
		if att.Type != "inline_keyboard" {
			continue
		}
		raw, err := json.Marshal(att.Payload)
		if err != nil {
			continue
		}
		var kb struct {
			Buttons [][]struct {
				Text   string `json:"text"`
				Type   string `json:"type"`
				Intent string `json:"intent"`
			} `json:"buttons"`
		}
		if err := json.Unmarshal(raw, &kb); err != nil {
			continue
		}
		for _, row := range kb.Buttons {
			sb.WriteString(`<div class="button-row">`)
			for _, btn := range row {
				intentClass := "btn-default"
				switch btn.Intent {
				case "positive":
					intentClass = "btn-positive"
				case "negative":
					intentClass = "btn-negative"
				case "primary":
					intentClass = "btn-primary"
				}
				sb.WriteString(fmt.Sprintf(`<button class="btn %s">%s</button>`, intentClass, html.EscapeString(btn.Text)))
			}
			sb.WriteString(`</div>`)
		}
	}
}

// SaveHTMLReport writes the HTML report to disk.
func SaveHTMLReport(filePath string, summary SuiteSummary, records []*TestRecord, bot *maxbot.User) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	htmlContent := GenerateHTMLReport(summary, records, bot)
	return os.WriteFile(filePath, []byte(htmlContent), 0644)
}
