package formatters

import (
	"strings"
	"testing"
	"time"

	"github.com/ksauraj/telectl/internal/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pod(name, ns, status string) k8s.ResourceInfo {
	return k8s.ResourceInfo{
		Name:      name,
		Namespace: ns,
		Kind:      "Pod",
		Status:    status,
		CreatedAt: metav1.NewTime(time.Now().Add(-11 * time.Hour)),
		Details: map[string]interface{}{
			"status": map[string]interface{}{
				"containerStatuses": []interface{}{
					map[string]interface{}{"ready": true, "restartCount": int64(0)},
				},
			},
		},
	}
}

// The rich list must be a real GFM table, not the monospace code block.
func TestRichResourceListRendersTable(t *testing.T) {
	got := RichResourceList([]k8s.ResourceInfo{
		pod("nginx-1", "default", "Running"),
		pod("redis-1", "kube-system", "CrashLoopBackOff"),
	}, false)

	if strings.Contains(got, "```") {
		t.Errorf("must not be code-fenced:\n%s", got)
	}
	for _, want := range []string{
		"### Pods — 2 item(s)",
		"| NAME | NAMESPACE | READY | STATUS | RESTARTS | AGE |",
		"nginx-1", "kube-system",
		"🟢", // Running
		"🔴", // CrashLoopBackOff
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// Rich and text tables share columnsForKind, so their headers cannot drift.
func TestRichAndTextTablesShareColumns(t *testing.T) {
	pods := []k8s.ResourceInfo{pod("a", "default", "Running")}
	rich := RichResourceList(pods, false)
	text := FormatResourceList(pods, "", false)

	for _, col := range []string{"NAME", "NAMESPACE", "READY", "STATUS", "RESTARTS", "AGE"} {
		if !strings.Contains(rich, col) {
			t.Errorf("rich table missing column %q", col)
		}
		if !strings.Contains(text, col) {
			t.Errorf("text table missing column %q", col)
		}
	}
}

func TestRichResourceListWideAddsColumns(t *testing.T) {
	pods := []k8s.ResourceInfo{pod("a", "default", "Running")}
	narrow := RichResourceList(pods, false)
	wide := RichResourceList(pods, true)

	if strings.Contains(narrow, "NODE") {
		t.Error("narrow table should not include NODE")
	}
	for _, col := range []string{"NODE", "IP", "LABELS"} {
		if !strings.Contains(wide, col) {
			t.Errorf("wide table missing %q:\n%s", col, wide)
		}
	}
}

func TestRichResourceListEmpty(t *testing.T) {
	if got := RichResourceList(nil, false); !strings.Contains(got, "No resources found") {
		t.Errorf("unexpected empty rendering: %q", got)
	}
}

// Labels/annotations go into collapsible sections so big objects stay readable.
func TestRichResourceUsesCollapsibleSections(t *testing.T) {
	r := pod("nginx-1", "default", "Running")
	r.Labels = map[string]string{"app": "nginx", "tier": "web"}
	r.Annotations = map[string]string{"owner": "team-a"}

	got := RichResource(&r, false)

	for _, want := range []string{
		"### 🟢 nginx-1",
		"| Field | Value |",
		"<details>",
		"<summary>🏷️ Labels (2)</summary>",
		"<summary>📝 Annotations (1)</summary>",
		"| app | nginx |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// Map keys must be sorted so output is stable between calls.
func TestRichResourceSortsMapKeys(t *testing.T) {
	r := pod("p", "default", "Running")
	r.Labels = map[string]string{"z": "1", "a": "2", "m": "3"}

	got := RichResource(&r, false)
	ia, im, iz := strings.Index(got, "| a |"), strings.Index(got, "| m |"), strings.Index(got, "| z |")
	if ia < 0 || im < 0 || iz < 0 {
		t.Fatalf("labels missing:\n%s", got)
	}
	if !(ia < im && im < iz) {
		t.Errorf("label keys not sorted (a=%d m=%d z=%d)", ia, im, iz)
	}
}

func TestRichResourceNil(t *testing.T) {
	if got := RichResource(nil, false); !strings.Contains(got, "not found") {
		t.Errorf("unexpected nil rendering: %q", got)
	}
}

func TestRichContextsMarksCurrent(t *testing.T) {
	got := RichContexts([]KubeContext{
		NewKubeContext("minikube", "minikube", "default", true),
		NewKubeContext("prod", "prod-cluster", "apps", false),
	})

	if !strings.Contains(got, "✅") {
		t.Errorf("current context not marked:\n%s", got)
	}
	for _, want := range []string{"minikube", "prod-cluster", "| Name | Cluster | Namespace |"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRichMetricsPodShape(t *testing.T) {
	m := k8s.ResourceInfo{
		Name: "nginx-1", Namespace: "default", Kind: "PodMetrics",
		Details: map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{"usage": map[string]interface{}{"cpu": "12m", "memory": "40Mi"}},
			},
		},
	}
	got := RichMetrics("📊 Pod Resource Usage", []k8s.ResourceInfo{m})

	for _, want := range []string{"| NAME | NAMESPACE | CPU | MEMORY |", "12m", "40Mi", "nginx-1"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRichMetricsNodeShape(t *testing.T) {
	m := k8s.ResourceInfo{
		Name: "node-1", Kind: "NodeMetrics",
		Details: map[string]interface{}{
			"usage": map[string]interface{}{"cpu": "250m", "memory": "1Gi"},
		},
	}
	got := RichMetrics("📊 Node Resource Usage", []k8s.ResourceInfo{m})

	if strings.Contains(got, "NAMESPACE") {
		t.Errorf("node metrics should omit NAMESPACE:\n%s", got)
	}
	for _, want := range []string{"250m", "1Gi", "node-1"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRichMetricsMissingUsage(t *testing.T) {
	m := k8s.ResourceInfo{Name: "x", Namespace: "default", Details: nil}
	got := RichMetrics("m", []k8s.ResourceInfo{m})
	if !strings.Contains(got, "-") {
		t.Errorf("expected dash placeholders for missing usage:\n%s", got)
	}
}

func TestRichLogsUsesCodeBlock(t *testing.T) {
	got := RichLogs("default/nginx-1", "app", "line one\nline two")

	if !strings.Contains(got, "```") {
		t.Errorf("logs should be a real code block:\n%s", got)
	}
	if !strings.Contains(got, "nginx-1") || !strings.Contains(got, "app") {
		t.Errorf("missing pod/container in heading:\n%s", got)
	}
	// Escaped brackets would show up literally as \[app\] in the client.
	if strings.Contains(got, `\[`) || strings.Contains(got, `\]`) {
		t.Errorf("heading contains escaped brackets that render literally:\n%s", got)
	}
}

func TestRichKeyValue(t *testing.T) {
	got := RichKeyValue("⚙️ Config", [][2]string{{"Dry Run", "false"}, {"Rate Limit", "30/min"}})
	for _, want := range []string{"### ⚙️ Config", "Dry Run", "30/min"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// A pod name containing Markdown metacharacters must not corrupt the table.
func TestRichEscapesHostileResourceNames(t *testing.T) {
	got := RichResourceList([]k8s.ResourceInfo{
		pod("weird|name*with_md", "default", "Running"),
	}, false)

	if strings.Contains(got, "weird|name") {
		t.Errorf("unescaped pipe would add a column:\n%s", got)
	}
	if !strings.Contains(got, `weird\|name`) {
		t.Errorf("expected escaped pipe:\n%s", got)
	}
}

func TestRichEventsTable(t *testing.T) {
	ev := k8s.ResourceInfo{
		Name: "e1", Namespace: "default", Kind: "Event",
		CreatedAt: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		Details: map[string]interface{}{
			"type":    "Warning",
			"reason":  "BackOff",
			"message": "Back-off restarting failed container",
			"involvedObject": map[string]interface{}{
				"kind": "Pod", "name": "nginx-1",
			},
		},
	}
	got := RichEvents([]k8s.ResourceInfo{ev})

	for _, want := range []string{"| Type | Reason | Object | Message | Age |", "BackOff"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestPluralKind(t *testing.T) {
	cases := map[string]string{"Pod": "Pods", "Ingress": "Ingresses", "": "Resources"}
	for in, want := range cases {
		if got := pluralKind(in, 2); got != want {
			t.Errorf("pluralKind(%q, 2) = %q, want %q", in, got, want)
		}
	}
	if got := pluralKind("Pod", 1); got != "Pod" {
		t.Errorf("singular should not be pluralised, got %q", got)
	}
}
