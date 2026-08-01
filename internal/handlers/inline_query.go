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

type InlineQueryHandler struct {
	*BaseHandler
}

func NewInlineQueryHandler(b *bot.Bot) *InlineQueryHandler {
	return &InlineQueryHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *InlineQueryHandler) HandleInlineQuery(ctx context.Context, inlineQuery *tgbotapi.InlineQuery) error {
	query := strings.TrimSpace(inlineQuery.Query)
	if query == "" {
		// Show help when empty query
		return h.showInlineHelp(inlineQuery)
	}

	// Parse inline query: @bot pods, @bot pods nginx, @bot pods -n kube-system
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return h.showInlineHelp(inlineQuery)
	}

	resourceType := strings.ToLower(parts[0])
	args := parts[1:]

	// Map resource aliases
	resourceMap := map[string]string{
		"po":    "pods",
		"pod":   "pods",
		"deploy": "deployments",
		"deployment": "deployments",
		"svc":   "services",
		"service": "services",
		"rs":    "replicasets",
		"replicaset": "replicasets",
		"ns":    "namespaces",
		"namespace": "namespaces",
		"no":    "nodes",
		"node":  "nodes",
		"cm":    "configmaps",
		"configmap": "configmaps",
		"pvc":   "pvcs",
		"pv":    "pvs",
		"ing":   "ingresses",
		"ingress": "ingresses",
		"ev":    "events",
		"event": "events",
	}

	if mapped, ok := resourceMap[resourceType]; ok {
		resourceType = mapped
	}

	// Valid resource types
	validResources := map[string]schema.GroupVersionResource{
		"pods":        {Group: "", Version: "v1", Resource: "pods"},
		"deployments": {Group: "apps", Version: "v1", Resource: "deployments"},
		"services":    {Group: "", Version: "v1", Resource: "services"},
		"replicasets": {Group: "apps", Version: "v1", Resource: "replicasets"},
		"namespaces":  {Group: "", Version: "v1", Resource: "namespaces"},
		"nodes":       {Group: "", Version: "v1", Resource: "nodes"},
		"configmaps":  {Group: "", Version: "v1", Resource: "configmaps"},
		"secrets":     {Group: "", Version: "v1", Resource: "secrets"},
		"pvcs":        {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		"pvs":         {Group: "", Version: "v1", Resource: "persistentvolumes"},
		"ingresses":   {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		"events":      {Group: "", Version: "v1", Resource: "events"},
	}

	gvr, ok := validResources[resourceType]
	if !ok {
		return h.showInlineHelp(inlineQuery)
	}

	// Parse flags
	namespace := ""
	name := ""
	labelSelector := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-n" || arg == "--namespace":
			if i+1 < len(args) {
				namespace = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "-n="):
			namespace = strings.TrimPrefix(arg, "-n=")
		case strings.HasPrefix(arg, "--namespace="):
			namespace = strings.TrimPrefix(arg, "--namespace=")
		case arg == "-l" || arg == "--selector":
			if i+1 < len(args) {
				labelSelector = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "-l="):
			labelSelector = strings.TrimPrefix(arg, "-l=")
		case strings.HasPrefix(arg, "--selector="):
			labelSelector = strings.TrimPrefix(arg, "--selector=")
		default:
			if name == "" {
				name = arg
			}
		}
	}

	client := h.getK8sClient()

	if name != "" {
		// Get specific resource
		resource, err := client.GetResource(ctx, gvr, namespace, name)
		if err != nil {
			return h.answerInlineQuery(inlineQuery, []tgbotapi.InlineQueryResult{
				{
					Type:        "article",
					ID:          "error",
					Title:       "Error",
					Description: err.Error(),
					InputMessageContent: &tgbotapi.InputTextMessageContent{
						Text:        fmt.Sprintf("❌ Error: %s", err.Error()),
						ParseMode:   "MarkdownV2",
					},
				},
			})
		}

		text := formatters.FormatResource(resource, "wide")
		return h.answerInlineQuery(inlineQuery, []tgbotapi.InlineQueryResult{
			{
				Type:        "article",
				ID:          "resource-" + name,
				Title:       fmt.Sprintf("%s/%s", resourceType, name),
				Description: fmt.Sprintf("Namespace: %s | Status: %s", namespace, resource.Status),
				InputMessageContent: &tgbotapi.InputTextMessageContent{
					Text:      "```\n" + text + "\n```",
					ParseMode: "MarkdownV2",
				},
			},
		})
	}

	// List resources
	resources, err := client.ListResources(ctx, gvr, namespace, labelSelector, "")
	if err != nil {
		return h.answerInlineQuery(inlineQuery, []tgbotapi.InlineQueryResult{
			{
				Type:        "article",
				ID:          "error",
				Title:       "Error",
				Description: err.Error(),
				InputMessageContent: &tgbotapi.InputTextMessageContent{
					Text:        fmt.Sprintf("❌ Error: %s", err.Error()),
					ParseMode:   "MarkdownV2",
				},
			},
		})
	}

	if len(resources) == 0 {
		return h.answerInlineQuery(inlineQuery, []tgbotapi.InlineQueryResult{
			{
				Type:        "article",
				ID:          "empty",
				Title:       "No resources found",
				Description: fmt.Sprintf("No %s in namespace %s", resourceType, namespace),
				InputMessageContent: &tgbotapi.InputTextMessageContent{
					Text:        fmt.Sprintf("📭 No %s found in namespace `%s`", resourceType, namespace),
					ParseMode:   "MarkdownV2",
				},
			},
		})
	}

	// Build results (max 50 for inline query)
	results := make([]tgbotapi.InlineQueryResult, 0, min(len(resources), 50))
	for _, r := range resources {
		if len(results) >= 50 {
			break
		}

		statusIcon := "⚪"
		switch r.Status {
		case "Running", "Active", "Ready":
			statusIcon = "🟢"
		case "Pending", "Creating":
			statusIcon = "🟡"
		case "Failed", "Error", "CrashLoopBackOff":
			statusIcon = "🔴"
		case "Succeeded", "Completed":
			statusIcon = "🔵"
		case "Terminating":
			statusIcon = "🟠"
		}

		displayName := r.Name
		if len(displayName) > 30 {
			displayName = displayName[:27] + "..."
		}

		ns := r.Namespace
		if ns == "" {
			ns = "cluster-wide"
		}

		text := formatters.FormatResource(&r, "wide")

		results = append(results, tgbotapi.InlineQueryResult{
			Type:        "article",
			ID:          "resource-" + r.Name,
			Title:       fmt.Sprintf("%s %s", statusIcon, displayName),
			Description: fmt.Sprintf("NS: %s | Status: %s", ns, r.Status),
			InputMessageContent: &tgbotapi.InputTextMessageContent{
				Text:      "```\n" + text + "\n```",
				ParseMode: "MarkdownV2",
			},
		})
	}

	return h.answerInlineQuery(inlineQuery, results)
}

func (h *InlineQueryHandler) showInlineHelp(inlineQuery *tgbotapi.InlineQuery) error {
	help := `📦 *k8s-telegram-bot Inline Query Help*

*Usage:* @bot <resource> [name] [flags]

*Resources:*
• pods, deployments, services, replicasets
• namespaces, nodes, configmaps, secrets
• pvcs, pvs, ingresses, events

*Flags:*
• -n, --namespace <ns>  - Namespace (default: current)
• -l, --selector <label> - Label selector

*Examples:*
@bot pods
@bot pods -n kube-system
@bot deployments nginx
@bot services -l app=nginx
@bot nodes`

	return h.answerInlineQuery(inlineQuery, []tgbotapi.InlineQueryResult{
		{
			Type:        "article",
			ID:          "help",
			Title:       "📖 Inline Query Help",
			Description: "Tap to see usage examples",
			InputMessageContent: &tgbotapi.InputTextMessageContent{
				Text:      help,
				ParseMode: "MarkdownV2",
			},
		},
	})
}

func (h *InlineQueryHandler) answerInlineQuery(inlineQuery *tgbotapi.InlineQuery, results []tgbotapi.InlineQueryResult) error {
	answer := tgbotapi.InlineQueryResultArray(results...)
	answer.QueryID = inlineQuery.ID
	answer.CacheTime = 60
	answer.IsPersonal = true
	return h.bot.api.AnswerInlineQuery(answer)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}