package handlers

import (
	"context"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/k8s-telegram-bot/internal/bot"
)

type ResourcesHandler struct {
	*BaseHandler
}

func NewResourcesHandler(b *bot.Bot) *ResourcesHandler {
	return &ResourcesHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *ResourcesHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	// This will trigger the menu system to show resource types
	h.bot.showResourceTypes(msg.Chat.ID, session)
	return nil
}

func (h *ResourcesHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}

type MonitorHandler struct {
	*BaseHandler
}

func NewMonitorHandler(b *bot.Bot) *MonitorHandler {
	return &MonitorHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *MonitorHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	h.bot.showMonitor(msg.Chat.ID, session)
	return nil
}

func (h *MonitorHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}

type OperationsHandler struct {
	*BaseHandler
}

func NewOperationsHandler(b *bot.Bot) *OperationsHandler {
	return &OperationsHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *OperationsHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	h.bot.showOperations(msg.Chat.ID, session)
	return nil
}

func (h *OperationsHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}

type SettingsHandler struct {
	*BaseHandler
}

func NewSettingsHandler(b *bot.Bot) *SettingsHandler {
	return &SettingsHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *SettingsHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	h.bot.showSettings(msg.Chat.ID, session)
	return nil
}

func (h *SettingsHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}