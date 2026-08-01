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

type CommandHandler interface {
	Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error
}

type BaseHandler struct {
	bot *bot.Bot
}

func NewBaseHandler(b *bot.Bot) *BaseHandler {
	return &BaseHandler{bot: b}
}

func (h *BaseHandler) getNamespace(session *bot.UserSession, args []string, defaultNS string) string {
	// Check for -n or --namespace flag
	for i, arg := range args {
		if arg == "-n" || arg == "--namespace" {
			if i+1 < len(args) {
				return args[i+1]
			}
		}
		if strings.HasPrefix(arg, "-n=") {
			return strings.TrimPrefix(arg, "-n=")
		}
		if strings.HasPrefix(arg, "--namespace=") {
			return strings.TrimPrefix(arg, "--namespace=")
		}
	}
	// Check session
	session.mu.RLock()
	ns := session.CurrentNS
	session.mu.RUnlock()
	if ns != "" {
		return ns
	}
	return defaultNS
}

func (h *BaseHandler) getContext(session *bot.UserSession) string {
	session.mu.RLock()
	ctx := session.CurrentCtx
	session.mu.RUnlock()
	return ctx
}

func (h *BaseHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}

func (h *BaseHandler) sendError(chatID int64, err error) {
	h.bot.sendMessage(chatID, fmt.Sprintf("❌ Error: %s", err.Error()))
}

func (h *BaseHandler) sendFormatted(chatID int64, resources []k8s.ResourceInfo, format string, wide bool) {
	output := formatters.FormatResourceList(resources, format, wide)
	h.sendResponse(chatID, output)
}

func (h *BaseHandler) sendSingleResource(chatID int64, resource *k8s.ResourceInfo, format string) {
	output := formatters.FormatResource(resource, format)
	h.sendResponse(chatID, output)
}

// Helper to parse common flags
func parseFlags(args []string) (namespace, output, selector, fieldSelector string, remaining []string) {
	namespace = ""
	output = ""
	selector = ""
	fieldSelector = ""
	remaining = []string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-n" || arg == "--namespace":
			if i+1 < len(args) {
				namespace = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "-n="):
			namespace = strings.TrimPrefix(arg, "-n=")
		case strings.HasPrefix(arg, "--namespace="):
			namespace = strings.TrimPrefix(arg, "--namespace=")
		case arg == "-o" || arg == "--output":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "-o="):
			output = strings.TrimPrefix(arg, "-o=")
		case strings.HasPrefix(arg, "--output="):
			output = strings.TrimPrefix(arg, "--output=")
		case arg == "-l" || arg == "--selector":
			if i+1 < len(args) {
				selector = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "-l="):
			selector = strings.TrimPrefix(arg, "-l=")
		case strings.HasPrefix(arg, "--selector="):
			selector = strings.TrimPrefix(arg, "--selector=")
		case arg == "--field-selector":
			if i+1 < len(args) {
				fieldSelector = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--field-selector="):
			fieldSelector = strings.TrimPrefix(arg, "--field-selector=")
		case arg == "--all-namespaces" || arg == "-A":
			namespace = ""
		default:
			remaining = append(remaining, arg)
		}
	}
	return
}

func (h *BaseHandler) getK8sClient() *k8s.Client {
	return h.bot.k8sClient
}

func (h *BaseHandler) getLogger() *zap.Logger {
	return h.bot.logger
}