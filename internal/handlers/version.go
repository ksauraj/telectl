package handlers

import (
	"context"
	"fmt"

	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
)

type VersionHandler struct {
	*BaseHandler
}

func NewVersionHandler(b types.BotInterface) *VersionHandler {
	return &VersionHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *VersionHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	version, err := h.getK8sClient().GetServerVersion(ctx)
	if err != nil {
		version = "unknown"
	}

	pairs := [][2]string{
		{"telectl", "v0.1.0"},
		{"Kubernetes Server", version},
		{"Language", "Go"},
		{"client-go", "v0.30.5"},
		{"Kubeconfig", h.getConfig().Kubernetes.KubeconfigPath},
		{"Default Namespace", h.getConfig().Kubernetes.DefaultNamespace},
		{"Dry Run", fmt.Sprintf("%v", h.getConfig().Kubernetes.DryRun)},
	}

	fallback := "<b>telectl v0.1.0</b>\n"
	for _, kv := range pairs {
		fallback += fmt.Sprintf("• %s: %s\n", kv[0], kv[1])
	}

	h.bot.SendRich(msg.Chat.ID, formatters.RichKeyValue("telectl", pairs), fallback)
	return nil
}
