package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/k8s-telegram-bot/internal/bot"
	"github.com/ksauraj/k8s-telegram-bot/internal/k8s"
	"github.com/ksauraj/k8s-telegram-bot/internal/utils/formatters"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"go.uber.org/zap"
)
type TopHandler struct {
	*BaseHandler
}

func NewTopHandler(b *bot.Bot) *TopHandler {
	return &TopHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *TopHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	if len(args) == 0 {
		h.sendResponse(msg.Chat.ID, "Usage: /top pods|nodes [-n namespace] [--sort cpu|memory]")
		return nil
	}

	resource := strings.ToLower(args[0])
	namespace := h.getNamespace(session, args[1:], h.bot.config.Kubernetes.DefaultNamespace)

	client := h.getK8sClient()

	switch resource {
	case "pod", "pods", "po":
		return h.topPods(ctx, msg, client, namespace, args[1:])
	case "node", "nodes", "no":
		return h.topNodes(ctx, msg, client, args[1:])
	default:
		h.sendResponse(msg.Chat.ID, "Unknown resource. Use 'pods' or 'nodes'")
		return nil
	}
}

func (h *TopHandler) topPods(ctx context.Context, msg *tgbotapi.Message, client *k8s.Client, namespace string, args []string) error {
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

		h.sendResponse(msg.Chat.ID, fmt.Sprintf("⚠️ Metrics server not available. Showing pod list instead:\n\n%s", formatters.FormatResourceList(pods, "wide", true)))
		return nil
	}

	// Sort by CPU or memory if requested
	sortBy := "cpu"
	for _, arg := range args {
		if arg == "--sort=cpu" || arg == "--sort=memory" {
			sortBy = strings.TrimPrefix(arg, "--sort=")
		}
	}

	output := formatters.FormatResourceList(resources, "wide", true)
	h.sendResponse(msg.Chat.ID, fmt.Sprintf("📊 Pod Resource Usage (%s):\n%s", sortBy, output))
	return nil
}

func (h *TopHandler) topNodes(ctx context.Context, msg *tgbotapi.Message, client *k8s.Client, args []string) error {
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
		h.sendResponse(msg.Chat.ID, fmt.Sprintf("⚠️ Metrics server not available. Showing node list:\n\n%s", formatters.FormatResourceList(nodes, "wide", true)))
		return nil
	}

	output := formatters.FormatResourceList(resources, "wide", true)
	h.sendResponse(msg.Chat.ID, fmt.Sprintf("📊 Node Resource Usage:\n%s", output))
	return nil
}

func (h *TopHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}

type EventsHandler struct {
	*BaseHandler
}

func NewEventsHandler(b *bot.Bot) *EventsHandler {
	return &EventsHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *EventsHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	namespace := h.getNamespace(session, args, h.bot.config.Kubernetes.DefaultNamespace)

	client := h.getK8sClient()
	events, err := client.GetEvents(ctx, namespace, "")
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	if len(events) == 0 {
		h.sendResponse(msg.Chat.ID, fmt.Sprintf("📭 No events in namespace %s", namespace))
		return nil
	}

	// Sort by timestamp (newest first) and limit
	// For now just show all
	output := formatters.FormatResourceList(events, "wide", true)
	h.sendResponse(msg.Chat.ID, fmt.Sprintf("📅 Events in %s:\n%s", namespace, output))
	return nil
}

func (h *EventsHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}

type WatchHandler struct {
	*BaseHandler
}

func NewWatchHandler(b *bot.Bot) *WatchHandler {
	return &WatchHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *WatchHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	if len(args) == 0 {
		h.sendResponse(msg.Chat.ID, "Usage: /watch <resource> [name] [-n namespace]")
		return nil
	}

	resourceArg := strings.ToLower(args[0])
	namespace := h.getNamespace(session, args[1:], h.bot.config.Kubernetes.DefaultNamespace)

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
	h.bot.sendLongMessage(chatID, text)
}

var resourceMap = map[string]schemaGroupVersionResource{
	"pod":              {Group: "", Version: "v1", Resource: "pods", Kind: "Pod"},
	"pods":             {Group: "", Version: "v1", Resource: "pods", Kind: "Pod"},
	"po":               {Group: "", Version: "v1", Resource: "pods", Kind: "Pod"},
	"deployment":       {Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
	"deployments":      {Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
	"deploy":           {Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
	"service":          {Group: "", Version: "v1", Resource: "services", Kind: "Service"},
	"services":         {Group: "", Version: "v1", Resource: "services", Kind: "Service"},
	"svc":              {Group: "", Version: "v1", Resource: "services", Kind: "Service"},
	"replicaset":       {Group: "apps", Version: "v1", Resource: "replicasets", Kind: "ReplicaSet"},
	"replicasets":      {Group: "apps", Version: "v1", Resource: "replicasets", Kind: "ReplicaSet"},
	"rs":               {Group: "apps", Version: "v1", Resource: "replicasets", Kind: "ReplicaSet"},
	"namespace":        {Group: "", Version: "v1", Resource: "namespaces", Kind: "Namespace"},
	"namespaces":       {Group: "", Version: "v1", Resource: "namespaces", Kind: "Namespace"},
	"ns":               {Group: "", Version: "v1", Resource: "namespaces", Kind: "Namespace"},
	"node":             {Group: "", Version: "v1", Resource: "nodes", Kind: "Node"},
	"nodes":            {Group: "", Version: "v1", Resource: "nodes", Kind: "Node"},
	"no":               {Group: "", Version: "v1", Resource: "nodes", Kind: "Node"},
	"configmap":        {Group: "", Version: "v1", Resource: "configmaps", Kind: "ConfigMap"},
	"configmaps":       {Group: "", Version: "v1", Resource: "configmaps", Kind: "ConfigMap"},
	"cm":               {Group: "", Version: "v1", Resource: "configmaps", Kind: "ConfigMap"},
	"secret":           {Group: "", Version: "v1", Resource: "secrets", Kind: "Secret"},
	"secrets":          {Group: "", Version: "v1", Resource: "secrets", Kind: "Secret"},
	"pvc":              {Group: "", Version: "v1", Resource: "persistentvolumeclaims", Kind: "PersistentVolumeClaim"},
	"pvcs":             {Group: "", Version: "v1", Resource: "persistentvolumeclaims", Kind: "PersistentVolumeClaim"},
	"pv":               {Group: "", Version: "v1", Resource: "persistentvolumes", Kind: "PersistentVolume"},
	"pvs":              {Group: "", Version: "v1", Resource: "persistentvolumes", Kind: "PersistentVolume"},
	"ingress":          {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses", Kind: "Ingress"},
	"ingresses":        {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses", Kind: "Ingress"},
	"ing":              {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses", Kind: "Ingress"},
	"event":            {Group: "", Version: "v1", Resource: "events", Kind: "Event"},
	"events":           {Group: "", Version: "v1", Resource: "events", Kind: "Event"},
	"ev":               {Group: "", Version: "v1", Resource: "events", Kind: "Event"},
}

type schemaGroupVersionResource struct {
	Group    string
	Version  string
	Resource string
	Kind     string
}