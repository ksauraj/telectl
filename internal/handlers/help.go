package handlers

import (
	"context"

	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
)

type HelpHandler struct {
	*BaseHandler
}

func NewHelpHandler(b types.BotInterface) *HelpHandler {
	return &HelpHandler{BaseHandler: NewBaseHandler(b)}
}

// Handle renders the shared command reference. The text lives in formatters so
// the Help button in the menus shows the same thing without a second copy.
func (h *HelpHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	h.bot.SendHTML(msg.Chat.ID, formatters.HelpText)
	return nil
}
