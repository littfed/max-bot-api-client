package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/internal/livetest"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// ANSI color escape codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

func main() {
	cfg := livetest.LoadConfig()

	// Flags
	flagInteractive := flag.Bool("interactive", true, "Enable interactive Human-in-the-Loop tests")
	flagCleanup := flag.Bool("cleanup", true, "Automatically delete sent test messages")
	flagRenderHTML := flag.Bool("render-html", true, "Generate HTML and Markdown reports")
	flagScreenshot := flag.Bool("screenshot", true, "Generate PNG screenshot using headless browser")
	flagUserID := flag.Int64("user-id", cfg.TestUserID, "Target user ID for interactive tests")
	flagChatID := flag.Int64("chat-id", cfg.TestChatID, "Target dialog/chat ID")
	flagTimeout := flag.Duration("timeout", 30*time.Second, "Timeout for interactive test actions")
	flag.Parse()

	cfg.Interactive = *flagInteractive
	cfg.Cleanup = *flagCleanup
	cfg.RenderHTML = *flagRenderHTML
	if *flagUserID != 0 {
		cfg.TestUserID = *flagUserID
	}
	if *flagChatID != 0 {
		cfg.TestChatID = *flagChatID
	}

	printHeader()

	if cfg.Token == "" {
		fmt.Printf("%s❌ Ошибка:%s MAX_BOT_TOKEN не задан. Укажите его в .env или в переменной окружения.\n", colorRed, colorReset)
		os.Exit(1)
	}

	suite, err := livetest.NewSuite(cfg)
	if err != nil {
		fmt.Printf("%s❌ Ошибка инициализации клиента:%s %v\n", colorRed, colorReset, err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Auto-discovery if interactive is requested but no user ID is configured
	if cfg.Interactive && cfg.TestUserID == 0 {
		fmt.Printf("%s⚠️ TEST_USER_ID не задан.%s Вход в режим авто-определения тестировщика...\n", colorYellow, colorReset)
		fmt.Printf("👉 Откройте бота в приложении MAX и %sотправьте любое сообщение%s (например: /start или привет)\n", colorBold, colorReset)

		matcher, err := livetest.NewMatcher(ctx, suite.Client)
		if err == nil {
			waitCtx, waitCancel := context.WithTimeout(ctx, 45*time.Second)
			update, _, waitErr := matcher.WaitFor(waitCtx, func(u *maxbot.Update) bool {
				return u.UpdateType == maxbot.UpdateTypeMessageCreated && u.Message != nil && u.Message.Sender.UserID != 0
			})
			waitCancel()

			if waitErr == nil && update != nil {
				cfg.TestUserID = update.Message.Sender.UserID
				cfg.TestChatID = update.Message.Recipient.ChatID
				fmt.Printf("%s✅ Тестировщик определен:%s %s (User ID: %d, Chat ID: %d)\n\n",
					colorGreen, colorReset, update.Message.Sender.Name, cfg.TestUserID, cfg.TestChatID)
			} else {
				fmt.Printf("%s⏰ Сообщение не получено в течение 45 секунд. Интерактивные тесты будут пропущены.%s\n\n", colorYellow, colorReset)
				cfg.Interactive = false
			}
		}
	}

	// Register test suite
	suite.Register(
		&livetest.TestCaseGetMe{},
		&livetest.TestCaseSetCommands{},
		&livetest.TestCaseUploadMedia{},
		&livetest.TestCaseMessageLifecycle{},
		&livetest.TestCaseInteractiveCallback{Timeout: *flagTimeout},
	)

	// Live progress handler
	caseIndex := 0
	suite.OnProgress = func(rec *livetest.TestRecord) {
		caseIndex++
		statusBadge := formatStatusBadge(rec.Status)
		dots := strings.Repeat(".", max(2, 36-len(rec.Name)))

		rttStr := ""
		if rec.RTT > 0 {
			rttStr = fmt.Sprintf(" %s(RTT: %s)%s", colorDim, rec.RTT.Round(time.Millisecond), colorReset)
		}

		fmt.Printf(" [%d/%d] %s%-30s%s %s %s (%s)%s\n",
			caseIndex, len(suite.Cases), colorBold, rec.Name, colorReset, dots, statusBadge,
			rec.Duration.Round(time.Millisecond), rttStr)

		if rec.ErrorMsg != "" && rec.Status != livetest.StatusSkipped {
			fmt.Printf("       %s↳ Ошибка: %s%s\n", colorRed, rec.ErrorMsg, colorReset)
		}
	}

	fmt.Printf("%s▶ Запуск набора тестов...%s\n\n", colorCyan, colorReset)

	records, summary := suite.RunAll(ctx)

	printSummaryTable(records, summary)

	// Generate and save reports
	if cfg.RenderHTML {
		timestamp := time.Now().Format("20060102_150405")
		mdPath := filepath.Join(cfg.ReportDir, fmt.Sprintf("live_test_%s.md", timestamp))
		htmlPath := filepath.Join(cfg.ReportDir, fmt.Sprintf("live_test_%s.html", timestamp))

		if err := livetest.SaveMarkdownReport(mdPath, summary, records); err == nil {
			fmt.Printf("\n%s📄 Markdown отчет:%s %s\n", colorGreen, colorReset, mdPath)
		}
		if err := livetest.SaveHTMLReport(htmlPath, summary, records, suite.BotUser); err == nil {
			fmt.Printf("%s🌐 HTML Веб-чат отчет:%s %s\n", colorGreen, colorReset, htmlPath)

			if *flagScreenshot {
				pngPath := filepath.Join(cfg.ReportDir, "screenshots", fmt.Sprintf("chat_%s.png", timestamp))
				if err := livetest.CaptureHTMLScreenshot(htmlPath, pngPath); err == nil {
					fmt.Printf("%s📸 Скриншот диалога (PNG):%s %s\n", colorGreen, colorReset, pngPath)
				} else {
					fmt.Printf("%sℹ️ Скриншот пропущен:%s %v\n", colorDim, colorReset, err)
				}
			}
		}
	}

	if summary.Failed > 0 {
		os.Exit(1)
	}
}

func printHeader() {
	fmt.Printf("\n%s%s==============================================================%s\n", colorBold, colorBlue, colorReset)
	fmt.Printf("%s%s   🧪 MAX Bot API Live Tester — Модуль живого тестирования   %s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s==============================================================%s\n\n", colorBold, colorBlue, colorReset)
}

func formatStatusBadge(status livetest.TestStatus) string {
	switch status {
	case livetest.StatusPassed:
		return fmt.Sprintf("%s✅ PASS%s", colorGreen, colorReset)
	case livetest.StatusFailed:
		return fmt.Sprintf("%s❌ FAIL%s", colorRed, colorReset)
	case livetest.StatusTimeout:
		return fmt.Sprintf("%s⏰ TIMEOUT%s", colorYellow, colorReset)
	case livetest.StatusSkipped:
		return fmt.Sprintf("%s⏭️ SKIP%s", colorDim, colorReset)
	default:
		return string(status)
	}
}

func printSummaryTable(records []*livetest.TestRecord, summary livetest.SuiteSummary) {
	fmt.Printf("\n%s%s--------------------------------------------------------------%s\n", colorBold, colorBlue, colorReset)
	fmt.Printf("%s ИТОГИ ТЕСТИРОВАНИЯ:%s Всего: %d | %sПройдено: %d%s | %sОшибок: %d%s | %sТаймаутов: %d%s | %sПропущено: %d%s | Время: %s\n",
		colorBold, colorReset, summary.Total,
		colorGreen, summary.Passed, colorReset,
		colorRed, summary.Failed, colorReset,
		colorYellow, summary.Timeout, colorReset,
		colorDim, summary.Skipped, colorReset,
		summary.Duration.Round(time.Millisecond))
	fmt.Printf("%s%s--------------------------------------------------------------%s\n", colorBold, colorBlue, colorReset)
}
