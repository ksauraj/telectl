package handlers

import (
	"context"
	"fmt"
	"strings"

	bottg "github.com/go-telegram/bot"
	botmodels "github.com/go-telegram/bot/models"
	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type InlineQueryHandler struct {
	*BaseHandler
}

func NewInlineQueryHandler(b types.BotInterface) *InlineQueryHandler {
	return &InlineQueryHandler{BaseHandler: NewBaseHandler(b)}
}

// maxInlineResults is Telegram's cap on inline query results.
const maxInlineResults = 50

// inlineArticle builds one inline result whose message body is a code block.
func inlineArticle(id, title, description, body string) tg.InlineQueryResultArticle {
	return tg.InlineQueryResultArticle{
		Type:        "article",
		ID:          id,
		Title:       title,
		Description: description,
		InputMessageContent: &tg.InputTextMessageContent{
			MessageText: body,
			ParseMode:   "MarkdownV2",
		},
	}
}

// inlineError renders a failure as a single result, so the user sees why the
// query returned nothing instead of an empty dropdown.
func inlineError(err error) []tg.InlineQueryResultArticle {
	return []tg.InlineQueryResultArticle{
		inlineArticle("error", "Error", err.Error(),
			formatters.Btn(formatters.GlyphBroken, fmt.Sprintf("Error: %s", err.Error()))),
	}
}

func (h *InlineQueryHandler) HandleInlineQuery(ctx context.Context, inlineQuery *tg.InlineQuery) error {
	parts := strings.Fields(strings.TrimSpace(inlineQuery.Query))
	if len(parts) == 0 {
		return h.showInlineHelp(inlineQuery)
	}

	// types.ResourceMap already maps every alias to its GVR; this function used
	// to carry its own copies of both the alias table and the GVR table, so a
	// resource added to the shared map silently stayed unavailable here.
	resourceType := strings.ToLower(parts[0])
	entry, ok := types.ResourceMap[resourceType]
	if !ok {
		return h.showInlineHelp(inlineQuery)
	}

	namespace, name, labelSelector := parseInlineArgs(parts[1:])
	client := h.getK8sClient()

	if name != "" {
		return h.answerInlineQuery(ctx, inlineQuery.ID,
			h.singleResourceResult(ctx, client, entry.GVR(), resourceType, namespace, name))
	}

	resources, err := client.ListResources(ctx, entry.GVR(), namespace, labelSelector, "")
	if err != nil {
		return h.answerInlineQuery(ctx, inlineQuery.ID, inlineError(err))
	}
	if len(resources) == 0 {
		return h.answerInlineQuery(ctx, inlineQuery.ID, []tg.InlineQueryResultArticle{
			inlineArticle("empty", "No resources found",
				fmt.Sprintf("No %s in namespace %s", resourceType, namespace),
				fmt.Sprintf("No %s found in namespace `%s`", resourceType, namespace)),
		})
	}

	return h.answerInlineQuery(ctx, inlineQuery.ID, resourceListResults(resources))
}

// singleResourceResult renders one named resource, or the error explaining why
// it could not be read.
func (h *InlineQueryHandler) singleResourceResult(
	ctx context.Context,
	client *k8s.Client,
	gvr schema.GroupVersionResource,
	resourceType, namespace, name string,
) []tg.InlineQueryResultArticle {
	resource, err := client.GetResource(ctx, gvr, namespace, name)
	if err != nil {
		return inlineError(err)
	}
	return []tg.InlineQueryResultArticle{
		inlineArticle(
			"resource-"+name,
			fmt.Sprintf("%s/%s", resourceType, name),
			fmt.Sprintf("Namespace: %s | Status: %s", namespace, resource.Status),
			"```\n"+formatters.FormatResource(resource, formatWide)+"\n```",
		),
	}
}

// resourceListResults renders up to Telegram's result cap.
func resourceListResults(resources []k8s.ResourceInfo) []tg.InlineQueryResultArticle {
	limit := len(resources)
	if limit > maxInlineResults {
		limit = maxInlineResults
	}

	results := make([]tg.InlineQueryResultArticle, 0, limit)
	for i := range resources[:limit] {
		r := &resources[i]

		ns := r.Namespace
		if ns == "" {
			ns = "cluster-wide"
		}

		results = append(results, inlineArticle(
			"resource-"+r.Name,
			// StatusGlyph rather than a private copy of the status switch:
			// the two had drifted, so the same status rendered differently
			// here and in the tables.
			fmt.Sprintf("%s %s", formatters.StatusGlyph(r.Status),
				formatters.TruncateString(r.Name, 30)),
			fmt.Sprintf("NS: %s | Status: %s", ns, r.Status),
			"```\n"+formatters.FormatResource(r, formatWide)+"\n```",
		))
	}
	return results
}

// parseInlineArgs pulls the namespace, an optional resource name and a label
// selector out of an inline query's arguments.
//
// Inline queries are typed a character at a time and re-sent on every keystroke,
// so this has to tolerate half-written flags without erroring.
func parseInlineArgs(args []string) (namespace, name, labelSelector string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == flagNamespaceShort || arg == flagNamespaceLong:
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
			// The first bare word is the resource name.
			if name == "" {
				name = arg
			}
		}
	}
	return namespace, name, labelSelector
}

func (h *InlineQueryHandler) showInlineHelp(inlineQuery *tg.InlineQuery) error {
	help := `*telectl Inline Query Help*

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

	return h.answerInlineQuery(context.Background(), inlineQuery.ID, []tg.InlineQueryResultArticle{
		{
			Type:        "article",
			ID:          "help",
			Title:       "Inline Query Help",
			Description: "Tap to see usage examples",
			InputMessageContent: &tg.InputTextMessageContent{
				MessageText: help,
				ParseMode:   "MarkdownV2",
			},
		},
	})
}

func (h *InlineQueryHandler) answerInlineQuery(
	ctx context.Context,
	inlineQueryID string,
	results []tg.InlineQueryResultArticle,
) error {
	modelResults := make([]botmodels.InlineQueryResult, len(results))
	for i, r := range results {
		var imc botmodels.InputMessageContent
		if r.InputMessageContent != nil {
			if textContent, ok := r.InputMessageContent.(*tg.InputTextMessageContent); ok {
				imc = toModelInputMessageContent(textContent)
			}
		}
		modelResults[i] = &botmodels.InlineQueryResultArticle{
			ID:                  r.ID,
			Title:               r.Title,
			Description:         r.Description,
			InputMessageContent: imc,
		}
	}
	params := &bottg.AnswerInlineQueryParams{
		InlineQueryID: inlineQueryID,
		Results:       modelResults,
		CacheTime:     60,
		IsPersonal:    true,
	}
	libBot := h.bot.API().(*bottg.Bot)
	_, err := libBot.AnswerInlineQuery(ctx, params)
	return err
}

func toModelInputMessageContent(c *tg.InputTextMessageContent) botmodels.InputMessageContent {
	if c == nil {
		return nil
	}
	return &botmodels.InputTextMessageContent{
		MessageText: c.MessageText,
		ParseMode:   botmodels.ParseMode(c.ParseMode),
	}
}
