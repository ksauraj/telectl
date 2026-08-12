package bot

import (
	"sort"
	"testing"

	"github.com/ksauraj/telectl/internal/config"
	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/menus"
	"github.com/ksauraj/telectl/internal/tg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This is the invariant behind the sixteen dead buttons: the keyboards rendered
// them, and no dispatcher branch existed, so tapping one replied "that action is
// not available yet" — a defect indistinguishable from a working button until
// someone tried it.
//
// Rather than trust a hand-maintained list, walk every keyboard the menu builder
// can produce, collect the action verbs, and require each to be dispatchable.
func TestEveryRenderedVerbHasAHandler(t *testing.T) {
	cfg := &config.Config{}
	cfg.Bot.MenuPageSize = 10
	mb := menus.NewMenuBuilder(cfg)

	pod := &k8s.ResourceInfo{
		Name: "api-1", Namespace: "default", Kind: "Pod",
		CreatedAt: metav1.Now(),
		Details: map[string]interface{}{"spec": map[string]interface{}{
			"nodeName": "node-1",
			"containers": []interface{}{
				map[string]interface{}{"name": "app"},
				map[string]interface{}{"name": "sidecar"},
			},
		}},
	}
	node := func(cordoned bool) *k8s.ResourceInfo {
		return &k8s.ResourceInfo{Name: "node-1", Kind: "Node",
			Details: map[string]interface{}{"spec": map[string]interface{}{
				"unschedulable": cordoned,
			}}}
	}

	keyboards := []tg.InlineKeyboardMarkup{
		mb.GetResourceActionInlineKeyboard("pods", "default", "api-1", pod),
		mb.GetResourceActionInlineKeyboard("deployments", "default", "api", nil),
		mb.GetResourceActionInlineKeyboard("replicasets", "default", "api-abc", nil),
		mb.GetResourceActionInlineKeyboard("services", "default", "api", nil),
		mb.GetResourceActionInlineKeyboard("nodes", "", "node-1", node(false)),
		mb.GetResourceActionInlineKeyboard("nodes", "", "node-1", node(true)),
		mb.GetResourceActionInlineKeyboard("namespaces", "", "kube-system", nil),
		mb.GetResourceActionInlineKeyboard("configmaps", "default", "cm-1", nil),
		mb.GetResourceActionInlineKeyboard("secrets", "default", "sec-1", nil),
		mb.GetScaleKeyboard("deployments", "default", "api", 3),
		mb.GetScaleKeyboard("replicasets", "default", "api-abc", 1),
		mb.GetLogOptionsKeyboard("default", "api-1", "app"),
		mb.GetConfirmDeleteKeyboard("pods", "default", "api-1"),
	}

	// Verbs served by the older dispatcher rather than the detail map.
	legacy := map[string]bool{
		"delete":        true,
		"confirmdelete": true,
		"restart":       true,
		"exec":          true,
		"portforward":   true,
	}

	seen := map[string]bool{}
	for _, kb := range keyboards {
		for _, row := range kb.InlineKeyboard {
			for _, btn := range row {
				if btn.CallbackData == "" {
					continue
				}
				full, ok := mb.ResolveCallback(btn.CallbackData)
				if !ok {
					t.Errorf("button %q resolved to nothing", btn.Text)
					continue
				}
				act := menus.ParseCallbackData(full)
				if act == nil {
					t.Errorf("button %q (%s) does not parse", btn.Text, full)
					continue
				}
				if act.Type != "action" {
					continue
				}
				seen[act.Action] = true
				if _, handled := detailVerbs[act.Action]; !handled && !legacy[act.Action] {
					t.Errorf("verb %q is rendered by button %q but has no handler — "+
						"tapping it replies \"not available yet\"",
						act.Action, btn.Text)
				}
			}
		}
	}

	if len(seen) == 0 {
		t.Fatal("no action verbs found; the walk is not reaching the keyboards")
	}
	t.Logf("verified %d distinct verbs are dispatchable", len(seen))
}

// The converse: a handler with no button is unreachable code. confirmdrain is
// the documented exception — it is only rendered inside the drain confirmation
// prompt, which the bot builds directly rather than through a keyboard builder.
func TestEveryDetailHandlerIsReachable(t *testing.T) {
	cfg := &config.Config{}
	cfg.Bot.MenuPageSize = 10
	mb := menus.NewMenuBuilder(cfg)

	pod := &k8s.ResourceInfo{
		Name: "api-1", Namespace: "default", Kind: "Pod",
		Details: map[string]interface{}{"spec": map[string]interface{}{
			"nodeName": "node-1",
			"containers": []interface{}{
				map[string]interface{}{"name": "app"},
				map[string]interface{}{"name": "sidecar"},
			},
		}},
	}
	node := &k8s.ResourceInfo{Name: "node-1", Kind: "Node",
		Details: map[string]interface{}{"spec": map[string]interface{}{"unschedulable": false}}}
	cordoned := &k8s.ResourceInfo{Name: "node-1", Kind: "Node",
		Details: map[string]interface{}{"spec": map[string]interface{}{"unschedulable": true}}}
	secret := &k8s.ResourceInfo{Name: "my-secret", Namespace: "default", Kind: "Secret",
		Details: map[string]interface{}{"data": map[string]interface{}{"key": "dmFsdWU="}}}

	rendered := map[string]bool{}
	for _, kb := range []tg.InlineKeyboardMarkup{
		mb.GetResourceActionInlineKeyboard("pods", "default", "api-1", pod),
		mb.GetResourceActionInlineKeyboard("deployments", "default", "api", nil),
		mb.GetResourceActionInlineKeyboard("replicasets", "default", "api-abc", nil),
		mb.GetResourceActionInlineKeyboard("services", "default", "api", nil),
		mb.GetResourceActionInlineKeyboard("nodes", "", "node-1", node),
		mb.GetResourceActionInlineKeyboard("nodes", "", "node-1", cordoned),
		mb.GetResourceActionInlineKeyboard("namespaces", "", "kube-system", nil),
		mb.GetResourceActionInlineKeyboard("secrets", "default", "my-secret", secret),
		mb.GetScaleKeyboard("deployments", "default", "api", 3),
		mb.GetLogOptionsKeyboard("default", "api-1", "app"),
	} {
		for _, row := range kb.InlineKeyboard {
			for _, btn := range row {
				full, ok := mb.ResolveCallback(btn.CallbackData)
				if !ok {
					continue
				}
				if act := menus.ParseCallbackData(full); act != nil && act.Type == "action" {
					rendered[act.Action] = true
				}
			}
		}
	}

	// Rendered only inside the drain confirmation prompt, built in confirmDrain.
	exempt := map[string]string{
		"confirmdrain": "rendered by confirmDrain's inline prompt",
	}

	orphans := make([]string, 0, len(detailVerbs))
	for verb := range detailVerbs {
		if rendered[verb] {
			continue
		}
		if _, ok := exempt[verb]; ok {
			continue
		}
		orphans = append(orphans, verb)
	}
	sort.Strings(orphans)
	if len(orphans) > 0 {
		t.Errorf("handlers with no button anywhere (unreachable): %v", orphans)
	}
}
