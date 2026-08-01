package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/k8s-telegram-bot/internal/bot"
	"github.com/ksauraj/k8s-telegram-bot/internal/k8s"
	"github.com/ksauraj/k8s-telegram-bot/internal/utils/formatters"
	"go.uber.org/zap"
)

type StartHandler struct {
	*BaseHandler
}

func NewStartHandler(b *bot.Bot) *StartHandler {
	return &StartHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *StartHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	// Show the main menu instead of static text
	h.bot.showMainMenu(msg.Chat.ID, session)
	return nil
}