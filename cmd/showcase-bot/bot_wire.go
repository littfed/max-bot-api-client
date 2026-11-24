package main

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// convertContextToDomainEvent converts an incoming *bot.Context into a unified *core.DomainEvent.
func convertContextToDomainEvent(c *bot.Context) *core.DomainEvent {
	if c == nil || c.Update == nil {
		return nil
	}

	event := &core.DomainEvent{
		Platform:  "max",
		CreatedAt: time.Now(),
	}

	if c.Update.Callback != nil {
		event.Type = core.EventTypeCallback
		var userID int64
		var username, firstName string
		if c.Update.Callback.User != nil {
			userID = c.Update.Callback.User.UserID
			username = c.Update.Callback.User.Username
			firstName = c.Update.Callback.User.FirstName
		}
		var chatID int64
		var messageID string
		if c.Update.Callback.Message != nil {
			messageID = c.Update.Callback.Message.MessageID
			if c.Update.Callback.Message.Recipient != nil {
				chatID = c.Update.Callback.Message.Recipient.ChatID
			}
		}
		event.PlatformUserID = strconv.FormatInt(userID, 10)
		event.Username = username
		event.FirstName = firstName
		event.Callback = &core.CallbackAction{
			ID:             c.Update.Callback.CallbackID,
			ChatID:         chatID,
			MessageID:      messageID,
			Platform:       "max",
			PlatformUserID: event.PlatformUserID,
			Data:           c.Update.Callback.Payload,
		}
		return event
	}

	if c.Update.Message != nil {
		event.Type = core.EventTypeMessage
		var userID int64
		var username, firstName string
		if c.Update.Message.Sender != nil {
			userID = c.Update.Message.Sender.UserID
			username = c.Update.Message.Sender.Username
			firstName = c.Update.Message.Sender.FirstName
		}
		var chatID int64
		if c.Update.Message.Recipient != nil {
			chatID = c.Update.Message.Recipient.ChatID
		}
		text := c.Update.Message.Body.Text
		event.PlatformUserID = strconv.FormatInt(userID, 10)
		event.Username = username
		event.FirstName = firstName
		event.Message = &core.IncomingMessage{
			ID:             c.Update.Message.MessageID,
			ChatID:         chatID,
			Platform:       "max",
			PlatformUserID: event.PlatformUserID,
			Username:       username,
			FirstName:      firstName,
			Text:           text,
			CreatedAt:      time.Now(),
		}
		return event
	}

	return nil
}

// sendDomainResponse sends a *core.DomainResponse back to the user via *bot.Context.
func sendDomainResponse(c *bot.Context, resp *core.DomainResponse) error {
	if resp == nil || (!resp.Handled && resp.Text == "") {
		return nil
	}

	var kb *maxbot.InlineKeyboard
	if len(resp.Buttons) > 0 {
		kb = maxbot.NewInlineKeyboard()
		for _, row := range resp.Buttons {
			var rowBtns []maxbot.Button
			for _, btn := range row {
				if btn.URL != "" {
					rowBtns = append(rowBtns, maxbot.NewLinkButton(btn.Text, btn.URL))
				} else if btn.WebApp != "" {
					rowBtns = append(rowBtns, maxbot.NewOpenAppButton(btn.Text, btn.WebApp))
				} else {
					rowBtns = append(rowBtns, maxbot.NewCallbackButton(btn.Text, btn.Data))
				}
			}
			if len(rowBtns) > 0 {
				kb.Row(rowBtns...)
			}
		}
	}

	if c.Update != nil && c.Update.Callback != nil {
		_ = c.AnswerCallback(nil)
		if kb != nil {
			_, err := c.EditOrReplyKeyboard(resp.Text, kb)
			return err
		}
		_, err := c.Reply(resp.Text)
		return err
	}

	if kb != nil {
		_, err := c.ReplyKeyboard(resp.Text, kb)
		return err
	}
	_, err := c.Reply(resp.Text)
	return err
}

// wireBotToOrchestrator registers Bot Hub commands and callbacks to dispatch to Core Orchestrator.
func wireBotToOrchestrator(b *bot.Bot, orchestrator *core.Orchestrator, logger *slog.Logger) {
	dispatchHandler := func(c *bot.Context) error {
		ctx := c.Ctx()
		event := convertContextToDomainEvent(c)
		if event == nil {
			return nil
		}
		if errTyping := c.Typing(); errTyping != nil {
			logger.Debug("initial typing dispatch note", slog.Any("error", errTyping))
		}
		stopTyping := make(chan struct{})
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopTyping:
					return
				case <-ticker.C:
					_ = c.Typing()
				}
			}
		}()

		resp, err := orchestrator.ProcessEvent(ctx, event)
		close(stopTyping)
		if err != nil {
			logger.Error("orchestrator: error processing event", slog.Any("error", err))
			return err
		}
		return sendDomainResponse(c, resp)
	}

	// Launch and global commands routed to Orchestrator
	b.OnStart(dispatchHandler)
	b.OnCommand("mode", dispatchHandler)
	b.OnCommand("lead", dispatchHandler)
	b.OnCommand("hub", dispatchHandler)

	// Callback prefixes routed to Orchestrator
	b.OnCallbackPrefix("mode:", dispatchHandler)
	b.OnCallbackPrefix("shop:", dispatchHandler)
	b.OnCallbackPrefix("reception:", dispatchHandler)
	b.OnCallbackPrefix("support:", dispatchHandler)
	b.OnCallbackPrefix("chat:", dispatchHandler)
	b.OnCallbackPrefix("lead:", dispatchHandler)

	// Plain text messages in active conversational modes (Reception, Support, Chat, etc.)
	b.OnIdleMessage(func(c *bot.Context) error {
		userText := strings.TrimSpace(c.Text())
		if userText == "" {
			return nil
		}
		// Intercept invite links sent to the bot (let other handlers process them)
		if strings.Contains(userText, "max.ru/join/") || strings.Contains(userText, "/join/") {
			return nil
		}

		ctx := c.Ctx()
		event := convertContextToDomainEvent(c)
		if event == nil {
			return nil
		}

		if errTyping := c.Typing(); errTyping != nil {
			logger.Debug("initial typing dispatch note", slog.Any("error", errTyping))
		}
		stopTyping := make(chan struct{})
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopTyping:
					return
				case <-ticker.C:
					_ = c.Typing()
				}
			}
		}()

		resp, err := orchestrator.ProcessEvent(ctx, event)
		close(stopTyping)
		if err != nil {
			logger.Error("orchestrator: error processing plain text message", slog.Any("error", err))
			return err
		}
		if resp != nil && (resp.Handled || resp.Text != "") {
			return sendDomainResponse(c, resp)
		}
		return nil
	})
}
