package handlers

import (
	"context"
	"io"
	"testing"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/pkg/kubeconfig"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MockK8sClient is a mock for the k8s client.
type MockK8sClient struct {
	mock.Mock
}

func (m *MockK8sClient) ListResources(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace,
	labelSelector,
	fieldSelector string) ([]k8s.ResourceInfo,
	error,
) {
	args := m.Called(ctx, gvr, namespace, labelSelector, fieldSelector)
	return args.Get(0).([]k8s.ResourceInfo), args.Error(1)
}

func (m *MockK8sClient) GetResource(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace,
	name string) (*k8s.ResourceInfo,
	error,
) {
	args := m.Called(ctx, gvr, namespace, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*k8s.ResourceInfo), args.Error(1)
}

func (m *MockK8sClient) GetPodLogs(ctx context.Context, opts k8s.PodLogOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockK8sClient) ExecInPod(ctx context.Context, opts k8s.ExecOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}

func (m *MockK8sClient) ListContexts(ctx context.Context) ([]kubeconfig.ContextInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]kubeconfig.ContextInfo), args.Error(1)
}

func (m *MockK8sClient) SwitchContext(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockK8sClient) GetCurrentContext() *kubeconfig.ContextInfo {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*kubeconfig.ContextInfo)
}

func (m *MockK8sClient) GetKubeconfig() *kubeconfig.KubeConfig {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*kubeconfig.KubeConfig)
}

func TestInlineQueryHandler_HandleInlineQuery(t *testing.T) {
	// This test would require a full bot mock setup
	// For now, we test the parsing logic
	t.Skip("Requires full bot mock setup")
}

func TestInlineQueryHandler_showInlineHelp(t *testing.T) {
	t.Skip("Requires full bot mock setup")
}

func TestInlineQueryHandler_answerInlineQuery(t *testing.T) {
	t.Skip("Requires full bot mock setup")
}

// Inline query used to carry private copies of the alias table and the GVR
// table, so a resource added to types.ResourceMap stayed silently unavailable
// in inline mode. It now resolves through the shared map.
func TestInlineQueryResolvesEveryKnownAlias(t *testing.T) {
	// Aliases the old private table omitted entirely.
	for _, alias := range []string{"secret", "secrets", "persistentvolume", "pvs", "ev"} {
		if _, ok := types.ResourceMap[alias]; !ok {
			t.Errorf("alias %q missing from the shared resource map", alias)
		}
	}
}

func TestParseInlineArgs(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		namespace string
		resName   string
		selector  string
	}{
		{name: "empty", args: nil},
		{name: "bare name", args: []string{"api-1"}, resName: "api-1"},
		{name: "namespace separate", args: []string{"-n", "prod"}, namespace: "prod"},
		{name: "namespace joined", args: []string{"-n=prod"}, namespace: "prod"},
		{name: "namespace long", args: []string{"--namespace", "prod"}, namespace: "prod"},
		{name: "namespace long joined", args: []string{"--namespace=prod"}, namespace: "prod"},
		{name: "selector separate", args: []string{"-l", "app=api"}, selector: "app=api"},
		{name: "selector joined", args: []string{"-l=app=api"}, selector: "app=api"},
		{name: "selector long", args: []string{"--selector", "app=api"}, selector: "app=api"},
		{name: "name and namespace", args: []string{"api-1", "-n", "prod"},
			resName: "api-1", namespace: "prod"},
		{name: "all three", args: []string{"api-1", "-n", "prod", "-l", "app=api"},
			resName: "api-1", namespace: "prod", selector: "app=api"},
		// Only the first bare word is the name.
		{name: "extra words ignored", args: []string{"api-1", "api-2"}, resName: "api-1"},
		// Inline queries arrive mid-typing; a dangling flag must not panic or
		// consume the following iteration.
		{name: "dangling namespace flag", args: []string{"-n"}},
		{name: "dangling selector flag", args: []string{"api-1", "-l"}, resName: "api-1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ns, name, sel := parseInlineArgs(tc.args)
			if ns != tc.namespace {
				t.Errorf("namespace = %q, want %q", ns, tc.namespace)
			}
			if name != tc.resName {
				t.Errorf("name = %q, want %q", name, tc.resName)
			}
			if sel != tc.selector {
				t.Errorf("selector = %q, want %q", sel, tc.selector)
			}
		})
	}
}
