package kubeconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// ContextInfo represents a kubeconfig context with full details.
type ContextInfo struct {
	Name          string    `json:"name" yaml:"name"`
	Cluster       string    `json:"cluster" yaml:"cluster"`
	ClusterServer string    `json:"cluster_server" yaml:"cluster_server"`
	User          string    `json:"user" yaml:"user"`
	Namespace     string    `json:"namespace" yaml:"namespace"`
	Current       bool      `json:"current" yaml:"current"`
	AuthInfo      *AuthInfo `json:"auth_info,omitempty" yaml:"auth_info,omitempty"`
}

// AuthInfo represents authentication information for a user.
type AuthInfo struct {
	Name           string   `json:"name,omitempty" yaml:"name,omitempty"`
	Username       string   `json:"username,omitempty" yaml:"username,omitempty"`
	Password       string   `json:"password,omitempty" yaml:"password,omitempty"`
	Token          string   `json:"token,omitempty" yaml:"token,omitempty"`
	ClientCert     string   `json:"client_cert,omitempty" yaml:"client_cert,omitempty"`
	ClientKey      string   `json:"client_key,omitempty" yaml:"client_key,omitempty"`
	ClientCertData string   `json:"client_cert_data,omitempty" yaml:"client_cert_data,omitempty"`
	ClientKeyData  string   `json:"client_key_data,omitempty" yaml:"client_key_data,omitempty"`
	ExecCommand    string   `json:"exec_command,omitempty" yaml:"exec_command,omitempty"`
	ExecArgs       []string `json:"exec_args,omitempty" yaml:"exec_args,omitempty"`
	ExecEnv        []string `json:"exec_env,omitempty" yaml:"exec_env,omitempty"`
}

// ClusterInfo represents cluster information.
type ClusterInfo struct {
	Name                 string `json:"name" yaml:"name"`
	Server               string `json:"server" yaml:"server"`
	CertificateAuthority string `json:"certificate_authority,omitempty" yaml:"certificate_authority,omitempty"`
	//nolint:lll // struct tags cannot be wrapped
	CertificateAuthorityData string `json:"certificate_authority_data,omitempty" yaml:"certificate_authority_data,omitempty"`
	InsecureSkipTLSVerify    bool   `json:"insecure_skip_tls_verify" yaml:"insecure_skip_tls_verify"`
}

// KubeConfig represents the full parsed kubeconfig.
type KubeConfig struct {
	Contexts       []ContextInfo        `json:"contexts" yaml:"contexts"`
	Clusters       []ClusterInfo        `json:"clusters" yaml:"clusters"`
	Users          []AuthInfo           `json:"users" yaml:"users"`
	CurrentContext string               `json:"current_context" yaml:"current_context"`
	ConfigFile     string               `json:"config_file" yaml:"config_file"`
	Raw            *clientcmdapi.Config `json:"-" yaml:"-"`
}

// ParseKubeconfig parses a kubeconfig file and returns structured data.
func ParseKubeconfig(configPath string) (*KubeConfig, error) {
	if configPath == "" {
		configPath = os.Getenv("KUBECONFIG")
	}
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home dir: %w", err)
		}
		configPath = filepath.Join(home, ".kube", "config")
	}

	configPath = expandPath(configPath)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("kubeconfig not found at: %s", configPath)
	}

	// Load using client-go's parser for full compatibility
	loadingRules := &clientcmd.ClientConfigLoadingRules{
		ExplicitPath: configPath,
	}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	rawConfig, err := loader.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	kubeConfig := &KubeConfig{
		ConfigFile:     configPath,
		CurrentContext: rawConfig.CurrentContext,
		Raw:            &rawConfig,
	}

	// Parse contexts
	for name, ctx := range rawConfig.Contexts {
		clusterInfo := rawConfig.Clusters[ctx.Cluster]
		authInfo := rawConfig.AuthInfos[ctx.AuthInfo]

		ci := ContextInfo{
			Name:          name,
			Cluster:       ctx.Cluster,
			ClusterServer: clusterInfo.Server,
			User:          ctx.AuthInfo,
			Namespace:     ctx.Namespace,
			Current:       name == rawConfig.CurrentContext,
		}

		if authInfo != nil {
			ci.AuthInfo = &AuthInfo{
				Username:       authInfo.Username,
				Password:       authInfo.Password,
				Token:          authInfo.Token,
				ClientCert:     authInfo.ClientCertificate,
				ClientKey:      authInfo.ClientKey,
				ClientCertData: string(authInfo.ClientCertificateData),
				ClientKeyData:  string(authInfo.ClientKeyData),
			}
			if authInfo.Exec != nil {
				ci.AuthInfo.ExecCommand = authInfo.Exec.Command
				ci.AuthInfo.ExecArgs = authInfo.Exec.Args
				ci.AuthInfo.ExecEnv = execEnvToStrings(authInfo.Exec.Env)
			}
		}

		kubeConfig.Contexts = append(kubeConfig.Contexts, ci)
	}

	// Parse clusters
	for name, cluster := range rawConfig.Clusters {
		kubeConfig.Clusters = append(kubeConfig.Clusters, ClusterInfo{
			Name:                     name,
			Server:                   cluster.Server,
			CertificateAuthority:     cluster.CertificateAuthority,
			CertificateAuthorityData: string(cluster.CertificateAuthorityData),
			InsecureSkipTLSVerify:    cluster.InsecureSkipTLSVerify,
		})
	}

	// Parse users
	for _, user := range rawConfig.AuthInfos {
		authInfo := AuthInfo{
			Username:       user.Username,
			Password:       user.Password,
			Token:          user.Token,
			ClientCert:     user.ClientCertificate,
			ClientKey:      user.ClientKey,
			ClientCertData: string(user.ClientCertificateData),
			ClientKeyData:  string(user.ClientKeyData),
		}
		if user.Exec != nil {
			authInfo.ExecCommand = user.Exec.Command
			authInfo.ExecArgs = user.Exec.Args
			authInfo.ExecEnv = execEnvToStrings(user.Exec.Env)
		}
		kubeConfig.Users = append(kubeConfig.Users, authInfo)
	}

	return kubeConfig, nil
}

// GetCurrentContext returns the current context.
func (kc *KubeConfig) GetCurrentContext() *ContextInfo {
	for i := range kc.Contexts {
		if kc.Contexts[i].Current {
			return &kc.Contexts[i]
		}
	}
	return nil
}

// GetContextByName returns a context by name.
func (kc *KubeConfig) GetContextByName(name string) *ContextInfo {
	for i := range kc.Contexts {
		if kc.Contexts[i].Name == name {
			return &kc.Contexts[i]
		}
	}
	return nil
}

// SwitchContext switches the current context in the kubeconfig file.
func (kc *KubeConfig) SwitchContext(contextName string) error {
	ctx := kc.GetContextByName(contextName)
	if ctx == nil {
		return fmt.Errorf("context not found: %s", contextName)
	}

	kc.Raw.CurrentContext = contextName
	kc.CurrentContext = contextName

	// Update current flags
	for i := range kc.Contexts {
		kc.Contexts[i].Current = kc.Contexts[i].Name == contextName
	}

	return kc.Save()
}

// AddContext adds a new context to the kubeconfig.
func (kc *KubeConfig) AddContext(name, cluster, user, namespace string) error {
	if kc.GetContextByName(name) != nil {
		return fmt.Errorf("context already exists: %s", name)
	}

	// Verify cluster and user exist
	clusterExists := false
	for _, c := range kc.Clusters {
		if c.Name == cluster {
			clusterExists = true
			break
		}
	}
	if !clusterExists {
		return fmt.Errorf("cluster not found: %s", cluster)
	}

	userExists := false
	for _, u := range kc.Users {
		if u.Username == user || u.Token != "" || u.ClientCert != "" {
			userExists = true
			break
		}
	}
	if !userExists {
		return fmt.Errorf("user not found: %s", user)
	}

	newCtx := clientcmdapi.Context{
		Cluster:  cluster,
		AuthInfo: user,
	}
	if namespace != "" {
		newCtx.Namespace = namespace
	}

	kc.Raw.Contexts[name] = &newCtx
	kc.Contexts = append(kc.Contexts, ContextInfo{
		Name:      name,
		Cluster:   cluster,
		User:      user,
		Namespace: namespace,
		Current:   false,
	})

	return kc.Save()
}

// DeleteContext deletes a context from the kubeconfig.
func (kc *KubeConfig) DeleteContext(name string) error {
	ctx := kc.GetContextByName(name)
	if ctx == nil {
		return fmt.Errorf("context not found: %s", name)
	}

	delete(kc.Raw.Contexts, name)

	newContexts := make([]ContextInfo, 0, len(kc.Contexts)-1)
	for _, c := range kc.Contexts {
		if c.Name != name {
			newContexts = append(newContexts, c)
		}
	}
	kc.Contexts = newContexts

	if kc.CurrentContext == name {
		if len(kc.Contexts) > 0 {
			kc.Raw.CurrentContext = kc.Contexts[0].Name
			kc.CurrentContext = kc.Contexts[0].Name
			kc.Contexts[0].Current = true
		} else {
			kc.Raw.CurrentContext = ""
			kc.CurrentContext = ""
		}
	}

	return kc.Save()
}

// Save writes the kubeconfig back to disk in the standard versioned v1 format.
//
// This must go through clientcmd.WriteToFile, never yaml.Marshal(kc.Raw).
// kc.Raw is a *clientcmdapi.Config — the *internal* representation — and
// marshalling it directly emits lowercased Go field names ("authinfos" instead
// of "users"), maps keyed by name where v1 requires lists of {name, cluster},
// and the internal-only "locationoforigin" field. The result is a file that no
// Kubernetes tool can read, including kubectl:
//
//	json: cannot unmarshal object into Go struct field Config.clusters
//	of type []v1.NamedCluster
//
// clientcmd.WriteToFile performs the internal -> v1 conversion. It also writes
// atomically, so an interrupted save cannot truncate an existing kubeconfig.
func (kc *KubeConfig) Save() error {
	if kc.Raw == nil {
		return fmt.Errorf("kubeconfig not loaded; refusing to write")
	}
	// Guard against writing an empty config over a populated file.
	if len(kc.Raw.Clusters) == 0 && len(kc.Raw.Contexts) == 0 {
		return fmt.Errorf("refusing to write a kubeconfig with no clusters or contexts")
	}
	if err := clientcmd.WriteToFile(*kc.Raw, kc.ConfigFile); err != nil {
		return fmt.Errorf("failed to write kubeconfig %q: %w", kc.ConfigFile, err)
	}
	return nil
}

// Validate validates the kubeconfig structure.
func (kc *KubeConfig) Validate() error {
	if len(kc.Contexts) == 0 {
		return fmt.Errorf("no contexts found in kubeconfig")
	}

	if kc.CurrentContext == "" {
		return fmt.Errorf("no current context set")
	}

	current := kc.GetCurrentContext()
	if current == nil {
		return fmt.Errorf("current context not found in contexts list")
	}

	return nil
}

// GetContextForCluster returns all contexts for a given cluster.
func (kc *KubeConfig) GetContextsForCluster(clusterName string) []ContextInfo {
	var result []ContextInfo
	for _, ctx := range kc.Contexts {
		if ctx.Cluster == clusterName {
			result = append(result, ctx)
		}
	}
	return result
}

// GetContextsForUser returns all contexts for a given user.
func (kc *KubeConfig) GetContextsForUser(userName string) []ContextInfo {
	var result []ContextInfo
	for _, ctx := range kc.Contexts {
		if ctx.User == userName {
			result = append(result, ctx)
		}
	}
	return result
}

func expandPath(path string) string {
	if len(path) > 1 && path[0] == '~' && (path[1] == '/' || path[1] == '\\') {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func execEnvToStrings(env []clientcmdapi.ExecEnvVar) []string {
	result := make([]string, len(env))
	for i, v := range env {
		result[i] = v.Name + "=" + v.Value
	}
	return result
}

// ToYAML returns the kubeconfig as YAML string.
func (kc *KubeConfig) ToYAML() (string, error) {
	data, err := yaml.Marshal(kc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
