package bot

import (
	"fmt"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/telectl/internal/config"
	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/menus"
	"go.uber.org/zap"
)

// SendMessage sends a plain text message.
func (b *Bot) SendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		b.logger.Error("Failed to send message", zap.Error(err))
	}
}

// SendLongMessage splits and sends a long message in chunks.
func (b *Bot) SendLongMessage(chatID int64, text string) {
	maxLen := 4096
	if len(text) <= maxLen {
		b.SendMessage(chatID, text)
		return
	}
	for len(text) > 0 {
		chunk := text
		if len(chunk) > maxLen {
			chunk = text[:maxLen]
			if idx := strings.LastIndex(chunk, "\n"); idx > maxLen/2 {
				chunk = text[:idx]
			}
		}
		b.SendMessage(chatID, chunk)
		text = text[len(chunk):]
	}
}

// SendMarkdown sends a message with Markdown parsing enabled.
func (b *Bot) SendMarkdown(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	if _, err := b.api.Send(msg); err != nil {
		// Fallback to plain text if markdown fails
		msg.ParseMode = ""
		b.api.Send(msg)
	}
}

// SendKeyboard sends a message with an inline keyboard.
func (b *Bot) SendKeyboard(chatID int64, text string, keyboard tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = &keyboard
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := b.api.Send(msg); err != nil {
		msg.ParseMode = ""
		b.api.Send(msg)
	}
}

// SendReplyKeyboard sends a message with a reply keyboard.
func (b *Bot) SendReplyKeyboard(chatID int64, text string, keyboard tgbotapi.ReplyKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
		b.logger.Error("Failed to send reply keyboard", zap.Error(err))
	}
}

// IsUserAllowed checks if the user is in the allowlist.
func (b *Bot) IsUserAllowed(userID int64) bool {
	if len(b.config.Telegram.AllowedUserIDs) == 0 {
		return true
	}
	for _, allowed := range b.config.Telegram.AllowedUserIDs {
		if allowed == userID {
			return true
		}
	}
	return false
}

// IsCommandAllowed checks if a command is allowed.
func (b *Bot) IsCommandAllowed(command string) bool {
	if len(b.config.Bot.AllowedCommands) == 0 {
		return true
	}
	cmd := strings.TrimPrefix(command, "/")
	for _, allowed := range b.config.Bot.AllowedCommands {
		if allowed == cmd {
			return true
		}
	}
	return false
}

// K8sClient returns the k8s client (typed accessor for handlers).
func (b *Bot) K8sClient() interface{} { return b.k8sClient }

// Config returns the bot config (typed accessor for handlers).
func (b *Bot) Config() interface{} { return b.config }

// API returns the Telegram BotAPI (typed accessor for handlers).
func (b *Bot) API() interface{} { return b.api }

// MenuBuilder returns the menu builder (typed accessor for handlers).
func (b *Bot) MenuBuilder() interface{} { return b.menuBuilder }

// Logger returns the zap logger (typed accessor for handlers).
func (b *Bot) Logger() interface{} { return b.logger }

// RateLimiter returns the rate limiter (typed accessor for handlers).
func (b *Bot) RateLimiter() interface{} { return b.rateLimiter }

// GetK8sClient returns the k8s client (kept for backwards compat).
func (b *Bot) GetK8sClient() interface{} { return b.k8sClient }

// GetConfig returns the bot config (kept for backwards compat).
func (b *Bot) GetConfig() interface{} { return b.config }

// GetAPI returns the Telegram BotAPI (kept for backwards compat).
func (b *Bot) GetAPI() interface{} { return b.api }

// GetMenuBuilder returns the menu builder (kept for backwards compat).
func (b *Bot) GetMenuBuilder() interface{} { return b.menuBuilder }

// GetLogger returns the zap logger (kept for backwards compat).
func (b *Bot) GetLogger() interface{} { return b.logger }

// compile-time assertion: *Bot implements BotInterface
var _ interface {
	SendMessage(int64, string)
	SendLongMessage(int64, string)
	SendMarkdown(int64, string)
	SendKeyboard(int64, string, tgbotapi.InlineKeyboardMarkup)
	SendReplyKeyboard(int64, string, tgbotapi.ReplyKeyboardMarkup)
	IsUserAllowed(int64) bool
	IsCommandAllowed(string) bool
	K8sClient() interface{}
	Config() interface{}
	API() interface{}
	MenuBuilder() interface{}
	Logger() interface{}
	RateLimiter() interface{}
} = (*Bot)(nil)

// configTyped returns *config.Config — helper used elsewhere if needed.
func (b *Bot) configTyped() *config.Config { return b.config }

// k8sClientTyped returns *k8s.Client — helper used elsewhere if needed.
func (b *Bot) k8sClientTyped() *k8s.Client { return b.k8sClient }

// menuBuilderTyped returns *menus.MenuBuilder — helper used elsewhere if needed.
func (b *Bot) menuBuilderTyped() *menus.MenuBuilder { return b.menuBuilder }

// fmtUnused prevents an import cycle warning if fmt is only used by helpers.
var _ = fmt.Sprintf
