package types

import "testing"

// Regression: describing a namespace from the menu failed with "the server
// could not find the requested resource" because the handler defaulted the
// namespace to "default" and then queried a cluster-scoped kind as namespaced.
func TestIsClusterScoped(t *testing.T) {
	clusterScoped := []string{
		"namespace", "namespaces", "ns",
		"node", "nodes", "no",
		"pv", "pvs", "persistentvolume",
	}
	for _, alias := range clusterScoped {
		if !IsClusterScoped(alias) {
			t.Errorf("IsClusterScoped(%q) = false, want true", alias)
		}
	}

	namespaced := []string{
		"pod", "pods", "po",
		"deployment", "deployments", "deploy",
		"service", "svc", "configmap", "secret",
		"pvc", "pvcs", "persistentvolumeclaim",
		"ingress", "event", "replicaset",
	}
	for _, alias := range namespaced {
		if IsClusterScoped(alias) {
			t.Errorf("IsClusterScoped(%q) = true, want false", alias)
		}
	}

	if IsClusterScoped("nonsense") {
		t.Error("unknown alias should not report cluster-scoped")
	}
}

// Every alias in ResourceMap must classify consistently with its GVR, so a new
// alias cannot silently get the wrong scope.
func TestClusterScopeMatchesGVR(t *testing.T) {
	for alias, gvr := range ResourceMap {
		want := clusterScopedResources[gvr.Resource]
		if got := IsClusterScoped(alias); got != want {
			t.Errorf("alias %q (resource %q): IsClusterScoped=%v, want %v",
				alias, gvr.Resource, got, want)
		}
	}
}
