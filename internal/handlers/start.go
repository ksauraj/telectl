package handlers

import (
	"context"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/telectl/internal/types"
)

type StartHandler struct {
	*BaseHandler
}

func NewStartHandler(b types.BotInterface) *StartHandler {
	return &StartHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *StartHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *types.UserSession) error {
	// Show the main menu instead of static text
	h.bot.ShowMainMenu(msg.Chat.ID, session)
	return nil
}