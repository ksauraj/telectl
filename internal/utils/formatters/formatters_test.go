package formatters

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFormatResource(t *testing.T) {
	resource := &k8s.ResourceInfo{
		Name:       "test-pod",
		Namespace:  "default",
		Kind:       "Pod",
		APIVersion: "v1",
		Status:     "Running",
		CreatedAt:  metav1Time(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)),
		Labels: map[string]string{
			"app": "nginx",
			"env": "prod",
		},
		Annotations: map[string]string{
			"description": "test pod",
		},
		Details: map[string]interface{}{
			"spec": map[string]interface{}{
				"nodeName": "node-1",
			},
			"status": map[string]interface{}{
				"podIP": "10.0.0.1",
			},
		},
	}

	// Test default format
	output := FormatResource(resource, "default")
	assert.Contains(t, output, "Name: test-pod")
	assert.Contains(t, output, "Namespace: default")
	assert.Contains(t, output, "Kind: Pod")
	assert.Contains(t, output, "app: nginx")

	// Test wide format
	output = FormatResource(resource, "wide")
	assert.Contains(t, output, "nodeName")
	assert.Contains(t, output, "podIP")

	// Test JSON format
	output = FormatResource(resource, "json")
	assert.Contains(t, output, "\"name\": \"test-pod\"")
	assert.Contains(t, output, "\"namespace\": \"default\"")
}

func TestFormatResourceList(t *testing.T) {
	resources := []k8s.ResourceInfo{
		{
			Name:       "pod-1",
			Namespace:  "default",
			Kind:       "Pod",
			APIVersion: "v1",
			Status:     "Running",
			CreatedAt:  metav1Time(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)),
			Labels:     map[string]string{"app": "nginx"},
		},
		{
			Name:       "pod-2",
			Namespace:  "kube-system",
			Kind:       "Pod",
			APIVersion: "v1",
			Status:     "Running",
			CreatedAt:  metav1Time(time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)),
			Labels:     map[string]string{"app": "coredns"},
		},
	}

	// Test table format
	output := FormatResourceList(resources, "table", false)
	assert.Contains(t, output, "pod-1")
	assert.Contains(t, output, "pod-2")
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "NAMESPACE")

	// Test wide format
	output = FormatResourceList(resources, "table", true)
	assert.Contains(t, output, "LABELS")

	// Test name format
	output = FormatResourceList(resources, "name", false)
	assert.Contains(t, output, "default/pod-1")
	assert.Contains(t, output, "kube-system/pod-2")

	// Test JSON format
	output = FormatResourceList(resources, "json", false)
	assert.Contains(t, output, "pod-1")
	assert.Contains(t, output, "pod-2")
}

// formatAge must be exercised directly. This test previously called a local
// formatAgeForTest that returned hardcoded strings, so it passed no matter what
// formatAge actually did.
func TestFormatAge(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name string
		when time.Time
		want string
	}{
		{"seconds", now.Add(-30 * time.Second), "30s"},
		{"minutes", now.Add(-5 * time.Minute), "5m"},
		{"hours", now.Add(-3 * time.Hour), "3h"},
		{"days", now.Add(-2 * 24 * time.Hour), "2d"},
		{"months", now.Add(-3 * 30 * 24 * time.Hour), "3mo"},
		{"years", now.Add(-2 * 365 * 24 * time.Hour), "2y"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, formatAge(tc.when))
		})
	}
}

// Boundaries are where an age formatter goes wrong: 24h must read as a day,
// not 24 hours, and the unit must switch exactly once.
func TestFormatAgeBoundaries(t *testing.T) {
	now := time.Now()
	cases := []struct {
		when time.Time
		want string
	}{
		{now.Add(-59 * time.Second), "59s"},
		{now.Add(-60 * time.Second), "1m"},
		{now.Add(-59 * time.Minute), "59m"},
		{now.Add(-60 * time.Minute), "1h"},
		{now.Add(-23 * time.Hour), "23h"},
		{now.Add(-24 * time.Hour), "1d"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, formatAge(tc.when),
			"age of %s", time.Since(tc.when).Round(time.Second))
	}
}

func TestFormatLabels(t *testing.T) {
	labels := map[string]string{
		"app":  "nginx",
		"env":  "prod",
		"tier": "frontend",
	}

	output := formatLabels(labels)
	assert.Contains(t, output, "app=nginx")
	assert.Contains(t, output, "env=prod")
	assert.Contains(t, output, "tier=frontend")

	// Test empty labels
	assert.Equal(t, "<none>", formatLabels(map[string]string{}))
}

func TestFormatPodLogs(t *testing.T) {
	logs := `line 1
line 2
line 3
line 4
line 5`

	// Test with limit
	output := FormatPodLogs(logs, 3)
	assert.Equal(t, "line 3\nline 4\nline 5", output)

	// Test without limit
	output = FormatPodLogs(logs, 0)
	assert.Equal(t, logs, output)
}

// EscapeMDV2 must escape every character Telegram's MarkdownV2 reserves;
// an unescaped one makes the API reject the whole message with a 400.
func TestEscapeMDV2(t *testing.T) {
	got := EscapeMDV2("Hello *world* _test_ [link](url) a.b-c!")
	for _, c := range []string{"*", "_", "[", "]", "(", ")", ".", "-", "!"} {
		assert.NotContains(t, strings.ReplaceAll(got, "\\"+c, ""), c,
			"unescaped %q would make Telegram reject the message", c)
	}
	assert.Equal(t, "plain", EscapeMDV2("plain"))
}

// EscapeHTML must neutralise the characters Telegram's HTML mode parses as
// markup, or a resource name containing < would truncate the message.
func TestEscapeHTML(t *testing.T) {
	assert.Equal(t, "&lt;b&gt;", EscapeHTML("<b>"))
	assert.Equal(t, "a &amp; b", EscapeHTML("a & b"))
	assert.Equal(t, "plain-name", EscapeHTML("plain-name"))
}

func TestTruncateString(t *testing.T) {
	longString := "This is a very long string that should be truncated"
	output := TruncateString(longString, 20)
	assert.Equal(t, "This is a very lo...", output)

	// Test short string
	shortString := "Short"
	output = TruncateString(shortString, 20)
	assert.Equal(t, "Short", output)
}

// StatusEmoji drives the status glyph in every list and detail pane; an
// unmapped status must fall back rather than render empty.
func TestStatusEmoji(t *testing.T) {
	cases := map[string]string{
		"Running":          "🟢",
		"running":          "🟢",
		"Pending":          "🟡",
		"Failed":           "🔴",
		"CrashLoopBackOff": "🔴",
		"Terminating":      "🟠",
		"Unknown":          "❓",
		"":                 "•",
		"SomethingNew":     "•",
	}
	for status, want := range cases {
		assert.Equal(t, want, StatusEmoji(status), "status %q", status)
	}
}

// Helper functions for testing.
func metav1Time(t time.Time) metav1.Time {
	return metav1.Time{Time: t}
}

// columnsForKind must hand back a slice that does not share storage with the
// package-level table. Callers append to the result, and when a kind has no
// wide columns `append(base)` returns the shared slice itself.
//
// Note the check is pointer identity, not "append and see if it leaks": the
// map's slice literals have len == cap, so an append always reallocates and an
// append-based test would pass even against a deliberately aliased return.
func TestColumnsForKindDoesNotAliasTheTable(t *testing.T) {
	for _, kind := range []string{"Pod", "Service", "Event"} {
		t.Run(kind, func(t *testing.T) {
			table := kindColumns[kind]
			require.NotEmpty(t, table, "fixture kind has no table")

			for _, wide := range []bool{false, true} {
				got := columnsForKind(kind, wide)
				require.NotEmpty(t, got)
				if &got[0] == &table[0] {
					t.Errorf("columnsForKind(%q, %v) returned the shared table; a caller's "+
						"append would mutate every later render", kind, wide)
				}
			}
		})
	}
}

// Wide output must add columns for kinds that define them and be a no-op for
// kinds that do not.
func TestColumnsForKindWide(t *testing.T) {
	narrow := columnsForKind("Pod", false)
	wide := columnsForKind("Pod", true)
	if len(wide) <= len(narrow) {
		t.Errorf("wide Pod columns (%d) should exceed narrow (%d)", len(wide), len(narrow))
	}

	headers := make([]string, 0, len(wide))
	for _, c := range wide {
		headers = append(headers, c.Header)
	}
	for _, want := range []string{"NODE", "IP", "LABELS"} {
		assert.Contains(t, headers, want)
	}

	// Service defines no wide columns; wide must not change the set.
	assert.Equal(t, len(columnsForKind("Service", false)), len(columnsForKind("Service", true)))
}

// An unknown kind must still render a usable table rather than none.
func TestColumnsForKindUnknownFallsBack(t *testing.T) {
	cols := columnsForKind("CustomResourceThing", false)
	if len(cols) == 0 {
		t.Fatal("unknown kind produced no columns")
	}
	assert.Equal(t, "NAME", cols[0].Header)
}

// Column alignment in the code-block tables depends on displayWidth: counting a
// status emoji as one column shifts every following cell left by one.
func TestDisplayWidth(t *testing.T) {
	cases := map[string]int{
		"":       0,
		"abc":    3,
		"🟢":      2, // emoji: two columns
		"🟢 pod":  6, // 2 + space + 3
		"日本":     4, // CJK: two columns each
		"한글":     4, // Hangul syllables
		"a日b":    4,
		"my-pod": 6,
	}
	for in, want := range cases {
		if got := displayWidth(in); got != want {
			t.Errorf("displayWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

// padRight must pad to the visual width, not the byte or rune count, or a
// column containing an emoji ends up one character short.
func TestPadRightUsesDisplayWidth(t *testing.T) {
	got := padRight("🟢", 4)
	if displayWidth(got) != 4 {
		t.Errorf("padRight(%q, 4) has display width %d, want 4", got, displayWidth(got))
	}
	// Already at or over the width: returned unchanged.
	if got := padRight("abcd", 4); got != "abcd" {
		t.Errorf("padRight at exact width = %q", got)
	}
	if got := padRight("abcdef", 4); got != "abcdef" {
		t.Errorf("padRight over width should not truncate, got %q", got)
	}
}

// The old NAME truncation sliced by byte (row[0][:maxNameWidth-1]), which cuts
// a multi-byte rune in half and emits invalid UTF-8 — Telegram rejects the
// whole message when that happens. Truncation must cut on rune boundaries.
func TestTruncateToWidthKeepsValidUTF8(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
	}{
		{"ascii", strings.Repeat("a", 60), 42},
		{"cjk", strings.Repeat("日", 60), 42},
		{"emoji", strings.Repeat("🟢", 60), 42},
		{"mixed", strings.Repeat("a日🟢", 30), 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateToWidth(tc.in, tc.width)
			if !utf8.ValidString(got) {
				t.Errorf("truncation produced invalid UTF-8: %q", got)
			}
			if w := displayWidth(got); w > tc.width {
				t.Errorf("truncated to display width %d, limit was %d", w, tc.width)
			}
			if !strings.HasSuffix(got, "…") {
				t.Errorf("truncated value should end with an ellipsis: %q", got)
			}
		})
	}

	// Short values are returned untouched, with no ellipsis.
	if got := truncateToWidth("short", 42); got != "short" {
		t.Errorf("short value was modified: %q", got)
	}
}

// A long pod name must not push the other columns out of alignment.
func TestFormatResourceListTableAlignsLongNames(t *testing.T) {
	resources := []k8s.ResourceInfo{
		{Name: strings.Repeat("very-long-pod-name", 5), Namespace: "default",
			Kind: "Pod", Status: "Running", CreatedAt: metav1Time(time.Now())},
		{Name: "short", Namespace: "default", Kind: "Pod", Status: "Running",
			CreatedAt: metav1Time(time.Now())},
	}
	out := formatResourceListTable(resources, false)

	if !utf8.ValidString(out) {
		t.Fatal("table contains invalid UTF-8")
	}

	// Every data row must present the same visual width up to the last column.
	lines := strings.Split(out, "\n")
	widths := make([]int, 0, len(lines))
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "```") || strings.Contains(line, "item(s)") {
			continue
		}
		widths = append(widths, displayWidth(line))
	}
	if len(widths) < 3 { // header, separator, two rows
		t.Fatalf("expected several table lines, got %d", len(widths))
	}
	for i, w := range widths[1:] {
		if w != widths[0] {
			t.Errorf("row %d has display width %d, header has %d — columns are misaligned",
				i+1, w, widths[0])
		}
	}
}
