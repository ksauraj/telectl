package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Node lifecycle operations, spoken directly to the API server. There is no
// kubectl here and never should be: cordon is a patch, drain is a sequence of
// eviction subresource calls.

// SetNodeSchedulable flips spec.unschedulable. Cordon is unschedulable=true,
// uncordon is false. A strategic merge patch is used rather than get/update so
// two concurrent operators cannot clobber each other's unrelated node edits.
func (c *Client) SetNodeSchedulable(ctx context.Context, name string, schedulable bool) error {
	if name == "" {
		return fmt.Errorf("node name is required")
	}
	if c.dryRun {
		c.logger.Info("DRY RUN: Would set node schedulability",
			zap.String("node", name), zap.Bool("schedulable", schedulable))
		return nil
	}

	patch := []byte(fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, !schedulable))
	_, err := c.clientset.CoreV1().Nodes().Patch(
		ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch node %q: %w", name, err)
	}
	return nil
}

// IsNodeCordoned reports the node's current spec.unschedulable value, so the UI
// can show which of cordon/uncordon is the meaningful action.
func (c *Client) IsNodeCordoned(ctx context.Context, name string) (bool, error) {
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get node %q: %w", name, err)
	}
	return node.Spec.Unschedulable, nil
}

// ListPodsOnNode returns the pods scheduled to a node, across all namespaces.
// spec.nodeName is one of the few field selectors the API server indexes, so
// this is a server-side filter rather than a list-everything-and-filter.
func (c *Client) ListPodsOnNode(ctx context.Context, nodeName string) ([]ResourceInfo, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("node name is required")
	}
	return c.ListPods(ctx, "", "", "spec.nodeName="+nodeName)
}

// DrainResult summarises a drain attempt. Drain is reported rather than waited
// on: evictions are asynchronous, and a chat client should not block for the
// minutes a real drain can take.
type DrainResult struct {
	Cordoned  bool
	Evicted   []string
	Skipped   []DrainSkip
	Failed    []DrainFailure
	Remaining int
}

type DrainSkip struct {
	Pod    string
	Reason string
}

type DrainFailure struct {
	Pod string
	Err string
}

// DrainNode cordons a node and evicts its pods.
//
// Two categories are deliberately left alone, matching kubectl's defaults:
//   - DaemonSet pods, which the controller would immediately recreate on the
//     same node, so evicting them accomplishes nothing.
//   - Mirror (static) pods, which have no controller and cannot be rescheduled;
//     evicting them takes down a component with no replacement.
//
// Pods that are already terminal (Succeeded/Failed) are skipped as no-ops.
// Eviction respects PodDisruptionBudgets, so a rejected eviction is reported
// rather than escalated to a delete.
func (c *Client) DrainNode(ctx context.Context, nodeName string) (*DrainResult, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("node name is required")
	}

	result := &DrainResult{}

	if err := c.SetNodeSchedulable(ctx, nodeName, false); err != nil {
		return nil, err
	}
	result.Cordoned = true

	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods on node %q: %w", nodeName, err)
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		ref := pod.Namespace + "/" + pod.Name

		if reason := drainSkipReason(pod); reason != "" {
			result.Skipped = append(result.Skipped, DrainSkip{Pod: ref, Reason: reason})
			continue
		}

		if c.dryRun {
			result.Evicted = append(result.Evicted, ref)
			continue
		}

		evictErr := c.clientset.CoreV1().Pods(pod.Namespace).EvictV1(ctx, &policyv1.Eviction{
			ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace},
		})
		switch {
		case evictErr == nil:
			result.Evicted = append(result.Evicted, ref)
		case apierrors.IsNotFound(evictErr):
			// Already gone; that is the outcome we wanted.
			result.Evicted = append(result.Evicted, ref)
		default:
			result.Failed = append(result.Failed, DrainFailure{Pod: ref, Err: evictErr.Error()})
		}
	}

	sort.Strings(result.Evicted)
	result.Remaining = len(result.Evicted) + len(result.Failed)

	if c.dryRun {
		c.logger.Info("DRY RUN: drain simulated",
			zap.String("node", nodeName), zap.Int("would_evict", len(result.Evicted)))
	}
	return result, nil
}

// drainSkipReason returns why a pod should not be evicted, or "" to evict it.
func drainSkipReason(pod *corev1.Pod) string {
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return "already terminal (" + string(pod.Status.Phase) + ")"
	}
	if _, mirror := pod.Annotations[corev1.MirrorPodAnnotationKey]; mirror {
		return "static/mirror pod"
	}
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "DaemonSet" {
			return "DaemonSet-managed"
		}
	}
	if pod.DeletionTimestamp != nil {
		return "already terminating"
	}
	return ""
}

// NodeConditionSummary renders a node's conditions as "Ready=True, ..." for
// display. Kept here so the node views agree on the ordering.
func NodeConditionSummary(r *ResourceInfo) string {
	if r == nil || r.Details == nil {
		return ""
	}
	status, ok := r.Details["status"].(map[string]interface{})
	if !ok {
		return ""
	}
	conds, ok := status["conditions"].([]interface{})
	if !ok {
		return ""
	}
	var parts []string
	for _, c := range conds {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		t, _ := cm["type"].(string)
		s, _ := cm["status"].(string)
		if t == "" {
			continue
		}
		parts = append(parts, t+"="+s)
	}
	return strings.Join(parts, ", ")
}
