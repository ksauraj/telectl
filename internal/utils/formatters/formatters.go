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

// StatusEmoji returns an emoji representing a Kubernetes resource status.
func StatusEmoji(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return "🟢"
	case "succeeded", "completed":
		return "⚫"
	case "pending":
		return "🟡"
	case "failed":
		return "🔴"
	case "crashloopbackoff":
		return "🔴"
	case "terminating":
		return "🟠"
	case "error":
		return "🔴"
	case "unknown":
		return "❓"
	case "":
		return "•"
	}
	return "•"
}

// FormatResource formats a single resource for display
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
		rows = append(rows, rowForKind(cols, &r, kind))
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

// columnsForKind returns the column set for a given resource kind.
func columnsForKind(kind string, wide bool) []tableColumn {
	switch kind {
	case "Pod":
		base := []tableColumn{
			{"NAME", func(r *k8s.ResourceInfo) string { return r.Name }},
			{"NAMESPACE", func(r *k8s.ResourceInfo) string { return r.Namespace }},
			{"READY", func(r *k8s.ResourceInfo) string { return podReady(r) }},
			{"STATUS", func(r *k8s.ResourceInfo) string { return StatusEmoji(r.Status) + " " + emptyToDash(r.Status) }},
			{"RESTARTS", func(r *k8s.ResourceInfo) string { return strconv.Itoa(podRestarts(r)) }},
			{"AGE", func(r *k8s.ResourceInfo) string { return formatAge(r.CreatedAt.Time) }},
		}
		if wide {
			base = append(base,
				tableColumn{"NODE", func(r *k8s.ResourceInfo) string { return podNode(r) }},
				tableColumn{"IP", func(r *k8s.ResourceInfo) string { return podIP(r) }},
				tableColumn{"LABELS", func(r *k8s.ResourceInfo) string { return formatLabels(r.Labels) }},
			)
		}
		return base
	case "Deployment":
		base := []tableColumn{
			{"NAME", func(r *k8s.ResourceInfo) string { return r.Name }},
			{"NAMESPACE", func(r *k8s.ResourceInfo) string { return r.Namespace }},
			{"READY", func(r *k8s.ResourceInfo) string { return deployReady(r) }},
			{"UP-TO-DATE", func(r *k8s.ResourceInfo) string { return deployUpToDate(r) }},
			{"AVAILABLE", func(r *k8s.ResourceInfo) string { return deployAvailable(r) }},
			{"AGE", func(r *k8s.ResourceInfo) string { return formatAge(r.CreatedAt.Time) }},
		}
		return base
	case "Service":
		return []tableColumn{
			{"NAME", func(r *k8s.ResourceInfo) string { return r.Name }},
			{"NAMESPACE", func(r *k8s.ResourceInfo) string { return r.Namespace }},
			{"TYPE", func(r *k8s.ResourceInfo) string { return svcType(r) }},
			{"CLUSTER-IP", func(r *k8s.ResourceInfo) string { return svcClusterIP(r) }},
			{"EXTERNAL-IP", func(r *k8s.ResourceInfo) string { return svcExternalIP(r) }},
			{"PORT(S)", func(r *k8s.ResourceInfo) string { return svcPorts(r) }},
			{"AGE", func(r *k8s.ResourceInfo) string { return formatAge(r.CreatedAt.Time) }},
		}
	case "ReplicaSet":
		return []tableColumn{
			{"NAME", func(r *k8s.ResourceInfo) string { return r.Name }},
			{"NAMESPACE", func(r *k8s.ResourceInfo) string { return r.Namespace }},
			{"DESIRED", func(r *k8s.ResourceInfo) string { return rsIntField(r, "replicas") }},
			{"CURRENT", func(r *k8s.ResourceInfo) string { return rsIntField(r, "availableReplicas") }},
			{"READY", func(r *k8s.ResourceInfo) string { return rsIntField(r, "readyReplicas") }},
			{"AGE", func(r *k8s.ResourceInfo) string { return formatAge(r.CreatedAt.Time) }},
		}
	case "Namespace":
		return []tableColumn{
			{"NAME", func(r *k8s.ResourceInfo) string { return r.Name }},
			{"STATUS", func(r *k8s.ResourceInfo) string { return StatusEmoji(r.Status) + " " + emptyToDash(r.Status) }},
			{"AGE", func(r *k8s.ResourceInfo) string { return formatAge(r.CreatedAt.Time) }},
		}
	case "Node":
		return []tableColumn{
			{"NAME", func(r *k8s.ResourceInfo) string { return r.Name }},
			{"STATUS", func(r *k8s.ResourceInfo) string { return StatusEmoji(r.Status) + " " + emptyToDash(r.Status) }},
			{"VERSION", func(r *k8s.ResourceInfo) string { return nodeVersion(r) }},
			{"AGE", func(r *k8s.ResourceInfo) string { return formatAge(r.CreatedAt.Time) }},
		}
	case "ConfigMap":
		return []tableColumn{
			{"NAME", func(r *k8s.ResourceInfo) string { return r.Name }},
			{"NAMESPACE", func(r *k8s.ResourceInfo) string { return r.Namespace }},
			{"DATA", func(r *k8s.ResourceInfo) string { return cmDataCount(r) }},
			{"AGE", func(r *k8s.ResourceInfo) string { return formatAge(r.CreatedAt.Time) }},
		}
	case "Secret":
		return []tableColumn{
			{"NAME", func(r *k8s.ResourceInfo) string { return r.Name }},
			{"NAMESPACE", func(r *k8s.ResourceInfo) string { return r.Namespace }},
			{"TYPE", func(r *k8s.ResourceInfo) string { return secretType(r) }},
			{"DATA", func(r *k8s.ResourceInfo) string { return cmDataCount(r) }},
			{"AGE", func(r *k8s.ResourceInfo) string { return formatAge(r.CreatedAt.Time) }},
		}
	case "PersistentVolumeClaim":
		return []tableColumn{
			{"NAME", func(r *k8s.ResourceInfo) string { return r.Name }},
			{"NAMESPACE", func(r *k8s.ResourceInfo) string { return r.Namespace }},
			{"STATUS", func(r *k8s.ResourceInfo) string { return StatusEmoji(r.Status) + " " + emptyToDash(r.Status) }},
			{"CAPACITY", func(r *k8s.ResourceInfo) string { return pvcCapacity(r) }},
			{"AGE", func(r *k8s.ResourceInfo) string { return formatAge(r.CreatedAt.Time) }},
		}
	case "Ingress":
		return []tableColumn{
			{"NAME", func(r *k8s.ResourceInfo) string { return r.Name }},
			{"NAMESPACE", func(r *k8s.ResourceInfo) string { return r.Namespace }},
			{"HOSTS", func(r *k8s.ResourceInfo) string { return ingressHosts(r) }},
			{"AGE", func(r *k8s.ResourceInfo) string { return formatAge(r.CreatedAt.Time) }},
		}
	case "Event":
		return []tableColumn{
			{"TYPE", func(r *k8s.ResourceInfo) string { return eventTypeEmoji(r) + " " + eventType(r) }},
			{"REASON", func(r *k8s.ResourceInfo) string { return eventReason(r) }},
			{"OBJECT", func(r *k8s.ResourceInfo) string { return eventObject(r) }},
			{"MESSAGE", func(r *k8s.ResourceInfo) string { return TruncateString(eventMessage(r), 60) }},
		}
	}

	// Default generic columns.
	return []tableColumn{
		{"NAME", func(r *k8s.ResourceInfo) string { return r.Name }},
		{"NAMESPACE", func(r *k8s.ResourceInfo) string { return emptyToDash(r.Namespace) }},
		{"STATUS", func(r *k8s.ResourceInfo) string { return StatusEmoji(r.Status) + " " + emptyToDash(r.Status) }},
		{"AGE", func(r *k8s.ResourceInfo) string { return formatAge(r.CreatedAt.Time) }},
	}
}

// rowForKind renders a row for the given column set.
func rowForKind(cols []tableColumn, r *k8s.ResourceInfo, kind string) []string {
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
		return "<none>"
	}
	if n := getString(spec, "nodeName"); n != "" {
		return n
	}
	return "<none>"
}

func podIP(r *k8s.ResourceInfo) string {
	status := getMap(r.Details, "status")
	if status == nil {
		return "<none>"
	}
	if ip := getString(status, "podIP"); ip != "" {
		return ip
	}
	return "<none>"
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
	return "<none>"
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
	return "<none>"
}

func statusSvc(r *k8s.ResourceInfo) map[string]interface{} {
	return getMap(r.Details, "status")
}

func svcPorts(r *k8s.ResourceInfo) string {
	spec := getMap(r.Details, "spec")
	ports, _ := spec["ports"].([]interface{})
	if len(ports) == 0 {
		return "<none>"
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
			if name != "" {
				out = append(out, fmt.Sprintf("%d/%s%s(%s)", port, proto, np, name))
			} else if np != "" {
				out = append(out, fmt.Sprintf("%d/%s%s", port, proto, np))
			} else {
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
	return "<none>"
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
	return "<none>"
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
		return "<none>"
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
		return "<none>"
	}
	return s
}

func formatAge(t time.Time) string {
	duration := time.Since(t)
	if duration.Hours() >= 24*365 {
		return fmt.Sprintf("%dy", int(duration.Hours()/24/365))
	} else if duration.Hours() >= 24*30 {
		return fmt.Sprintf("%dmo", int(duration.Hours()/24/30))
	} else if duration.Hours() >= 24 {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	} else if duration.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else if duration.Minutes() >= 1 {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	return fmt.Sprintf("%ds", int(duration.Seconds()))
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "<none>"
	}
	var parts []string
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

// FormatPodLogs formats pod logs for display
func FormatPodLogs(logs string, maxLines int) string {
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}

// FormatExecOutput formats exec command output
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

// FormatContexts formats kubeconfig contexts for display
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

// ---------- HTML Rich Formatting (for Telegram HTML parse mode) ----------

// FormatResourceAsHTML renders resources as a Telegram-compatible HTML table.
func FormatResourceAsHTML(resources []k8s.ResourceInfo, kind string) string {
	if len(resources) == 0 {
		return "<i>No resources found</i>"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>📦 %s (%d)</b>\n\n", kind, len(resources)))
	sb.WriteString("<pre>")
	sb.WriteString(htmlTableHeader(kind))
	for _, r := range resources {
		sb.WriteString(htmlRow(r, kind))
	}
	sb.WriteString("</pre>")
	return sb.String()
}

func htmlTableHeader(kind string) string {
	switch kind {
	case "Pod":
		return "NAME                    | NS    | READY | STATUS     | RESTARTS | AGE\n"
	case "Deployment":
		return "NAME              | NS    | READY | UP-TO-DATE | AVAILABLE | AGE\n"
	default:
		return "NAME        | NS    | STATUS | AGE\n"
	}
}

func htmlRow(r k8s.ResourceInfo, kind string) string {
	name := truncate(r.Name, 42)
	ns := r.Namespace
	if ns == "" {
		ns = "-"
	}
	emoji := StatusEmoji(r.Status)
	status := emptyToDash(r.Status)
	age := formatAge(r.CreatedAt.Time)
	switch kind {
	case "Pod":
		ready := podReady(&r)
		restarts := strconv.Itoa(podRestarts(&r))
		return fmt.Sprintf("%s | %s | %s | %s %s | %s | %s\n", name, ns, ready, emoji, status, restarts, age)
	case "Deployment":
		ready := deployReady(&r)
		up := deployUpToDate(&r)
		avail := deployAvailable(&r)
		return fmt.Sprintf("%s | %s | %s | %s | %s | %s\n", name, ns, ready, up, avail, age)
	default:
		return fmt.Sprintf("%s | %s | %s %s | %s\n", name, ns, emoji, status, age)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
