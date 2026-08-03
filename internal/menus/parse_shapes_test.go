package menus

import "testing"

// The field-table rewrite of ParseCallbackData must be behaviourally identical
// to the length-check chain it replaced. This exercises every callback shape the
// keyboards actually emit, plus the malformed and adversarial ones.
func TestParseCallbackDataExhaustiveShapes(t *testing.T) {
	cases := []struct {
		data      string
		typ       string
		action    string
		resType   string
		namespace string
		name      string
		container string
		extra     string
		nilWanted bool
	}{
		// Navigation
		{data: "menu:main", typ: "main"},
		{data: "menu:noop", typ: "noop"},
		{data: "menu:help", typ: "help"},

		// Resource browsing
		{data: "menu:resource:types", typ: "resource", action: "types"},
		{data: "menu:resource:pods", typ: "resource", action: "pods"},
		{data: "menu:resource:view:pods:default:api-1", typ: "resource", action: "view",
			resType: "pods", namespace: "default", name: "api-1"},
		{data: "menu:resource:view:nodes::node-1", typ: "resource", action: "view",
			resType: "nodes", namespace: "", name: "node-1"},
		{data: "menu:resource:list:pods:default", typ: "resource", action: "list",
			resType: "pods", namespace: "default"},
		{data: "menu:resource:refresh:pods:default", typ: "resource", action: "refresh",
			resType: "pods", namespace: "default"},
		// page puts the trailing field in Extra, not Name
		{data: "menu:resource:page:pods:default:2", typ: "resource", action: "page",
			resType: "pods", namespace: "default", extra: "2"},

		// Per-resource verbs
		{data: "menu:action:describe:pods:default:api-1", typ: "action", action: "describe",
			resType: "pods", namespace: "default", name: "api-1"},
		{data: "menu:action:cordon:nodes::node-1", typ: "action", action: "cordon",
			resType: "nodes", name: "node-1"},
		{data: "menu:action:logs:pods:default:api-1:sidecar", typ: "action", action: "logs",
			resType: "pods", namespace: "default", name: "api-1", container: "sidecar"},
		{data: "menu:action:logs:pods:default:api-1:sidecar:100", typ: "action", action: "logs",
			resType: "pods", namespace: "default", name: "api-1", container: "sidecar", extra: "100"},
		// scaleset routes its value argument to Extra, leaving Container empty
		{data: "menu:action:scaleset:deployments:prod:api:5", typ: "action", action: "scaleset",
			resType: "deployments", namespace: "prod", name: "api", extra: "5"},

		// Contexts, monitor, ops
		{data: "menu:ctx:switch:minikube", typ: "ctx", action: "switch", name: "minikube"},
		{data: "menu:ctx:refresh", typ: "ctx", action: "refresh"},
		{data: "menu:monitor:home", typ: "monitor", action: "home"},
		{data: "menu:monitor:top:pods", typ: "monitor", action: "top", resType: "pods"},
		{data: "menu:ops:home", typ: "ops", action: "home"},

		// Namespace / settings: the trailing name absorbs colons
		{data: "menu:ns:set:kube-system", typ: "ns", action: "set", name: "kube-system"},
		{data: "menu:ns:set:", typ: "ns", action: "set", name: ""},
		{data: "menu:ns:set:weird:name", typ: "ns", action: "set", name: "weird:name"},
		{data: "menu:ns:page:2", typ: "ns", action: "page", name: "2"},
		{data: "menu:settings:home", typ: "settings", action: "home"},
		{data: "menu:settings:namespace", typ: "settings", action: "namespace"},

		// Not ours
		{data: "", nilWanted: true},
		{data: "menu", nilWanted: true},
		{data: "notmenu:main", nilWanted: true},
		{data: "exec:exit", nilWanted: true},
	}

	for _, tc := range cases {
		t.Run(tc.data, func(t *testing.T) {
			got := ParseCallbackData(tc.data)
			if tc.nilWanted {
				if got != nil {
					t.Fatalf("ParseCallbackData(%q) = %+v, want nil", tc.data, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseCallbackData(%q) = nil", tc.data)
			}
			check := func(field, want, have string) {
				if want != have {
					t.Errorf("%s = %q, want %q (data %q)", field, have, want, tc.data)
				}
			}
			check("Type", tc.typ, got.Type)
			check("Action", tc.action, got.Action)
			check("ResourceType", tc.resType, got.ResourceType)
			check("Namespace", tc.namespace, got.Namespace)
			check("Name", tc.name, got.Name)
			check("Container", tc.container, got.Container)
			check("Extra", tc.extra, got.Extra)
		})
	}
}

// An unknown type must parse to its type alone rather than nil, so the
// dispatcher can log it instead of treating it as foreign data.
func TestParseCallbackDataUnknownType(t *testing.T) {
	got := ParseCallbackData("menu:somethingnew:with:fields")
	if got == nil {
		t.Fatal("unknown menu type parsed to nil")
	}
	if got.Type != "somethingnew" {
		t.Errorf("Type = %q", got.Type)
	}
	if got.Action != "" {
		t.Errorf("unknown type should not populate fields, got Action = %q", got.Action)
	}
}
