package formatters

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/pkg/kubeconfig"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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

// FormatResourceList formats a list of resources as a table
func FormatResourceList(resources []k8s.ResourceInfo, format string, wide bool) string {
	if len(resources) == 0 {
		return "No resources found"
	}

	switch format {
	case "json":
		return formatResourceListJSON(resources)
	case "yaml":
		return formatResourceListYAML(resources)
	case "name":
		return formatResourceListNames(resources)
	default:
		return formatResourceListTable(resources, wide)
	}
}

func formatResourceListTable(resources []k8s.ResourceInfo, wide bool) string {
	if len(resources) == 0 {
		return "No resources found"
	}

	// Determine columns based on resource type
	kind := resources[0].Kind
	columns := getColumnsForKind(kind, wide)

	// Calculate column widths
	widths := make(map[string]int)
	for _, col := range columns {
		widths[col] = len(col)
	}

	rows := make([]map[string]string, len(resources))
	for i, r := range resources {
		row := make(map[string]string)
		row["NAME"] = r.Name
		row["NAMESPACE"] = r.Namespace
		row["STATUS"] = r.Status
		row["AGE"] = formatAge(r.CreatedAt.Time)

		if wide {
			row["LABELS"] = formatLabels(r.Labels)
			if kind == "Pod" {
				if spec, ok := r.Details["spec"].(map[string]interface{}); ok {
					if node, ok := spec["nodeName"].(string); ok {
						row["NODE"] = node
					}
				}
				if status, ok := r.Details["status"].(map[string]interface{}); ok {
					if ip, ok := status["podIP"].(string); ok {
						row["IP"] = ip
					}
				}
			}
		}

		rows[i] = row
		for _, col := range columns {
			if len(row[col]) > widths[col] {
				widths[col] = len(row[col])
			}
		}
	}

	var sb strings.Builder

	// Header
	for i, col := range columns {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(padRight(col, widths[col]))
	}
	sb.WriteString("\n")

	// Separator
	for i, col := range columns {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(strings.Repeat("-", widths[col]))
	}
	sb.WriteString("\n")

	// Rows
	for _, row := range rows {
		for i, col := range columns {
			if i > 0 {
				sb.WriteString("  ")
			}
			sb.WriteString(padRight(row[col], widths[col]))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func getColumnsForKind(kind string, wide bool) []string {
	base := []string{"NAME", "NAMESPACE", "STATUS", "AGE"}
	if wide {
		switch kind {
		case "Pod":
			return append(base, "NODE", "IP", "LABELS")
		case "Deployment":
			return append(base, "READY", "UP-TO-DATE", "AVAILABLE", "LABELS")
		case "Service":
			return append(base, "TYPE", "CLUSTER-IP", "EXTERNAL-IP", "PORT(S)", "LABELS")
		case "Node":
			return append(base, "VERSION", "OS", "LABELS")
		default:
			return append(base, "LABELS")
		}
	}
	return base
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

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
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