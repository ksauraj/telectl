package bot

import (
	"context"
	"strings"
	"testing"

	bottg "github.com/go-telegram/bot"
	"github.com/ksauraj/telectl/internal/k8s"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// detailBot assembles a Bot whose k8s client is backed by fakes, so the detail
// pane verbs run through the real dispatch path without a cluster.
//
// The typed fake and the dynamic fake are seeded from the same objects: the
// typed one serves cordon/drain/scale/logs, the dynamic one serves every
// get/list that flows through ResourceInfo.
func detailBot(t *testing.T, objs ...runtime.Object) (*Bot, *bottg.Bot, *fakeTelegram) {
	t.Helper()

	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)

	// The dynamic fake derives its list kinds from the scheme and panics on a
	// LIST for anything unregistered, so every group the namespace summary
	// counts has to be present here.
	scheme := runtime.NewScheme()
	for name, add := range map[string]func(*runtime.Scheme) error{
		"corev1":       corev1.AddToScheme,
		"appsv1":       appsv1.AddToScheme,
		"batchv1":      batchv1.AddToScheme,
		"networkingv1": networkingv1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatalf("add %s to scheme: %v", name, err)
		}
	}

	cs := k8sfake.NewSimpleClientset(objs...)
	installScaleReactors(cs)

	b.k8sClient = k8s.NewClientForTest(
		cs,
		dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, metricsListKinds(), objs...),
		zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel)),
		false,
	)
	b.registerUpdateHandlers(lib)
	return b, lib, fake
}

// metricsListKinds registers the metrics.k8s.io list kinds with the dynamic
// fake.
//
// metrics.k8s.io types are not in any client-go scheme — they come from an
// aggregated API server — so the fake has no list kind for them and panics
// outright on a LIST rather than returning an error. The Monitoring pane lists
// them, so without this the test that walks every callback dies in the fake
// instead of exercising the code.
func metricsListKinds() map[schema.GroupVersionResource]string {
	return map[schema.GroupVersionResource]string{
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:  "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}: "NodeMetricsList",
	}
}

// installScaleReactors teaches the fake clientset about the scale subresource.
//
// The default object tracker has no notion of subresources: a GET on
// deployments/scale returns the Deployment, and client-go's generated fake then
// asserts it to *autoscalingv1.Scale and panics. These reactors read and write
// spec.replicas on the tracked object, which is what a real API server does.
func installScaleReactors(cs *k8sfake.Clientset) {
	replicasOf := func(obj runtime.Object) int32 {
		switch o := obj.(type) {
		case *appsv1.Deployment:
			if o.Spec.Replicas != nil {
				return *o.Spec.Replicas
			}
		case *appsv1.ReplicaSet:
			if o.Spec.Replicas != nil {
				return *o.Spec.Replicas
			}
		}
		return 0
	}
	setReplicas := func(obj runtime.Object, n int32) {
		switch o := obj.(type) {
		case *appsv1.Deployment:
			o.Spec.Replicas = &n
		case *appsv1.ReplicaSet:
			o.Spec.Replicas = &n
		}
	}

	for _, resource := range []string{"deployments", "replicasets"} {
		gvr := appsv1.SchemeGroupVersion.WithResource(resource)

		cs.PrependReactor("get", resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
			ga, ok := action.(k8stesting.GetAction)
			if !ok || ga.GetSubresource() != "scale" {
				return false, nil, nil
			}
			obj, err := cs.Tracker().Get(gvr, ga.GetNamespace(), ga.GetName())
			if err != nil {
				return true, nil, err
			}
			return true, &autoscalingv1.Scale{
				ObjectMeta: metav1.ObjectMeta{Name: ga.GetName(), Namespace: ga.GetNamespace()},
				Spec:       autoscalingv1.ScaleSpec{Replicas: replicasOf(obj)},
			}, nil
		})

		cs.PrependReactor("update", resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
			ua, ok := action.(k8stesting.UpdateAction)
			if !ok || ua.GetSubresource() != "scale" {
				return false, nil, nil
			}
			scale, ok := ua.GetObject().(*autoscalingv1.Scale)
			if !ok {
				return false, nil, nil
			}
			obj, err := cs.Tracker().Get(gvr, ua.GetNamespace(), scale.Name)
			if err != nil {
				return true, nil, err
			}
			setReplicas(obj, scale.Spec.Replicas)
			if err := cs.Tracker().Update(gvr, obj, ua.GetNamespace()); err != nil {
				return true, nil, err
			}
			return true, scale, nil
		})
	}
}

// typedClientset returns the fake clientset behind the bot's k8s client, so
// tests can intercept API calls (e.g. to see which container was asked for).
func typedClientset(b *Bot) *k8sfake.Clientset {
	if cs, ok := b.k8sClient.GetClientset().(*k8sfake.Clientset); ok {
		return cs
	}
	return nil
}

func podObj(name, ns, node string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: ns,
			Labels: map[string]string{"app": "api"},
		},
		Spec: corev1.PodSpec{
			NodeName:   node,
			Containers: []corev1.Container{{Name: "app"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func deployObj(name, ns string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: ns,
			Annotations: map[string]string{"deployment.kubernetes.io/revision": "2"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "api"}},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name: "app", Image: "nginx:1.25",
				}}},
			},
		},
	}
}

func nodeObj(name string, cordoned bool) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{Unschedulable: cordoned},
	}
}

func svcObj(name, ns string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "api"},
			Ports:    []corev1.ServicePort{{Port: 80}},
		},
	}
}

// richMarkdown returns every rich-message body the bot sent.
func richMarkdown(f *fakeTelegram) string {
	var parts []string
	for _, c := range f.sent() {
		if s, ok := c.Body["rich_message"].(string); ok {
			parts = append(parts, s)
		}
		if rm, ok := c.Body["rich_message"].(map[string]any); ok {
			if s, ok := rm["markdown"].(string); ok {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, "\n---\n")
}

// everything the bot sent, rich or plain, for substring assertions.
func allOutput(f *fakeTelegram) string {
	return strings.Join(f.allTexts(), "\n---\n") + "\n---\n" + richMarkdown(f)
}

// buttonLabels collects every inline-button label the bot has sent, for
// asserting that a keyboard offers a particular action.
func buttonLabels(f *fakeTelegram) string {
	var labels []string
	for _, c := range f.sent() {
		rm, ok := c.Body["reply_markup"].(map[string]any)
		if !ok {
			continue
		}
		rows, ok := rm["inline_keyboard"].([]any)
		if !ok {
			continue
		}
		for _, row := range rows {
			btns, ok := row.([]any)
			if !ok {
				continue
			}
			for _, btn := range btns {
				bm, ok := btn.(map[string]any)
				if !ok {
					continue
				}
				if text, ok := bm["text"].(string); ok {
					labels = append(labels, text)
				}
			}
		}
	}
	return strings.Join(labels, " | ")
}

// Tapping a resource must open the detail pane with its action keyboard.
// It used to run /describe directly, which dumped the object and offered no
// way to act on it.
func TestResourceTapOpensDetailPane(t *testing.T) {
	_, lib, fake := detailBot(t, podObj("api-abc", "default", "node-1"))

	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:resource:view:pods:default:api-abc", 7))

	var sawKeyboard bool
	for _, c := range fake.sent() {
		rm, ok := c.Body["reply_markup"].(map[string]any)
		if !ok {
			continue
		}
		if rows, ok := rm["inline_keyboard"].([]any); ok && len(rows) > 0 {
			sawKeyboard = true
		}
	}
	if !sawKeyboard {
		t.Errorf("detail pane sent no keyboard — user would be stuck; calls: %v", methodsOf(fake))
	}
}

// Each verb must produce output referencing the object it was invoked on.
// Before this change every one of them hit the "not available yet" default.
func TestDetailVerbsProduceOutput(t *testing.T) {
	objs := []runtime.Object{
		podObj("api-abc", "default", "node-1"),
		deployObj("api", "default", 3),
		svcObj("api", "default"),
		nodeObj("node-1", false),
	}

	tests := []struct {
		name string
		data string
		want string
	}{
		{"pod labels", "menu:action:labels:pods:default:api-abc", "app"},
		{"pod events", "menu:action:events:pods:default:api-abc", "event"},
		{"deployment selector", "menu:action:selector:deployments:default:api", "app=api"},
		{"deployment pods", "menu:action:pods:deployments:default:api", "api-abc"},
		{"deployment history", "menu:action:history:deployments:default:api", "istory"},
		{"deployment manifest", "menu:action:edit:deployments:default:api", "Manifest"},
		{"service endpoints", "menu:action:endpoints:services:default:api", "ndpoint"},
		{"node pods", "menu:action:nodepods:nodes::node-1", "api-abc"},
		{"namespace summary", "menu:action:nsresources:namespaces::default", "Pods"},
		{"contextual help", "menu:action:help:pods:default:api-abc", "Describe"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, lib, fake := detailBot(t, objs...)
			lib.ProcessUpdate(context.Background(), callbackUpdate(tc.data, 7))

			out := allOutput(fake)
			if strings.Contains(out, "not available yet") {
				t.Fatalf("%q still falls through to the unhandled-action default", tc.data)
			}
			if out == "\n---\n" || strings.TrimSpace(out) == "---" {
				t.Fatalf("%q produced no output at all; calls: %v", tc.data, methodsOf(fake))
			}
			if !strings.Contains(strings.ToLower(out), strings.ToLower(tc.want)) {
				t.Errorf("%q output missing %q.\ngot: %s", tc.data, tc.want, out)
			}
		})
	}
}

// Cordon must actually patch the node, not just report success.
func TestCordonPatchesNode(t *testing.T) {
	b, lib, _ := detailBot(t, nodeObj("node-1", false))

	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:action:cordon:nodes::node-1", 7))

	cordoned, err := b.k8sClient.IsNodeCordoned(context.Background(), "node-1")
	if err != nil {
		t.Fatalf("read node: %v", err)
	}
	if !cordoned {
		t.Error("cordon button did not set spec.unschedulable")
	}

	// And uncordon must reverse it.
	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:action:uncordon:nodes::node-1", 7))
	cordoned, err = b.k8sClient.IsNodeCordoned(context.Background(), "node-1")
	if err != nil {
		t.Fatalf("read node: %v", err)
	}
	if cordoned {
		t.Error("uncordon button did not clear spec.unschedulable")
	}
}

// Drain must be gated behind a confirmation: it is the most destructive thing
// a single tap can do.
func TestDrainAsksBeforeEvicting(t *testing.T) {
	b, lib, fake := detailBot(t, nodeObj("node-1", false), podObj("api-abc", "default", "node-1"))

	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:action:drain:nodes::node-1", 7))

	out := allOutput(fake)
	if !strings.Contains(out, "Drain node") {
		t.Errorf("drain did not ask for confirmation: %s", out)
	}
	// Nothing may have happened yet.
	if cordoned, _ := b.k8sClient.IsNodeCordoned(context.Background(), "node-1"); cordoned {
		t.Error("drain cordoned the node before the user confirmed")
	}

	fake.reset()
	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:action:confirmdrain:nodes::node-1", 7))

	if cordoned, _ := b.k8sClient.IsNodeCordoned(context.Background(), "node-1"); !cordoned {
		t.Error("confirmed drain did not cordon the node")
	}
	if out := allOutput(fake); !strings.Contains(out, "Drain") {
		t.Errorf("confirmed drain reported nothing: %s", out)
	}
}

// Scaling from the menu must reach the API, and the quick-scale buttons must
// carry their replica count through the callback round trip.
func TestScaleFromMenuChangesReplicas(t *testing.T) {
	b, lib, _ := detailBot(t, deployObj("api", "default", 3))

	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:action:scaleset:deployments:default:api:5", 7))

	got, err := b.k8sClient.GetDeploymentReplicas(context.Background(), "default", "api")
	if err != nil {
		t.Fatalf("read replicas: %v", err)
	}
	if got != 5 {
		t.Errorf("replicas = %d, want 5 — the scaleset button did not apply", got)
	}
}

// The scale chooser must show the live replica count so the active option is
// marked correctly.
func TestScaleMenuShowsCurrentReplicas(t *testing.T) {
	_, lib, fake := detailBot(t, deployObj("api", "default", 3))

	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:action:scale:deployments:default:api", 7))

	out := allOutput(fake)
	if !strings.Contains(out, "Current replicas") {
		t.Errorf("scale menu did not report the current count: %s", out)
	}
}

// A verb aimed at a missing object must explain itself rather than go silent.
func TestMissingObjectReportsError(t *testing.T) {
	_, lib, fake := detailBot(t)

	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:action:labels:pods:default:ghost", 7))

	out := allOutput(fake)
	if !strings.Contains(out, "Failed to") && !strings.Contains(out, "not found") {
		t.Errorf("missing object produced no diagnostic: %s", out)
	}
}

// A multi-container pod must offer one Logs button per container, and that
// button must fetch from that container — not silently use the first one.
func TestPerContainerLogsButton(t *testing.T) {
	pod := podObj("multi-abc", "default", "node-1")
	pod.Spec.Containers = []corev1.Container{
		{Name: "app"}, {Name: "sidecar"},
	}
	b, lib, fake := detailBot(t, pod)

	// The pane must render a button for the second container. Button labels
	// live in reply_markup, not in the message body.
	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:resource:view:pods:default:multi-abc", 7))
	if !strings.Contains(buttonLabels(fake), "sidecar") {
		t.Errorf("multi-container pod rendered no per-container log button; labels: %s",
			buttonLabels(fake))
	}

	// Tapping that button must actually fetch (no "not available"), and must
	// ask for the *sidecar* container, not silently fall back to the first one.
	// The fake clientset returns the same body for every container, so the only
	// way to tell them apart is to intercept the API call.
	fake.reset()
	cs := typedClientset(b)
	if cs == nil {
		t.Fatal("expected a fake clientset behind the k8s client")
	}
	var askedFor *corev1.PodLogOptions
	cs.PrependReactor("get", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		// GetLogs records a GenericActionImpl, which does not satisfy GetAction
		// (it has no name); read the subresource off the base Action instead.
		if action.GetSubresource() != "log" {
			return false, nil, nil
		}
		if generic, ok := action.(k8stesting.GenericAction); ok {
			if opts, ok := generic.GetValue().(*corev1.PodLogOptions); ok {
				askedFor = opts
			}
		}
		return false, nil, nil // let the fake serve the body
	})

	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:action:logs:pods:default:multi-abc:sidecar", 7))

	if askedFor == nil || askedFor.Container != "sidecar" {
		t.Errorf("per-container logs requested container %v, want sidecar", askedFor)
	}
	out := allOutput(fake)
	if strings.Contains(out, "not available yet") {
		t.Error("per-container logs button is still dead")
	}
	if !strings.Contains(out, "Logs") {
		t.Errorf("per-container logs produced no output: %s", out)
	}
}

// The tail-size buttons (50/100/500) must carry the count through to the
// fetch, so tapping "Last 100" gives 100 lines, not the default.
func TestTailSizeButtonCarriesCount(t *testing.T) {
	_, lib, fake := detailBot(t, podObj("api-abc", "default", "node-1"))

	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:action:logs:pods:default:api-abc:app:100", 7))

	out := allOutput(fake)
	if !strings.Contains(out, "Logs") {
		t.Errorf("tail-size logs produced no output: %s", out)
	}
	if strings.Contains(out, "not available yet") {
		t.Error("tail-size logs button is still dead")
	}
}
