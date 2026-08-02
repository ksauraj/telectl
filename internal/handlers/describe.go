package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type DescribeHandler struct {
	*BaseHandler
}

func NewDescribeHandler(b types.BotInterface) *DescribeHandler {
	return &DescribeHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *DescribeHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	if len(args) < 2 {
		h.sendResponse(msg.Chat.ID, "Usage: /describe <resource> <name> [-n namespace]")
		return nil
	}

	resourceArg := strings.ToLower(args[0])
	name := args[1]

	gvr, ok := resourceMap[resourceArg]
	if !ok {
		h.sendResponse(msg.Chat.ID, fmt.Sprintf("Unknown resource: %s", resourceArg))
		return nil
	}

	// Cluster-scoped kinds must be queried with an empty namespace. Defaulting
	// them to "default" makes the API server look for a namespaced variant that
	// does not exist ("the server could not find the requested resource").
	namespace := ""
	if !types.IsClusterScoped(resourceArg) {
		namespace = h.getNamespace(session, args[2:], h.getConfig().Kubernetes.DefaultNamespace)
	}

	client := h.getK8sClient()
	resource, err := client.GetResource(ctx, schema.GroupVersionResource{
		Group:    gvr.Group,
		Version:  gvr.Version,
		Resource: gvr.Resource,
	}, namespace, name)

	if err != nil {
		return fmt.Errorf("failed to describe %s/%s: %w", gvr.Kind, name, err)
	}

	// Use wide format for describe
	h.sendSingleResource(msg.Chat.ID, resource, "wide")
	return nil
}

func (h *DescribeHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}

func (h *DescribeHandler) sendSingleResource(chatID int64, resource *k8s.ResourceInfo, format string) {
	// Describe output is long, so the rich rendering puts labels, annotations
	// and raw details behind collapsible sections.
	h.bot.SendRich(chatID,
		formatters.RichResource(resource, format == "wide"),
		formatters.FormatResource(resource, format))
}
