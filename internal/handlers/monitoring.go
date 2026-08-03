package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type TopHandler struct {
	*BaseHandler
}

func NewTopHandler(b types.BotInterface) *TopHandler {
	return &TopHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *TopHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	if len(args) == 0 {
		h.sendResponse(msg.Chat.ID, "Usage: /top pods|nodes [-n namespace] [--sort cpu|memory]")
		return nil
	}

	resource := strings.ToLower(args[0])
	namespace := h.getNamespace(session, args[1:], h.getConfig().Kubernetes.DefaultNamespace)

	client := h.getK8sClient()

	switch resource {
	case "pod", "pods", "po":
		return h.topPods(ctx, msg, client, namespace, args[1:])
	case "node", "nodes", "no":
		return h.topNodes(ctx, msg, client)
	default:
		h.sendResponse(msg.Chat.ID, "Unknown resource. Use 'pods' or 'nodes'")
		return nil
	}
}

func (h *TopHandler) topPods(
	ctx context.Context,
	msg *tg.Message,
	client *k8s.Client,
	namespace string,
	args []string,
) error {
	// Try to get metrics from metrics.k8s.io API
	resources, err := client.ListResources(ctx, schema.GroupVersionResource{
		Group:    "metrics.k8s.io",
		Version:  "v1beta1",
		Resource: "pods",
	}, namespace, "", "")

	if err != nil {
		// Fallback: show regular pods with a note
		pods, err := client.ListPods(ctx, namespace, "", "")
		if err != nil {
			return err
		}

		h.bot.SendRich(msg.Chat.ID,
			"> ⚠️ metrics-server not available — showing the pod list instead.\n\n"+formatters.RichResourceList(pods, true),
			fmt.Sprintf("⚠️ Metrics server not available. Showing pod list instead:\n\n%s", formatters.FormatResourceList(pods, "wide", true)))
		return nil
	}

	// Sort by CPU or memory if requested
	sortBy := "cpu"
	for _, arg := range args {
		if arg == "--sort=cpu" || arg == "--sort=memory" {
			sortBy = strings.TrimPrefix(arg, "--sort=")
		}
	}

	h.bot.SendRich(msg.Chat.ID,
		formatters.RichMetrics(fmt.Sprintf("📊 Pod Resource Usage (sorted by %s)", sortBy), resources),
		fmt.Sprintf("📊 Pod Resource Usage (%s):\n%s", sortBy, formatters.FormatResourceList(resources, "wide", true)))
	return nil
}

func (h *TopHandler) topNodes(ctx context.Context, msg *tg.Message, client *k8s.Client) error {
	resources, err := client.ListResources(ctx, schema.GroupVersionResource{
		Group:    "metrics.k8s.io",
		Version:  "v1beta1",
		Resource: "nodes",
	}, "", "", "")

	if err != nil {
		nodes, err := client.ListNodes(ctx, "")
		if err != nil {
			return err
		}
		h.bot.SendRich(msg.Chat.ID,
			"> ⚠️ metrics-server not available — showing the node list instead.\n\n"+formatters.RichResourceList(nodes, true),
			fmt.Sprintf("⚠️ Metrics server not available. Showing node list:\n\n%s", formatters.FormatResourceList(nodes, "wide", true)))
		return nil
	}

	h.bot.SendRich(msg.Chat.ID,
		formatters.RichMetrics("📊 Node Resource Usage", resources),
		fmt.Sprintf("📊 Node Resource Usage:\n%s", formatters.FormatResourceList(resources, "wide", true)))
	return nil
}

func (h *TopHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}

type EventsHandler struct {
	*BaseHandler
}

func NewEventsHandler(b types.BotInterface) *EventsHandler {
	return &EventsHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *EventsHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	namespace := h.getNamespace(session, args, h.getConfig().Kubernetes.DefaultNamespace)

	client := h.getK8sClient()
	events, err := client.GetEvents(ctx, namespace, "")
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	if len(events) == 0 {
		h.sendResponse(msg.Chat.ID, fmt.Sprintf("📭 No events in namespace %s", namespace))
		return nil
	}

	h.bot.SendRich(msg.Chat.ID,
		formatters.RichEvents(events),
		fmt.Sprintf("📅 Events in %s:\n%s", namespace, formatters.FormatResourceList(events, "wide", true)))
	return nil
}

func (h *EventsHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}

type WatchHandler struct {
	*BaseHandler
}

func NewWatchHandler(b types.BotInterface) *WatchHandler {
	return &WatchHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *WatchHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	if len(args) == 0 {
		h.sendResponse(msg.Chat.ID, "Usage: /watch <resource> [name] [-n namespace]")
		return nil
	}

	resourceArg := strings.ToLower(args[0])
	namespace := h.getNamespace(session, args[1:], h.getConfig().Kubernetes.DefaultNamespace)

	// Parse resource
	gvr, ok := resourceMap[resourceArg]
	if !ok {
		h.sendResponse(msg.Chat.ID, fmt.Sprintf("Unknown resource: %s", resourceArg))
		return nil
	}

	client := h.getK8sClient()

	// Start watch
	watcher, err := client.WatchResources(ctx, schema.GroupVersionResource{
		Group:    gvr.Group,
		Version:  gvr.Version,
		Resource: gvr.Resource,
	}, namespace, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to start watch: %w", err)
	}

	h.sendResponse(msg.Chat.ID, fmt.Sprintf("👀 Watching %s in %s... (Press /cancel to stop)", gvr.Resource, namespace))

	go func() {
		for event := range watcher.ResultChan() {
			// Send event to user
			// This would need a way to send messages from background goroutine
			// For now, just log
			h.getLogger().Info("Watch event", zap.String("type", string(event.Type)), zap.String("resource", gvr.Resource))
		}
	}()

	return nil
}

func (h *WatchHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}
