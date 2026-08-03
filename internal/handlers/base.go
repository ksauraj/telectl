package handlers

import (
	"context"
	"strings"

	"github.com/ksauraj/telectl/internal/config"
	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"go.uber.org/zap"
)

// Flag spellings accepted by the command parsers, and the one output format
// name compared against in more than one place.
const (
	flagNamespaceShort = "-n"
	flagNamespaceLong  = "--namespace"
	formatWide         = "wide"
)

type CommandHandler interface {
	Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error
}

type BaseHandler struct {
	bot types.BotInterface
}

func NewBaseHandler(b types.BotInterface) *BaseHandler {
	return &BaseHandler{bot: b}
}

func (h *BaseHandler) getNamespace(session *types.UserSession, args []string, defaultNS string) string {
	// Check for -n or --namespace flag
	for i, arg := range args {
		if arg == flagNamespaceShort || arg == flagNamespaceLong {
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
	ns := session.GetNamespace()
	if ns != "" {
		return ns
	}
	return defaultNS
}

func (h *BaseHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}

// Helper to parse common flags.
func parseFlags(args []string) (namespace, output, selector, fieldSelector string, remaining []string) {
	namespace = ""
	output = ""
	selector = ""
	fieldSelector = ""
	remaining = []string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == flagNamespaceShort || arg == flagNamespaceLong:
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
	return namespace, output, selector, fieldSelector, remaining
}

func (h *BaseHandler) getK8sClient() *k8s.Client {
	if c, ok := h.bot.K8sClient().(*k8s.Client); ok {
		return c
	}
	return nil
}

func (h *BaseHandler) getLogger() *zap.Logger {
	if l, ok := h.bot.Logger().(*zap.Logger); ok {
		return l
	}
	return nil
}

// getConfig returns the typed *config.Config from the bot.
func (h *BaseHandler) getConfig() *config.Config {
	if c, ok := h.bot.Config().(*config.Config); ok {
		return c
	}
	return nil
}
