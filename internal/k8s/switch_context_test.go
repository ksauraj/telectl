package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

const twoClusterKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: cluster-one
  cluster:
    server: https://one.example.com
    insecure-skip-tls-verify: true
- name: cluster-two
  cluster:
    server: https://two.example.com
    insecure-skip-tls-verify: true
users:
- name: user-one
  user:
    token: token-one
- name: user-two
  user:
    token: token-two
contexts:
- name: ctx-one
  context:
    cluster: cluster-one
    user: user-one
- name: ctx-two
  context:
    cluster: cluster-two
    user: user-two
current-context: ctx-one
`

func newTestClient(t *testing.T) (*Client, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(twoClusterKubeconfig), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := NewClient(path, "", false, zap.NewNop())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, path
}

// The clientset and rest.Config were built once in NewClient and never rebuilt,
// so switching context changed the kubeconfig but left the bot querying the
// previous cluster while reporting success.
func TestSwitchContextRebuildsConnection(t *testing.T) {
	c, _ := newTestClient(t)

	if got := c.GetRESTConfig().Host; got != "https://one.example.com" {
		t.Fatalf("initial host = %q", got)
	}
	if got := c.CurrentContextName(); got != "ctx-one" {
		t.Errorf("initial context = %q, want ctx-one", got)
	}

	if err := c.SwitchContext("ctx-two"); err != nil {
		t.Fatalf("SwitchContext: %v", err)
	}

	if got := c.GetRESTConfig().Host; got != "https://two.example.com" {
		t.Errorf("host after switch = %q; connection was not rebuilt", got)
	}
	if got := c.CurrentContextName(); got != "ctx-two" {
		t.Errorf("context after switch = %q, want ctx-two", got)
	}
	if c.GetClientset() == nil {
		t.Error("clientset is nil after switch")
	}
}

// A chat message must not rewrite the operator's ~/.kube/config: that file is
// shared with kubectl and with every other user of the bot.
func TestSwitchContextDoesNotTouchDisk(t *testing.T) {
	c, path := newTestClient(t)

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.SwitchContext("ctx-two"); err != nil {
		t.Fatalf("SwitchContext: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("SwitchContext modified the kubeconfig on disk; it must be session-scoped")
	}
}

// The explicit persistent variant may write, and must stay loadable.
func TestSwitchContextPersistentWritesLoadableFile(t *testing.T) {
	c, path := newTestClient(t)

	if err := c.SwitchContextPersistent("ctx-two"); err != nil {
		t.Fatalf("SwitchContextPersistent: %v", err)
	}

	reparsed, err := NewClient(path, "", false, zap.NewNop())
	if err != nil {
		t.Fatalf("kubeconfig unusable after persistent switch: %v", err)
	}
	if got := reparsed.CurrentContextName(); got != "ctx-two" {
		t.Errorf("persisted context = %q, want ctx-two", got)
	}
}

// An unknown context must fail cleanly and leave the client on the old cluster.
func TestSwitchContextUnknownKeepsPreviousConnection(t *testing.T) {
	c, _ := newTestClient(t)

	if err := c.SwitchContext("nope"); err == nil {
		t.Fatal("expected an error for an unknown context")
	}
	if got := c.GetRESTConfig().Host; got != "https://one.example.com" {
		t.Errorf("failed switch changed the connection to %q", got)
	}
	if got := c.CurrentContextName(); got != "ctx-one" {
		t.Errorf("failed switch changed the context to %q", got)
	}
}
