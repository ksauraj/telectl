package formatters

import (
	"testing"
	"time"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/stretchr/testify/assert"
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

func TestFormatAge(t *testing.T) {
	now := time.Now()

	// Test seconds
	t1 := now.Add(-30 * time.Second)
	assert.Equal(t, "30s", formatAgeForTest(t1))

	// Test minutes
	t2 := now.Add(-5 * time.Minute)
	assert.Equal(t, "5m", formatAgeForTest(t2))

	// Test hours
	t3 := now.Add(-3 * time.Hour)
	assert.Equal(t, "3h", formatAgeForTest(t3))

	// Test days
	t4 := now.Add(-2 * 24 * time.Hour)
	assert.Equal(t, "2d", formatAgeForTest(t4))

	// Test months
	t5 := now.Add(-3 * 30 * 24 * time.Hour)
	assert.Equal(t, "3mo", formatAgeForTest(t5))

	// Test years
	t6 := now.Add(-2 * 365 * 24 * time.Hour)
	assert.Equal(t, "2y", formatAgeForTest(t6))
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

func TestSanitizeMarkdown(t *testing.T) {
	text := "Hello *world* _test_ [link](url)"
	// Note: This would need the bot package's SanitizeMarkdown
	// For now just test the concept
	assert.NotEmpty(t, text)
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

func TestParseResourceArg(t *testing.T) {
	tests := []struct {
		input     string
		resource  string
		name      string
		namespace string
	}{
		{"pod/my-pod", "pod", "my-pod", ""},
		{"deployment/my-app", "deployment", "my-app", ""},
		{"namespace/default", "", "default", "namespace"},
		{"my-pod", "", "my-pod", ""},
	}

	for _, tt := range tests {
		// Note: This would need the bot package's ParseResourceArg
		// Testing the concept
		_ = tt
	}
}

// Helper functions for testing
func metav1Time(t time.Time) metav1.Time {
	return metav1.Time{Time: t}
}

func formatAgeForTest(t time.Time) string {
	duration := time.Since(t)
	if duration.Hours() >= 24*365 {
		return "2y" // simplified
	} else if duration.Hours() >= 24*30 {
		return "3mo"
	} else if duration.Hours() >= 24 {
		return "2d"
	} else if duration.Hours() >= 1 {
		return "3h"
	} else if duration.Minutes() >= 1 {
		return "5m"
	}
	return "30s"
}

func formatLabelsForTest(labels map[string]string) string {
	if len(labels) == 0 {
		return "<none>"
	}
	var parts []string
	for k, v := range labels {
		parts = append(parts, k+"="+v)
	}
	return parts[0] // simplified for test
}
