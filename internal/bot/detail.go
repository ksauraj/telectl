package bot

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/menus"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

// The per-resource detail pane and the verbs it exposes.
//
// Every one of these used to be a button that produced nothing: the callback
// parsed, fell through dispatchResourceAction's switch, and hit the "not
// available yet" default. They are wired to the same k8s client the typed
// commands use, so there is one implementation per operation.

// Canonical plural resource names this file switches on. These are the values
// menus.CanonicalResource produces, so they must match the keys in
// types.ResourceMap exactly.
const (
	kindPods        = "pods"
	kindDeployments = "deployments"
	kindReplicaSets = "replicasets"
	kindNodes       = "nodes"
)

// showResourceDetail renders the detail pane for a single resource: a summary
// header plus the action keyboard for its kind.
//
// This replaces the old behaviour where tapping a resource ran Describe
// directly, which dumped the whole object and offered no way to act on it.
func (b *Bot) showResourceDetail(
	ctx context.Context,
	chatID int64,
	messageID int,
	resourceType,
	namespace,
	name string,
	session *types.UserSession,
) {
	kind := menus.CanonicalResource(resourceType)
	req := detailReqFor(chatID, messageID, kind, namespace, name, session)

	gvr, known := types.ResourceMap[kind]
	if !known {
		b.showNotice(ctx, req, "Unknown resource type",
			resourceType+" is not a resource this bot knows about.")
		return
	}
	if types.IsClusterScoped(kind) {
		namespace = ""
		req.ns = ""
	}

	resource, err := b.K8sClientForSession(session).GetResource(ctx, gvr.GVR(), namespace, name)
	if err != nil {
		b.reportPaneError(ctx, req, "load "+kind, err)
		return
	}

	session.SetMenuState(&types.MenuState{
		CurrentView:  "resource_detail",
		ResourceType: kind,
		Namespace:    namespace,
		Filter:       name,
	})

	kb := b.menuBuilder.GetResourceActionInlineKeyboard(kind, namespace, name, resource)
	b.showPane(ctx, chatID, messageID, pane{
		rich: formatters.RichResourceSummary(resource),
		fallback: fmt.Sprintf("%s <b>%s</b>\n%s in %s",
			formatters.StatusGlyph(resource.Status),
			formatters.EscapeHTML(name),
			formatters.EscapeHTML(resource.Kind),
			formatters.EscapeHTML(nsDisplay(namespace))),
		kb: &kb,
	})
}

// detailReq carries the resolved parameters of one detail-pane verb, so each
// handler takes a single argument instead of repeating the same six.
type detailReq struct {
	chatID    int64
	messageID int
	kind      string // canonical plural, e.g. "pods"
	ns        string
	name      string
	container string
	extra     string
	session   *types.UserSession
}

// detailVerbs maps each detail-pane verb to its handler.
//
// A map rather than a switch so the verb set is enumerable: a test asserts that
// every button the keyboards render has an entry here, which is what the
// "That action is not available yet" replies used to be — a button with no
// handler, indistinguishable from a bug until someone tapped it.
var detailVerbs = map[string]func(*Bot, context.Context, detailReq){
	"describe":       (*Bot).detailDescribe,
	"labels":         (*Bot).showLabels,
	"events":         (*Bot).showObjectEvents,
	"selector":       (*Bot).showSelector,
	"endpoints":      (*Bot).showEndpoints,
	"pods":           (*Bot).showWorkloadPods,
	"rspods":         (*Bot).showWorkloadPods,
	"nodepods":       (*Bot).showNodePods,
	"top":            (*Bot).showNodeTop,
	"history":        (*Bot).showRolloutHistory,
	"edit":           (*Bot).showManifest,
	"cordon":         func(b *Bot, ctx context.Context, r detailReq) { b.setNodeSchedulable(ctx, r, false) },
	"uncordon":       func(b *Bot, ctx context.Context, r detailReq) { b.setNodeSchedulable(ctx, r, true) },
	"drain":          (*Bot).confirmDrain,
	"confirmdrain":   (*Bot).drainNode,
	"scale":          (*Bot).showScaleOptions,
	"rsscale":        (*Bot).showScaleOptions,
	"scaleset":       (*Bot).applyScale,
	"scalecustom":    (*Bot).promptCustomScale,
	"logsopts":       (*Bot).showLogOptions,
	cmdLogs:          (*Bot).detailPodLogs,
	"logsfollow":     (*Bot).showFollowLogs,
	"logsfollowstop": (*Bot).showFollowLogsStop,
	"logsprevious":   (*Bot).showPreviousLogs,
	"logsfull":       (*Bot).showFullLog,
	"nsresources":    (*Bot).showNamespaceSummary,
	"help":           (*Bot).showResourceHelp,
	"decoded":        (*Bot).detailDecodedSecret,
}

// dispatchDetailAction handles the verbs reachable from a detail pane. It
// returns false for actions it does not own, so the caller can fall through to
// the older handlers.
func (b *Bot) dispatchDetailAction(
	ctx context.Context,
	chatID int64,
	messageID int,
	action *menus.CallbackAction,
	session *types.UserSession,
) bool {
	handler, ok := detailVerbs[action.Action]
	if !ok {
		return false
	}
	handler(b, ctx, detailReq{
		chatID:    chatID,
		messageID: messageID,
		kind:      menus.CanonicalResource(action.ResourceType),
		ns:        action.Namespace,
		name:      action.Name,
		container: action.Container,
		extra:     action.Extra,
		session:   session,
	})
	return true
}

// detailDescribe renders the full object into the pane.
//
// This used to delegate to the /describe command handler to keep one
// implementation. That handler sends a message, which is what the single pane
// forbids, so the shared piece is now the renderer (formatters.RichResource)
// rather than the handler — the menu and the typed command still cannot render
// the same object differently, because they call the same formatter.
func (b *Bot) detailDescribe(ctx context.Context, r detailReq) {
	res, err := b.getResource(ctx, r.kind, r.ns, r.name, r.session)
	if err != nil {
		b.reportPaneError(ctx, r, "describe "+r.kind, err)
		return
	}
	b.showVerbResult(ctx, r,
		formatters.RichResource(res, true),
		formatters.FormatResource(res, "wide"))
}

// detailDecodedSecret fetches a secret and displays its decoded values.
// Sensitive values are wrapped in Telegram spoiler text (||value||).
func (b *Bot) detailDecodedSecret(ctx context.Context, r detailReq) {
	if r.kind != "secrets" {
		b.reportPaneError(ctx, r, "decode secret", fmt.Errorf("decoded action only available for secrets"))
		return
	}

	res, err := b.getResource(ctx, r.kind, r.ns, r.name, r.session)
	if err != nil {
		b.reportPaneError(ctx, r, "get secret", err)
		return
	}

	// Extract secret data from resource Details
	dataMap, ok := res.Details["data"].(map[string]interface{})
	if !ok {
		b.showNotice(ctx, r, "No secret data", "This secret has no data field or it is empty.")
		return
	}

	// Pre-allocate: at least one header line + one per key (max len(dataMap))
	lines := make([]string, 0, len(dataMap)+1)
	lines = append(lines, "<b>Decoded Secret: "+formatters.EscapeHTML(r.ns)+"/"+formatters.EscapeHTML(r.name)+"</b>\n")

	for key, val := range dataMap {
		strVal, ok := val.(string)
		if !ok {
			continue
		}
		// Decode base64
		decoded, err := base64.StdEncoding.DecodeString(strVal)
		if err != nil {
			lines = append(lines, formatters.EscapeHTML(key)+": ||<i>decode error</i>||")
			continue
		}
		decodedStr := string(decoded)
		// Wrap in spoiler text
		lines = append(lines, formatters.EscapeHTML(key)+": ||"+formatters.EscapeHTML(decodedStr)+"||")
	}

	if len(lines) == 1 {
		lines = append(lines, "<i>No data keys found</i>")
	}

	text := strings.Join(lines, "\n")
	b.showVerbResult(ctx, r, text, text)
}

// detailPodLogs serves the per-container and tail-size log buttons: Container
// is the container name, Extra the optional line count.
func (b *Bot) detailPodLogs(ctx context.Context, r detailReq) {
	if r.name == "" {
		return
	}
	opts := k8s.PodLogOptions{Namespace: r.ns, PodName: r.name, Container: r.container}
	if r.extra != "" {
		if n, err := strconv.ParseInt(r.extra, 10, 64); err == nil && n > 0 {
			opts.TailLines = types.Int64Ptr(n)
		}
	}
	logs, err := b.fetchLogs(ctx, opts, r.session)
	if err != nil {
		b.reportPaneError(ctx, r, "read logs", err)
		return
	}
	label := r.ns + "/" + r.name
	if r.container != "" {
		label += " · " + r.container
	}
	b.showLogResult(ctx, r, formatters.RichLogs(label, "", logs),
		"Logs for "+formatters.EscapeHTML(label))
}

func (b *Bot) showResourceHelp(ctx context.Context, r detailReq) {
	b.showVerbResult(ctx, r,
		formatters.RichHelpForResource(r.kind),
		"Actions for "+formatters.EscapeHTML(r.kind))
}

func (b *Bot) getResource(ctx context.Context, kind, namespace, name string, session *types.UserSession) (*k8s.ResourceInfo, error) {
	gvr, known := types.ResourceMap[kind]
	if !known {
		return nil, fmt.Errorf("unknown resource type %q", kind)
	}
	if types.IsClusterScoped(kind) {
		namespace = ""
	}
	return b.K8sClientForSession(session).GetResource(ctx, gvr.GVR(), namespace, name)
}

func (b *Bot) showLabels(ctx context.Context, r detailReq) {
	res, err := b.getResource(ctx, r.kind, r.ns, r.name, r.session)
	if err != nil {
		b.reportPaneError(ctx, r, "read labels", err)
		return
	}
	b.showVerbResult(ctx, r, formatters.RichLabels(res),
		fmt.Sprintf("%s: %s", formatters.EscapeHTML(r.name),
			formatters.EscapeHTML(labelsFallback(res))))
}

func labelsFallback(r *k8s.ResourceInfo) string {
	if r == nil || len(r.Labels) == 0 {
		return "no labels"
	}
	keys := formatters.SortedKeys(r.Labels)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+r.Labels[k])
	}
	return strings.Join(parts, ", ")
}

// showObjectEvents lists events whose involvedObject is this resource. The
// field selector is applied server-side so a busy namespace does not have to be
// pulled down in full.
func (b *Bot) showObjectEvents(ctx context.Context, r detailReq) {
	events, err := b.K8sClientForSession(r.session).GetEvents(ctx, r.ns, "involvedObject.name="+r.name)
	if err != nil {
		b.reportPaneError(ctx, r, "read events", err)
		return
	}
	if len(events) == 0 {
		b.showNotice(ctx, r, "No recent events for "+r.name,
			"Events expire after about an hour, so this is normal for a stable object.")
		return
	}
	b.showVerbResult(ctx, r, formatters.RichEvents(events),
		fmt.Sprintf("%d event(s) for %s", len(events), formatters.EscapeHTML(r.name)))
}

func (b *Bot) showSelector(ctx context.Context, r detailReq) {
	res, err := b.getResource(ctx, r.kind, r.ns, r.name, r.session)
	if err != nil {
		b.reportPaneError(ctx, r, "read selector", err)
		return
	}
	selector, complete := k8s.SelectorForWorkload(res)
	if selector == "" {
		b.showNotice(ctx, r, "No pod selector", r.name+" does not select pods.")
		return
	}

	pods, listErr := b.K8sClientForSession(r.session).ListPods(ctx, r.ns, selector, "")
	if listErr != nil {
		b.reportPaneError(ctx, r, "list matching pods", listErr)
		return
	}

	rich := formatters.RichSelector(res, selector, pods)
	if !complete {
		// Silently showing a partial match would misreport ownership.
		rich += "\n\n> " + formatters.GlyphDestructive +
			" This selector also has matchExpressions, which are not " +
			"applied here — the real selection may be narrower."
	}
	b.showVerbResult(ctx, r, rich,
		fmt.Sprintf("%s selector: <code>%s</code> — %d pod(s)",
			formatters.EscapeHTML(r.name), formatters.EscapeHTML(selector), len(pods)))
}

func (b *Bot) showEndpoints(ctx context.Context, r detailReq) {
	ep, err := b.K8sClientForSession(r.session).GetEndpoints(ctx, r.ns, r.name)
	if err != nil {
		b.reportPaneError(ctx, r, "read endpoints", err)
		return
	}
	ready, notReady := k8s.EndpointAddresses(ep)
	b.showVerbResult(ctx, r, formatters.RichEndpoints(r.name, ready, notReady),
		fmt.Sprintf("%s: %d ready, %d not ready",
			formatters.EscapeHTML(r.name), len(ready), len(notReady)))
}

// showWorkloadPods lists the pods a deployment/replicaset owns, as a browsable
// pod list so each result is still tappable.
func (b *Bot) showWorkloadPods(ctx context.Context, r detailReq) {
	pods, selector, err := b.K8sClientForSession(r.session).ListPodsForWorkload(ctx, r.kind, r.ns, r.name)
	if err != nil {
		b.reportPaneError(ctx, r, "list pods for "+r.kind, err)
		return
	}
	b.showPodResults(ctx, r, pods, r.ns,
		fmt.Sprintf("Pods of %s %s", r.kind, r.name),
		fmt.Sprintf("selector: `%s`", selector))
}

func (b *Bot) showNodePods(ctx context.Context, r detailReq) {
	pods, err := b.K8sClientForSession(r.session).ListPodsOnNode(ctx, r.name)
	if err != nil {
		b.reportPaneError(ctx, r, "list pods on node", err)
		return
	}
	b.showPodResults(ctx, r, pods, "",
		"Pods on node "+r.name, "Across all namespaces.")
}

// showPodResults renders an ad-hoc pod list with the standard list keyboard, so
// results from a workload or node drill-down behave like any other pod list.
func (b *Bot) showPodResults(
	ctx context.Context,
	r detailReq,
	pods []k8s.ResourceInfo,
	ns,
	heading,
	note string,
) {
	if len(pods) == 0 {
		b.showNotice(ctx, r, heading, "None found.")
		return
	}

	r.session.SetMenuState(&types.MenuState{
		CurrentView:  "resource_list",
		ResourceType: kindPods,
		Namespace:    ns,
	})

	pageSize := b.menuBuilder.GetPageSize()
	end := pageSize
	if end > len(pods) {
		end = len(pods)
	}

	kb := b.menuBuilder.GetResourceListInlineKeyboard(kindPods, pods, 0, pageSize, ns)
	rich := fmt.Sprintf("### %s — %d\n\n%s\n\n%s",
		heading, len(pods), note, formatters.RichResourceList(pods[:end], false))
	b.showPane(ctx, r.chatID, r.messageID, pane{
		rich:     rich,
		fallback: fmt.Sprintf("<b>%s</b> — %d", formatters.EscapeHTML(heading), len(pods)),
		kb:       &kb,
	})
}

func (b *Bot) showNodeTop(ctx context.Context, r detailReq) {
	metrics, err := b.K8sClientForSession(r.session).GetNodeMetrics(ctx)
	if err != nil {
		// metrics-server is optional; say so rather than showing a raw 404.
		b.showNotice(ctx, r, "Metrics unavailable for "+r.name,
			"This needs metrics-server installed in the cluster. "+err.Error())
		return
	}

	// GetNodeMetrics returns every node; narrow to the one that was tapped.
	var mine []k8s.ResourceInfo
	for i := range metrics {
		if metrics[i].Name == r.name {
			mine = append(mine, metrics[i])
		}
	}
	if len(mine) == 0 {
		b.showNotice(ctx, r, "No metrics for "+r.name,
			"metrics-server is reachable but reported nothing for this node.")
		return
	}
	b.showVerbResult(ctx, r, formatters.RichMetrics("Node usage — "+r.name, mine),
		"Usage for "+formatters.EscapeHTML(r.name))
}

func (b *Bot) showRolloutHistory(ctx context.Context, r detailReq) {
	revisions, err := b.K8sClientForSession(r.session).RolloutHistory(ctx, r.ns, r.name)
	if err != nil {
		b.reportPaneError(ctx, r, "read rollout history", err)
		return
	}
	b.showVerbResult(ctx, r, formatters.RichRolloutHistory(r.name, revisions),
		fmt.Sprintf("%s: %d revision(s)", formatters.EscapeHTML(r.name), len(revisions)))
}

// showManifest renders the live object as YAML.
//
// This is the read half of what an "Edit" button implies. Writing a manifest
// back from a chat message is not offered: there is no way to show a diff or
// take a lock, so an apply from here could silently overwrite a change someone
// else made seconds earlier.
func (b *Bot) showManifest(ctx context.Context, r detailReq) {
	res, err := b.getResource(ctx, r.kind, r.ns, r.name, r.session)
	if err != nil {
		b.reportPaneError(ctx, r, "read manifest", err)
		return
	}
	if res == nil || res.Details == nil {
		b.showNotice(ctx, r, "No manifest available", "The API server returned no object body.")
		return
	}

	out, marshalErr := yaml.Marshal(res.Details)
	if marshalErr != nil {
		b.reportPaneError(ctx, r, "render manifest", marshalErr)
		return
	}

	// Left below the pane budget so the fenced block plus the note still fit;
	// showPane's own truncation would otherwise cut inside the fence.
	text := string(out)
	const maxManifest = 2600
	truncated := false
	if len(text) > maxManifest {
		text = string([]rune(text)[:maxManifest])
		truncated = true
	}

	rich := formatters.RichManifest(res, text)
	if truncated {
		rich += "\n\n> Truncated. Use `/get " + r.kind + " " + r.name +
			" -o yaml` for the full manifest."
	}
	b.showVerbResult(ctx, r, rich, "Manifest for "+formatters.EscapeHTML(r.name))
}

// setNodeSchedulable cordons or uncordons, then re-renders the node's detail
// pane so the keyboard reflects the new state.
//
// The outcome is reported in the pane the confirmation was showing, and the
// detail pane it returns to is a fresh read — so the result the user sees is the
// cluster's state, not this function's assumption about it.
func (b *Bot) setNodeSchedulable(ctx context.Context, r detailReq, schedulable bool) {
	verb := "Cordon"
	if schedulable {
		verb = "Uncordon"
	}

	client := b.K8sClientForSession(r.session)
	if err := client.SetNodeSchedulable(ctx, r.name, schedulable); err != nil {
		b.reportPaneError(ctx, r, strings.ToLower(verb)+" node", err)
		return
	}

	// Under dry run nothing changed, so re-rendering the pane would show the old
	// state with no hint that the tap was a no-op. Say so instead.
	if client.IsDryRun() {
		b.showNotice(ctx, r, "Dry run — "+strings.ToLower(verb)+" not applied",
			"telectl is running with dry-run enabled, so "+r.name+" was not changed.")
		return
	}

	// Re-read rather than assume: the detail pane is the report.
	b.showResourceDetail(ctx, r.chatID, r.messageID, kindNodes, "", r.name, r.session)
}

// confirmDrain gates the drain behind an explicit confirmation. Drain evicts
// every eligible pod on a node; it is the most disruptive thing this bot can
// do from a single tap.
func (b *Bot) confirmDrain(ctx context.Context, r detailReq) {
	pods, err := b.K8sClientForSession(r.session).ListPodsOnNode(ctx, r.name)
	if err != nil {
		b.reportPaneError(ctx, r, "inspect node before drain", err)
		return
	}

	kb := tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			b.menuBuilder.Button(formatters.Btn(formatters.GlyphDestructive, "Yes, drain"),
				"menu:action:confirmdrain:"+kindNodes+"::"+r.name),
			b.menuBuilder.Button(formatters.Btn(formatters.GlyphCancel, "Cancel"),
				"menu:resource:view:"+kindNodes+"::"+r.name),
		),
	)
	b.editView(ctx, r.chatID, r.messageID, fmt.Sprintf(
		"<b>Drain node <code>%s</code>?</b>\n\n"+
			"It currently runs <b>%d</b> pod(s).\n\n"+
			"The node will be cordoned and its pods evicted. DaemonSet and static "+
			"pods are left in place. Evictions respect PodDisruptionBudgets, so "+
			"some may be refused.",
		formatters.EscapeHTML(r.name), len(pods)), &kb)
}

func (b *Bot) drainNode(ctx context.Context, r detailReq) {
	client := b.K8sClientForSession(r.session)
	res, err := client.DrainNode(ctx, r.name)
	if err != nil {
		b.reportPaneError(ctx, r, "drain node", err)
		return
	}

	// The result is the report, and it is what the user needs to read: which
	// pods were evicted, skipped or refused. Returning straight to the detail
	// pane would discard it.
	b.showVerbResult(ctx, r, formatters.RichDrainResult(r.name, res),
		fmt.Sprintf("Drained %s: %d evicted, %d skipped, %d failed",
			formatters.EscapeHTML(r.name), len(res.Evicted), len(res.Skipped), len(res.Failed)))
}

func (b *Bot) showScaleOptions(ctx context.Context, r detailReq) {
	var (
		current int32
		err     error
	)
	client := b.K8sClientForSession(r.session)
	switch r.kind {
	case kindDeployments:
		current, err = client.GetDeploymentReplicas(ctx, r.ns, r.name)
	case kindReplicaSets:
		current, err = client.GetReplicaSetReplicas(ctx, r.ns, r.name)
	default:
		b.showNotice(ctx, r, "Not scalable", "Only deployments and replicasets can be scaled.")
		return
	}
	if err != nil {
		b.reportPaneError(ctx, r, "read replica count", err)
		return
	}

	text := fmt.Sprintf("<b>Scale %s</b>\n\n<code>%s</code> in <code>%s</code>\nCurrent replicas: <b>%d</b>",
		formatters.EscapeHTML(r.kind), formatters.EscapeHTML(r.name),
		formatters.EscapeHTML(nsDisplay(r.ns)), current)
	if r.kind == kindReplicaSets {
		text += "\n\n" + formatters.GlyphDestructive +
			" If a Deployment owns this ReplicaSet, its controller will " +
			"revert the change within seconds. Scale the Deployment instead."
	}

	kb := b.menuBuilder.GetScaleKeyboard(r.kind, r.ns, r.name, current)
	b.editView(ctx, r.chatID, r.messageID, text, &kb)
}

// applyScale sets the replica count, then re-renders the detail pane so the
// reported figure is one the API server confirmed.
func (b *Bot) applyScale(ctx context.Context, r detailReq) {
	replicas, err := strconv.ParseInt(r.extra, 10, 32)
	if err != nil || replicas < 0 {
		b.showNotice(ctx, r, "Invalid replica count",
			"Could not read a replica count from "+r.extra+".")
		return
	}

	client := b.K8sClientForSession(r.session)
	b.logger.Info("User action: scale resource",
		zap.Int64("telegram_user_id", r.session.UserID),
		zap.String("resource_type", r.kind),
		zap.String("namespace", r.ns),
		zap.String("name", r.name),
		zap.Int32("replicas", int32(replicas)),
	)

	switch r.kind {
	case kindDeployments:
		err = client.ScaleDeployment(ctx, r.ns, r.name, int32(replicas))
	case kindReplicaSets:
		err = client.ScaleReplicaSet(ctx, r.ns, r.name, int32(replicas))
	default:
		b.showNotice(ctx, r, "Not scalable", "Only deployments and replicasets can be scaled.")
		return
	}
	if err != nil {
		b.reportPaneError(ctx, r, "scale "+r.kind, err)
		return
	}

	if client.IsDryRun() {
		b.showNotice(ctx, r, "Dry run — scale not applied",
			fmt.Sprintf("telectl is running with dry-run enabled, so %s was not scaled to %d.",
				r.name, replicas))
		return
	}

	// Scaling is asynchronous, so the pane shows the spec value the controller
	// is converging to rather than a count this function asserts.
	b.showResourceDetail(ctx, r.chatID, r.messageID, r.kind, r.ns, r.name, r.session)
}

func (b *Bot) promptCustomScale(ctx context.Context, r detailReq) {
	singular := strings.TrimSuffix(r.kind, "s")
	b.showUsage(ctx, r, "Custom replica count",
		fmt.Sprintf("/scale %s %s <replicas> -n %s", singular, r.name, nsDisplay(r.ns)))
}

func (b *Bot) showLogOptions(ctx context.Context, r detailReq) {
	container := ""
	if pod, err := b.K8sClientForSession(r.session).GetPod(ctx, r.ns, r.name); err == nil {
		if names := podContainers(pod); len(names) > 0 {
			container = names[0]
		}
	}
	kb := b.menuBuilder.GetLogOptionsKeyboard(r.ns, r.name, container)
	label := "first container"
	if container != "" {
		label = "<code>" + formatters.EscapeHTML(container) + "</code>"
	}
	b.editView(ctx, r.chatID, r.messageID, fmt.Sprintf(
		"<b>Logs</b>\n\nPod <code>%s</code> in <code>%s</code>\nContainer: %s\n\nPick how much to fetch.",
		formatters.EscapeHTML(r.name), formatters.EscapeHTML(nsDisplay(r.ns)), label), &kb)
}

func podContainers(pod *k8s.ResourceInfo) []string {
	if pod == nil || pod.Details == nil {
		return nil
	}
	spec, ok := pod.Details["spec"].(map[string]interface{})
	if !ok {
		return nil
	}
	raw, ok := spec["containers"].([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, c := range raw {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if n, ok := cm["name"].(string); ok && n != "" {
			out = append(out, n)
		}
	}
	return out
}

// followLogState tracks a single live follow-logs operation.
type followLogState struct {
	cancel  context.CancelFunc
	message int // messageID being edited
}

// showFollowLogs starts a live log follow: it edits the current pane to show
// logs with a Stop button, then spawns a goroutine that re-fetches and edits
// the same pane message every 5 seconds for up to 5 minutes (60 iterations).
func (b *Bot) showFollowLogs(ctx context.Context, r detailReq) {
	if r.name == "" || r.ns == "" {
		return
	}

	// Create a cancellable context for this follow session.
	followCtx, cancel := context.WithCancel(context.Background())

	// Store the cancellation state in the session so the Stop button can find it.
	stateKey := "followlog:" + r.ns + "/" + r.name + "/" + r.container
	r.session.SetState(stateKey, &followLogState{cancel: cancel, message: r.messageID})

	// Fetch initial snapshot.
	opts := k8s.PodLogOptions{
		Namespace: r.ns,
		PodName:   r.name,
		Container: r.container,
		TailLines: types.Int64Ptr(200),
	}
	logs, err := b.fetchLogs(followCtx, opts, r.session)
	if err != nil {
		b.reportPaneError(ctx, r, "read logs", err)
		cancel()
		r.session.DeleteState(stateKey)
		return
	}

	// Truncate before rendering: 200 lines of logs comfortably overrun
	// Telegram's message limit, and an over-long body makes the edit fail with
	// MESSAGE_TOO_LONG and the follow silently freeze. Keep the tail (newest
	// lines) and repair the code fence, exactly like the one-shot log view.
	if len(logs) > paneLimit {
		logs = truncateForPaneTail(formatters.RichLogs(r.ns+"/"+r.name, r.container, logs))
	} else {
		logs = formatters.RichLogs(r.ns+"/"+r.name, r.container, logs)
	}

	// Build Stop button keyboard.
	stopData := "menu:action:logsfollowstop:pods:" + r.ns + ":" + r.name
	if r.container != "" {
		stopData += ":" + r.container
	}
	kb := tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			tg.InlineButtonData("⏹ Stop", stopData),
		),
	)

	label := r.ns + "/" + r.name
	if r.container != "" {
		label += " · " + r.container
	}
	rich := logs + "\n\n> Live follow active — updates every 5s for 5 min. Tap Stop to end."
	fallback := "Live follow: " + formatters.EscapeHTML(label)

	// Render through the pane so it is truncated and has a way out. The Stop
	// keyboard replaces the normal verb keyboard for the duration of a follow.
	b.showPane(ctx, r.chatID, r.messageID, pane{
		rich:     rich,
		fallback: fallback,
		kb:       &kb,
		keepTail: true,
	})

	// Start the background updater.
	go b.runFollowLogsUpdater(followCtx, r, stateKey)
}

// runFollowLogsUpdater fetches logs and edits the pane message every 5s for 5 min.
func (b *Bot) runFollowLogsUpdater(ctx context.Context, r detailReq, stateKey string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	iterations := 0
	const maxIterations = 60 // 5 minutes / 5 seconds

	for {
		select {
		case <-ctx.Done():
			// Clean up session state.
			r.session.DeleteState(stateKey)
			return
		case <-ticker.C:
			iterations++
			if iterations > maxIterations {
				// Time's up — show final message in pane and clean up.
				finalRich := formatters.RichLogs(r.ns+"/"+r.name, r.container, "") +
					"\n\n> Live follow ended (5 min limit reached)."
				b.showPane(ctx, r.chatID, r.messageID, b.verbPane(r, finalRich,
					"Follow ended: "+formatters.EscapeHTML(r.ns+"/"+r.name)))
				r.session.DeleteState(stateKey)
				return
			}

			// Fetch latest logs.
			opts := k8s.PodLogOptions{
				Namespace: r.ns,
				PodName:   r.name,
				Container: r.container,
				TailLines: types.Int64Ptr(200),
			}
			logs, err := b.fetchLogs(ctx, opts, r.session)
			if err != nil {
				// Don't stop on transient fetch errors; just skip this iteration.
				b.logger.Debug("Follow-logs fetch failed", zap.Error(err), zap.String("key", stateKey))
				continue
			}

			label := r.ns + "/" + r.name
			if r.container != "" {
				label += " · " + r.container
			}
			// Truncate to fit the pane; keep the newest lines.
			rich := formatters.RichLogs(label, r.container, logs)
			if len(rich) > paneLimit {
				rich = truncateForPaneTail(rich)
			}
			rich += "\n\n> Live follow active — updates every 5s for 5 min. Tap Stop to end."

			// Edit the pane message, reusing the Stop keyboard from the follow
			// so the user can still cancel.
			stopData := "menu:action:logsfollowstop:pods:" + r.ns + ":" + r.name
			if r.container != "" {
				stopData += ":" + r.container
			}
			stopKB := tg.InlineKeyboard(
				tg.InlineKeyboardRow(
					tg.InlineButtonData("⏹ Stop", stopData),
				),
			)
			fallback := "Live follow: " + formatters.EscapeHTML(label)
			b.showPane(ctx, r.chatID, r.messageID, pane{
				rich:     rich,
				fallback: fallback,
				kb:       &stopKB,
				keepTail: true,
			})
		}
	}
}

// showFollowLogsStop handles the Stop button for live follow-logs.
func (b *Bot) showFollowLogsStop(ctx context.Context, r detailReq) {
	stateKey := "followlog:" + r.ns + "/" + r.name + "/" + r.container

	if v, ok := r.session.GetState(stateKey); ok {
		if s, ok := v.(*followLogState); ok && s.cancel != nil {
			s.cancel()
		}
	}
	r.session.DeleteState(stateKey)

	// Leave the pane in the normal post-follow state: the current log tail with
	// the standard verb keyboard, so the user is not stranded on a Stop button
	// that no longer does anything.
	opts := k8s.PodLogOptions{
		Namespace: r.ns,
		PodName:   r.name,
		Container: r.container,
		TailLines: types.Int64Ptr(200),
	}
	logs, err := b.fetchLogs(ctx, opts, r.session)
	if err != nil {
		b.reportPaneError(ctx, r, "read logs", err)
		return
	}
	b.showLogResult(ctx, r, formatters.RichLogs(r.ns+"/"+r.name, r.container, logs),
		"Logs for "+formatters.EscapeHTML(r.ns+"/"+r.name))
}

func (b *Bot) showPreviousLogs(ctx context.Context, r detailReq) {
	logs, err := b.fetchLogs(ctx, k8s.PodLogOptions{
		Namespace: r.ns, PodName: r.name, Container: r.container,
		Previous: true, TailLines: types.Int64Ptr(200),
	}, r.session)
	if err != nil {
		// The common case is a container that has never restarted, which the
		// API server reports as a somewhat opaque error.
		b.showNotice(ctx, r, "No previous logs for "+r.name,
			"This usually means the container has not restarted, so there is no "+
				"earlier instance to read. "+err.Error())
		return
	}
	b.showLogResult(ctx, r,
		formatters.RichLogs(r.ns+"/"+r.name+" (previous)", r.container, logs),
		"Previous logs for "+formatters.EscapeHTML(r.name))
}

// showFullLogs sends the entire log tail as a fresh message, not the pane.
//
// The pane truncates long bodies to fit a single message; for a log tail that
// can mean the detail view shows a slice even when the user asked for a full
// container. "Full log" bypasses the pane cap and posts the whole requested
// tail as its own message (split if it exceeds Telegram's 4096-char limit),
// so nothing the API server returned is hidden.
func (b *Bot) showFullLog(ctx context.Context, r detailReq) {
	if r.name == "" {
		return
	}
	opts := k8s.PodLogOptions{Namespace: r.ns, PodName: r.name, Container: r.container}
	if r.extra != "" {
		if n, err := strconv.ParseInt(r.extra, 10, 64); err == nil && n > 0 {
			opts.TailLines = types.Int64Ptr(n)
		}
	}
	logs, err := b.fetchLogs(ctx, opts, r.session)
	if err != nil {
		b.reportPaneError(ctx, r, "read full logs", err)
		return
	}

	label := r.ns + "/" + r.name
	if r.container != "" {
		label += " · " + r.container
	}
	b.SendRich(r.chatID, formatters.RichLogs(label, r.container, logs),
		"Logs for "+formatters.EscapeHTML(label))
}

func (b *Bot) fetchLogs(ctx context.Context, opts k8s.PodLogOptions, session *types.UserSession) (string, error) {
	reader, err := b.K8sClientForSession(session).GetPodLogs(ctx, opts)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	if len(raw) == 0 {
		return "(no output)", nil
	}
	// Cap only when the caller did not ask for a specific tail: the API server
	// already limits the fetch when TailLines is set, so re-capping here would
	// discard lines the user explicitly requested.
	formatted := string(raw)
	if opts.TailLines == nil || *opts.TailLines <= 0 {
		formatted = formatters.FormatPodLogs(formatted, 200)
	}
	return formatted, nil
}

func (b *Bot) showNamespaceSummary(ctx context.Context, r detailReq) {
	if r.name == "" {
		b.showNotice(ctx, r, "No namespace selected", "Pick a namespace first.")
		return
	}
	summary := b.K8sClientForSession(r.session).SummariseNamespace(ctx, r.name)
	b.showVerbResult(ctx, r, formatters.RichNamespaceSummary(summary),
		"Contents of "+formatters.EscapeHTML(r.name))
}
