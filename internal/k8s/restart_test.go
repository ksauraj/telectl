package k8s

import (
	"context"
	"encoding/json"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"go.uber.org/zap"
)

// RestartDeployment builds its strategic-merge patch by string concatenation.
// The template had five opening braces (spec/template/metadata/annotations plus
// the annotation object) but only four closing braces, so the JSON was
// truncated: the API server rejected it with "unexpected end of JSON input" and
// no deployment was ever restarted. This pins the patch as parseable JSON that
// actually carries the restartedAt annotation, and fails against the missing
// brace.
func TestRestartDeploymentSendsValidPatch(t *testing.T) {
	clientset := k8sfake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
	})

	var gotPatch []byte
	clientset.PrependReactor("patch", "deployments",
		func(action clienttesting.Action) (bool, runtime.Object, error) {
			gotPatch = action.(clienttesting.PatchAction).GetPatch()
			return false, nil, nil
		})

	c := NewClientForTest(clientset, dynfake.NewSimpleDynamicClient(runtime.NewScheme()), zap.NewNop(), false)

	if err := c.RestartDeployment(context.Background(), "default", "web"); err != nil {
		t.Fatalf("RestartDeployment: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(gotPatch, &decoded); err != nil {
		t.Fatalf("patch is not valid JSON (%v): %s", err, gotPatch)
	}

	ann := decoded["spec"].(map[string]any)["template"].(map[string]any)["metadata"].(map[string]any)["annotations"].(map[string]any)
	if _, ok := ann["kubectl.kubernetes.io/restartedAt"]; !ok {
		t.Fatalf("patch missing restartedAt annotation: %s", gotPatch)
	}
}
