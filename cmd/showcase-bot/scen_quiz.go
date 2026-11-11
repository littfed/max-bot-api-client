package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// QuizQuestion represents a single question item in the interactive quiz.
type QuizQuestion struct {
	ID         string
	Question   string
	Options    []string
	CorrectIdx int
}

var demoQuizQuestions = []QuizQuestion{
	{
		ID:         "q1",
		Question:   "1️⃣ Какой максимальный лимит символов в одном текстовом сообщении MAX Bot API?",
		Options:    []string{"1 024 символа", "4 096 символов (UTF-16)", "10 000 символов"},
		CorrectIdx: 1,
	},
	{
		ID:         "q2",
		Question:   "2️⃣ Какой тип кнопки используется в MAX для запуска встроенных Mini Apps?",
		Options:    []string{"web_view", "open_app", "launch_mini_app"},
		CorrectIdx: 1,
	},
	{
		ID:         "q3",
		Question:   "3️⃣ Какой криптографический алгоритм валидирует подпись initData в WebApps?",
		Options:    []string{"RSA-4096", "MD5", "HMAC-SHA256"},
		CorrectIdx: 2,
	},
}

// RegisterQuizScenario installs the interactive quiz, TTK reaction tracking, and clipboard sharing.
func RegisterQuizScenario(b *bot.Bot, hood *HoodEngine) {
	// 1. Technical cards
	hood.Register("quiz_buttons", TechCard{
		Title:       "Стили и цветовые интенты кнопок",
		Endpoint:    "/messages",
		Method:      "POST",
		Description: "MAX Messenger поддерживает 4 визуальных интента кнопок: positive (зеленый), negative (красный), primary (акцентный синий), default (серый).",
		DocsLink:    "https://dev.max.ru/docs/buttons",
	})

	hood.Register("quiz_ttk", TechCard{
		Title:       "Замер Time-to-Click (TTK) пользователя",
		Endpoint:    "/updates",
		Method:      "LONG-POLLING",
		Description: "Модуль pkg/telemetry замеряет разницу между меткой времени отправки сообщения и таймстемпом клика callback. Это позволяет оценивать скорость реакции и вовлеченность.",
		DocsLink:    "https://dev.max.ru/docs/callbacks",
	})

	hood.Register("quiz_clipboard", TechCard{
		Title:       "Копирование в буфер обмена (clipboard)",
		Endpoint:    "/messages",
		Method:      "POST",
		Description: "Кнопка с type: clipboard при нажатии нативно помещает текст из поля payload в системный буфер обмена ОС пользователя (Windows, macOS, Android, iOS) без сетевого запроса.",
		DocsLink:    "https://dev.max.ru/docs/buttons",
	})

	// 2. Launch Quiz
	b.OnCallbackPayload("scen:quiz", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)

		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.Set("quiz_score", float64(0))
			_ = fsmCtx.Set("quiz_idx", float64(0))
			_ = fsmCtx.Set("quiz_start_time", float64(time.Now().UnixMilli()))
		}

		return sendQuizQuestion(c, hood, 0)
	})

	// Answer callback: quiz:ans:<qIdx>:<optIdx>
	b.OnCallbackPrefix("quiz:ans:", func(c *bot.Context) error {
		cb := c.Callback()
		if cb == nil {
			return nil
		}

		parts := strings.Split(cb.Payload, ":")
		if len(parts) < 4 {
			return c.AnswerNotification("Ошибка формата вопроса")
		}

		qIdx, _ := strconv.Atoi(parts[2])
		optIdx, _ := strconv.Atoi(parts[3])

		if qIdx < 0 || qIdx >= len(demoQuizQuestions) {
			return c.AnswerNotification("Вопрос не найден")
		}

		q := demoQuizQuestions[qIdx]
		isCorrect := optIdx == q.CorrectIdx

		fsmCtx := fsm.FromContext(c)
		score := float64(0)
		if fsmCtx != nil {
			data, _ := fsmCtx.GetData()
			if sc, ok := data["quiz_score"].(float64); ok {
				score = sc
			}
			if isCorrect {
				score++
				_ = fsmCtx.Set("quiz_score", score)
			}
		}

		if isCorrect {
			_ = c.AnswerCallback(&maxbot.AnswerCallbackRequest{
				Notification: "✅ Абсолютно верно!",
			})
		} else {
			_ = c.AnswerCallback(&maxbot.AnswerCallbackRequest{
				Notification: "❌ Неверно! Правильный ответ был: " + q.Options[q.CorrectIdx],
			})
		}

		// Next question or Final Results
		nextIdx := qIdx + 1
		if nextIdx < len(demoQuizQuestions) {
			return sendQuizQuestion(c, hood, nextIdx)
		}

		// Final Results
		startTime := float64(time.Now().UnixMilli())
		if fsmCtx != nil {
			data, _ := fsmCtx.GetData()
			if st, ok := data["quiz_start_time"].(float64); ok {
				startTime = st
			}
			_ = fsmCtx.Reset()
		}

		totalDurationSec := float64(time.Now().UnixMilli()-int64(startTime)) / 1000.0
		if totalDurationSec <= 0 {
			totalDurationSec = 3.5
		}

		userID := int64(0)
		if cb.User != nil {
			userID = cb.User.UserID
		}

		shareText := fmt.Sprintf("Я прошел тест возможностей MAX Bot API с результатом %.0f из %d за %.1f сек! 🚀",
			score, len(demoQuizQuestions), totalDurationSec)

		resultMsg := fmt.Sprintf("🏆 **Результаты Викторины MAX!**\n\n"+
			"• **Правильных ответов:** `%.0f из %d`\n"+
			"• **Общее время прохождения:** `%.1f сек`\n"+
			"• **Средний Time-to-Click (TTK):** `%.2f сек`\n\n"+
			"💡 *Нажмите кнопку «Скопировать результат», чтобы текст скопировался в буфер обмена без перехода по ссылке!*",
			score, len(demoQuizQuestions), totalDurationSec, totalDurationSec/float64(len(demoQuizQuestions)))

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewClipboardButton("📋 Скопировать результат", shareText)).
			Row(hood.Button("quiz_clipboard")).
			Row(hood.Button("quiz_ttk")).
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		formatted := hood.Format(userID, resultMsg, "quiz_clipboard")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})
}

func sendQuizQuestion(c *bot.Context, hood *HoodEngine, qIdx int) error {
	q := demoQuizQuestions[qIdx]
	kb := maxbot.NewInlineKeyboard()

	for i, opt := range q.Options {
		payload := fmt.Sprintf("quiz:ans:%d:%d", qIdx, i)
		kb.Row(maxbot.NewCallbackButton(opt, payload))
	}
	kb.Row(hood.Button("quiz_buttons"))
	kb.Row(maxbot.NewCallbackButton("❌ Прервать квиз", "nav:menu"))

	userID := int64(0)
	if sender := c.Sender(); sender != nil {
		userID = sender.UserID
	}

	text := fmt.Sprintf("🧠 **Вопрос %d из %d**\n\n%s\n\n_Выберите правильный вариант ответа:_ ",
		qIdx+1, len(demoQuizQuestions), q.Question)

	formatted := hood.Format(userID, text, "quiz_buttons")
	_, err := c.ReplyKeyboard(formatted, kb)
	return err
}
