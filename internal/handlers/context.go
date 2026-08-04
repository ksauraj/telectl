package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	bottg "github.com/go-telegram/bot"
	"github.com/ksauraj/telectl/internal/config"
	"github.com/ksauraj/telectl/internal/menus"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
)

type ContextsHandler struct {
	*BaseHandler
}

func NewContextsHandler(b types.BotInterface) *ContextsHandler {
	return &ContextsHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *ContextsHandler) Handle(
	ctx context.Context,
	msg *tg.Message,
	args []string,
	session *types.UserSession,
) error {
	client := h.getK8sClient(session)
	kubeconfig := client.GetKubeconfig()

	if kubeconfig == nil {
		h.sendResponse(msg.Chat.ID, formatters.Btn(formatters.GlyphBroken, "Kubeconfig not loaded"))
		return nil
	}

	contexts := kubeconfig.Contexts
	if len(contexts) == 0 {
		h.sendResponse(msg.Chat.ID, "No contexts found in kubeconfig")
		return nil
	}

	mb, _ := h.bot.MenuBuilder().(*menus.MenuBuilder)
	keyboard := mb.GetContextsInlineKeyboard(contexts)

	rich := make([]formatters.KubeContext, 0, len(contexts))
	for _, c := range contexts {
		rich = append(rich, formatters.NewKubeContext(c.Name, c.Cluster, c.Namespace, c.Current))
	}
	h.bot.SendRichKeyboard(msg.Chat.ID,
		formatters.RichContexts(rich),
		formatters.FormatContexts(contexts),
		&keyboard)
	return nil
}

type UseContextHandler struct {
	*BaseHandler
}

func NewUseContextHandler(b types.BotInterface) *UseContextHandler {
	return &UseContextHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *UseContextHandler) Handle(
	ctx context.Context,
	msg *tg.Message,
	args []string,
	session *types.UserSession,
) error {
	if len(args) == 0 {
		h.sendResponse(msg.Chat.ID, "Usage: /use-context <context-name>")
		return nil
	}

	contextName := args[0]
	client := h.getK8sClient(session)

	// Session-scoped: this rebuilds the bot's API clients but deliberately does
	// not rewrite ~/.kube/config, which is shared with kubectl and with every
	// other user of this bot.
	if err := client.SwitchContext(contextName); err != nil {
		return fmt.Errorf("failed to switch context: %w", err)
	}

	session.CurrentCtx = contextName

	pairs := [][2]string{
		{"Context", contextName},
		{"Server", client.GetRESTConfig().Host},
		{"Scope", "this bot session only"},
	}
	h.bot.SendRich(msg.Chat.ID,
		formatters.RichKeyValue(formatters.Btn(formatters.GlyphDone, "Context switched"), pairs),
		fmt.Sprintf("Switched to context: %s\n(session only; ~/.kube/config unchanged)", contextName))
	return nil
}

type ConfigHandler struct {
	*BaseHandler
}

func NewConfigHandler(b types.BotInterface) *ConfigHandler {
	return &ConfigHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *ConfigHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	client := h.getK8sClient(session)
	kc := client.GetKubeconfig()

	currentCtx := "none"
	if current := kc.GetCurrentContext(); current != nil {
		currentCtx = current.Name
	}

	currentNS := session.GetNamespace()
	cfg, _ := h.bot.Config().(*config.Config)
	libBot := h.bot.API().(*bottg.Bot)

	botUsername := "telectl"
	if libBot != nil {
		me, _ := libBot.GetMe(context.Background())
		if me != nil {
			botUsername = me.Username
		}
	}
	parseMode := "Markdown"
	if cfg.Telegram.ParseMode != "" {
		parseMode = cfg.Telegram.ParseMode
	}

	pairs := [][2]string{
		{"Kubeconfig", cfg.Kubernetes.KubeconfigPath},
		{"Current Context", currentCtx},
		{"Available Contexts", fmt.Sprintf("%d", len(kc.Contexts))},
		{"Default Namespace", cfg.Kubernetes.DefaultNamespace},
		{"Session Namespace", currentNS},
		{"Dry Run Mode", fmt.Sprintf("%v", cfg.Kubernetes.DryRun)},
		{"Bot", "@" + botUsername},
		{"Parse Mode", parseMode},
		{"Rate Limit", fmt.Sprintf("%d/min", cfg.Bot.RateLimit)},
		{"Max Message Length", fmt.Sprintf("%d", cfg.Bot.MaxMessageLength)},
		{"Command Prefix", cfg.Bot.CommandPrefix},
	}

	fallback := "Bot Configuration\n"
	for _, kv := range pairs {
		fallback += fmt.Sprintf("• %s: %s\n", kv[0], kv[1])
	}

	h.bot.SendRich(msg.Chat.ID, formatters.RichKeyValue("Bot Configuration", pairs), fallback)
	return nil
}

type PortForwardHandler struct {
	*BaseHandler
}

func NewPortForwardHandler(b types.BotInterface) *PortForwardHandler {
	return &PortForwardHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *PortForwardHandler) Handle(
	ctx context.Context,
	msg *tg.Message,
	args []string,
	session *types.UserSession,
) error {
	if len(args) < 2 {
		h.sendResponse(msg.Chat.ID, "Usage: /portforward <pod> <local:remote> [-n namespace]")
		return nil
	}

	podName := args[0]
	portSpec := args[1]
	cfg, _ := h.bot.Config().(*config.Config)
	defaultNS := ""
	if cfg != nil {
		defaultNS = cfg.Kubernetes.DefaultNamespace
	}
	namespace := h.getNamespace(session, args[2:], defaultNS)

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

	h.sendResponse(msg.Chat.ID, fmt.Sprintf(
		"Port forwarding %s/%s %d -> %d\n"+
			"(Not fully implemented - would start background port-forward)",
		namespace, podName, localPort, remotePort))
	return nil
}
