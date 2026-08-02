package handlers

import (
	"context"

	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
)

type StartHandler struct {
	*BaseHandler
}

func NewStartHandler(b types.BotInterface) *StartHandler {
	return &StartHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *StartHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	// Show the main menu instead of static text
	h.bot.ShowMainMenu(ctx, msg.Chat.ID, session)
	return nil
}
