package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/k8s-telegram-bot/internal/bot"
	"github.com/ksauraj/k8s-telegram-bot/internal/k8s"
	"github.com/ksauraj/k8s-telegram-bot/internal/utils/formatters"
)

type ContextsHandler struct {
	*BaseHandler
}

func NewContextsHandler(b *bot.Bot) *ContextsHandler {
	return &ContextsHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *ContextsHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	client := h.getK8sClient()
	kubeconfig := client.GetKubeconfig()

	if kubeconfig == nil {
		h.sendResponse(msg.Chat.ID, "❌ Kubeconfig not loaded")
		return nil
	}

	contexts := kubeconfig.Contexts
	if len(contexts) == 0 {
		h.sendResponse(msg.Chat.ID, "📭 No contexts found in kubeconfig")
		return nil
	}

	// Use the new menu system's keyboard
	keyboard := h.bot.menuBuilder.GetContextsInlineKeyboard(contexts)
	output := formatters.FormatContexts(contexts)
	h.bot.sendKeyboard(msg.Chat.ID, output, keyboard)
	return nil
}

type UseContextHandler struct {
	*BaseHandler
}

func NewUseContextHandler(b *bot.Bot) *UseContextHandler {
	return &UseContextHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *UseContextHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	if len(args) == 0 {
		h.sendResponse(msg.Chat.ID, "Usage: /use-context <context-name>")
		return nil
	}

	contextName := args[0]
	client := h.getK8sClient()

	err := client.SwitchContext(contextName)
	if err != nil {
		return fmt.Errorf("failed to switch context: %w", err)
	}

	session.mu.Lock()
	session.CurrentCtx = contextName
	session.mu.Unlock()

	h.sendResponse(msg.Chat.ID, fmt.Sprintf("✅ Switched to context: %s", contextName))
	return nil
}

type ConfigHandler struct {
	*BaseHandler
}

func NewConfigHandler(b *bot.Bot) *ConfigHandler {
	return &ConfigHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *ConfigHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	client := h.getK8sClient()
	kc := client.GetKubeconfig()

	currentCtx := "none"
	if current := kc.GetCurrentContext(); current != nil {
		currentCtx = current.Name
	}

	session.mu.RLock()
	currentNS := session.CurrentNS
	session.mu.RUnlock()

	config := fmt.Sprintf(`⚙️ *Bot Configuration*

*Kubernetes:*
• Kubeconfig: %s
• Current Context: %s
• Available Contexts: %d
• Default Namespace: %s
• Session Namespace: %s
• Dry Run Mode: %v

*Telegram:*
• Bot: @%s
• Parse Mode: %s
• Rate Limit: %d/min

*Bot Settings:*
• Max Message Length: %d
• Command Prefix: %s
• Markdown Enabled: %v`, 
		h.bot.config.Kubernetes.KubeconfigPath,
		currentCtx,
		len(kc.Contexts),
		h.bot.config.Kubernetes.DefaultNamespace,
		currentNS,
		h.bot.config.Kubernetes.DryRun,
		h.bot.api.Self.UserName,
		h.bot.config.Telegram.ParseMode,
		h.bot.config.Bot.RateLimit,
		h.bot.config.Bot.MaxMessageLength,
		h.bot.config.Bot.CommandPrefix,
		h.bot.config.Bot.EnableMarkdown,
	)

	h.bot.sendMarkdown(msg.Chat.ID, config)
	return nil
}

type PortForwardHandler struct {
	*BaseHandler
}

func NewPortForwardHandler(b *bot.Bot) *PortForwardHandler {
	return &PortForwardHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *PortForwardHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	if len(args) < 2 {
		h.sendResponse(msg.Chat.ID, "Usage: /portforward <pod> <local:remote> [-n namespace]")
		return nil
	}

	podName := args[0]
	portSpec := args[1]
	namespace := h.getNamespace(session, args[2:], h.bot.config.Kubernetes.DefaultNamespace)

	// Parse port spec (local:remote)
	parts := strings.Split(portSpec, ":")
	if len(parts) != 2 {
		h.sendResponse(msg.Chat.ID, "Invalid port format. Use local:remote (e.g., 8080:80)")
		return nil
	}

	localPort, err := strconv.Atoi(parts[0])
	if err != nil {
		h.sendResponse(msg.Chat.ID, "Invalid local port")
		return nil
	}

	remotePort, err := strconv.Atoi(parts[1])
	if err != nil {
		h.sendResponse(msg.Chat.ID, "Invalid remote port")
		return nil
	}

	h.sendResponse(msg.Chat.ID, fmt.Sprintf("🔌 Port forwarding %s/%s %d -> %d\n(Not fully implemented - would start background port-forward)", namespace, podName, localPort, remotePort))
	return nil
}

func (h *ContextsHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}

func (h *UseContextHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}

func (h *ConfigHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}

func (h *PortForwardHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}