package k8s

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Drain has two hard rules: never evict a DaemonSet pod (its controller would
// immediately recreate it on the same node, so the drain would never finish)
// and never evict a mirror/static pod (nothing exists to recreate it, so the
// component would just disappear).
func TestDrainSkipReasons(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want string // substring; "" means the pod must be evicted
	}{
		{
			name: "plain running pod is drainable",
			pod:  pod(func(p *corev1.Pod) {}),
			want: "",
		},
		{
			name: "replicaset pod is drainable",
			pod: pod(func(p *corev1.Pod) {
				p.OwnerReferences = []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "rs-x"}}
			}),
			want: "",
		},
		{
			name: "succeeded pod is a no-op",
			pod: pod(func(p *corev1.Pod) {
				p.Status.Phase = corev1.PodSucceeded
			}),
			want: "already terminal",
		},
		{
			name: "failed pod is a no-op",
			pod: pod(func(p *corev1.Pod) {
				p.Status.Phase = corev1.PodFailed
			}),
			want: "already terminal",
		},
		{
			name: "daemonset pod must survive",
			pod: pod(func(p *corev1.Pod) {
				p.OwnerReferences = []metav1.OwnerReference{{Kind: "DaemonSet", Name: "ds-x"}}
			}),
			want: "DaemonSet",
		},
		{
			name: "mirror pod must survive",
			pod: pod(func(p *corev1.Pod) {
				p.Annotations = map[string]string{corev1.MirrorPodAnnotationKey: "abc"}
			}),
			want: "mirror",
		},
		{
			name: "already terminating pod is a no-op",
			pod: pod(func(p *corev1.Pod) {
				now := metav1.Now()
				p.DeletionTimestamp = &now
			}),
			want: "terminating",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := drainSkipReason(tc.pod)
			if tc.want == "" {
				if got != "" {
					t.Errorf("pod should have been evicted, got skip reason %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("drainSkipReason = %q, want it to mention %q", got, tc.want)
			}
		})
	}
}

func TestNodeConditionSummary(t *testing.T) {
	// Missing and malformed inputs must not panic — these come straight from
	// the API server as untyped maps.
	for _, r := range []*ResourceInfo{
		nil,
		{},
		{Details: map[string]interface{}{}},
		{Details: map[string]interface{}{"status": "not-a-map"}},
		{Details: map[string]interface{}{"status": map[string]interface{}{"conditions": "not-a-slice"}}},
	} {
		if got := NodeConditionSummary(r); got != "" {
			t.Errorf("NodeConditionSummary(%+v) = %q, want empty", r, got)
		}
	}

	r := &ResourceInfo{Details: map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "MemoryPressure", "status": "False"},
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	// Order follows the API server's own ordering, not a sort.
	if got, want := NodeConditionSummary(r), "MemoryPressure=False, Ready=True"; got != want {
		t.Errorf("NodeConditionSummary = %q, want %q", got, want)
	}
}

func pod(mutate func(*corev1.Pod)) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	mutate(p)
	return p
}
