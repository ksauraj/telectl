package kubeconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKubeconfig(t *testing.T) {
	// Create a temporary kubeconfig for testing
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "config")

	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- name: test-cluster
  cluster:
    server: https://kubernetes.example.com
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCg==
users:
- name: test-user
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCg==
    client-key-data: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQo=
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
    namespace: default
current-context: test-context
`

	err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)
	require.NoError(t, err)

	kc, err := ParseKubeconfig(kubeconfigPath)
	require.NoError(t, err)

	assert.Equal(t, "test-context", kc.CurrentContext)
	assert.Len(t, kc.Contexts, 1)
	assert.Len(t, kc.Clusters, 1)
	assert.Len(t, kc.Users, 1)

	ctx := kc.GetCurrentContext()
	require.NotNil(t, ctx)
	assert.Equal(t, "test-context", ctx.Name)
	assert.Equal(t, "test-cluster", ctx.Cluster)
	assert.Equal(t, "test-user", ctx.User)
	assert.Equal(t, "default", ctx.Namespace)
	assert.True(t, ctx.Current)
}

func TestParseKubeconfigMultipleContexts(t *testing.T) {
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "config")

	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- name: cluster-1
  cluster:
    server: https://cluster1.example.com
- name: cluster-2
  cluster:
    server: https://cluster2.example.com
users:
- name: user-1
  user:
    token: token-1
- name: user-2
  user:
    token: token-2
contexts:
- name: context-1
  context:
    cluster: cluster-1
    user: user-1
    namespace: ns-1
- name: context-2
  context:
    cluster: cluster-2
    user: user-2
    namespace: ns-2
current-context: context-1
`

	err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)
	require.NoError(t, err)

	kc, err := ParseKubeconfig(kubeconfigPath)
	require.NoError(t, err)

	assert.Len(t, kc.Contexts, 2)
	assert.Len(t, kc.Clusters, 2)
	assert.Len(t, kc.Users, 2)

	ctx1 := kc.GetContextByName("context-1")
	require.NotNil(t, ctx1)
	assert.Equal(t, "cluster-1", ctx1.Cluster)
	assert.Equal(t, "ns-1", ctx1.Namespace)

	ctx2 := kc.GetContextByName("context-2")
	require.NotNil(t, ctx2)
	assert.Equal(t, "cluster-2", ctx2.Cluster)
	assert.Equal(t, "ns-2", ctx2.Namespace)
}

func TestSwitchContext(t *testing.T) {
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "config")

	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- name: cluster-1
  cluster:
    server: https://cluster1.example.com
users:
- name: user-1
  user:
    token: token-1
contexts:
- name: context-1
  context:
    cluster: cluster-1
    user: user-1
- name: context-2
  context:
    cluster: cluster-1
    user: user-1
current-context: context-1
`

	err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)
	require.NoError(t, err)

	kc, err := ParseKubeconfig(kubeconfigPath)
	require.NoError(t, err)

	// Switch context
	err = kc.SwitchContext("context-2")
	require.NoError(t, err)

	assert.Equal(t, "context-2", kc.CurrentContext)
	ctx := kc.GetCurrentContext()
	require.NotNil(t, ctx)
	assert.Equal(t, "context-2", ctx.Name)
	assert.True(t, ctx.Current)
}

func TestAddContext(t *testing.T) {
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "config")

	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- name: cluster-1
  cluster:
    server: https://cluster1.example.com
users:
- name: user-1
  user:
    token: token-1
contexts:
- name: context-1
  context:
    cluster: cluster-1
    user: user-1
current-context: context-1
`

	err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)
	require.NoError(t, err)

	kc, err := ParseKubeconfig(kubeconfigPath)
	require.NoError(t, err)

	// Add new context
	err = kc.AddContext("context-3", "cluster-1", "user-1", "new-ns")
	require.NoError(t, err)

	assert.Len(t, kc.Contexts, 2)
	ctx := kc.GetContextByName("context-3")
	require.NotNil(t, ctx)
	assert.Equal(t, "new-ns", ctx.Namespace)
}

func TestDeleteContext(t *testing.T) {
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "config")

	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- name: cluster-1
  cluster:
    server: https://cluster1.example.com
users:
- name: user-1
  user:
    token: token-1
contexts:
- name: context-1
  context:
    cluster: cluster-1
    user: user-1
- name: context-2
  context:
    cluster: cluster-1
    user: user-1
current-context: context-1
`

	err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)
	require.NoError(t, err)

	kc, err := ParseKubeconfig(kubeconfigPath)
	require.NoError(t, err)

	// Delete context
	err = kc.DeleteContext("context-2")
	require.NoError(t, err)

	assert.Len(t, kc.Contexts, 1)
	assert.Nil(t, kc.GetContextByName("context-2"))
}

func TestParseKubeconfigNotFound(t *testing.T) {
	_, err := ParseKubeconfig("/nonexistent/path/config")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	path := expandPath("~/test")
	expected := filepath.Join(home, "test")
	assert.Equal(t, expected, path)
}
