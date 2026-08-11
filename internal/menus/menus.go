package menus

import (
	"strconv"
	"strings"

	"github.com/ksauraj/telectl/internal/config"
	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	f "github.com/ksauraj/telectl/internal/utils/formatters"
	"github.com/ksauraj/telectl/pkg/kubeconfig"
)

// Reply-keyboard labels.
//
// A reply keyboard sends its label back as ordinary message text, so the bot
// recognises a tap by comparing against the exact string it rendered. That makes
// these labels a wire contract between this file and the bot's
// handleReplyKeyboard, not decoration: when they were literals in both places, a
// label edited here silently stopped matching there and the button did nothing.
// Declared once, referenced from both sides.
var (
	LabelResources   = f.Btn(f.GlyphAction, "Resources")
	LabelLogs        = f.Btn(f.GlyphAction, "Logs")
	LabelExec        = f.Btn(f.GlyphAction, "Exec")
	LabelPortForward = f.Btn(f.GlyphAction, "Port Forward")
	LabelContexts    = f.Btn(f.GlyphAction, "Contexts")
	LabelMonitor     = f.Btn(f.GlyphAction, "Monitor")
	LabelOperations  = f.Btn(f.GlyphAction, "Operations")
	LabelSettings    = f.Btn(f.GlyphAction, "Settings")
	LabelHelp        = f.Btn(f.GlyphAction, "Help")
)

// Row widths used by the keyboard builders. Telegram renders inline buttons
// left-to-right, so a partial final row hugs the left edge; these are kept as
// named constants because the "flush the row" checks below must match the
// capacities the slices are preallocated with.
const (
	replyKeyboardCols = 3
	namespaceCols     = 3
	scaleCols         = 3
)

// MenuBuilder builds various Telegram keyboards for the bot.
type MenuBuilder struct {
	config *config.Config
	tokens *TokenStore
}

func NewMenuBuilder(cfg *config.Config) *MenuBuilder {
	return &MenuBuilder{config: cfg, tokens: NewTokenStore(4096)}
}

// btn builds a callback button, routing the data through the token store so it
// can never exceed Telegram's 64-byte callback_data limit. Every generated
// button in this file goes through here: exceeding the limit makes Telegram
// reject the entire keyboard with BUTTON_DATA_INVALID, not just one button.
func (mb *MenuBuilder) btn(text, data string) tg.InlineKeyboardButton {
	return tg.InlineButtonData(text, mb.tokens.Shorten(data))
}

// Button is the exported form of btn, for keyboards assembled outside this
// package (e.g. the drain confirmation in the bot).
func (mb *MenuBuilder) Button(text, data string) tg.InlineKeyboardButton {
	return mb.btn(text, data)
}

// ResolveCallback expands token callback data back to its full form. Returns
// false for a token minted before a restart, so the caller can prompt a refresh.
func (mb *MenuBuilder) ResolveCallback(data string) (string, bool) {
	return mb.tokens.Resolve(data)
}

// ============================================================================
// Bot Menu Commands (Persistent header menu)
// ============================================================================

// GetBotCommands returns the list of commands for the bot menu button.
//
// Telegram renders these as a plain command list, not as buttons, so they carry
// no action marker — a glyph here would be decoration with nothing to mark.
func (mb *MenuBuilder) GetBotCommands() []tg.BotCommand {
	return []tg.BotCommand{
		{Command: "resources", Description: "Browse resources"},
		{Command: "logs", Description: "View pod logs"},
		{Command: "exec", Description: "Run a command in a pod"},
		{Command: "contexts", Description: "Switch kubeconfig context"},
		{Command: "monitor", Description: "Monitoring"},
		{Command: "operations", Description: "Operations"},
		{Command: "settings", Description: "Settings"},
		{Command: "help", Description: "Help and command reference"},
		{Command: "about", Description: "Version, links, and project info"},
	}
}

// ============================================================================
// Reply Keyboard (Persistent bottom bar)
// ============================================================================

// GetMainReplyKeyboard returns the main persistent reply keyboard.
func (mb *MenuBuilder) GetMainReplyKeyboard() tg.ReplyKeyboardMarkup {
	keyboard := tg.ReplyKeyboard(
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText(LabelResources),
			tg.KeyboardButtonText(LabelLogs),
			tg.KeyboardButtonText(LabelExec),
		),
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText(LabelPortForward),
			tg.KeyboardButtonText(LabelContexts),
			tg.KeyboardButtonText(LabelMonitor),
		),
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText(LabelOperations),
			tg.KeyboardButtonText(LabelSettings),
			tg.KeyboardButtonText(LabelHelp),
		),
	)
	// Resize to content height. resize_keyboard=false would make the bar take
	// the full on-screen keyboard height on mobile (excess padding around the
	// buttons); shrinking it keeps the compact layout. Note: this affects
	// height only — Telegram reply keyboards always span the full width.
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false
	keyboard.InputFieldPlaceholder = "Choose an action or type a command..."
	return keyboard
}

// GetResourceTypeReplyKeyboard returns keyboard for resource type selection.
func (mb *MenuBuilder) GetResourceTypeReplyKeyboard() tg.ReplyKeyboardMarkup {
	keyboard := tg.ReplyKeyboard(
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText(f.Btn(f.GlyphAction, "Pods")),
			tg.KeyboardButtonText(f.Btn(f.GlyphAction, "Deployments")),
			tg.KeyboardButtonText(f.Btn(f.GlyphAction, "Services")),
		),
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText(f.Btn(f.GlyphAction, "ReplicaSets")),
			tg.KeyboardButtonText(f.Btn(f.GlyphAction, "Namespaces")),
			tg.KeyboardButtonText(f.Btn(f.GlyphAction, "Nodes")),
		),
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText(f.Btn(f.GlyphAction, "ConfigMaps")),
			tg.KeyboardButtonText(f.Btn(f.GlyphAction, "Secrets")),
			tg.KeyboardButtonText(f.Btn(f.GlyphAction, "PVCs")),
		),
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText(f.Btn(f.GlyphAction, "Ingresses")),
			tg.KeyboardButtonText(f.Btn(f.GlyphAction, "Events")),
			tg.KeyboardButtonText(f.Btn(f.GlyphBack, "Back")),
		),
	)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false
	keyboard.InputFieldPlaceholder = "Select resource type..."
	return keyboard
}

// GetNamespaceReplyKeyboard returns keyboard for namespace selection.
func (mb *MenuBuilder) GetNamespaceReplyKeyboard(namespaces []string, currentNS string) tg.ReplyKeyboardMarkup {
	rows := make([][]tg.KeyboardButton, 0, len(namespaces)/replyKeyboardCols+2)
	currentRow := make([]tg.KeyboardButton, 0, replyKeyboardCols)

	// Add "All Namespaces" button
	currentRow = append(currentRow, tg.KeyboardButtonText("All namespaces"))

	for _, ns := range namespaces {
		label := ns
		if ns == currentNS {
			label = f.Btn(f.GlyphSelected, ns)
		}
		currentRow = append(currentRow, tg.KeyboardButtonText(label))

		if len(currentRow) >= replyKeyboardCols {
			rows = append(rows, currentRow)
			currentRow = []tg.KeyboardButton{}
		}
	}

	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}

	rows = append(rows, tg.KeyboardButtonRow(
		tg.KeyboardButtonText(f.Btn(f.GlyphBack, "Back")),
		tg.KeyboardButtonText("New namespace"),
	))

	keyboard := tg.ReplyKeyboard(rows...)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false
	keyboard.InputFieldPlaceholder = "Select namespace..."
	return keyboard
}

// ============================================================================
// Inline Keyboards (Navigation & Actions)
// ============================================================================

// GetResourceTypeInlineKeyboard returns inline keyboard for resource type selection.
func (mb *MenuBuilder) GetResourceTypeInlineKeyboard() tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Pods"), "menu:resource:pods"),
			mb.btn(f.Btn(f.GlyphAction, "Deployments"), "menu:resource:deployments"),
			mb.btn(f.Btn(f.GlyphAction, "Services"), "menu:resource:services"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "ReplicaSets"), "menu:resource:replicasets"),
			mb.btn(f.Btn(f.GlyphAction, "Namespaces"), "menu:resource:namespaces"),
			mb.btn(f.Btn(f.GlyphAction, "Nodes"), "menu:resource:nodes"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "ConfigMaps"), "menu:resource:configmaps"),
			mb.btn(f.Btn(f.GlyphAction, "Secrets"), "menu:resource:secrets"),
			mb.btn(f.Btn(f.GlyphAction, "PVCs"), "menu:resource:pvcs"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Ingresses"), "menu:resource:ingresses"),
			mb.btn(f.Btn(f.GlyphAction, "Events"), "menu:resource:events"),
			mb.btn(f.Btn(f.GlyphAction, "PVs"), "menu:resource:pvs"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphHome, "Main Menu"), "menu:main"),
		),
	)
}

// GetResourceListInlineKeyboard returns paginated inline keyboard for resource list.
func (mb *MenuBuilder) GetResourceListInlineKeyboard(
	resourceType string,
	resources []k8s.ResourceInfo,
	page,
	pageSize int,
	namespace string,
) tg.InlineKeyboardMarkup {
	var rows [][]tg.InlineKeyboardButton

	start := page * pageSize
	end := start + pageSize
	if end > len(resources) {
		end = len(resources)
	}

	// Resource rows (2 per row for better mobile UX)
	for i := start; i < end; i += 2 {
		var row []tg.InlineKeyboardButton
		r1 := resources[i]
		btn1 := mb.btn(
			mb.formatResourceButton(r1),
			"menu:resource:view:"+resourceType+":"+r1.Namespace+":"+r1.Name,
		)
		row = append(row, btn1)

		if i+1 < end {
			r2 := resources[i+1]
			btn2 := mb.btn(
				mb.formatResourceButton(r2),
				"menu:resource:view:"+resourceType+":"+r2.Namespace+":"+r2.Name,
			)
			row = append(row, btn2)
		}
		rows = append(rows, row)
	}

	// Pagination row
	var paginationRow []tg.InlineKeyboardButton
	totalPages := (len(resources) + pageSize - 1) / pageSize

	if page > 0 {
		paginationRow = append(paginationRow, mb.btn(f.Btn(f.GlyphPrev, "Prev"),
			"menu:resource:page:"+resourceType+":"+namespace+":"+strconv.Itoa(page-1)))
	}

	paginationRow = append(paginationRow, mb.btn(
		strconv.Itoa(page+1)+"/"+strconv.Itoa(totalPages),
		"menu:noop",
	))

	if page+1 < totalPages {
		paginationRow = append(paginationRow, mb.btn(f.Btn(f.GlyphNext, "Next"),
			"menu:resource:page:"+resourceType+":"+namespace+":"+strconv.Itoa(page+1)))
	}

	if len(paginationRow) > 1 {
		rows = append(rows, paginationRow)
	}

	// Action row
	rows = append(rows, tg.InlineKeyboardRow(
		mb.btn(f.Btn(f.GlyphRefresh, "Refresh"), "menu:resource:refresh:"+resourceType+":"+namespace),
		mb.btn(nsButtonLabel(namespace), "menu:ns:pick:"+resourceType),
		mb.btn(f.Btn(f.GlyphBack, "Types"), "menu:resource:types"),
	))

	return tg.InlineKeyboard(rows...)
}

// formatResourceButton labels a resource button with its status glyph and name.
//
// The status switch this used to carry was a second, drifted copy of
// formatters.StatusGlyph: it was case-sensitive (so a lowercase "running" fell
// through to the unknown glyph) and it knew nothing of "Bound" or "Available",
// so a healthy PVC rendered as unknown in the button while the table beside it
// showed it as healthy. One status vocabulary, one function.
func (mb *MenuBuilder) formatResourceButton(r k8s.ResourceInfo) string {
	// Rune-aware: a byte slice can split a multi-byte character, and Telegram
	// rejects a keyboard whose label is invalid UTF-8.
	return f.Btn(f.StatusGlyph(r.Status), f.TruncateString(r.Name, 20))
}

// Section callback data, exported so the bot names a section with the same
// string this package routes on.
const (
	SectionMonitor    = "menu:monitor:home"
	SectionOperations = "menu:ops:home"
	SectionSettings   = "menu:settings:home"
	SectionMain       = "menu:main"
)

// GetSectionResultKeyboard is the keyboard under output that belongs to a
// top-level section rather than to a resource: back to that section, or home.
func (mb *MenuBuilder) GetSectionResultKeyboard(section string) tg.InlineKeyboardMarkup {
	if section == "" || section == SectionMain {
		return tg.InlineKeyboard(tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphHome, "Main"), SectionMain),
		))
	}
	return tg.InlineKeyboard(tg.InlineKeyboardRow(
		mb.btn(f.Btn(f.GlyphBack, "Back"), section),
		mb.btn(f.Btn(f.GlyphHome, "Main"), SectionMain),
	))
}

// GetVerbResultKeyboard is the keyboard under a verb's output: back to the
// resource the verb was invoked on, out to its list, or home.
//
// Every verb result carries this. Verb output used to arrive as a bare message
// with no keyboard at all, so the only way to continue was to scroll back up to
// a pane from several taps earlier — which is exactly the problem the single
// pane exists to fix. A view with no way out is a dead end.
//
// A cluster-scoped kind has no namespace, so its list link omits one; passing a
// namespace for such a kind would produce a list callback the dispatcher then
// resolves to an empty result.
func (mb *MenuBuilder) GetVerbResultKeyboard(kind, namespace, name string) tg.InlineKeyboardMarkup {
	if types.IsClusterScoped(kind) {
		namespace = ""
	}

	// With no resource to go back to there is nothing to render but the way
	// home — better than a Back button that resolves to nothing.
	if kind == "" || name == "" {
		return tg.InlineKeyboard(tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphHome, "Main"), "menu:main"),
		))
	}

	return tg.InlineKeyboard(tg.InlineKeyboardRow(
		mb.btn(f.Btn(f.GlyphBack, shortLabel(name, 14)), "menu:resource:view:"+kind+":"+namespace+":"+name),
		mb.btn(f.Btn(f.GlyphList, "List"), "menu:resource:list:"+kind+":"+namespace),
		mb.btn(f.Btn(f.GlyphHome, "Main"), "menu:main"),
	))
}

// GetResourceActionInlineKeyboard returns the detail pane for one resource:
// the inspection verbs every kind supports, then the verbs specific to this
// kind, then navigation.
//
// Every button here is built with actionData, so the positional contract that
// ParseCallbackData relies on ("menu:action:<verb>:<type>:<ns>:<name>:<args…>")
// holds for all of them. Buttons that omitted the type field used to shift
// every later field left, which is why the pod Logs button parsed with an empty
// Name and silently did nothing.
func (mb *MenuBuilder) GetResourceActionInlineKeyboard(
	resourceType,
	namespace,
	name string,
	resource *k8s.ResourceInfo,
) tg.InlineKeyboardMarkup {
	kind := CanonicalResource(resourceType)
	var rows [][]tg.InlineKeyboardButton

	act := func(verb string, args ...string) string {
		return actionData(verb, kind, namespace, name, args...)
	}

	// Inspection verbs, meaningful for every kind.
	rows = append(rows, tg.InlineKeyboardRow(
		mb.btn(f.Btn(f.GlyphAction, "Describe"), act("describe")),
		mb.btn(f.Btn(f.GlyphAction, "Labels"), act("labels")),
		mb.btn(f.Btn(f.GlyphAction, "Events"), act("events")),
	))

	rows = append(rows, mb.kindActionRows(kind, name, resource, act)...)

	// Destructive verb kept on its own row so it is not tapped by accident.
	rows = append(rows, tg.InlineKeyboardRow(
		mb.btn(f.Btn(f.GlyphDestructive, "Delete"), act("delete")),
	))

	rows = append(rows, tg.InlineKeyboardRow(
		mb.btn(f.Btn(f.GlyphRefresh, "Refresh"), "menu:resource:view:"+kind+":"+namespace+":"+name),
		mb.btn(f.Btn(f.GlyphList, "List"), "menu:resource:list:"+kind+":"+namespace),
		mb.btn(f.Btn(f.GlyphAction, "Help"), act("help")),
		mb.btn(f.Btn(f.GlyphHome, "Main"), "menu:main"),
	))

	return tg.InlineKeyboard(rows...)
}

// kindActionRows returns the verb rows specific to one kind. Split out of
// GetResourceActionInlineKeyboard so each pane's layout is readable on its own.
//
// act builds callback data for this resource; see actionData for the shape.
func (mb *MenuBuilder) kindActionRows(
	kind, name string,
	resource *k8s.ResourceInfo,
	act func(verb string, args ...string) string,
) [][]tg.InlineKeyboardButton {
	var rows [][]tg.InlineKeyboardButton

	switch kind {
	case "pods":
		rows = append(rows, tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Logs"), act("logsopts")),
			mb.btn(f.Btn(f.GlyphAction, "Exec"), act("exec")),
			mb.btn(f.Btn(f.GlyphAction, "Forward"), act("portforward")),
		))
		// One log button per container, but only when there is a choice to make.
		if names := podContainerNames(resource); len(names) > 1 {
			row := make([]tg.InlineKeyboardButton, 0, namespaceCols)
			for _, c := range names {
				row = append(row, mb.btn(f.Btn(f.GlyphAction, c), act("logs", c)))
				if len(row) == namespaceCols {
					rows = append(rows, row)
					row = nil
				}
			}
			if len(row) > 0 {
				rows = append(rows, row)
			}
		}
		if node := podNodeName(resource); node != "" {
			rows = append(rows, tg.InlineKeyboardRow(
				mb.btn(f.Btn(f.GlyphAction, "Node: "+shortLabel(node, 16)), "menu:resource:view:nodes::"+node),
			))
		}

	case "deployments":
		rows = append(rows, tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Restart"), act("restart")),
			mb.btn(f.Btn(f.GlyphAction, "Scale"), act("scale")),
			mb.btn(f.Btn(f.GlyphAction, "Pods"), act("pods")),
		))
		rows = append(rows, tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Selector"), act("selector")),
			mb.btn(f.Btn(f.GlyphAction, "History"), act("history")),
			mb.btn(f.Btn(f.GlyphAction, "YAML"), act("edit")),
		))

	case "replicasets":
		rows = append(rows, tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Pods"), act("rspods")),
			mb.btn(f.Btn(f.GlyphAction, "Scale"), act("rsscale")),
			mb.btn(f.Btn(f.GlyphAction, "Selector"), act("selector")),
		))

	case "services":
		rows = append(rows, tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Endpoints"), act("endpoints")),
			mb.btn(f.Btn(f.GlyphAction, "Selector"), act("selector")),
			mb.btn(f.Btn(f.GlyphAction, "Forward"), act("portforward")),
		))

	case "nodes":
		rows = append(rows, tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Top"), act("top")),
			mb.btn(f.Btn(f.GlyphAction, "Pods"), act("nodepods")),
		))
		// Cordon and uncordon are opposites; showing only the one that would
		// change something makes the node's current state readable from the
		// keyboard alone.
		if nodeIsCordoned(resource) {
			rows = append(rows, tg.InlineKeyboardRow(
				mb.btn(f.Btn(f.GlyphAction, "Uncordon"), act("uncordon")),
				mb.btn(f.Btn(f.GlyphAction, "Drain"), act("drain")),
			))
		} else {
			rows = append(rows, tg.InlineKeyboardRow(
				mb.btn(f.Btn(f.GlyphAction, "Cordon"), act("cordon")),
				mb.btn(f.Btn(f.GlyphAction, "Drain"), act("drain")),
			))
		}

	case "namespaces":
		rows = append(rows, tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Resources"), act("nsresources")),
			mb.btn(f.Btn(f.GlyphAction, "Switch to"), "menu:ns:set:"+name),
		))
	}

	return rows
}

// actionData builds callback data in the one shape ParseCallbackData decodes.
func actionData(verb, resourceType, namespace, name string, args ...string) string {
	data := "menu:action:" + verb + ":" + resourceType + ":" + namespace + ":" + name
	for _, a := range args {
		data += ":" + a
	}
	return data
}

// CanonicalResource maps any alias ("po", "pod") to the plural resource name
// ("pods") so callback data is stable no matter which alias produced it.
func CanonicalResource(alias string) string {
	if gvr, ok := types.ResourceMap[alias]; ok {
		return gvr.Resource
	}
	return alias
}

// podContainerNames lists a pod's containers, used to offer per-container logs.
func podContainerNames(resource *k8s.ResourceInfo) []string {
	if resource == nil || resource.Details == nil {
		return nil
	}
	spec, ok := resource.Details["spec"].(map[string]interface{})
	if !ok {
		return nil
	}
	containers, ok := spec["containers"].([]interface{})
	if !ok {
		return nil
	}
	var names []string
	for _, c := range containers {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if n, ok := cm["name"].(string); ok && n != "" {
			names = append(names, n)
		}
	}
	return names
}

func podNodeName(resource *k8s.ResourceInfo) string {
	if resource == nil || resource.Details == nil {
		return ""
	}
	spec, ok := resource.Details["spec"].(map[string]interface{})
	if !ok {
		return ""
	}
	n, _ := spec["nodeName"].(string)
	return n
}

func nodeIsCordoned(resource *k8s.ResourceInfo) bool {
	if resource == nil || resource.Details == nil {
		return false
	}
	spec, ok := resource.Details["spec"].(map[string]interface{})
	if !ok {
		return false
	}
	v, _ := spec["unschedulable"].(bool)
	return v
}

func shortLabel(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// GetContextsInlineKeyboard returns inline keyboard for context management.
func (mb *MenuBuilder) GetContextsInlineKeyboard(contexts []kubeconfig.ContextInfo) tg.InlineKeyboardMarkup {
	// One row per context, plus the trailing navigation row.
	rows := make([][]tg.InlineKeyboardButton, 0, len(contexts)+1)

	for _, ctx := range contexts {
		label := ctx.Name
		if ctx.Current {
			label = f.Btn(f.GlyphSelected, label)
		}
		rows = append(rows, tg.InlineKeyboardRow(
			mb.btn(label, "menu:ctx:switch:"+ctx.Name),
		))
	}

	rows = append(rows, tg.InlineKeyboardRow(
		mb.btn(f.Btn(f.GlyphRefresh, "Refresh"), "menu:ctx:refresh"),
		mb.btn(f.Btn(f.GlyphHome, "Main"), "menu:main"),
	))

	return tg.InlineKeyboard(rows...)
}

// GetMonitorInlineKeyboard returns inline keyboard for monitoring
// GetNamespaceInlineKeyboard returns a paginated namespace picker. The active
// namespace is marked, and "All Namespaces" clears the filter. backTo is the
// callback to return to (e.g. "menu:settings:home" or a resource list).
func (mb *MenuBuilder) GetNamespaceInlineKeyboard(
	namespaces []string,
	current string,
	page int,
	backTo string,
) tg.InlineKeyboardMarkup {
	const perPage = 9 // 3 rows of 3 keeps labels readable on mobile

	var rows [][]tg.InlineKeyboardButton

	allLabel := "All namespaces"
	if current == "" {
		allLabel = f.Btn(f.GlyphSelected, "All namespaces")
	}
	rows = append(rows, tg.InlineKeyboardRow(mb.btn(allLabel, "menu:ns:set:")))

	start := page * perPage
	if start > len(namespaces) {
		start = len(namespaces)
	}
	end := start + perPage
	if end > len(namespaces) {
		end = len(namespaces)
	}

	row := make([]tg.InlineKeyboardButton, 0, namespaceCols)
	for _, ns := range namespaces[start:end] {
		label := ns
		if ns == current {
			label = f.Btn(f.GlyphSelected, ns)
		}
		row = append(row, mb.btn(label, "menu:ns:set:"+ns))
		if len(row) == namespaceCols {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	totalPages := (len(namespaces) + perPage - 1) / perPage
	if totalPages > 1 {
		var nav []tg.InlineKeyboardButton
		if page > 0 {
			nav = append(nav, mb.btn(f.Btn(f.GlyphPrev, "Prev"), "menu:ns:page:"+strconv.Itoa(page-1)))
		}
		nav = append(nav, mb.btn(strconv.Itoa(page+1)+"/"+strconv.Itoa(totalPages), "menu:noop"))
		if page+1 < totalPages {
			nav = append(nav, mb.btn(f.Btn(f.GlyphNext, "Next"), "menu:ns:page:"+strconv.Itoa(page+1)))
		}
		rows = append(rows, nav)
	}

	if backTo == "" {
		backTo = "menu:main"
	}
	rows = append(rows, tg.InlineKeyboardRow(mb.btn(f.Btn(f.GlyphBack, "Back"), backTo)))

	return tg.InlineKeyboard(rows...)
}

// GetMainMenuInlineKeyboard returns the top-level inline keyboard. Callback data
// uses the same "menu:<section>[:...]" scheme every other builder here uses.
func (mb *MenuBuilder) GetMainMenuInlineKeyboard() tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Resources"), "menu:resource:types"),
			mb.btn(f.Btn(f.GlyphAction, "Monitor"), "menu:monitor:home"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Operations"), "menu:ops:home"),
			mb.btn(f.Btn(f.GlyphAction, "Settings"), "menu:settings:home"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Help"), "menu:help"),
		),
	)
}

// GetHelpInlineKeyboard returns the help categories inline keyboard.
func (mb *MenuBuilder) GetHelpInlineKeyboard() tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Resources"), "menu:help:resources"),
			mb.btn(f.Btn(f.GlyphAction, "Workloads"), "menu:help:workloads"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Networking"), "menu:help:networking"),
			mb.btn(f.Btn(f.GlyphAction, "Storage"), "menu:help:storage"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Monitoring"), "menu:help:monitoring"),
			mb.btn(f.Btn(f.GlyphAction, "Operations"), "menu:help:operations"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Settings"), "menu:help:settings"),
			mb.btn(f.Btn(f.GlyphAction, "All Commands"), "menu:help:all"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphHome, "Main"), "menu:main"),
		),
	)
}

// GetMonitorInlineKeyboard returns inline keyboard for monitoring.
func (mb *MenuBuilder) GetMonitorInlineKeyboard() tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Top Pods"), "menu:monitor:top:pods"),
			mb.btn(f.Btn(f.GlyphAction, "Top Nodes"), "menu:monitor:top:nodes"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Events"), "menu:monitor:events"),
			mb.btn(f.Btn(f.GlyphAction, "Watch"), "menu:monitor:watch"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphHome, "Main"), "menu:main"),
		),
	)
}

// GetOperationsInlineKeyboard returns inline keyboard for operations.
func (mb *MenuBuilder) GetOperationsInlineKeyboard() tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Restart Deployment"), "menu:ops:restart"),
			mb.btn(f.Btn(f.GlyphAction, "Scale Deployment"), "menu:ops:scale"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Delete Resource"), "menu:ops:delete"),
			mb.btn(f.Btn(f.GlyphAction, "Edit Resource"), "menu:ops:edit"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphHome, "Main"), "menu:main"),
		),
	)
}

// GetSettingsInlineKeyboard returns inline keyboard for settings.
func (mb *MenuBuilder) GetSettingsInlineKeyboard() tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Namespace"), "menu:settings:namespace"),
			mb.btn(f.Btn(f.GlyphAction, "Context"), "menu:settings:context"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Theme"), "menu:settings:theme"),
			mb.btn(f.Btn(f.GlyphAction, "Notifications"), "menu:settings:notifications"),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphHome, "Main"), "menu:main"),
		),
	)
}

// GetConfirmDeleteKeyboard returns confirmation keyboard for delete actions.
func (mb *MenuBuilder) GetConfirmDeleteKeyboard(resourceType, namespace, name string) tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphDestructive, "Yes, delete"), "menu:action:confirmdelete:"+resourceType+":"+namespace+":"+name),
			mb.btn(f.Btn(f.GlyphCancel, "Cancel"), "menu:resource:view:"+resourceType+":"+namespace+":"+name),
		),
	)
}

// GetScaleKeyboard builds the quick-scale and custom-scale chooser for a
// deployment or replicaset. kind is the canonical plural resource name; the
// actionData contract puts it in the slot ParseCallbackData expects.
func (mb *MenuBuilder) GetScaleKeyboard(kind, namespace, name string, currentReplicas int32) tg.InlineKeyboardMarkup {
	var rows [][]tg.InlineKeyboardButton

	// Quick scale buttons
	quickScales := []int32{0, 1, 2, 3, 5, 10}
	scaleRow := make([]tg.InlineKeyboardButton, 0, scaleCols)
	for _, r := range quickScales {
		label := strconv.Itoa(int(r))
		if r == currentReplicas {
			label = f.Btn(f.GlyphSelected, label)
		}
		scaleRow = append(scaleRow, mb.btn(label, actionData("scaleset", kind, namespace, name, strconv.Itoa(int(r)))))
		if len(scaleRow) == scaleCols {
			rows = append(rows, scaleRow)
			scaleRow = []tg.InlineKeyboardButton{}
		}
	}
	if len(scaleRow) > 0 {
		rows = append(rows, scaleRow)
	}

	// Custom scale
	rows = append(rows, tg.InlineKeyboardRow(
		mb.btn(f.Btn(f.GlyphAction, "Custom"), actionData("scalecustom", kind, namespace, name)),
		mb.btn(f.Btn(f.GlyphBack, "Back"), "menu:resource:view:"+kind+":"+namespace+":"+name),
	))

	return tg.InlineKeyboard(rows...)
}

// GetLogOptionsKeyboard offers the log verbs for one pod/container: tail
// sizes, follow, and previous-container logs.
func (mb *MenuBuilder) GetLogOptionsKeyboard(namespace, name, container string) tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Last 50"), actionData("logs", "pods", namespace, name, container, "50")),
			mb.btn(f.Btn(f.GlyphAction, "Last 100"), actionData("logs", "pods", namespace, name, container, "100")),
			mb.btn(f.Btn(f.GlyphAction, "Last 500"), actionData("logs", "pods", namespace, name, container, "500")),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Follow"), actionData("logsfollow", "pods", namespace, name, container)),
			mb.btn(f.Btn(f.GlyphAction, "Previous"), actionData("logsprevious", "pods", namespace, name, container)),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphAction, "Full log"), actionData("logsfull", "pods", namespace, name, container)),
		),
		tg.InlineKeyboardRow(
			mb.btn(f.Btn(f.GlyphBack, "Back"), "menu:resource:view:pods:"+namespace+":"+name),
		),
	)
}

// ============================================================================
// Callback Data Parsing
// ============================================================================

// Callback verbs that appear in more than one place, exported so the bot's
// dispatcher compares against the same values this package emits.
const (
	VerbPage = "page"
	VerbHome = "home"
)

// CallbackAction represents a parsed callback action.
type CallbackAction struct {
	Type         string
	ResourceType string
	Namespace    string
	Name         string
	Container    string
	Page         int
	Action       string
	Extra        string
}

// callbackFields lists which CallbackAction field each colon-separated position
// maps to, per callback type. Index 0 is the field after the type, i.e. parts[2].
//
// The wire format is positional — "menu:<type>:<f0>:<f1>:…" — and every bug in
// this area has been a field landing one slot off: a button that omitted its
// resource-type field shifted Name into Namespace and left Name empty, and the
// dispatcher's empty-name guard then dropped the tap in silence. Declaring the
// layout as data makes the contract inspectable instead of implied by the order
// of a chain of length checks.
var callbackFields = map[string][]func(*CallbackAction, string){
	"resource": {
		func(a *CallbackAction, v string) { a.Action = v }, // view, page, list, refresh, filter, types
		func(a *CallbackAction, v string) { a.ResourceType = v },
		func(a *CallbackAction, v string) { a.Namespace = v },
		// "page" carries a page number here; every other verb carries a name.
		func(a *CallbackAction, v string) {
			if a.Action == VerbPage {
				a.Extra = v
			} else {
				a.Name = v
			}
		},
		func(a *CallbackAction, v string) { a.Extra = v },
	},
	"action": {
		func(a *CallbackAction, v string) { a.Action = v }, // describe, logs, scale, cordon, …
		func(a *CallbackAction, v string) { a.ResourceType = v },
		func(a *CallbackAction, v string) { a.Namespace = v },
		func(a *CallbackAction, v string) { a.Name = v },
		// Usually a container name. For verbs whose trailing argument is a
		// value rather than a container, it is routed to Extra below.
		func(a *CallbackAction, v string) { a.Container = v },
		func(a *CallbackAction, v string) { a.Extra = v },
	},
	"ctx": {
		func(a *CallbackAction, v string) { a.Action = v }, // switch, refresh
		func(a *CallbackAction, v string) { a.Name = v },
	},
	"monitor": {
		func(a *CallbackAction, v string) { a.Action = v }, // top, events, watch
		func(a *CallbackAction, v string) { a.ResourceType = v },
	},
	"ops": {
		func(a *CallbackAction, v string) { a.Action = v }, // restart, scale, delete, edit
	},
	"settings": {
		func(a *CallbackAction, v string) { a.Action = v }, // namespace, context, theme, …
		// Absorbs the tail: a namespace name may contain colons.
		func(a *CallbackAction, v string) { a.Name = v },
	},
	"ns": {
		func(a *CallbackAction, v string) { a.Action = v }, // pick, set, page
		// Absorbs the tail: a namespace name may contain colons. Empty means
		// "all namespaces".
		func(a *CallbackAction, v string) { a.Name = v },
	},

	// Types with no payload. Present so an unknown type is distinguishable
	// from a known one that takes no fields.
	"main": {},
	"noop": {},
	"help": {
		func(a *CallbackAction, v string) { a.Action = v }, // resources, workloads, networking, storage, monitoring, operations, settings, all
	},
}

// tailJoinTypes are the callback types whose final field is a name that may
// itself contain colons — a namespace or context name. For these the remainder
// of the data is joined back together rather than truncated at the first colon.
var tailJoinTypes = map[string]bool{
	"settings": true,
	"ns":       true,
}

// valueInContainerSlot are "action" verbs whose first trailing argument is a
// value, not a container name. The dispatcher reads it from Extra.
var valueInContainerSlot = map[string]bool{
	"scaleset": true,
}

// ParseCallbackData decodes the callback data carried by an inline button.
//
// Returns nil for anything that is not ours, so a stray callback from another
// source cannot be mistaken for a menu action.
func ParseCallbackData(data string) *CallbackAction {
	parts := strings.Split(data, ":")
	if len(parts) < 2 || parts[0] != "menu" {
		return nil
	}

	action := &CallbackAction{Type: parts[1]}
	setters, known := callbackFields[action.Type]
	if !known {
		// An unrecognised type still parses to its type alone; the dispatcher
		// logs and ignores it.
		return action
	}

	fields := parts[2:]
	for i, set := range setters {
		if i >= len(fields) {
			break
		}
		value := fields[i]
		// The last declared field of a tail-join type absorbs the remainder.
		if i == len(setters)-1 && tailJoinTypes[action.Type] {
			value = strings.Join(fields[i:], ":")
		}
		set(action, value)
	}

	if action.Type == "action" && valueInContainerSlot[action.Action] {
		action.Extra = action.Container
		action.Container = ""
	}

	return action
}

// nsButtonLabel renders the namespace switcher's label, truncated so the button
// stays readable on mobile.
func nsButtonLabel(namespace string) string {
	if namespace == "" {
		return "All NS"
	}
	if len(namespace) > 12 {
		return namespace[:11] + "…"
	}
	return namespace
}

// ============================================================================
// Helper Functions
// ============================================================================

// GetPageSize returns the configured page size.
func (mb *MenuBuilder) GetPageSize() int {
	if mb.config.Bot.MenuPageSize > 0 {
		return mb.config.Bot.MenuPageSize
	}
	return 10
}

// IsMenuEnabled returns true if menu system is enabled.
func (mb *MenuBuilder) IsMenuEnabled() bool {
	return mb.config.Bot.EnableMenuButton || mb.config.Bot.EnableReplyKeyboard
}

// IsMenuButtonEnabled returns true if menu button is enabled.
func (mb *MenuBuilder) IsMenuButtonEnabled() bool {
	return mb.config.Bot.EnableMenuButton
}

// IsReplyKeyboardEnabled returns true if reply keyboard is enabled.
func (mb *MenuBuilder) IsReplyKeyboardEnabled() bool {
	return mb.config.Bot.EnableReplyKeyboard
}
