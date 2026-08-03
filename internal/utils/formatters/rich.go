package formatters

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/tg"
)

// Rich renderers produce Rich Markdown (Bot API 10.1+) so Telegram draws real
// tables instead of the monospace code-block fallback in formatters.go. They
// deliberately reuse columnsForKind/rowForKind, so a column added for the text
// table shows up in the rich table too and the two cannot drift.

// RichResourceList renders a resource list as a native table.
func RichResourceList(resources []k8s.ResourceInfo, wide bool) string {
	d := tg.NewRichDoc()
	if len(resources) == 0 {
		d.Paragraph("📭 No resources found")
		return d.String()
	}

	kind := resources[0].Kind
	if kind == "" {
		kind = "Resource"
	}
	cols := columnsForKind(kind, wide)

	headers := make([]string, len(cols))
	align := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = c.Header
		align[i] = "left"
	}

	rows := make([][]string, 0, len(resources))
	for i := range resources {
		rows = append(rows, rowForKind(cols, &resources[i]))
	}

	d.Heading(3, fmt.Sprintf("%s — %d item(s)", pluralKind(kind, len(resources)), len(resources)))
	d.Table(headers, rows, tg.TableOpts{Align: align})
	return d.String()
}

// RichResource renders a single resource: a summary table, then labels,
// annotations and details tucked into collapsible sections so a large object
// does not flood the chat.
func RichResource(resource *k8s.ResourceInfo, wide bool) string {
	d := tg.NewRichDoc()
	if resource == nil {
		d.Paragraph("📭 Resource not found")
		return d.String()
	}

	d.Heading(3, fmt.Sprintf("%s %s", StatusEmoji(resource.Status), resource.Name))

	pairs := [][2]string{
		{"Kind", emptyToDash(resource.Kind)},
		{"Namespace", emptyToDash(resource.Namespace)},
		{"Status", emptyToDash(resource.Status)},
		{"API Version", emptyToDash(resource.APIVersion)},
	}
	if !resource.CreatedAt.IsZero() {
		pairs = append(pairs,
			[2]string{"Age", formatAge(resource.CreatedAt.Time)},
			[2]string{"Created", resource.CreatedAt.Format(time.RFC3339)},
		)
	}
	d.KeyValue(pairs)

	if len(resource.Labels) > 0 {
		d.Details(fmt.Sprintf("🏷️ Labels (%d)", len(resource.Labels)),
			mapTable("Label", resource.Labels))
	}
	if len(resource.Annotations) > 0 {
		d.Details(fmt.Sprintf("📝 Annotations (%d)", len(resource.Annotations)),
			mapTable("Annotation", resource.Annotations))
	}
	if wide && len(resource.Details) > 0 {
		keys := make([]string, 0, len(resource.Details))
		for k := range resource.Details {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		rows := make([][]string, 0, len(keys))
		for _, k := range keys {
			rows = append(rows, []string{k, truncate(fmt.Sprintf("%v", resource.Details[k]), 300)})
		}
		inner := tg.NewRichDoc()
		inner.Table([]string{"Field", "Value"}, rows, tg.TableOpts{})
		d.Details("🔍 Details", inner.String())
	}

	return d.String()
}

// mapTable renders a string map as a two-column table with sorted keys.
func mapTable(keyHeader string, m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	rows := make([][]string, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, []string{k, truncate(m[k], 200)})
	}
	d := tg.NewRichDoc()
	d.Table([]string{keyHeader, "Value"}, rows, tg.TableOpts{})
	return d.String()
}

// RichContexts renders kubeconfig contexts as a table, marking the active one.
func RichContexts(contexts []KubeContext) string {
	d := tg.NewRichDoc()
	if len(contexts) == 0 {
		d.Paragraph("📭 No contexts found in kubeconfig")
		return d.String()
	}

	d.Heading(3, fmt.Sprintf("🌐 Contexts — %d", len(contexts)))
	rows := make([][]string, 0, len(contexts))
	for _, c := range contexts {
		marker := ""
		if c.Current {
			marker = "✅"
		}
		rows = append(rows, []string{
			marker, c.Name, emptyToDash(c.Cluster), emptyToDash(c.Namespace),
		})
	}
	d.Table([]string{"", "Name", "Cluster", "Namespace"}, rows,
		tg.TableOpts{Align: []string{"center", "left", "left", "left"}})
	return d.String()
}

// KubeContext mirrors the fields of kubeconfig.ContextInfo needed for display,
// declared locally so callers need not thread the kubeconfig type through.
type KubeContext struct {
	Name      string
	Cluster   string
	Namespace string
	Current   bool
}

// NewKubeContext adapts a kubeconfig context for RichContexts.
func NewKubeContext(name, cluster, namespace string, current bool) KubeContext {
	return KubeContext{Name: name, Cluster: cluster, Namespace: namespace, Current: current}
}

// RichEvents renders events newest-first with type emojis.
func RichEvents(events []k8s.ResourceInfo) string {
	d := tg.NewRichDoc()
	if len(events) == 0 {
		d.Paragraph("📭 No events found")
		return d.String()
	}

	d.Heading(3, fmt.Sprintf("📅 Events — %d", len(events)))
	rows := make([][]string, 0, len(events))
	for i := range events {
		e := &events[i]
		rows = append(rows, []string{
			eventTypeEmoji(e) + " " + eventType(e),
			eventReason(e),
			eventObject(e),
			truncate(eventMessage(e), 120),
			formatAge(e.CreatedAt.Time),
		})
	}
	d.Table([]string{"Type", "Reason", "Object", "Message", "Age"}, rows, tg.TableOpts{})
	return d.String()
}

// RichKeyValue renders a labelled key/value table under a heading.
func RichKeyValue(heading string, pairs [][2]string) string {
	d := tg.NewRichDoc()
	if heading != "" {
		d.Heading(3, heading)
	}
	d.KeyValue(pairs)
	return d.String()
}

// RichLogs renders a log tail as a fenced code block, which keeps alignment and
// gives Telegram clients a copy button.
func RichLogs(podName, container, logs string) string {
	d := tg.NewRichDoc()
	// Avoid [] here: the heading is escaped, so brackets would render as \[..\].
	title := "📋 Logs: " + podName
	if container != "" {
		title += " · " + container
	}
	d.Heading(3, title)
	if logs == "" {
		d.Paragraph("(no output)")
		return d.String()
	}
	d.Code("log", logs)
	return d.String()
}

// RichMetrics renders metrics.k8s.io PodMetrics/NodeMetrics as a table.
// Those objects carry no status/phase, so the generic resource columns render
// them as empty rows — the usage figures live under `containers[].usage` for
// pods and `usage` for nodes.
func RichMetrics(heading string, metrics []k8s.ResourceInfo) string {
	d := tg.NewRichDoc()
	if len(metrics) == 0 {
		d.Paragraph("📭 No metrics available")
		return d.String()
	}

	d.Heading(3, fmt.Sprintf("%s — %d", heading, len(metrics)))

	isNode := metrics[0].Namespace == ""
	headers := []string{"NAME", "CPU", "MEMORY"}
	align := []string{"left", "right", "right"}
	if !isNode {
		headers = []string{"NAME", "NAMESPACE", "CPU", "MEMORY"}
		align = []string{"left", "left", "right", "right"}
	}

	rows := make([][]string, 0, len(metrics))
	for i := range metrics {
		m := &metrics[i]
		cpu, mem := metricUsage(m)
		if isNode {
			rows = append(rows, []string{m.Name, cpu, mem})
		} else {
			rows = append(rows, []string{m.Name, emptyToDash(m.Namespace), cpu, mem})
		}
	}
	d.Table(headers, rows, tg.TableOpts{Align: align})
	return d.String()
}

// metricUsage pulls cpu/memory out of a metrics object, summing across
// containers for PodMetrics.
func metricUsage(r *k8s.ResourceInfo) (cpu, mem string) {
	if r.Details == nil {
		return "-", "-"
	}
	// NodeMetrics: top-level "usage".
	if u := getMap(r.Details, "usage"); u != nil {
		return emptyToDash(getString(u, "cpu")), emptyToDash(getString(u, "memory"))
	}
	// PodMetrics: per-container "usage" under "containers".
	containers, ok := r.Details["containers"].([]interface{})
	if !ok || len(containers) == 0 {
		return "-", "-"
	}
	var cpuParts, memParts []string
	for _, c := range containers {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		u := getMap(cm, "usage")
		if u == nil {
			continue
		}
		if v := getString(u, "cpu"); v != "" {
			cpuParts = append(cpuParts, v)
		}
		if v := getString(u, "memory"); v != "" {
			memParts = append(memParts, v)
		}
	}
	if len(cpuParts) == 0 && len(memParts) == 0 {
		return "-", "-"
	}
	return emptyToDash(strings.Join(cpuParts, "+")), emptyToDash(strings.Join(memParts, "+"))
}

func pluralKind(kind string, n int) string {
	if n == 1 {
		return kind
	}
	switch kind {
	case "Ingress":
		return "Ingresses"
	case "":
		return "Resources"
	default:
		return kind + "s"
	}
}
