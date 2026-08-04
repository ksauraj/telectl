package handlers

import (
	"context"
	"fmt"
	"runtime"

	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
)

type AboutHandler struct {
	*BaseHandler
}

func NewAboutHandler(b types.BotInterface) *AboutHandler {
	return &AboutHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *AboutHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	version, err := h.getK8sClient(session).GetServerVersion(ctx)
	if err != nil {
		version = "unknown"
	}

	// Build info - these are set at build time via ldflags
	buildVersion := "dev"
	buildCommit := "unknown"
	buildDate := "unknown"

	// Get from config if available
	if cfg := h.getConfig(); cfg != nil {
		// Version info would come from the bot's build metadata
	}

	info := fmt.Sprintf(`<b>telectl</b> — Kubernetes cluster management from Telegram

<b>Version:</b> %s
<b>Commit:</b> %s
<b>Built:</b> %s
<b>Go:</b> %s
<b>client-go:</b> v0.30.5
<b>Kubernetes Server:</b> %s

<b>Configuration:</b>
• Kubeconfig: %s
• Default Namespace: %s
• Dry Run: %v
• Impersonation: %v

<b>Links:</b>
• Repository: <a href="https://github.com/ksauraj/telectl">github.com/ksauraj/telectl</a>
• Docker: <a href="https://hub.docker.com/r/ksauraj/telectl">hub.docker.com/r/ksauraj/telectl</a>
• Issues: <a href="https://github.com/ksauraj/telectl/issues">github.com/ksauraj/telectl/issues</a>

<b>License:</b> Apache 2.0
<b>Author:</b> Sauraj Kumar Singh (@ksauraj)

Use /help for command reference, or tap the menu buttons to explore.`,
		buildVersion, buildCommit, buildDate,
		runtime.Version(),
		version,
		h.getConfig().Kubernetes.KubeconfigPath,
		h.getConfig().Kubernetes.DefaultNamespace,
		h.getConfig().Kubernetes.DryRun,
		h.getConfig().Impersonation.Enabled,
	)

	h.bot.SendRich(msg.Chat.ID,
		formatters.RichAbout(info),
		info)

	return nil
}