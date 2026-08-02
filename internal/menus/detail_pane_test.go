package menus

import (
	"testing"

	"github.com/ksauraj/telectl/internal/k8s"
)

// podFixture is a pod with two containers, scheduled to a node.
func podFixture() *k8s.ResourceInfo {
	return &k8s.ResourceInfo{
		Name: "api-server-5b7d9f4c82-klmno", Namespace: "prod", Kind: "Pod",
		Details: map[string]interface{}{"spec": map[string]interface{}{
			"nodeName": "pool-1-node-3",
			"containers": []interface{}{
				map[string]interface{}{"name": "app"},
				map[string]interface{}{"name": "sidecar"},
			},
		}},
	}
}

func nodeFixture(cordoned bool) *k8s.ResourceInfo {
	return &k8s.ResourceInfo{
		Name: "pool-1-node-3", Kind: "Node",
		Details: map[string]interface{}{"spec": map[string]interface{}{
			"unschedulable": cordoned,
		}},
	}
}

// Regression test for the shape bug that made most action buttons dead.
//
// ParseCallbackData decodes "menu:action:<verb>:<type>:<ns>:<name>" by
// position. Buttons that omitted the type field shifted every later field one
// slot left, so the pod Logs button parsed with Namespace="api-server-…" and an
// empty Name — and dispatchResourceAction's `if action.Name == "" { return }`
// discarded it in silence. Same for Restart, Scale, Endpoints, History, and the
// node verbs.
//
// Every button on every detail pane must therefore round-trip to the field
// values the dispatcher reads.
func TestDetailPaneCallbacksCarryCorrectFields(t *testing.T) {
	mb := &MenuBuilder{config: testConfig(), tokens: NewTokenStore(4096)}

	type want struct {
		resourceType string
		namespace    string
		name         string
	}

	cases := []struct {
		pane     string
		keyboard []string
		want     want
	}{
		{
			pane: "pods",
			keyboard: buttonData(mb.GetResourceActionInlineKeyboard(
				"pods", "prod", "api-server-5b7d9f4c82-klmno", podFixture())),
			want: want{"pods", "prod", "api-server-5b7d9f4c82-klmno"},
		},
		{
			pane: "deployments",
			keyboard: buttonData(mb.GetResourceActionInlineKeyboard(
				"deployments", "prod", "api-server", nil)),
			want: want{"deployments", "prod", "api-server"},
		},
		{
			pane: "replicasets",
			keyboard: buttonData(mb.GetResourceActionInlineKeyboard(
				"replicasets", "prod", "api-server-5b7d9f4c82", nil)),
			want: want{"replicasets", "prod", "api-server-5b7d9f4c82"},
		},
		{
			pane: "services",
			keyboard: buttonData(mb.GetResourceActionInlineKeyboard(
				"services", "prod", "api-server", nil)),
			want: want{"services", "prod", "api-server"},
		},
		{
			// Cluster-scoped: the namespace slot is empty, which is exactly
			// where an off-by-one shift used to go unnoticed.
			pane: "nodes",
			keyboard: buttonData(mb.GetResourceActionInlineKeyboard(
				"nodes", "", "pool-1-node-3", nodeFixture(false))),
			want: want{"nodes", "", "pool-1-node-3"},
		},
		{
			pane: "namespaces",
			keyboard: buttonData(mb.GetResourceActionInlineKeyboard(
				"namespaces", "", "kube-system", nil)),
			want: want{"namespaces", "", "kube-system"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.pane, func(t *testing.T) {
			var sawAction bool
			for _, data := range tc.keyboard {
				full, ok := mb.ResolveCallback(data)
				if !ok {
					t.Fatalf("could not resolve %q", data)
				}
				act := ParseCallbackData(full)
				if act == nil {
					t.Errorf("%q does not parse — dead button", full)
					continue
				}
				if act.Type != "action" {
					continue // navigation buttons have their own shape
				}
				sawAction = true

				if act.Action == "" {
					t.Errorf("%q parsed with no verb", full)
				}
				if act.ResourceType != tc.want.resourceType {
					t.Errorf("%s: ResourceType = %q, want %q (data %q)",
						act.Action, act.ResourceType, tc.want.resourceType, full)
				}
				if act.Namespace != tc.want.namespace {
					t.Errorf("%s: Namespace = %q, want %q (data %q)",
						act.Action, act.Namespace, tc.want.namespace, full)
				}
				if act.Name != tc.want.name {
					t.Errorf("%s: Name = %q, want %q (data %q)",
						act.Action, act.Name, tc.want.name, full)
				}
			}
			if !sawAction {
				t.Error("pane rendered no action buttons")
			}
		})
	}
}

// The scale keyboard must carry the replica count where the dispatcher reads it
// (Extra), and the log keyboard must carry the container name in Container.
func TestScaleAndLogKeyboardsCarryArguments(t *testing.T) {
	mb := &MenuBuilder{config: testConfig(), tokens: NewTokenStore(4096)}

	var sawReplicas bool
	for _, data := range buttonData(mb.GetScaleKeyboard("deployments", "prod", "api", 3)) {
		full, ok := mb.ResolveCallback(data)
		if !ok {
			t.Fatalf("unresolvable %q", data)
		}
		act := ParseCallbackData(full)
		if act == nil || act.Action != "scaleset" {
			continue
		}
		sawReplicas = true
		if act.ResourceType != "deployments" || act.Namespace != "prod" || act.Name != "api" {
			t.Errorf("scaleset fields wrong: %+v (data %q)", act, full)
		}
		if act.Extra == "" {
			t.Errorf("scaleset carries no replica count: %q", full)
		}
	}
	if !sawReplicas {
		t.Error("scale keyboard produced no scaleset buttons")
	}

	var sawFollow bool
	for _, data := range buttonData(mb.GetLogOptionsKeyboard("prod", "api-abc", "sidecar")) {
		full, ok := mb.ResolveCallback(data)
		if !ok {
			t.Fatalf("unresolvable %q", data)
		}
		act := ParseCallbackData(full)
		if act == nil || act.Action != "logsfollow" {
			continue
		}
		sawFollow = true
		if act.Name != "api-abc" {
			t.Errorf("logsfollow Name = %q, want api-abc", act.Name)
		}
		if act.Container != "sidecar" {
			t.Errorf("logsfollow Container = %q, want sidecar", act.Container)
		}
	}
	if !sawFollow {
		t.Error("log options keyboard produced no follow button")
	}
}

// Every verb the detail dispatcher implements must actually appear on some
// pane. A verb with no button is unreachable; a button with no verb is dead.
func TestEveryDetailVerbIsReachable(t *testing.T) {
	mb := &MenuBuilder{config: testConfig(), tokens: NewTokenStore(4096)}

	panes := [][]string{
		buttonData(mb.GetResourceActionInlineKeyboard("pods", "prod", "p", podFixture())),
		buttonData(mb.GetResourceActionInlineKeyboard("deployments", "prod", "d", nil)),
		buttonData(mb.GetResourceActionInlineKeyboard("replicasets", "prod", "r", nil)),
		buttonData(mb.GetResourceActionInlineKeyboard("services", "prod", "s", nil)),
		buttonData(mb.GetResourceActionInlineKeyboard("nodes", "", "n", nodeFixture(false))),
		buttonData(mb.GetResourceActionInlineKeyboard("nodes", "", "n", nodeFixture(true))),
		buttonData(mb.GetResourceActionInlineKeyboard("namespaces", "", "ns", nil)),
		buttonData(mb.GetScaleKeyboard("deployments", "prod", "d", 1)),
		buttonData(mb.GetLogOptionsKeyboard("prod", "p", "c")),
	}

	seen := map[string]bool{}
	for _, pane := range panes {
		for _, data := range pane {
			full, ok := mb.ResolveCallback(data)
			if !ok {
				continue
			}
			if act := ParseCallbackData(full); act != nil && act.Type == "action" {
				seen[act.Action] = true
			}
		}
	}

	// The verbs the user asked to have wired, plus the ones the detail pane
	// introduced.
	for _, verb := range []string{
		"cordon", "uncordon", "drain", "nodepods", "top",
		"endpoints", "history", "edit", "pods", "rspods", "rsscale",
		"scaleset", "scalecustom", "logsfollow", "logsprevious", "nsresources",
		"describe", "labels", "events", "selector", "logs", "logsopts",
		"exec", "portforward", "delete", "restart", "scale",
	} {
		if !seen[verb] {
			t.Errorf("verb %q has no button on any detail pane — unreachable", verb)
		}
	}
}

// A cordoned node must offer Uncordon, and an open node must offer Cordon.
// Showing both always made it impossible to tell a node's state from the pane.
func TestNodePaneReflectsCordonState(t *testing.T) {
	mb := &MenuBuilder{config: testConfig(), tokens: NewTokenStore(4096)}

	verbs := func(resource *k8s.ResourceInfo) map[string]bool {
		out := map[string]bool{}
		for _, data := range buttonData(mb.GetResourceActionInlineKeyboard("nodes", "", "n", resource)) {
			full, _ := mb.ResolveCallback(data)
			if act := ParseCallbackData(full); act != nil && act.Type == "action" {
				out[act.Action] = true
			}
		}
		return out
	}

	open := verbs(nodeFixture(false))
	if !open["cordon"] || open["uncordon"] {
		t.Errorf("schedulable node should offer cordon only, got %v", open)
	}

	cordoned := verbs(nodeFixture(true))
	if !cordoned["uncordon"] || cordoned["cordon"] {
		t.Errorf("cordoned node should offer uncordon only, got %v", cordoned)
	}
}

// Aliases must collapse to the canonical plural so callback data is stable no
// matter which alias the user's command used.
func TestCanonicalResource(t *testing.T) {
	cases := map[string]string{
		"po": "pods", "pod": "pods", "pods": "pods",
		"deploy": "deployments", "svc": "services",
		"no": "nodes", "ns": "namespaces", "rs": "replicasets",
		"unknown-kind": "unknown-kind",
	}
	for in, want := range cases {
		if got := CanonicalResource(in); got != want {
			t.Errorf("CanonicalResource(%q) = %q, want %q", in, got, want)
		}
	}
}
