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
)

type DescribeHandler struct {
	*BaseHandler
}

func NewDescribeHandler(b *bot.Bot) *DescribeHandler {
	return &DescribeHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *DescribeHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
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

	namespace := h.getNamespace(session, args[2:], h.bot.config.Kubernetes.DefaultNamespace)

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

func (h *DescribeHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}

func (h *DescribeHandler) sendSingleResource(chatID int64, resource *k8s.ResourceInfo, format string) {
	output := formatters.FormatResource(resource, format)
	h.bot.sendLongMessage(chatID, output)
}