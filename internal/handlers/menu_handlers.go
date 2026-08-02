package handlers

import (
	"context"

	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
)

type ResourcesHandler struct {
	*BaseHandler
}

func NewResourcesHandler(b types.BotInterface) *ResourcesHandler {
	return &ResourcesHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *ResourcesHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	// This will trigger the menu system to show resource types
	h.bot.ShowResourceTypes(ctx, msg.Chat.ID, session)
	return nil
}

func (h *ResourcesHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}

type MonitorHandler struct {
	*BaseHandler
}

func NewMonitorHandler(b types.BotInterface) *MonitorHandler {
	return &MonitorHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *MonitorHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	h.bot.ShowMonitor(ctx, msg.Chat.ID, session)
	return nil
}

func (h *MonitorHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}

type OperationsHandler struct {
	*BaseHandler
}

func NewOperationsHandler(b types.BotInterface) *OperationsHandler {
	return &OperationsHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *OperationsHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	h.bot.ShowOperations(ctx, msg.Chat.ID, session)
	return nil
}

func (h *OperationsHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}

type SettingsHandler struct {
	*BaseHandler
}

func NewSettingsHandler(b types.BotInterface) *SettingsHandler {
	return &SettingsHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *SettingsHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	h.bot.ShowSettings(ctx, msg.Chat.ID, session)
	return nil
}

func (h *SettingsHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}
