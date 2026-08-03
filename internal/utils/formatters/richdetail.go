package formatters

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/tg"
)

// Renderers for the per-resource detail pane and the node/workload verbs it
// exposes. They all return Rich Markdown; every caller pairs them with a plain
// text fallback so a server that rejects rich content still shows something.

// RichResourceSummary renders the compact header shown above a resource's
// action buttons: what it is, whether it is healthy, and how old it is.
//
// Deliberately short — the full object is one tap away behind Describe, and a
// detail pane that fills the screen buries the buttons underneath it.
func RichResourceSummary(r *k8s.ResourceInfo) string {
	d := tg.NewRichDoc()
	if r == nil {
		d.Paragraph("📭 Resource not found")
		return d.String()
	}

	d.Heading(3, fmt.Sprintf("%s %s", StatusEmoji(r.Status), r.Name))

	pairs := [][2]string{{"Kind", emptyToDash(r.Kind)}}
	if r.Namespace != "" {
		pairs = append(pairs, [2]string{"Namespace", r.Namespace})
	}
	pairs = append(pairs, [2]string{"Status", emptyToDash(r.Status)})
	if !r.CreatedAt.IsZero() {
		pairs = append(pairs, [2]string{"Age", formatAge(r.CreatedAt.Time)})
	}

	// A few per-kind facts that answer the first question an operator asks.
	switch r.Kind {
	case "Pod":
		pairs = append(pairs,
			[2]string{"Ready", emptyToDash(podReady(r))},
			[2]string{"Restarts", strconv.Itoa(podRestarts(r))},
			[2]string{"Node", emptyToDash(podNode(r))},
			[2]string{"IP", emptyToDash(podIP(r))},
		)
	case "Deployment":
		pairs = append(pairs,
			[2]string{"Ready", emptyToDash(deployReady(r))},
			[2]string{"Up-to-date", emptyToDash(deployUpToDate(r))},
			[2]string{"Available", emptyToDash(deployAvailable(r))},
		)
	case "ReplicaSet":
		pairs = append(pairs,
			[2]string{"Desired", emptyToDash(rsIntField(r, "replicas"))},
			[2]string{"Ready", emptyToDash(rsIntField(r, "readyReplicas"))},
		)
	case "Service":
		pairs = append(pairs,
			[2]string{"Type", emptyToDash(svcType(r))},
			[2]string{"Cluster IP", emptyToDash(svcClusterIP(r))},
			[2]string{"Ports", emptyToDash(svcPorts(r))},
		)
	case "Node":
		pairs = append(pairs,
			[2]string{"Version", emptyToDash(nodeVersion(r))},
			[2]string{"Schedulable", nodeSchedulableLabel(r)},
			[2]string{"Conditions", emptyToDash(k8s.NodeConditionSummary(r))},
		)
	}

	d.KeyValue(pairs)
	d.Paragraph("Pick an action below.")
	return d.String()
}

func nodeSchedulableLabel(r *k8s.ResourceInfo) string {
	spec := getMap(r.Details, "spec")
	if spec == nil {
		return "-"
	}
	if v, ok := spec["unschedulable"].(bool); ok && v {
		return "no (cordoned)"
	}
	return "yes"
}

// RichLabels renders labels and annotations as two tables. Annotations are
// truncated: last-applied-configuration alone can exceed a Telegram message.
func RichLabels(r *k8s.ResourceInfo) string {
	d := tg.NewRichDoc()
	if r == nil {
		d.Paragraph("📭 Resource not found")
		return d.String()
	}

	d.Heading(3, "🏷️ "+r.Name)

	if len(r.Labels) == 0 {
		d.Paragraph("No labels.")
	} else {
		d.Raw(mapTable("Label", r.Labels))
	}

	if len(r.Annotations) > 0 {
		d.Details(fmt.Sprintf("📝 Annotations (%d)", len(r.Annotations)),
			mapTable("Annotation", r.Annotations))
	}
	return d.String()
}

// RichSelector renders a workload's pod selector plus the pods it currently
// matches, which is how you tell "selector is wrong" from "pods are missing".
func RichSelector(r *k8s.ResourceInfo, selector string, pods []k8s.ResourceInfo) string {
	d := tg.NewRichDoc()
	if r == nil {
		d.Paragraph("📭 Resource not found")
		return d.String()
	}

	d.Heading(3, "🎯 Selector — "+r.Name)
	if selector == "" {
		d.Paragraph("This resource has no pod selector.")
		return d.String()
	}
	d.Code("", selector)

	if len(pods) == 0 {
		d.Paragraph("⚠️ The selector currently matches no pods.")
		return d.String()
	}
	d.Paragraph(fmt.Sprintf("Matches %d pod(s):", len(pods)))
	d.Raw(RichResourceList(pods, false))
	return d.String()
}

// RichEndpoints renders a service's backing addresses, keeping ready and
// not-ready separate. A service with only not-ready addresses looks healthy in
// a list view but serves no traffic.
func RichEndpoints(serviceName string, ready, notReady []string) string {
	d := tg.NewRichDoc()
	d.Heading(3, "🔌 Endpoints — "+serviceName)

	if len(ready) == 0 && len(notReady) == 0 {
		d.Paragraph("⚠️ No endpoints. Nothing is backing this service — check the selector and pod readiness.")
		return d.String()
	}

	if len(ready) > 0 {
		d.Paragraph(fmt.Sprintf("✅ Ready (%d)", len(ready)))
		d.List(ready)
	} else {
		d.Paragraph("⚠️ No ready endpoints.")
	}
	if len(notReady) > 0 {
		d.Paragraph(fmt.Sprintf("🔴 Not ready (%d)", len(notReady)))
		d.List(notReady)
	}
	return d.String()
}

// RichRolloutHistory renders a deployment's revisions, newest first.
func RichRolloutHistory(name string, revisions []k8s.Revision) string {
	d := tg.NewRichDoc()
	d.Heading(3, "📜 Rollout history — "+name)

	if len(revisions) == 0 {
		d.Paragraph("No revisions found. The deployment may have been created without any rollout yet.")
		return d.String()
	}

	rows := make([][]string, 0, len(revisions))
	for _, rev := range revisions {
		marker := ""
		if rev.Current {
			marker = "✅"
		}
		rows = append(rows, []string{
			marker,
			strconv.FormatInt(rev.Number, 10),
			strconv.FormatInt(rev.Replicas, 10),
			strconv.FormatInt(rev.Ready, 10),
			truncate(rev.Image, 60),
			ageOrDash(rev.CreationTime.Time),
		})
	}
	d.Table([]string{"", "REV", "DESIRED", "READY", "IMAGE", "AGE"}, rows,
		tg.TableOpts{Align: []string{"center", "right", "right", "right", "left", "right"}})

	// Change causes are only present when recorded, so they get their own
	// section rather than an almost-always-empty column.
	var causes []string
	for _, rev := range revisions {
		if rev.ChangeCause != "" {
			causes = append(causes, fmt.Sprintf("rev %d: %s", rev.Number, rev.ChangeCause))
		}
	}
	if len(causes) > 0 {
		d.Details("Change causes", listDoc(causes))
	}
	return d.String()
}

// RichDrainResult reports what a drain actually did. Every category is shown:
// a drain that silently skipped half the node would be worse than no drain.
func RichDrainResult(node string, res *k8s.DrainResult) string {
	d := tg.NewRichDoc()
	d.Heading(3, "💤 Drain — "+node)

	if res == nil {
		d.Paragraph("No result returned.")
		return d.String()
	}

	pairs := [][2]string{
		{"Cordoned", boolLabel(res.Cordoned)},
		{"Evicted", strconv.Itoa(len(res.Evicted))},
		{"Skipped", strconv.Itoa(len(res.Skipped))},
		{"Failed", strconv.Itoa(len(res.Failed))},
	}
	d.KeyValue(pairs)

	if len(res.Evicted) > 0 {
		d.Details(fmt.Sprintf("✅ Evicted (%d)", len(res.Evicted)), listDoc(res.Evicted))
	}
	if len(res.Skipped) > 0 {
		rows := make([][]string, 0, len(res.Skipped))
		for _, s := range res.Skipped {
			rows = append(rows, []string{s.Pod, s.Reason})
		}
		inner := tg.NewRichDoc()
		inner.Table([]string{"POD", "REASON"}, rows, tg.TableOpts{})
		d.Details(fmt.Sprintf("⏭️ Skipped (%d)", len(res.Skipped)), inner.String())
	}
	if len(res.Failed) > 0 {
		rows := make([][]string, 0, len(res.Failed))
		for _, f := range res.Failed {
			rows = append(rows, []string{f.Pod, truncate(f.Err, 160)})
		}
		inner := tg.NewRichDoc()
		inner.Table([]string{"POD", "ERROR"}, rows, tg.TableOpts{})
		d.Details(fmt.Sprintf("🔴 Failed (%d)", len(res.Failed)), inner.String())
		d.Paragraph("Failures are usually a PodDisruptionBudget refusing the " +
			"eviction. The node stays cordoned, so retrying is safe.")
	}

	d.Paragraph("Evictions are asynchronous — the pods above have been asked to terminate, not confirmed gone.")
	return d.String()
}

// RichNamespaceSummary renders per-kind object counts for a namespace.
func RichNamespaceSummary(s *k8s.NamespaceSummary) string {
	d := tg.NewRichDoc()
	if s == nil {
		d.Paragraph("📭 No summary available")
		return d.String()
	}

	d.Heading(3, "📋 Contents of "+s.Namespace)

	rows := make([][]string, 0, len(s.Counts))
	var total int
	var denied []string
	for _, c := range s.Counts {
		if c.Err != "" {
			denied = append(denied, c.Kind)
			rows = append(rows, []string{c.Kind, "—"})
			continue
		}
		total += c.Count
		rows = append(rows, []string{c.Kind, strconv.Itoa(c.Count)})
	}
	d.Table([]string{"KIND", "COUNT"}, rows, tg.TableOpts{Align: []string{"left", "right"}})
	d.Paragraph(fmt.Sprintf("Total objects counted: %d", total))

	// "—" would otherwise be indistinguishable from a genuine zero.
	if len(denied) > 0 {
		d.Paragraph("— means the count could not be read (usually RBAC): " + strings.Join(denied, ", "))
	}
	return d.String()
}

// RichManifest renders a resource's live manifest as YAML.
func RichManifest(r *k8s.ResourceInfo, yamlText string) string {
	d := tg.NewRichDoc()
	name := "resource"
	if r != nil {
		name = r.Name
	}
	d.Heading(3, "📄 Manifest — "+name)
	if yamlText == "" {
		d.Paragraph("(empty)")
		return d.String()
	}
	d.Code("yaml", yamlText)
	return d.String()
}

// RichActionResult renders the outcome of a mutating verb.
func RichActionResult(heading string, pairs [][2]string, note string) string {
	d := tg.NewRichDoc()
	d.Heading(3, heading)
	if len(pairs) > 0 {
		d.KeyValue(pairs)
	}
	if note != "" {
		d.Paragraph(note)
	}
	return d.String()
}

// RichHelpForResource explains what each button on a detail pane does, for the
// kind currently open.
func RichHelpForResource(kind string) string {
	d := tg.NewRichDoc()
	d.Heading(3, "❓ Actions for "+kind)

	common := [][]string{
		{"📝 Describe", "Full object: spec, status, labels, annotations."},
		{"🏷️ Labels", "Labels and annotations on their own."},
		{"📅 Events", "Recent events naming this object — the first place to look when something is wrong."},
		{"🗑️ Delete", "Asks for confirmation, then deletes."},
	}
	perKind := map[string][][]string{
		"pods": {
			{"📋 Logs", "Choose a tail size, follow, or the previous container's logs."},
			{"🖥️ Exec", "Run a command in the pod."},
			{"🔌 Forward", "Port-forward to the pod."},
			{"🖥️ Node", "Jump to the node this pod runs on."},
		},
		"deployments": {
			{"🔄 Restart", "Rolling restart via a template annotation — no downtime for a healthy Deployment."},
			{"📈 Scale", "Set the replica count."},
			{"📋 Pods", "Pods this deployment currently owns."},
			{"🎯 Selector", "The pod selector, and what it matches right now."},
			{"📜 History", "Revisions, reconstructed from the owned ReplicaSets."},
			{"📄 YAML", "The live manifest."},
		},
		"replicasets": {
			{"📋 Pods", "Pods this ReplicaSet owns."},
			{"📈 Scale", "Set the replica count. If a Deployment owns this ReplicaSet, its controller will undo the change."},
			{"🎯 Selector", "The pod selector, and what it matches right now."},
		},
		"services": {
			{"📋 Endpoints", "Backing addresses, split into ready and not-ready."},
			{"🎯 Selector", "The pod selector, and what it matches right now."},
			{"🔌 Forward", "Port-forward to the service."},
		},
		"nodes": {
			{"📊 Top", "CPU and memory usage (needs metrics-server)."},
			{"📋 Pods", "Everything scheduled here, across all namespaces."},
			{"🔧 Cordon", "Stop new pods scheduling here. Running pods stay."},
			{"🔓 Uncordon", "Allow scheduling again."},
			{"💤 Drain", "Cordon, then evict pods. DaemonSet and static pods are left alone."},
		},
		"namespaces": {
			{"📋 Resources", "Object counts per kind in this namespace."},
			{"🌐 Switch to", "Make this the namespace the menus browse."},
		},
	}

	rows := make([][]string, 0, 12)
	rows = append(rows, perKind[kind]...)
	rows = append(rows, common...)
	d.Table([]string{"BUTTON", "WHAT IT DOES"}, rows, tg.TableOpts{})
	return d.String()
}

func listDoc(items []string) string {
	inner := tg.NewRichDoc()
	inner.List(items)
	return inner.String()
}

func boolLabel(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func ageOrDash(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return formatAge(t)
}

// SortedKeys is a small helper for deterministic map rendering in callers.
func SortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
