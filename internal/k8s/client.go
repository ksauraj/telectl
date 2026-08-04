package k8s

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ksauraj/telectl/pkg/kubeconfig"
	"go.uber.org/zap"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
)

// The GroupVersionResources this client works with. Declared once rather than
// built inline at each call site: an inline literal with a typo'd group or
// version fails only at runtime, as a "server could not find the requested
// resource" that reads like a cluster problem rather than a code bug.
var (
	podsGVR        = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	servicesGVR    = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
	namespacesGVR  = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	nodesGVR       = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}
	configMapsGVR  = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	secretsGVR     = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	pvcsGVR        = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}
	pvsGVR         = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumes"}
	eventsGVR      = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "events"}
	endpointsGVR   = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "endpoints"}
	deploymentsGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	replicaSetsGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	ingressesGVR   = schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}

	// Served by metrics-server, which is optional; callers must handle a
	// NotFound from these as "metrics unavailable", not as a failure.
	nodeMetricsGVR = schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}
)

type Client struct {
	// clientset is the interface, not *kubernetes.Clientset, so tests can
	// inject a fake and exercise the typed verbs (cordon, drain, scale)
	// without a cluster.
	clientset     kubernetes.Interface
	dynamicClient dynamic.Interface
	restConfig    *rest.Config
	kubeconfig    *kubeconfig.KubeConfig
	logger        *zap.Logger
	dryRun        bool
	// currentContext tracks the context this client is using. It can diverge
	// from the kubeconfig's current-context, because switching in-process
	// deliberately does not write to disk.
	currentContext string
}

type PodLogOptions struct {
	Namespace    string
	PodName      string
	Container    string
	Follow       bool
	Previous     bool
	Timestamps   bool
	TailLines    *int64
	LimitBytes   *int64
	SinceSeconds *int64
	SinceTime    *metav1.Time
}

type ExecOptions struct {
	Namespace string
	PodName   string
	Container string
	Command   []string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	TTY       bool
}

type PortForwardOptions struct {
	Namespace string
	PodName   string
	Ports     []string
	Addresses []string
	StopChan  chan struct{}
	ReadyChan chan struct{}
	Stdout    io.Writer
	Stderr    io.Writer
}

type ResourceInfo struct {
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	Kind        string                 `json:"kind"`
	APIVersion  string                 `json:"api_version"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
	CreatedAt   metav1.Time            `json:"created_at"`
	Status      string                 `json:"status"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// loadingRulesFor builds kubeconfig loading rules for an optional explicit path.
//
// Setting ExplicitPath to "" does not mean "use the default" — it disables the
// search entirely and yields "no configuration has been provided". Callers that
// skip config.ValidateConfig (which fills the default path) therefore failed to
// find ~/.kube/config at all. Fall back to clientcmd's standard rules, which
// also honour $KUBECONFIG.
func loadingRulesFor(kubeConfigPath string) clientcmd.ClientConfigLoader {
	if kubeConfigPath != "" {
		return &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath}
	}
	return clientcmd.NewDefaultClientConfigLoadingRules()
}

func NewClient(kubeConfigPath, context string, dryRun bool, logger *zap.Logger) (*Client, error) {
	loadingRules := loadingRulesFor(kubeConfigPath)
	overrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		overrides.CurrentContext = context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create rest config: %w", err)
	}

	// Set reasonable defaults
	restConfig.Burst = 10
	restConfig.QPS = 5.0
	restConfig.Timeout = 30 * time.Second

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Parse kubeconfig for context info
	kc, err := kubeconfig.ParseKubeconfig(kubeConfigPath)
	if err != nil {
		logger.Warn("Failed to parse kubeconfig for context info", zap.Error(err))
		kc = &kubeconfig.KubeConfig{ConfigFile: kubeConfigPath}
	}

	// Seed currentContext so CurrentContextName is accurate before any switch.
	// The explicit --context flag wins; otherwise it is the kubeconfig's own
	// current-context.
	active := context
	if active == "" {
		if cur := kc.GetCurrentContext(); cur != nil {
			active = cur.Name
		}
	}

	return &Client{
		clientset:      clientset,
		dynamicClient:  dynamicClient,
		restConfig:     restConfig,
		kubeconfig:     kc,
		logger:         logger,
		dryRun:         dryRun,
		currentContext: active,
	}, nil
}

func (c *Client) ListContexts(ctx context.Context) ([]kubeconfig.ContextInfo, error) {
	if c.kubeconfig == nil {
		return nil, fmt.Errorf("kubeconfig not loaded")
	}
	return c.kubeconfig.Contexts, nil
}

func (c *Client) GetCurrentContext() *kubeconfig.ContextInfo {
	if c.kubeconfig == nil {
		return nil
	}
	return c.kubeconfig.GetCurrentContext()
}

// SwitchContext points this client at a different kubeconfig context.
//
// Two things this must get right:
//
//  1. The clientset/dynamicClient are built once in NewClient from a single
//     rest.Config. Changing the context without rebuilding them left the bot
//     talking to the *previous* cluster while reporting success, until restart.
//  2. Writing the change back to ~/.kube/config is a global side effect on the
//     host, triggered by a chat message: it changes the active context for every
//     other user of the bot and for any kubectl on that machine. It is therefore
//     opt-in via persist, and off for the menu/command paths.
func (c *Client) SwitchContext(contextName string) error {
	if c.kubeconfig == nil {
		return fmt.Errorf("kubeconfig not loaded")
	}
	if c.kubeconfig.GetContextByName(contextName) == nil {
		return fmt.Errorf("context not found: %s", contextName)
	}
	return c.useContext(contextName)
}

// useContext rebuilds the API clients against contextName without touching disk.
func (c *Client) useContext(contextName string) error {
	loadingRules := &clientcmd.ClientConfigLoadingRules{
		ExplicitPath: c.kubeconfig.ConfigFile,
	}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}

	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, overrides).ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to build rest config for context %q: %w", contextName, err)
	}
	restConfig.Burst = c.restConfig.Burst
	restConfig.QPS = c.restConfig.QPS
	restConfig.Timeout = c.restConfig.Timeout

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create clientset for context %q: %w", contextName, err)
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client for context %q: %w", contextName, err)
	}

	// Swap in only after every step succeeded, so a failed switch leaves the
	// client usable against the previous context.
	c.restConfig = restConfig
	c.clientset = clientset
	c.dynamicClient = dynamicClient
	c.currentContext = contextName

	c.logger.Info("Switched Kubernetes context",
		zap.String("context", contextName),
		zap.String("server", restConfig.Host))
	return nil
}

// SwitchContextPersistent switches context and writes the choice back to the
// kubeconfig file. Only for explicit administrative use: this mutates state
// shared with kubectl and with every other user of this bot.
func (c *Client) SwitchContextPersistent(contextName string) error {
	if err := c.SwitchContext(contextName); err != nil {
		return err
	}
	return c.kubeconfig.SwitchContext(contextName)
}

// CurrentContextName returns the context this client is currently using.
func (c *Client) CurrentContextName() string {
	if c.currentContext != "" {
		return c.currentContext
	}
	if cur := c.GetCurrentContext(); cur != nil {
		return cur.Name
	}
	return ""
}

func (c *Client) GetRESTConfig() *rest.Config {
	return c.restConfig
}

func (c *Client) GetClientset() kubernetes.Interface {
	return c.clientset
}

// ===== Generic Resource Operations =====

func (c *Client) ListResources(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace,
	labelSelector,
	fieldSelector string) ([]ResourceInfo,
	error,
) {
	var ri dynamic.ResourceInterface
	if namespace != "" {
		ri = c.dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		ri = c.dynamicClient.Resource(gvr)
	}

	list, err := ri.List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	resources := make([]ResourceInfo, 0, len(list.Items))
	for _, item := range list.Items {
		resources = append(resources, c.unstructuredToResourceInfo(&item))
	}
	return resources, nil
}

func (c *Client) GetResource(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace,
	name string,
) (*ResourceInfo, error) {
	var ri dynamic.ResourceInterface
	if namespace != "" {
		ri = c.dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		ri = c.dynamicClient.Resource(gvr)
	}

	item, err := ri.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	info := c.unstructuredToResourceInfo(item)
	return &info, nil
}

func (c *Client) DeleteResource(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace,
	name string,
	options *metav1.DeleteOptions,
) error {
	var ri dynamic.ResourceInterface
	if namespace != "" {
		ri = c.dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		ri = c.dynamicClient.Resource(gvr)
	}

	if c.dryRun {
		c.logger.Info("DRY RUN: Would delete resource", zap.String("resource", name), zap.String("namespace", namespace))
		return nil
	}

	return ri.Delete(ctx, name, *options)
}

func (c *Client) unstructuredToResourceInfo(u *unstructured.Unstructured) ResourceInfo {
	labels := u.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	annotations := u.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Extract status
	status := "Unknown"
	if s, found, _ := unstructured.NestedString(u.Object, "status", "phase"); found {
		status = s
	} else if conditions, found, _ := unstructured.NestedSlice(
		u.Object, "status", "conditions"); found && len(conditions) > 0 {
		if cond, ok := conditions[0].(map[string]interface{}); ok {
			if t, ok := cond["type"].(string); ok {
				status = t
			}
		}
	}

	return ResourceInfo{
		Name:        u.GetName(),
		Namespace:   u.GetNamespace(),
		Kind:        u.GetKind(),
		APIVersion:  u.GetAPIVersion(),
		Labels:      labels,
		Annotations: annotations,
		CreatedAt:   u.GetCreationTimestamp(),
		Status:      status,
		Details:     u.Object,
	}
}

// ===== Pod Operations =====

func (c *Client) ListPods(ctx context.Context, namespace, labelSelector, fieldSelector string) ([]ResourceInfo, error) {
	return c.ListResources(ctx, podsGVR, namespace, labelSelector, fieldSelector)
}

func (c *Client) GetPod(ctx context.Context, namespace, name string) (*ResourceInfo, error) {
	return c.GetResource(ctx, podsGVR, namespace, name)
}

func (c *Client) GetPodLogs(ctx context.Context, opts PodLogOptions) (io.ReadCloser, error) {
	req := c.clientset.CoreV1().Pods(opts.Namespace).GetLogs(opts.PodName, &corev1.PodLogOptions{
		Container:    opts.Container,
		Follow:       opts.Follow,
		Previous:     opts.Previous,
		Timestamps:   opts.Timestamps,
		TailLines:    opts.TailLines,
		LimitBytes:   opts.LimitBytes,
		SinceSeconds: opts.SinceSeconds,
		SinceTime:    opts.SinceTime,
	})

	return req.Stream(ctx)
}

func (c *Client) ExecInPod(ctx context.Context, opts ExecOptions) error {
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(opts.PodName).
		Namespace(opts.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: opts.Container,
			Command:   opts.Command,
			Stdin:     opts.Stdin != nil,
			Stdout:    opts.Stdout != nil,
			Stderr:    opts.Stderr != nil,
			TTY:       opts.TTY,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	// StreamWithContext, not Stream: the deprecated form ignores ctx, so a
	// cancelled or timed-out exec left the SPDY connection and its goroutines
	// alive until the remote command happened to exit on its own.
	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  opts.Stdin,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
		Tty:    opts.TTY,
	})
}

func (c *Client) PortForward(ctx context.Context, opts PortForwardOptions) error {
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(opts.PodName).
		Namespace(opts.Namespace).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(c.restConfig)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	fw, err := portforward.New(dialer, opts.Ports, opts.StopChan, opts.ReadyChan, opts.Stdout, opts.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create port forwarder: %w", err)
	}

	return fw.ForwardPorts()
}

// ===== Deployment Operations =====

func (c *Client) ListDeployments(ctx context.Context, namespace, labelSelector string) ([]ResourceInfo, error) {
	return c.ListResources(ctx, deploymentsGVR, namespace, labelSelector, "")
}

func (c *Client) GetDeployment(ctx context.Context, namespace, name string) (*ResourceInfo, error) {
	return c.GetResource(ctx, deploymentsGVR, namespace, name)
}

func (c *Client) ScaleDeployment(ctx context.Context, namespace, name string, replicas int32) error {
	if c.dryRun {
		c.logger.Info("DRY RUN: Would scale deployment", zap.String("deployment", name), zap.Int32("replicas", replicas))
		return nil
	}

	scale, err := c.clientset.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment scale: %w", err)
	}

	replicasCopy := replicas
	scale.Spec.Replicas = replicasCopy
	_, err = c.clientset.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	return err
}

func (c *Client) RestartDeployment(ctx context.Context, namespace, name string) error {
	if c.dryRun {
		c.logger.Info("DRY RUN: Would restart deployment", zap.String("deployment", name))
		return nil
	}

	// The same annotation kubectl rollout restart sets: changing the pod
	// template is what makes the Deployment controller start a new rollout.
	patch := []byte(`{"spec":{"template":{"metadata":{"annotations":` +
		`{"kubectl.kubernetes.io/restartedAt":"` +
		time.Now().Format(time.RFC3339) + `"}}}}}`)
	_, err := c.clientset.AppsV1().Deployments(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	return err
}

// ===== Service Operations =====

func (c *Client) ListServices(ctx context.Context, namespace, labelSelector string) ([]ResourceInfo, error) {
	return c.ListResources(ctx, servicesGVR, namespace, labelSelector, "")
}

func (c *Client) GetService(ctx context.Context, namespace, name string) (*ResourceInfo, error) {
	return c.GetResource(ctx, servicesGVR, namespace, name)
}

// ===== ReplicaSet Operations =====

func (c *Client) ListReplicaSets(ctx context.Context, namespace, labelSelector string) ([]ResourceInfo, error) {
	return c.ListResources(ctx, replicaSetsGVR, namespace, labelSelector, "")
}

func (c *Client) GetReplicaSet(ctx context.Context, namespace, name string) (*ResourceInfo, error) {
	return c.GetResource(ctx, replicaSetsGVR, namespace, name)
}

// ===== Namespace Operations =====

func (c *Client) ListNamespaces(ctx context.Context, labelSelector string) ([]ResourceInfo, error) {
	return c.ListResources(ctx, namespacesGVR, "", labelSelector, "")
}

func (c *Client) GetNamespace(ctx context.Context, name string) (*ResourceInfo, error) {
	return c.GetResource(ctx, namespacesGVR, "", name)
}

func (c *Client) CreateNamespace(ctx context.Context, name string, labels map[string]string) error {
	if c.dryRun {
		c.logger.Info("DRY RUN: Would create namespace", zap.String("namespace", name))
		return nil
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
	_, err := c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	if c.dryRun {
		c.logger.Info("DRY RUN: Would delete namespace", zap.String("namespace", name))
		return nil
	}
	return c.clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
}

// ===== Node Operations =====

func (c *Client) ListNodes(ctx context.Context, labelSelector string) ([]ResourceInfo, error) {
	return c.ListResources(ctx, nodesGVR, "", labelSelector, "")
}

func (c *Client) GetNode(ctx context.Context, name string) (*ResourceInfo, error) {
	return c.GetResource(ctx, nodesGVR, "", name)
}

func (c *Client) GetNodeMetrics(ctx context.Context) ([]ResourceInfo, error) {
	// This would require metrics-server
	return c.ListResources(ctx, nodeMetricsGVR, "", "", "")
}

// ===== ConfigMap & Secret Operations =====

func (c *Client) ListConfigMaps(ctx context.Context, namespace, labelSelector string) ([]ResourceInfo, error) {
	return c.ListResources(ctx, configMapsGVR, namespace, labelSelector, "")
}

func (c *Client) ListSecrets(ctx context.Context, namespace, labelSelector string) ([]ResourceInfo, error) {
	return c.ListResources(ctx, secretsGVR, namespace, labelSelector, "")
}

// ===== PVC/PV Operations =====

func (c *Client) ListPVCs(ctx context.Context, namespace, labelSelector string) ([]ResourceInfo, error) {
	return c.ListResources(ctx, pvcsGVR, namespace, labelSelector, "")
}

func (c *Client) ListPVs(ctx context.Context, labelSelector string) ([]ResourceInfo, error) {
	return c.ListResources(ctx, pvsGVR, "", labelSelector, "")
}

// ===== Ingress Operations =====

func (c *Client) ListIngresses(ctx context.Context, namespace, labelSelector string) ([]ResourceInfo, error) {
	return c.ListResources(ctx, ingressesGVR, namespace, labelSelector, "")
}

// ===== Events =====

func (c *Client) GetEvents(ctx context.Context, namespace, fieldSelector string) ([]ResourceInfo, error) {
	return c.ListResources(ctx, eventsGVR, namespace, "", fieldSelector)
}

// ===== Watch Operations =====

func (c *Client) WatchResources(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace string,
	opts metav1.ListOptions) (watch.Interface,
	error,
) {
	var ri dynamic.ResourceInterface
	if namespace != "" {
		ri = c.dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		ri = c.dynamicClient.Resource(gvr)
	}
	return ri.Watch(ctx, opts)
}

// ===== Utility Functions =====

func (c *Client) GetKubeconfig() *kubeconfig.KubeConfig {
	return c.kubeconfig
}

func (c *Client) IsDryRun() bool {
	return c.dryRun
}

func (c *Client) GetServerVersion(ctx context.Context) (string, error) {
	version, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return version.String(), nil
}

func (c *Client) GetAPIResources(ctx context.Context) ([]*metav1.APIResourceList, error) {
	return c.clientset.Discovery().ServerPreferredResources()
}

func (c *Client) CheckPermission(ctx context.Context, verb, resource, namespace string) (bool, error) {
	sar := &authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:      verb,
				Resource:  resource,
				Namespace: namespace,
			},
		},
	}
	result, err := c.clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	return result.Status.Allowed, nil
}

// NewClientForTest builds a Client around injected API clients.
//
// The production constructor needs a reachable cluster and a kubeconfig on
// disk, which makes the menu verbs (cordon, drain, scale, rollout history)
// untestable. Passing fakes here exercises the same code paths the bot uses.
func NewClientForTest(
	clientset kubernetes.Interface,
	dynamicClient dynamic.Interface,
	logger *zap.Logger,
	dryRun bool,
) *Client {
	return &Client{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		restConfig:    &rest.Config{Host: "https://test.invalid"},
		kubeconfig:    &kubeconfig.KubeConfig{},
		logger:        logger,
		dryRun:        dryRun,
	}
}
