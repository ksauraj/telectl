package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/telectl/internal/types"
)

type RestartHandler struct {
	*BaseHandler
}

func NewRestartHandler(b types.BotInterface) *RestartHandler {
	return &RestartHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *RestartHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *types.UserSession) error {
	if len(args) < 2 {
		h.sendResponse(msg.Chat.ID, "Usage: /restart deployment <name> [-n namespace]")
		return nil
	}

	resource := strings.ToLower(args[0])
	name := args[1]
	namespace := h.getNamespace(session, args[2:], h.getConfig().Kubernetes.DefaultNamespace)

	if resource != "deployment" && resource != "deploy" {
		h.sendResponse(msg.Chat.ID, "Only deployments can be restarted")
		return nil
	}

	client := h.getK8sClient()

	if h.getConfig().Kubernetes.DryRun {
		h.sendResponse(msg.Chat.ID, fmt.Sprintf("🔄 [DRY RUN] Would restart deployment %s/%s", namespace, name))
		return nil
	}

	err := client.RestartDeployment(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to restart deployment: %w", err)
	}

	h.sendResponse(msg.Chat.ID, fmt.Sprintf("🔄 Restarted deployment %s/%s", namespace, name))
	return nil
}

func (h *RestartHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}

type ScaleHandler struct {
	*BaseHandler
}

func NewScaleHandler(b types.BotInterface) *ScaleHandler {
	return &ScaleHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *ScaleHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *types.UserSession) error {
	if len(args) < 3 {
		h.sendResponse(msg.Chat.ID, "Usage: /scale deployment <name> <replicas> [-n namespace]")
		return nil
	}

	resource := strings.ToLower(args[0])
	name := args[1]
	replicasStr := args[2]
	namespace := h.getNamespace(session, args[3:], h.getConfig().Kubernetes.DefaultNamespace)

	if resource != "deployment" && resource != "deploy" {
		h.sendResponse(msg.Chat.ID, "Only deployments can be scaled")
		return nil
	}

	replicas, err := strconv.ParseInt(replicasStr, 10, 32)
	if err != nil {
		h.sendResponse(msg.Chat.ID, "Invalid replicas value")
		return nil
	}

	if replicas < 0 {
		h.sendResponse(msg.Chat.ID, "Replicas must be >= 0")
		return nil
	}

	client := h.getK8sClient()

	if h.getConfig().Kubernetes.DryRun {
		h.sendResponse(msg.Chat.ID, fmt.Sprintf("📈 [DRY RUN] Would scale deployment %s/%s to %d replicas", namespace, name, replicas))
		return nil
	}

	err = client.ScaleDeployment(ctx, namespace, name, int32(replicas))
	if err != nil {
		return fmt.Errorf("failed to scale deployment: %w", err)
	}

	h.sendResponse(msg.Chat.ID, fmt.Sprintf("📈 Scaled deployment %s/%s to %d replicas", namespace, name, replicas))
	return nil
}

func (h *ScaleHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}