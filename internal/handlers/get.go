package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/utils/formatters"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type GetHandler struct {
	*BaseHandler
}

func NewGetHandler(b types.BotInterface) *GetHandler {
	return &GetHandler{BaseHandler: NewBaseHandler(b)}
}

var resourceMap = types.ResourceMap

func (h *GetHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *types.UserSession) error {
	if len(args) == 0 {
		h.sendResponse(msg.Chat.ID, "Usage: /get <resource> [name] [-n namespace] [-o format] [-l selector]")
		return nil
	}

	// Parse resource type
	resourceArg := strings.ToLower(args[0])
	gvr, ok := resourceMap[resourceArg]
	if !ok {
		h.sendResponse(msg.Chat.ID, fmt.Sprintf("Unknown resource: %s\nSupported: pods, deployments, services, replicasets, namespaces, nodes, configmaps, secrets, pvcs, pvs, ingresses, events", resourceArg))
		return nil
	}

	// Parse flags
	namespace, output, selector, fieldSelector, remaining := parseFlags(args[1:])

	// Use session namespace if not specified
	if namespace == "" && gvr.Resource != "namespaces" && gvr.Resource != "nodes" && gvr.Resource != "persistentvolumes" {
		namespace = h.getNamespace(session, args, h.getConfig().Kubernetes.DefaultNamespace)
	}

	// Check if getting a specific resource
	var name string
	if len(remaining) > 0 {
		name = remaining[0]
	}

	client := h.getK8sClient()

	if name != "" {
		// Get single resource
		resource, err := client.GetResource(ctx, schema.GroupVersionResource{
			Group:    gvr.Group,
			Version:  gvr.Version,
			Resource: gvr.Resource,
		}, namespace, name)
		if err != nil {
			return fmt.Errorf("failed to get %s/%s: %w", gvr.Kind, name, err)
		}
		h.sendSingleResource(msg.Chat.ID, resource, output)
	} else {
		// List resources
		resources, err := client.ListResources(ctx, schema.GroupVersionResource{
			Group:    gvr.Group,
			Version:  gvr.Version,
			Resource: gvr.Resource,
		}, namespace, selector, fieldSelector)
		if err != nil {
			return fmt.Errorf("failed to list %s: %w", gvr.Resource, err)
		}

		wide := output == "wide"
		h.sendFormatted(msg.Chat.ID, resources, output, wide)
	}

	return nil
}

func (h *GetHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}

func (h *GetHandler) sendSingleResource(chatID int64, resource *k8s.ResourceInfo, format string) {
	output := formatters.FormatResource(resource, format)
	h.bot.SendLongMessage(chatID, output)
}

func (h *GetHandler) sendFormatted(chatID int64, resources []k8s.ResourceInfo, format string, wide bool) {
	output := formatters.FormatResourceList(resources, format, wide)
	h.bot.SendLongMessage(chatID, output)
}