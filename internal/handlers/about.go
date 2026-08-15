package handlers

import (
	"context"
	"fmt"
	"runtime"

	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
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

	buildVersion := h.bot.BuildVersion()
	buildCommit := h.bot.BuildCommit()
	buildDate := h.bot.BuildDate()

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

	// About is plain HTML, not a Rich Message: sending it through SendRich
	// wraps the string in a rich Paragraph, which renders the <b>/<a> tags
	// literally instead of formatting them. SendHTML keeps the markup.
	h.bot.SendHTML(msg.Chat.ID, info)

	return nil
}
