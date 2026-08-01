package handlers

import (
	"context"
	"fmt"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/k8s-telegram-bot/internal/bot"
)

type VersionHandler struct {
	*BaseHandler
}

func NewVersionHandler(b *bot.Bot) *VersionHandler {
	return &VersionHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *VersionHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	version, err := h.getK8sClient().GetServerVersion(ctx)
	if err != nil {
		version = "unknown"
	}

	info := fmt.Sprintf(`*k8s-telegram-bot v0.1.0*

*Kubernetes Server Version:* %s
*Bot:* k8s-telegram-bot
*Language:* Go
*Client-go:* v0.30.5

*Configuration:*
• Kubeconfig: %s
• Default Namespace: %s
• Dry Run: %v`, version, h.bot.config.Kubernetes.KubeconfigPath, h.bot.config.Kubernetes.DefaultNamespace, h.bot.config.Kubernetes.DryRun)

	h.bot.sendMarkdown(msg.Chat.ID, info)
	return nil
}