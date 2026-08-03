package formatters

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/pkg/kubeconfig"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TelegramMarkdownV2Escape escapes a string for Telegram MarkdownV2.
// Per https://core.telegram.org/bots/api#markdownv2-style these chars must be
// backslash-escaped: _ * [ ] ( ) ~ ` > # + - = | { } . !
// noneValue is what an absent field renders as, matching kubectl's own output
// so a column reads the same in chat as it does in a terminal.
const noneValue = "<none>"

func EscapeMDV2(s string) string {
	special := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	out := s
	for _, c := range special {
		out = strings.ReplaceAll(out, c, "\\"+c)
	}
	return out
}

// EscapeMD escapes only the minimal Markdown chars (old Markdown style).
func EscapeMD(s string) string {
	r := strings.NewReplacer("_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]", "`", "\\`")
	return r.Replace(s)
}

// EscapeHTML escapes HTML special chars (for HTML parse mode).
func EscapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "\u0026amp;")
	s = strings.ReplaceAll(s, "<", "\u0026lt;")
	s = strings.ReplaceAll(s, ">", "\u0026gt;")
	s = strings.ReplaceAll(s, "\"", "\u0026quot;")
	s = strings.ReplaceAll(s, "'", "\u0026#39;")
	return s
}

// Status glyphs. Named so the "which statuses are bad" grouping below is
// explicit rather than three separate copies of the same emoji.
const (
	emojiHealthy  = "🟢"
	emojiDone     = "⚫"
	emojiWaiting  = "🟡"
	emojiBroken   = "🔴"
	emojiStopping = "🟠"
	emojiUnknown  = "❓"
	emojiNeutral  = "•"
)

// StatusEmoji returns an emoji representing a Kubernetes resource status.
//
// An unrecognised status falls back to the neutral glyph rather than an empty
// string: a missing leading character silently shifts every column of the
// fixed-width tables.
func StatusEmoji(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return emojiHealthy
	case "succeeded", "completed":
		return emojiDone
	case "pending":
		return emojiWaiting
	case "failed", "crashloopbackoff", "error":
		return emojiBroken
	case "terminating":
		return emojiStopping
	case "unknown":
		return emojiUnknown
	default:
		return emojiNeutral
	}
}

// FormatResource formats a single resource for display.
func FormatResource(resource *k8s.ResourceInfo, format string) string {
	switch format {
	case "json":
		return formatJSON(resource)
	case "yaml":
		return formatYAML(resource)
	case "wide":
		return formatWide(resource)
	default:
		return formatDefault(resource)
	}
}

func formatDefault(resource *k8s.ResourceInfo) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Name: %s\n", resource.Name))
	sb.WriteString(fmt.Sprintf("Namespace: %s\n", resource.Namespace))
	sb.WriteString(fmt.Sprintf("Kind: %s\n", resource.Kind))
	sb.WriteString(fmt.Sprintf("API Version: %s\n", resource.APIVersion))
	sb.WriteString(fmt.Sprintf("Status: %s\n", resource.Status))
	sb.WriteString(fmt.Sprintf("Created: %s\n", resource.CreatedAt.Format(time.RFC3339)))

	if len(resource.Labels) > 0 {
		sb.WriteString("Labels:\n")
		for k, v := range resource.Labels {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	if len(resource.Annotations) > 0 {
		sb.WriteString("Annotations:\n")
		for k, v := range resource.Annotations {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return sb.String()
}

func formatWide(resource *k8s.ResourceInfo) string {
	var sb strings.Builder
	sb.WriteString(formatDefault(resource))

	if len(resource.Details) > 0 {
		sb.WriteString("Details:\n")
		for k, v := range resource.Details {
			sb.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
		}
	}

	return sb.String()
}

func formatJSON(resource *k8s.ResourceInfo) string {
	data, _ := json.MarshalIndent(resource, "", "  ")
	return string(data)
}

func formatYAML(resource *k8s.ResourceInfo) string {
	// Convert to unstructured for YAML output
	u := &unstructured.Unstructured{Object: resource.Details}
	u.SetName(resource.Name)
	u.SetNamespace(resource.Namespace)
	u.SetKind(resource.Kind)
	u.SetAPIVersion(resource.APIVersion)
	u.SetLabels(resource.Labels)
	u.SetAnnotations(resource.Annotations)
	u.SetCreationTimestamp(resource.CreatedAt)

	data, _ := yaml.Marshal(u.Object)
	return string(data)
}

// FormatResourceList formats a list of resources as a table.
// Default format returns a MarkdownV2 code block with an aligned,
// resource-aware table (the pretty k9s-style output the user wants).
func FormatResourceList(resources []k8s.ResourceInfo, format string, wide bool) string {
	if len(resources) == 0 {
		return "📭 No resources found"
	}

	switch format {
	case "json":
		return formatResourceListJSON(resources)
	case "yaml":
		return formatResourceListYAML(resources)
	case "name":
		return formatResourceListNames(resources)
	case "wide":
		wide = true
		fallthrough
	default:
		return formatResourceListTable(resources, wide)
	}
}

// formatResourceListTable renders resources as a MarkdownV2 code-block table
// with resource-specific columns and status emojis.
func formatResourceListTable(resources []k8s.ResourceInfo, wide bool) string {
	if len(resources) == 0 {
		return "📭 No resources found"
	}

	kind := resources[0].Kind
	if kind == "" {
		kind = "Resource"
	}
	cols := columnsForKind(kind, wide)

	// Build rows with display values (pre-escaping).
	headers := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = c.Header
	}
	rows := make([][]string, 0, len(resources))
	for _, r := range resources {
		rows = append(rows, rowForKind(cols, &r))
	}

	// Compute column widths.
	widths := make([]int, len(cols))
	for i, h := range headers {
		widths[i] = displayWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if w := displayWidth(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}
	// Cap very wide NAME columns so the table fits in Telegram.
	maxNameWidth := 42
	for i, c := range cols {
		if c.Header == "NAME" && widths[i] > maxNameWidth {
			widths[i] = maxNameWidth
		}
	}

	var sb strings.Builder
	// Title line (MarkdownV2 — but this goes inside a code block so no escaping).
	sb.WriteString(fmt.Sprintf("%s %d item(s)\n", kind, len(resources)))

	// Header row
	sb.WriteString(padRight(headers[0], widths[0]))
	for j := 1; j < len(headers); j++ {
		sb.WriteString("  ")
		sb.WriteString(padRight(headers[j], widths[j]))
	}
	sb.WriteString("\n")

	// Separator row
	sb.WriteString(strings.Repeat("─", widths[0]))
	for j := 1; j < len(headers); j++ {
		sb.WriteString("  ")
		sb.WriteString(strings.Repeat("─", widths[j]))
	}
	sb.WriteString("\n")

	// Data rows
	for _, row := range rows {
		// Truncate NAME cell if too wide.
		if len(row[0]) > maxNameWidth && widths[0] == maxNameWidth {
			row[0] = row[0][:maxNameWidth-1] + "…"
		}
		sb.WriteString(padRight(row[0], widths[0]))
		for j := 1; j < len(row); j++ {
			sb.WriteString("  ")
			sb.WriteString(padRight(row[j], widths[j]))
		}
		sb.WriteString("\n")
	}

	// Wrap in a MarkdownV2 code block. Inside ``` no escaping is needed,
	// which is why we kept the raw text above.
	return "```\n" + sb.String() + "```"
}

// tableColumn describes a single table column.
type tableColumn struct {
	Header string
	Render func(r *k8s.ResourceInfo) string
}

// Column builders shared across kinds. NAME/NAMESPACE/AGE/STATUS appear in
// almost every table, and previously each kind re-declared its own closure for
// them — thirteen copies that had to be kept in step by hand.
var (
	colName      = tableColumn{"NAME", func(r *k8s.ResourceInfo) string { return r.Name }}
	colNamespace = tableColumn{"NAMESPACE", func(r *k8s.ResourceInfo) string { return r.Namespace }}
	colAge       = tableColumn{"AGE", func(r *k8s.ResourceInfo) string { return formatAge(r.CreatedAt.Time) }}
	colStatus    = tableColumn{"STATUS", func(r *k8s.ResourceInfo) string {
		return StatusEmoji(r.Status) + " " + emptyToDash(r.Status)
	}}
)

// kindColumns maps a Kubernetes kind to the columns its table shows, in order.
// A kind with no entry falls back to genericColumns.
var kindColumns = map[string][]tableColumn{
	"Pod": {
		colName, colNamespace,
		{"READY", podReady},
		colStatus,
		{"RESTARTS", func(r *k8s.ResourceInfo) string { return strconv.Itoa(podRestarts(r)) }},
		colAge,
	},
	"Deployment": {
		colName, colNamespace,
		{"READY", deployReady},
		{"UP-TO-DATE", deployUpToDate},
		{"AVAILABLE", deployAvailable},
		colAge,
	},
	"Service": {
		colName, colNamespace,
		{"TYPE", svcType},
		{"CLUSTER-IP", svcClusterIP},
		{"EXTERNAL-IP", svcExternalIP},
		{"PORT(S)", svcPorts},
		colAge,
	},
	"ReplicaSet": {
		colName, colNamespace,
		{"DESIRED", func(r *k8s.ResourceInfo) string { return rsIntField(r, "replicas") }},
		{"CURRENT", func(r *k8s.ResourceInfo) string { return rsIntField(r, "availableReplicas") }},
		{"READY", func(r *k8s.ResourceInfo) string { return rsIntField(r, "readyReplicas") }},
		colAge,
	},
	"Namespace": {colName, colStatus, colAge},
	"Node": {
		colName, colStatus,
		{"VERSION", nodeVersion},
		colAge,
	},
	"ConfigMap": {
		colName, colNamespace,
		{"DATA", cmDataCount},
		colAge,
	},
	"Secret": {
		colName, colNamespace,
		{"TYPE", secretType},
		{"DATA", cmDataCount},
		colAge,
	},
	"PersistentVolumeClaim": {
		colName, colNamespace, colStatus,
		{"CAPACITY", pvcCapacity},
		colAge,
	},
	"Ingress": {
		colName, colNamespace,
		{"HOSTS", ingressHosts},
		colAge,
	},
	"Event": {
		{"TYPE", func(r *k8s.ResourceInfo) string { return eventTypeEmoji(r) + " " + eventType(r) }},
		{"REASON", eventReason},
		{"OBJECT", eventObject},
		{"MESSAGE", func(r *k8s.ResourceInfo) string { return TruncateString(eventMessage(r), 60) }},
	},
}

// wideColumns are appended in -o wide, per kind.
var wideColumns = map[string][]tableColumn{
	"Pod": {
		{"NODE", podNode},
		{"IP", podIP},
		{"LABELS", func(r *k8s.ResourceInfo) string { return formatLabels(r.Labels) }},
	},
}

// genericColumns is the fallback for kinds with no specific table.
var genericColumns = []tableColumn{
	colName,
	{"NAMESPACE", func(r *k8s.ResourceInfo) string { return emptyToDash(r.Namespace) }},
	colStatus,
	colAge,
}

// columnsForKind returns the column set for a resource kind, plus the extra
// columns for wide output.
//
// The returned slice is always a fresh copy: callers append to it, and handing
// back the package-level slice would let one call mutate the table for every
// later one.
func columnsForKind(kind string, wide bool) []tableColumn {
	base, ok := kindColumns[kind]
	if !ok {
		base = genericColumns
	}
	extra := wideColumns[kind]
	if !wide {
		extra = nil
	}

	out := make([]tableColumn, 0, len(base)+len(extra))
	out = append(out, base...)
	out = append(out, extra...)
	return out
}

// rowForKind renders a row for the given column set. The kind is already
// baked into cols by columnsForKind, so it is not needed again here.
func rowForKind(cols []tableColumn, r *k8s.ResourceInfo) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Render(r)
	}
	return out
}

// displayWidth returns the number of visible columns of s, treating a UTF-8
// rune (including emoji and CJK) as width 2. This is a conservative estimate
// that works well enough for fixed-width code blocks in Telegram.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case r >= 0x1F300 && r <= 0x1FAFF: // emoji symbols
			w += 2
		case r >= 0x1100 && r <= 0x115F: // Hangul Jamo
			w += 2
		case r >= 0x2E80 && r <= 0xA4CF: // CJK radicals
			w += 2
		case r >= 0xAC00 && r <= 0xD7A3: // Hangul syllables
			w += 2
		case r >= 0xF900 && r <= 0xFAFF: // CJK compat ideographs
			w += 2
		case r >= 0xFE30 && r <= 0xFE4F: // CJK compat forms
			w += 2
		case r >= 0xFF00 && r <= 0xFF60: // fullwidth forms
			w += 2
		case r >= 0xFFE0 && r <= 0xFFE6: // fullwidth signs
			w += 2
		default:
			w += 1
		}
	}
	return w
}

// padRight pads s with spaces to reach width columns (respects displayWidth).
func padRight(s string, width int) string {
	cur := displayWidth(s)
	if cur >= width {
		return s
	}
	return s + strings.Repeat(" ", width-cur)
}

// ---- Helpers to dig into Details (unstructured maps) ----

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	v, _ := m[key].(map[string]interface{})
	return v
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(int64); ok {
		return v
	}
	if f, ok := m[key].(float64); ok {
		return int64(f)
	}
	return 0
}

// ---- Pod helpers ----

func podReady(r *k8s.ResourceInfo) string {
	status := getMap(r.Details, "status")
	if status == nil {
		return "0/0"
	}
	containers, _ := status["containerStatuses"].([]interface{})
	total := len(containers)
	if total == 0 {
		// fall back to spec.containers
		spec := getMap(r.Details, "spec")
		if cs, ok := spec["containers"].([]interface{}); ok {
			total = len(cs)
		}
	}
	ready := 0
	for _, c := range containers {
		if cs, ok := c.(map[string]interface{}); ok {
			if readyBool(cs["ready"]) {
				ready++
			}
		}
	}
	return fmt.Sprintf("%d/%d", ready, total)
}

func readyBool(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func podRestarts(r *k8s.ResourceInfo) int {
	status := getMap(r.Details, "status")
	if status == nil {
		return 0
	}
	containers, _ := status["containerStatuses"].([]interface{})
	restarts := 0
	for _, c := range containers {
		if cs, ok := c.(map[string]interface{}); ok {
			restarts += int(getInt64(cs, "restartCount"))
		}
	}
	return restarts
}

func podNode(r *k8s.ResourceInfo) string {
	spec := getMap(r.Details, "spec")
	if spec == nil {
		return noneValue
	}
	if n := getString(spec, "nodeName"); n != "" {
		return n
	}
	return noneValue
}

func podIP(r *k8s.ResourceInfo) string {
	status := getMap(r.Details, "status")
	if status == nil {
		return noneValue
	}
	if ip := getString(status, "podIP"); ip != "" {
		return ip
	}
	return noneValue
}

// ---- Deployment helpers ----

func deployReady(r *k8s.ResourceInfo) string {
	spec := getMap(r.Details, "spec")
	status := getMap(r.Details, "status")
	desired := getInt64(spec, "replicas")
	ready := getInt64(status, "readyReplicas")
	return fmt.Sprintf("%d/%d", ready, desired)
}

func deployUpToDate(r *k8s.ResourceInfo) string {
	status := getMap(r.Details, "status")
	return strconv.FormatInt(getInt64(status, "updatedReplicas"), 10)
}

func deployAvailable(r *k8s.ResourceInfo) string {
	status := getMap(r.Details, "status")
	return strconv.FormatInt(getInt64(status, "availableReplicas"), 10)
}

// ---- Service helpers ----

func svcType(r *k8s.ResourceInfo) string {
	spec := getMap(r.Details, "spec")
	return emptyToDash(getString(spec, "type"))
}

func svcClusterIP(r *k8s.ResourceInfo) string {
	spec := getMap(r.Details, "spec")
	if ip := getString(spec, "clusterIP"); ip != "" {
		return ip
	}
	return noneValue
}

func svcExternalIP(r *k8s.ResourceInfo) string {
	spec := getMap(r.Details, "spec")
	if ips, ok := spec["externalIPs"].([]interface{}); ok && len(ips) > 0 {
		var out []string
		for _, ip := range ips {
			if s, ok := ip.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return strings.Join(out, ",")
		}
	}
	if lb := getMap(statusSvc(r), "loadBalancer"); lb != nil {
		if ing, ok := lb["ingress"].([]interface{}); ok && len(ing) > 0 {
			if first, ok := ing[0].(map[string]interface{}); ok {
				if ip := getString(first, "ip"); ip != "" {
					return ip
				}
				if h := getString(first, "hostname"); h != "" {
					return h
				}
			}
		}
	}
	return noneValue
}

func statusSvc(r *k8s.ResourceInfo) map[string]interface{} {
	return getMap(r.Details, "status")
}

func svcPorts(r *k8s.ResourceInfo) string {
	spec := getMap(r.Details, "spec")
	ports, _ := spec["ports"].([]interface{})
	if len(ports) == 0 {
		return noneValue
	}
	var out []string
	for _, p := range ports {
		if pm, ok := p.(map[string]interface{}); ok {
			port := getInt64(pm, "port")
			proto := getString(pm, "protocol")
			if proto == "" {
				proto = "TCP"
			}
			name := getString(pm, "name")
			np := ""
			if t := getInt64(pm, "nodePort"); t > 0 {
				np = fmt.Sprintf(":%d", t)
			}
			switch {
			case name != "":
				out = append(out, fmt.Sprintf("%d/%s%s(%s)", port, proto, np, name))
			case np != "":
				out = append(out, fmt.Sprintf("%d/%s%s", port, proto, np))
			default:
				out = append(out, fmt.Sprintf("%d/%s", port, proto))
			}
		}
	}
	return strings.Join(out, ",")
}

// ---- ReplicaSet helpers ----

func rsIntField(r *k8s.ResourceInfo, field string) string {
	src := getMap(r.Details, "status")
	if src == nil {
		src = getMap(r.Details, "spec")
	}
	return strconv.FormatInt(getInt64(src, field), 10)
}

// ---- Node helpers ----

func nodeVersion(r *k8s.ResourceInfo) string {
	status := getMap(r.Details, "status")
	info := getMap(status, "nodeInfo")
	return emptyToDash(getString(info, "kubeletVersion"))
}

// ---- ConfigMap / Secret helpers ----

func cmDataCount(r *k8s.ResourceInfo) string {
	if data, ok := r.Details["data"].(map[string]interface{}); ok {
		return strconv.Itoa(len(data))
	}
	if binData, ok := r.Details["binaryData"].(map[string]interface{}); ok {
		return strconv.Itoa(len(binData))
	}
	return "0"
}

func secretType(r *k8s.ResourceInfo) string {
	if t := getString(r.Details, "type"); t != "" {
		return t
	}
	return noneValue
}

// ---- PVC helpers ----

func pvcCapacity(r *k8s.ResourceInfo) string {
	spec := getMap(r.Details, "spec")
	resources := getMap(spec, "resources")
	requests := getMap(resources, "requests")
	storage := getMap(requests, "storage")
	if storage != nil {
		// Usually {"storage": "10Gi"} but client-go may return a map
		if q, ok := storage["storage"].(string); ok {
			return q
		}
	}
	// Fall back to status.capacity
	status := getMap(r.Details, "status")
	cap := getMap(status, "capacity")
	if cap != nil {
		if q, ok := cap["storage"].(string); ok {
			return q
		}
	}
	return noneValue
}

// ---- Ingress helpers ----

func ingressHosts(r *k8s.ResourceInfo) string {
	spec := getMap(r.Details, "spec")
	rules, _ := spec["rules"].([]interface{})
	if len(rules) == 0 {
		// try TLS hosts
		tls, _ := spec["tls"].([]interface{})
		if len(tls) > 0 {
			if first, ok := tls[0].(map[string]interface{}); ok {
				hosts, _ := first["hosts"].([]interface{})
				var out []string
				for _, h := range hosts {
					if s, ok := h.(string); ok {
						out = append(out, s)
					}
				}
				if len(out) > 0 {
					return strings.Join(out, ",")
				}
			}
		}
		return "*"
	}
	var out []string
	for _, rule := range rules {
		if rm, ok := rule.(map[string]interface{}); ok {
			if h := getString(rm, "host"); h != "" {
				out = append(out, h)
			}
		}
	}
	if len(out) == 0 {
		return "*"
	}
	return strings.Join(out, ",")
}

// ---- Event helpers ----

func eventType(r *k8s.ResourceInfo) string {
	return emptyToDash(getString(r.Details, "type"))
}

func eventTypeEmoji(r *k8s.ResourceInfo) string {
	t := getString(r.Details, "type")
	if t == "Warning" {
		return "🟠"
	}
	return "🟢"
}

func eventReason(r *k8s.ResourceInfo) string {
	return emptyToDash(getString(r.Details, "reason"))
}

func eventObject(r *k8s.ResourceInfo) string {
	inv := getMap(r.Details, "involvedObject")
	if inv == nil {
		return noneValue
	}
	kind := getString(inv, "kind")
	name := getString(inv, "name")
	ns := getString(inv, "namespace")
	if ns != "" {
		return fmt.Sprintf("%s/%s/%s", ns, kind, name)
	}
	return fmt.Sprintf("%s/%s", kind, name)
}

func eventMessage(r *k8s.ResourceInfo) string {
	return emptyToDash(getString(r.Details, "message"))
}

// ---- Generic helpers ----

func emptyToDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return noneValue
	}
	return s
}

func formatAge(t time.Time) string {
	duration := time.Since(t)
	switch hours := duration.Hours(); {
	case hours >= 24*365:
		return fmt.Sprintf("%dy", int(hours/24/365))
	case hours >= 24*30:
		return fmt.Sprintf("%dmo", int(hours/24/30))
	case hours >= 24:
		return fmt.Sprintf("%dd", int(hours/24))
	case hours >= 1:
		return fmt.Sprintf("%dh", int(hours))
	case duration.Minutes() >= 1:
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	return fmt.Sprintf("%ds", int(duration.Seconds()))
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return noneValue
	}
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func formatResourceListJSON(resources []k8s.ResourceInfo) string {
	data, _ := json.MarshalIndent(resources, "", "  ")
	return string(data)
}

func formatResourceListYAML(resources []k8s.ResourceInfo) string {
	var sb strings.Builder
	for i, r := range resources {
		if i > 0 {
			sb.WriteString("---\n")
		}
		u := &unstructured.Unstructured{Object: r.Details}
		u.SetName(r.Name)
		u.SetNamespace(r.Namespace)
		u.SetKind(r.Kind)
		u.SetAPIVersion(r.APIVersion)
		u.SetLabels(r.Labels)
		u.SetAnnotations(r.Annotations)
		u.SetCreationTimestamp(r.CreatedAt)
		data, _ := yaml.Marshal(u.Object)
		sb.WriteString(string(data))
	}
	return sb.String()
}

func formatResourceListNames(resources []k8s.ResourceInfo) string {
	var names []string
	for _, r := range resources {
		if r.Namespace != "" {
			names = append(names, fmt.Sprintf("%s/%s", r.Namespace, r.Name))
		} else {
			names = append(names, r.Name)
		}
	}
	return strings.Join(names, "\n")
}

// TruncateString truncates a string to maxLen, adding "..." if truncated.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// FormatPodLogs formats pod logs for display.
func FormatPodLogs(logs string, maxLines int) string {
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}

// FormatExecOutput formats exec command output.
func FormatExecOutput(stdout, stderr string) string {
	var sb strings.Builder
	if stdout != "" {
		sb.WriteString("STDOUT:\n")
		sb.WriteString(stdout)
		sb.WriteString("\n")
	}
	if stderr != "" {
		sb.WriteString("STDERR:\n")
		sb.WriteString(stderr)
		sb.WriteString("\n")
	}
	return sb.String()
}

// FormatContexts formats kubeconfig contexts for display.
func FormatContexts(contexts []kubeconfig.ContextInfo) string {
	if len(contexts) == 0 {
		return "No contexts found"
	}

	var sb strings.Builder
	sb.WriteString("Available Contexts:\n")
	sb.WriteString("==================\n\n")

	for _, ctx := range contexts {
		current := ""
		if ctx.Current {
			current = " * (current)"
		}
		sb.WriteString(fmt.Sprintf("%s%s\n", ctx.Name, current))
		sb.WriteString(fmt.Sprintf("  Cluster: %s (%s)\n", ctx.Cluster, ctx.ClusterServer))
		sb.WriteString(fmt.Sprintf("  User: %s\n", ctx.User))
		sb.WriteString(fmt.Sprintf("  Namespace: %s\n", ctx.Namespace))
		sb.WriteString("\n")
	}

	return sb.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
