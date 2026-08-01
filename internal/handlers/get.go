package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/k8s-telegram-bot/internal/bot"
	"github.com/ksauraj/k8s-telegram-bot/internal/k8s"
	"github.com/ksauraj/k8s-telegram-bot/internal/utils/formatters"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"go.uber.org/zap"
)

type GetHandler struct {
	*BaseHandler
}

func NewGetHandler(b *bot.Bot) *GetHandler {
	return &GetHandler{BaseHandler: NewBaseHandler(b)}
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
	"persistentvolumeclaim": {Group: "", Version: "v1", Resource: "persistentvolumeclaims", Kind: "PersistentVolumeClaim"},
	"pv":               {Group: "", Version: "v1", Resource: "persistentvolumes", Kind: "PersistentVolume"},
	"pvs":              {Group: "", Version: "v1", Resource: "persistentvolumes", Kind: "PersistentVolume"},
	"persistentvolume": {Group: "", Version: "v1", Resource: "persistentvolumes", Kind: "PersistentVolume"},
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

func (h *GetHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
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
		namespace = h.getNamespace(session, args, h.bot.config.Kubernetes.DefaultNamespace)
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
	h.bot.sendLongMessage(chatID, text)
}

func (h *GetHandler) sendSingleResource(chatID int64, resource *k8s.ResourceInfo, format string) {
	output := formatters.FormatResource(resource, format)
	h.bot.sendLongMessage(chatID, output)
}

func (h *GetHandler) sendFormatted(chatID int64, resources []k8s.ResourceInfo, format string, wide bool) {
	output := formatters.FormatResourceList(resources, format, wide)
	h.bot.sendLongMessage(chatID, output)
}