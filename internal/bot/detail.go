package bot

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

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
	gvr, known := types.ResourceMap[kind]
	if !known {
		b.SendMessage(chatID, fmt.Sprintf("❌ Unknown resource type: %s", formatters.EscapeHTML(resourceType)))
		return
	}
	if types.IsClusterScoped(kind) {
		namespace = ""
	}

	resource, err := b.k8sClient.GetResource(ctx, gvr.GVR(), namespace, name)
	if err != nil {
		b.reportError(chatID, "load "+kind, err)
		return
	}

	session.SetMenuState(&types.MenuState{
		CurrentView:  "resource_detail",
		ResourceType: kind,
		Namespace:    namespace,
		Filter:       name,
	})

	kb := b.menuBuilder.GetResourceActionInlineKeyboard(kind, namespace, name, resource)
	b.editRichView(ctx, chatID, messageID,
		formatters.RichResourceSummary(resource),
		fmt.Sprintf("%s <b>%s</b>\n%s in %s",
			formatters.StatusEmoji(resource.Status),
			formatters.EscapeHTML(name),
			formatters.EscapeHTML(resource.Kind),
			formatters.EscapeHTML(nsDisplay(namespace))),
		&kb)
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
	kind := menus.CanonicalResource(action.ResourceType)
	ns := action.Namespace
	name := action.Name

	switch action.Action {
	case "describe":
		if kind == "" || name == "" {
			return true
		}
		args := []string{kind, name}
		if ns != "" {
			args = append(args, "-n", ns)
		}
		// Reuses the /describe handler so the menu and the typed command can
		// never render the same object differently.
		b.runCommand(ctx, "describe", args, chatID, session)

	case "labels":
		b.showLabels(ctx, chatID, kind, ns, name)

	case "events":
		b.showObjectEvents(ctx, chatID, ns, name)

	case "selector":
		b.showSelector(ctx, chatID, kind, ns, name)

	case "endpoints":
		b.showEndpoints(ctx, chatID, ns, name)

	case "pods", "rspods":
		b.showWorkloadPods(ctx, chatID, messageID, kind, ns, name, session)

	case "nodepods":
		b.showNodePods(ctx, chatID, messageID, name, session)

	case "top":
		b.showNodeTop(ctx, chatID, name)

	case "history":
		b.showRolloutHistory(ctx, chatID, ns, name)

	case "edit":
		b.showManifest(ctx, chatID, kind, ns, name)

	case "cordon":
		b.setNodeSchedulable(ctx, chatID, messageID, name, false, session)

	case "uncordon":
		b.setNodeSchedulable(ctx, chatID, messageID, name, true, session)

	case "drain":
		b.confirmDrain(ctx, chatID, messageID, name)

	case "confirmdrain":
		b.drainNode(ctx, chatID, messageID, name, session)

	case "scale", "rsscale":
		b.showScaleOptions(ctx, chatID, messageID, kind, ns, name)

	case "scaleset":
		b.applyScale(ctx, chatID, messageID, kind, ns, name, action.Extra, session)

	case "scalecustom":
		b.promptCustomScale(chatID, kind, ns, name)

	case "logsopts":
		b.showLogOptions(ctx, chatID, messageID, ns, name)

	case cmdLogs:
		// Per-container and tail-size buttons: Container is the container name,
		// Extra the optional line count. Routed here rather than to the legacy
		// path so the container selection is not silently dropped.
		if name == "" {
			return false
		}
		b.showPodLogs(ctx, chatID, ns, name, action.Container, action.Extra)

	case "logsfollow":
		b.showFollowLogs(ctx, chatID, ns, name, action.Container)

	case "logsprevious":
		b.showPreviousLogs(ctx, chatID, ns, name, action.Container)

	case "nsresources":
		b.showNamespaceSummary(ctx, chatID, name)

	case "help":
		b.SendRich(chatID, formatters.RichHelpForResource(kind),
			"Actions for "+formatters.EscapeHTML(kind))

	default:
		return false
	}
	return true
}

// reportError logs the underlying failure and gives the user the API server's
// own message, which is almost always the actionable part (Forbidden, NotFound,
// "metrics-server not available").
func (b *Bot) reportError(chatID int64, what string, err error) {
	b.logger.Error("Menu action failed", zap.String("action", what), zap.Error(err))
	b.SendMessage(chatID, fmt.Sprintf("❌ Failed to %s: %s",
		formatters.EscapeHTML(what), formatters.EscapeHTML(err.Error())))
}

func (b *Bot) getResource(ctx context.Context, kind, namespace, name string) (*k8s.ResourceInfo, error) {
	gvr, known := types.ResourceMap[kind]
	if !known {
		return nil, fmt.Errorf("unknown resource type %q", kind)
	}
	if types.IsClusterScoped(kind) {
		namespace = ""
	}
	return b.k8sClient.GetResource(ctx, gvr.GVR(), namespace, name)
}

func (b *Bot) showLabels(ctx context.Context, chatID int64, kind, ns, name string) {
	r, err := b.getResource(ctx, kind, ns, name)
	if err != nil {
		b.reportError(chatID, "read labels", err)
		return
	}
	b.SendRich(chatID, formatters.RichLabels(r),
		fmt.Sprintf("🏷️ %s: %s", formatters.EscapeHTML(name),
			formatters.EscapeHTML(labelsFallback(r))))
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
func (b *Bot) showObjectEvents(ctx context.Context, chatID int64, ns, name string) {
	selector := "involvedObject.name=" + name
	events, err := b.k8sClient.GetEvents(ctx, ns, selector)
	if err != nil {
		b.reportError(chatID, "read events", err)
		return
	}
	if len(events) == 0 {
		b.SendMessage(chatID, fmt.Sprintf("📭 No recent events for <code>%s</code>.\n\nEvents expire after about an hour, so this is normal for a stable object.",
			formatters.EscapeHTML(name)))
		return
	}
	b.SendRich(chatID, formatters.RichEvents(events),
		fmt.Sprintf("📅 %d event(s) for %s", len(events), formatters.EscapeHTML(name)))
}

func (b *Bot) showSelector(ctx context.Context, chatID int64, kind, ns, name string) {
	r, err := b.getResource(ctx, kind, ns, name)
	if err != nil {
		b.reportError(chatID, "read selector", err)
		return
	}
	selector, complete := k8s.SelectorForWorkload(r)
	if selector == "" {
		b.SendMessage(chatID, fmt.Sprintf("ℹ️ <code>%s</code> has no pod selector.", formatters.EscapeHTML(name)))
		return
	}

	pods, listErr := b.k8sClient.ListPods(ctx, ns, selector, "")
	if listErr != nil {
		b.reportError(chatID, "list matching pods", listErr)
		return
	}

	rich := formatters.RichSelector(r, selector, pods)
	if !complete {
		// Silently showing a partial match would misreport ownership.
		rich += "\n\n> ⚠️ This selector also has matchExpressions, which are not applied here — the real selection may be narrower."
	}
	b.SendRich(chatID, rich,
		fmt.Sprintf("🎯 %s selector: <code>%s</code> — %d pod(s)",
			formatters.EscapeHTML(name), formatters.EscapeHTML(selector), len(pods)))
}

func (b *Bot) showEndpoints(ctx context.Context, chatID int64, ns, name string) {
	ep, err := b.k8sClient.GetEndpoints(ctx, ns, name)
	if err != nil {
		b.reportError(chatID, "read endpoints", err)
		return
	}
	ready, notReady := k8s.EndpointAddresses(ep)
	b.SendRich(chatID, formatters.RichEndpoints(name, ready, notReady),
		fmt.Sprintf("🔌 %s: %d ready, %d not ready",
			formatters.EscapeHTML(name), len(ready), len(notReady)))
}

// showWorkloadPods lists the pods a deployment/replicaset owns, as a browsable
// pod list so each result is still tappable.
func (b *Bot) showWorkloadPods(
	ctx context.Context,
	chatID int64,
	messageID int,
	kind,
	ns,
	name string,
	session *types.UserSession,
) {
	pods, selector, err := b.k8sClient.ListPodsForWorkload(ctx, kind, ns, name)
	if err != nil {
		b.reportError(chatID, "list pods for "+kind, err)
		return
	}
	b.showPodResults(ctx, chatID, messageID, pods, ns,
		fmt.Sprintf("📋 Pods of %s %s", kind, name),
		fmt.Sprintf("selector: `%s`", selector), session)
}

func (b *Bot) showNodePods(ctx context.Context, chatID int64, messageID int, node string, session *types.UserSession) {
	pods, err := b.k8sClient.ListPodsOnNode(ctx, node)
	if err != nil {
		b.reportError(chatID, "list pods on node", err)
		return
	}
	b.showPodResults(ctx, chatID, messageID, pods, "",
		fmt.Sprintf("📋 Pods on node %s", node),
		"Across all namespaces.", session)
}

// showPodResults renders an ad-hoc pod list with the standard list keyboard, so
// results from a workload or node drill-down behave like any other pod list.
func (b *Bot) showPodResults(
	ctx context.Context,
	chatID int64,
	messageID int,
	pods []k8s.ResourceInfo,
	ns,
	heading,
	note string,
	session *types.UserSession,
) {
	if len(pods) == 0 {
		b.SendMessage(chatID, "📭 "+formatters.EscapeHTML(heading)+" — none found.")
		return
	}

	session.SetMenuState(&types.MenuState{
		CurrentView:  "resource_list",
		ResourceType: "pods",
		Namespace:    ns,
	})

	pageSize := b.menuBuilder.GetPageSize()
	end := pageSize
	if end > len(pods) {
		end = len(pods)
	}

	kb := b.menuBuilder.GetResourceListInlineKeyboard("pods", pods, 0, pageSize, ns)
	rich := fmt.Sprintf("### %s — %d\n\n%s\n\n%s",
		heading, len(pods), note, formatters.RichResourceList(pods[:end], false))
	b.editRichView(ctx, chatID, messageID, rich,
		fmt.Sprintf("<b>%s</b> — %d", formatters.EscapeHTML(heading), len(pods)), &kb)
}

func (b *Bot) showNodeTop(ctx context.Context, chatID int64, node string) {
	metrics, err := b.k8sClient.GetNodeMetrics(ctx)
	if err != nil {
		// metrics-server is optional; say so rather than showing a raw 404.
		b.SendMessage(chatID, fmt.Sprintf(
			"📊 Metrics unavailable for <code>%s</code>.\n\nThis needs metrics-server installed in the cluster.\n\n<i>%s</i>",
			formatters.EscapeHTML(node), formatters.EscapeHTML(err.Error())))
		return
	}

	// GetNodeMetrics returns every node; narrow to the one that was tapped.
	var mine []k8s.ResourceInfo
	for i := range metrics {
		if metrics[i].Name == node {
			mine = append(mine, metrics[i])
		}
	}
	if len(mine) == 0 {
		b.SendMessage(chatID, fmt.Sprintf("📭 No metrics reported for node <code>%s</code>.",
			formatters.EscapeHTML(node)))
		return
	}
	b.SendRich(chatID, formatters.RichMetrics("📊 Node usage — "+node, mine),
		fmt.Sprintf("📊 Usage for %s", formatters.EscapeHTML(node)))
}

func (b *Bot) showRolloutHistory(ctx context.Context, chatID int64, ns, name string) {
	revisions, err := b.k8sClient.RolloutHistory(ctx, ns, name)
	if err != nil {
		b.reportError(chatID, "read rollout history", err)
		return
	}
	b.SendRich(chatID, formatters.RichRolloutHistory(name, revisions),
		fmt.Sprintf("📜 %s: %d revision(s)", formatters.EscapeHTML(name), len(revisions)))
}

// showManifest renders the live object as YAML.
//
// This is the read half of what an "Edit" button implies. Writing a manifest
// back from a chat message is not offered: there is no way to show a diff or
// take a lock, so an apply from here could silently overwrite a change someone
// else made seconds earlier.
func (b *Bot) showManifest(ctx context.Context, chatID int64, kind, ns, name string) {
	r, err := b.getResource(ctx, kind, ns, name)
	if err != nil {
		b.reportError(chatID, "read manifest", err)
		return
	}
	if r == nil || r.Details == nil {
		b.SendMessage(chatID, "📭 No manifest available.")
		return
	}

	out, marshalErr := yaml.Marshal(r.Details)
	if marshalErr != nil {
		b.reportError(chatID, "render manifest", marshalErr)
		return
	}
	text := string(out)
	const maxManifest = 3000
	truncated := false
	if len(text) > maxManifest {
		text = text[:maxManifest]
		truncated = true
	}

	rich := formatters.RichManifest(r, text)
	if truncated {
		rich += "\n\n> ✂️ Truncated. Use `/get " + kind + " " + name + " -o yaml` for the full manifest."
	}
	b.SendRich(chatID, rich, fmt.Sprintf("📄 Manifest for %s", formatters.EscapeHTML(name)))
}

func (b *Bot) setNodeSchedulable(
	ctx context.Context,
	chatID int64,
	messageID int,
	node string,
	schedulable bool,
	session *types.UserSession,
) {
	verb := "Cordon"
	if schedulable {
		verb = "Uncordon"
	}

	if err := b.k8sClient.SetNodeSchedulable(ctx, node, schedulable); err != nil {
		b.reportError(chatID, strings.ToLower(verb)+" node", err)
		return
	}

	note := "New pods will not be scheduled here. Pods already running are untouched — use Drain to move them."
	if schedulable {
		note = "The scheduler can place pods here again."
	}
	if b.k8sClient.IsDryRun() {
		note = "🧪 Dry run — nothing was changed."
	}

	b.SendRich(chatID, formatters.RichActionResult("🔧 "+verb+" — "+node,
		[][2]string{{"Node", node}, {"Schedulable", boolWord(schedulable)}}, note),
		fmt.Sprintf("🔧 %s %s", verb, formatters.EscapeHTML(node)))

	// Re-render the detail pane so the keyboard reflects the new state.
	b.showResourceDetail(ctx, chatID, messageID, "nodes", "", node, session)
}

func boolWord(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// confirmDrain gates the drain behind an explicit confirmation. Drain evicts
// every eligible pod on a node; it is the most disruptive thing this bot can
// do from a single tap.
func (b *Bot) confirmDrain(ctx context.Context, chatID int64, messageID int, node string) {
	pods, err := b.k8sClient.ListPodsOnNode(ctx, node)
	if err != nil {
		b.reportError(chatID, "inspect node before drain", err)
		return
	}

	kb := tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			b.menuBuilder.Button("✅ Yes, drain", "menu:action:confirmdrain:nodes::"+node),
			b.menuBuilder.Button("❌ Cancel", "menu:resource:view:nodes::"+node),
		),
	)
	b.editView(ctx, chatID, messageID, fmt.Sprintf(
		"⚠️ Drain node <code>%s</code>?\n\nIt currently runs <b>%d</b> pod(s).\n\nThe node will be cordoned and its pods evicted. DaemonSet and static pods are left in place. Evictions respect PodDisruptionBudgets, so some may be refused.",
		formatters.EscapeHTML(node), len(pods)), &kb)
}

func (b *Bot) drainNode(ctx context.Context, chatID int64, messageID int, node string, session *types.UserSession) {
	b.SendMessage(chatID, fmt.Sprintf("💤 Draining <code>%s</code>…", formatters.EscapeHTML(node)))

	res, err := b.k8sClient.DrainNode(ctx, node)
	if err != nil {
		b.reportError(chatID, "drain node", err)
		return
	}

	b.SendRich(chatID, formatters.RichDrainResult(node, res),
		fmt.Sprintf("💤 Drained %s: %d evicted, %d skipped, %d failed",
			formatters.EscapeHTML(node), len(res.Evicted), len(res.Skipped), len(res.Failed)))

	b.showResourceDetail(ctx, chatID, messageID, "nodes", "", node, session)
}

func (b *Bot) showScaleOptions(ctx context.Context, chatID int64, messageID int, kind, ns, name string) {
	var (
		current int32
		err     error
	)
	switch kind {
	case kindDeployments:
		current, err = b.k8sClient.GetDeploymentReplicas(ctx, ns, name)
	case kindReplicaSets:
		current, err = b.k8sClient.GetReplicaSetReplicas(ctx, ns, name)
	default:
		b.SendMessage(chatID, "❌ Only deployments and replicasets can be scaled.")
		return
	}
	if err != nil {
		b.reportError(chatID, "read replica count", err)
		return
	}

	text := fmt.Sprintf("📈 <b>Scale %s</b>\n\n<code>%s</code> in <code>%s</code>\nCurrent replicas: <b>%d</b>",
		formatters.EscapeHTML(kind), formatters.EscapeHTML(name),
		formatters.EscapeHTML(nsDisplay(ns)), current)
	if kind == kindReplicaSets {
		text += "\n\n⚠️ If a Deployment owns this ReplicaSet, its controller will revert the change within seconds. Scale the Deployment instead."
	}

	kb := b.menuBuilder.GetScaleKeyboard(kind, ns, name, current)
	b.editView(ctx, chatID, messageID, text, &kb)
}

func (b *Bot) applyScale(
	ctx context.Context,
	chatID int64,
	messageID int,
	kind,
	ns,
	name,
	replicasArg string,
	session *types.UserSession,
) {
	replicas, err := strconv.ParseInt(replicasArg, 10, 32)
	if err != nil || replicas < 0 {
		b.SendMessage(chatID, fmt.Sprintf("❌ Invalid replica count: %s", formatters.EscapeHTML(replicasArg)))
		return
	}

	switch kind {
	case kindDeployments:
		err = b.k8sClient.ScaleDeployment(ctx, ns, name, int32(replicas))
	case kindReplicaSets:
		err = b.k8sClient.ScaleReplicaSet(ctx, ns, name, int32(replicas))
	default:
		b.SendMessage(chatID, "❌ Only deployments and replicasets can be scaled.")
		return
	}
	if err != nil {
		b.reportError(chatID, "scale "+kind, err)
		return
	}

	note := "Scaling is asynchronous — the controller will converge to this count."
	if b.k8sClient.IsDryRun() {
		note = "🧪 Dry run — nothing was changed."
	}
	b.SendRich(chatID, formatters.RichActionResult("📈 Scaled "+name,
		[][2]string{
			{"Kind", kind},
			{"Namespace", nsDisplay(ns)},
			{"Replicas", strconv.FormatInt(replicas, 10)},
		}, note),
		fmt.Sprintf("📈 Scaled %s to %d", formatters.EscapeHTML(name), replicas))

	b.showResourceDetail(ctx, chatID, messageID, kind, ns, name, session)
}

func (b *Bot) promptCustomScale(chatID int64, kind, ns, name string) {
	singular := strings.TrimSuffix(kind, "s")
	b.SendMessage(chatID, fmt.Sprintf(
		"✏️ Send the replica count as a command:\n\n<code>/scale %s %s &lt;replicas&gt; -n %s</code>",
		formatters.EscapeHTML(singular), formatters.EscapeHTML(name),
		formatters.EscapeHTML(nsDisplay(ns))))
}

func (b *Bot) showLogOptions(ctx context.Context, chatID int64, messageID int, ns, name string) {
	container := ""
	if pod, err := b.k8sClient.GetPod(ctx, ns, name); err == nil {
		if names := podContainers(pod); len(names) > 0 {
			container = names[0]
		}
	}
	kb := b.menuBuilder.GetLogOptionsKeyboard(ns, name, container)
	label := "first container"
	if container != "" {
		label = "<code>" + formatters.EscapeHTML(container) + "</code>"
	}
	b.editView(ctx, chatID, messageID, fmt.Sprintf(
		"📋 <b>Logs</b>\n\nPod <code>%s</code> in <code>%s</code>\nContainer: %s\n\nPick how much to fetch.",
		formatters.EscapeHTML(name), formatters.EscapeHTML(nsDisplay(ns)), label), &kb)
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

// showPodLogs fetches a pod's logs with an optional container and line count.
func (b *Bot) showPodLogs(ctx context.Context, chatID int64, ns, name, container, tailArg string) {
	opts := k8s.PodLogOptions{Namespace: ns, PodName: name, Container: container}
	if tailArg != "" {
		if n, err := strconv.ParseInt(tailArg, 10, 64); err == nil && n > 0 {
			opts.TailLines = types.Int64Ptr(n)
		}
	}
	logs, err := b.fetchLogs(ctx, opts)
	if err != nil {
		b.reportError(chatID, "read logs", err)
		return
	}
	label := ns + "/" + name
	if container != "" {
		label += " · " + container
	}
	b.SendRich(chatID, formatters.RichLogs(label, "", logs),
		fmt.Sprintf("📋 Logs for %s", formatters.EscapeHTML(label)))
}

// showFollowLogs fetches a large recent slice instead of streaming.
//
// A true follow would need a long-lived goroutine per user pushing edits into
// the chat, and Telegram's rate limits make that a poor fit. Saying so is
// better than a "Follow" button that quietly behaves like a one-shot fetch.
func (b *Bot) showFollowLogs(ctx context.Context, chatID int64, ns, name, container string) {
	logs, err := b.fetchLogs(ctx, k8s.PodLogOptions{
		Namespace: ns, PodName: name, Container: container,
		TailLines: types.Int64Ptr(200),
	})
	if err != nil {
		b.reportError(chatID, "read logs", err)
		return
	}
	rich := formatters.RichLogs(ns+"/"+name, container, logs) +
		"\n\n> ℹ️ Live streaming is not available in chat. This is the last 200 lines — tap again for a fresh snapshot."
	b.SendRich(chatID, rich, fmt.Sprintf("📋 Last 200 lines of %s", formatters.EscapeHTML(name)))
}

func (b *Bot) showPreviousLogs(ctx context.Context, chatID int64, ns, name, container string) {
	logs, err := b.fetchLogs(ctx, k8s.PodLogOptions{
		Namespace: ns, PodName: name, Container: container,
		Previous: true, TailLines: types.Int64Ptr(200),
	})
	if err != nil {
		// The common case is a container that has never restarted, which the
		// API server reports as a somewhat opaque error.
		b.SendMessage(chatID, fmt.Sprintf(
			"⏮️ No previous logs for <code>%s</code>.\n\nThis usually means the container has not restarted, so there is no earlier instance to read.\n\n<i>%s</i>",
			formatters.EscapeHTML(name), formatters.EscapeHTML(err.Error())))
		return
	}
	b.SendRich(chatID,
		formatters.RichLogs(ns+"/"+name+" (previous)", container, logs),
		fmt.Sprintf("⏮️ Previous logs for %s", formatters.EscapeHTML(name)))
}

func (b *Bot) fetchLogs(ctx context.Context, opts k8s.PodLogOptions) (string, error) {
	reader, err := b.k8sClient.GetPodLogs(ctx, opts)
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
	return formatters.FormatPodLogs(string(raw), 200), nil
}

func (b *Bot) showNamespaceSummary(ctx context.Context, chatID int64, namespace string) {
	if namespace == "" {
		b.SendMessage(chatID, "❌ No namespace selected.")
		return
	}
	summary := b.k8sClient.SummariseNamespace(ctx, namespace)
	b.SendRich(chatID, formatters.RichNamespaceSummary(summary),
		fmt.Sprintf("📋 Contents of %s", formatters.EscapeHTML(namespace)))
}
