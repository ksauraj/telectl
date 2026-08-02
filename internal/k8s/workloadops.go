package k8s

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Workload helpers backing the per-resource menus: "which pods does this
// deployment own", "what does this service point at", "what revisions exist".
// All of them go through the typed or dynamic client — never a subprocess.

// SelectorForWorkload extracts the label selector of a deployment/replicaset
// (spec.selector.matchLabels) or a service (spec.selector) and renders it in
// the "k=v,k2=v2" form the API server expects.
//
// matchExpressions are deliberately not flattened here: they cannot be
// expressed in that form, and silently dropping them would produce a selector
// that matches more pods than the workload actually owns. Callers get the
// matchLabels part plus a flag saying the selector was incomplete.
func SelectorForWorkload(r *ResourceInfo) (selector string, complete bool) {
	if r == nil || r.Details == nil {
		return "", false
	}
	spec, ok := r.Details["spec"].(map[string]interface{})
	if !ok {
		return "", false
	}

	// Services carry a flat spec.selector map.
	if flat, ok := spec["selector"].(map[string]interface{}); ok {
		if ml, nested := flat["matchLabels"].(map[string]interface{}); nested {
			_, hasExpr := flat["matchExpressions"].([]interface{})
			return joinSelector(ml), !hasExpr
		}
		return joinSelector(flat), true
	}
	return "", false
}

func joinSelector(m map[string]interface{}) string {
	set := labels.Set{}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			set[k] = v
		}
	}
	if len(set) == 0 {
		return ""
	}
	return set.String()
}

// ListPodsForWorkload returns the pods a deployment/replicaset/service selects.
//
// It resolves the selector from the live object rather than guessing from the
// name, because a pod's name prefix is not a reliable ownership signal.
func (c *Client) ListPodsForWorkload(ctx context.Context, kind, namespace, name string) ([]ResourceInfo, string, error) {
	var (
		obj *ResourceInfo
		err error
	)
	switch kind {
	case "deployment", "deployments", "deploy":
		obj, err = c.GetDeployment(ctx, namespace, name)
	case "replicaset", "replicasets", "rs":
		obj, err = c.GetReplicaSet(ctx, namespace, name)
	case "service", "services", "svc":
		obj, err = c.GetService(ctx, namespace, name)
	default:
		return nil, "", fmt.Errorf("cannot list pods for kind %q", kind)
	}
	if err != nil {
		return nil, "", err
	}

	selector, complete := SelectorForWorkload(obj)
	if selector == "" {
		return nil, "", fmt.Errorf("%s %q has no usable label selector", kind, name)
	}
	if !complete {
		c.logger.Warn("Selector has matchExpressions that were not applied",
			zap.String("kind", kind), zap.String("name", name))
	}

	pods, err := c.ListPods(ctx, namespace, selector, "")
	if err != nil {
		return nil, selector, err
	}
	return pods, selector, nil
}

// GetEndpoints returns the Endpoints object backing a service, which is what
// tells you whether a service actually has ready backends.
func (c *Client) GetEndpoints(ctx context.Context, namespace, name string) (*ResourceInfo, error) {
	return c.GetResource(ctx, endpointsGVR(), namespace, name)
}

// EndpointAddresses flattens an Endpoints object into "ip:port" strings, split
// into ready and not-ready. A service with subsets but no ready addresses is
// the classic "everything looks fine but nothing works" case, so the two are
// reported separately rather than merged.
func EndpointAddresses(ep *ResourceInfo) (ready, notReady []string) {
	if ep == nil || ep.Details == nil {
		return nil, nil
	}
	subsets, ok := ep.Details["subsets"].([]interface{})
	if !ok {
		return nil, nil
	}
	for _, s := range subsets {
		sm, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		ports := endpointPorts(sm)
		ready = append(ready, endpointAddrs(sm, "addresses", ports)...)
		notReady = append(notReady, endpointAddrs(sm, "notReadyAddresses", ports)...)
	}
	sort.Strings(ready)
	sort.Strings(notReady)
	return ready, notReady
}

func endpointPorts(subset map[string]interface{}) []string {
	raw, ok := subset["ports"].([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, p := range raw {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		switch v := pm["port"].(type) {
		case int64:
			out = append(out, strconv.FormatInt(v, 10))
		case float64:
			out = append(out, strconv.FormatInt(int64(v), 10))
		}
	}
	return out
}

func endpointAddrs(subset map[string]interface{}, field string, ports []string) []string {
	raw, ok := subset[field].([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, a := range raw {
		am, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		ip, _ := am["ip"].(string)
		if ip == "" {
			continue
		}
		if len(ports) == 0 {
			out = append(out, ip)
			continue
		}
		for _, p := range ports {
			out = append(out, ip+":"+p)
		}
	}
	return out
}

// Revision describes one entry of a deployment's rollout history.
type Revision struct {
	Number       int64
	ReplicaSet   string
	Replicas     int64
	Ready        int64
	Image        string
	ChangeCause  string
	CreationTime metav1.Time
	Current      bool
}

// RolloutHistory reconstructs a deployment's revision list from the
// ReplicaSets it owns.
//
// There is no history API: kubectl derives the same list the same way, by
// reading each ReplicaSet's deployment.kubernetes.io/revision annotation. Only
// ReplicaSets whose ownerReferences point at this deployment are counted, so
// an unrelated ReplicaSet sharing labels cannot appear as a phantom revision.
func (c *Client) RolloutHistory(ctx context.Context, namespace, name string) ([]Revision, error) {
	deploy, err := c.GetDeployment(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	selector, _ := SelectorForWorkload(deploy)
	sets, err := c.ListReplicaSets(ctx, namespace, selector)
	if err != nil {
		return nil, err
	}

	currentRevision := annotation(deploy, "deployment.kubernetes.io/revision")

	var out []Revision
	for i := range sets {
		rs := &sets[i]
		if !ownedBy(rs, "Deployment", name) {
			continue
		}
		revStr := annotation(rs, "deployment.kubernetes.io/revision")
		rev, convErr := strconv.ParseInt(revStr, 10, 64)
		if convErr != nil {
			continue
		}
		out = append(out, Revision{
			Number:       rev,
			ReplicaSet:   rs.Name,
			Replicas:     nestedInt(rs.Details, "spec", "replicas"),
			Ready:        nestedInt(rs.Details, "status", "readyReplicas"),
			Image:        firstImage(rs),
			ChangeCause:  annotation(rs, "kubernetes.io/change-cause"),
			CreationTime: rs.CreatedAt,
			Current:      revStr != "" && revStr == currentRevision,
		})
	}

	// Newest revision first: that is the one an operator is asking about.
	sort.Slice(out, func(i, j int) bool { return out[i].Number > out[j].Number })
	return out, nil
}

// ScaleReplicaSet sets a ReplicaSet's replica count via the scale subresource.
//
// Note for callers: if the ReplicaSet is owned by a Deployment, its controller
// will revert this almost immediately. The UI warns about that rather than
// silently redirecting to the Deployment, which would scale something the user
// did not select.
func (c *Client) ScaleReplicaSet(ctx context.Context, namespace, name string, replicas int32) error {
	if c.dryRun {
		c.logger.Info("DRY RUN: Would scale replicaset",
			zap.String("replicaset", name), zap.Int32("replicas", replicas))
		return nil
	}

	scale, err := c.clientset.AppsV1().ReplicaSets(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get replicaset scale: %w", err)
	}
	scale.Spec.Replicas = replicas
	_, err = c.clientset.AppsV1().ReplicaSets(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	return err
}

// GetDeploymentReplicas returns the current desired replica count, used to mark
// the active choice in the scale keyboard.
func (c *Client) GetDeploymentReplicas(ctx context.Context, namespace, name string) (int32, error) {
	scale, err := c.clientset.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to get deployment scale: %w", err)
	}
	return scale.Spec.Replicas, nil
}

// GetReplicaSetReplicas returns a ReplicaSet's current desired replica count.
func (c *Client) GetReplicaSetReplicas(ctx context.Context, namespace, name string) (int32, error) {
	scale, err := c.clientset.AppsV1().ReplicaSets(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to get replicaset scale: %w", err)
	}
	return scale.Spec.Replicas, nil
}

// NamespaceSummary counts the common workload kinds in a namespace.
type NamespaceSummary struct {
	Namespace string
	Counts    []KindCount
}

type KindCount struct {
	Kind  string
	Count int
	Err   string
}

// SummariseNamespace counts each common kind in a namespace.
//
// A per-kind error is recorded rather than aborting: RBAC commonly permits
// listing pods but not secrets, and a partial summary is far more useful than
// a single "forbidden" for the whole view.
func (c *Client) SummariseNamespace(ctx context.Context, namespace string) *NamespaceSummary {
	summary := &NamespaceSummary{Namespace: namespace}
	for _, k := range namespaceSummaryKinds {
		items, err := c.ListResources(ctx, k.GVR, namespace, "", "")
		if err != nil {
			summary.Counts = append(summary.Counts, KindCount{Kind: k.Label, Err: err.Error()})
			continue
		}
		summary.Counts = append(summary.Counts, KindCount{Kind: k.Label, Count: len(items)})
	}
	return summary
}

func annotation(r *ResourceInfo, key string) string {
	if r == nil || r.Annotations == nil {
		return ""
	}
	return r.Annotations[key]
}

// namespaceSummaryKinds lists the kinds the namespace summary counts. Defined
// here with a label and the GVR rather than reusing ResourceMap, because the
// summary wants stable plural labels and a stable order.
var namespaceSummaryKinds = []struct {
	Label string
	GVR   schema.GroupVersionResource
}{
	{"Pods", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}},
	{"Deployments", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}},
	{"ReplicaSets", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}},
	{"Services", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}},
	{"Endpoints", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "endpoints"}},
	{"ConfigMaps", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}},
	{"Secrets", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}},
	{"PVCs", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}},
	{"Ingresses", schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}},
	{"Jobs", schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}},
	{"CronJobs", schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}},
	{"StatefulSets", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}},
	{"DaemonSets", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}},
	{"Events", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "events"}},
}

// endpointsGVR returns the GVR for the core Endpoints resource.
func endpointsGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "endpoints"}
}

func ownedBy(r *ResourceInfo, kind, name string) bool {
	if r == nil || r.Details == nil {
		return false
	}
	meta, ok := r.Details["metadata"].(map[string]interface{})
	if !ok {
		return false
	}
	owners, ok := meta["ownerReferences"].([]interface{})
	if !ok {
		return false
	}
	for _, o := range owners {
		om, ok := o.(map[string]interface{})
		if !ok {
			continue
		}
		if k, _ := om["kind"].(string); k != kind {
			continue
		}
		if n, _ := om["name"].(string); n == name {
			return true
		}
	}
	return false
}

func nestedInt(obj map[string]interface{}, path ...string) int64 {
	cur := obj
	for i, p := range path {
		if i == len(path)-1 {
			switch v := cur[p].(type) {
			case int64:
				return v
			case float64:
				return int64(v)
			}
			return 0
		}
		next, ok := cur[p].(map[string]interface{})
		if !ok {
			return 0
		}
		cur = next
	}
	return 0
}

func firstImage(r *ResourceInfo) string {
	if r == nil || r.Details == nil {
		return ""
	}
	spec, ok := r.Details["spec"].(map[string]interface{})
	if !ok {
		return ""
	}
	tmpl, ok := spec["template"].(map[string]interface{})
	if !ok {
		return ""
	}
	tspec, ok := tmpl["spec"].(map[string]interface{})
	if !ok {
		return ""
	}
	containers, ok := tspec["containers"].([]interface{})
	if !ok || len(containers) == 0 {
		return ""
	}
	cm, ok := containers[0].(map[string]interface{})
	if !ok {
		return ""
	}
	img, _ := cm["image"].(string)
	return img
}
