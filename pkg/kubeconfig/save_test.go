package kubeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
)

// validKubeconfig is a standard v1 kubeconfig exercising the credential shapes
// that must survive a save: exec plugin, client cert data, and a bearer token.
const validKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: cluster-a
  cluster:
    server: https://a.example.com
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCg==
- name: cluster-b
  cluster:
    server: https://b.example.com
    insecure-skip-tls-verify: true
users:
- name: user-cert
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCg==
    client-key-data: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQo=
- name: user-token
  user:
    token: sha256~abcdefghijklmnop
- name: user-exec
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: kubelogin
      args:
      - get-token
      - --environment
      - AzurePublicCloud
      env:
      - name: SOME_VAR
        value: some-value
      installHint: install kubelogin
      interactiveMode: IfAvailable
contexts:
- name: ctx-a
  context:
    cluster: cluster-a
    user: user-cert
    namespace: alpha
- name: ctx-b
  context:
    cluster: cluster-b
    user: user-exec
    namespace: beta
- name: ctx-token
  context:
    cluster: cluster-a
    user: user-token
current-context: ctx-a
`

func writeKubeconfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

// Regression test for a bug that destroyed a real kubeconfig.
//
// Save() used yaml.Marshal(kc.Raw), where Raw is the *internal*
// clientcmdapi.Config. That emits lowercased Go field names, maps keyed by name
// instead of v1's lists, and the internal-only locationoforigin field. The
// result could not be read by kubectl or client-go:
//
//	json: cannot unmarshal object into Go struct field Config.clusters
//	of type []v1.NamedCluster
//
// Switching a context from the bot therefore corrupted the operator's
// kubeconfig on disk.
func TestSaveWritesLoadableV1Format(t *testing.T) {
	path := writeKubeconfig(t, validKubeconfig)

	kc, err := ParseKubeconfig(path)
	require.NoError(t, err)
	require.NoError(t, kc.SwitchContext("ctx-b"))

	// The saved file must be loadable by client-go, which is the same path
	// kubectl uses.
	reloaded, err := clientcmd.LoadFromFile(path)
	require.NoError(t, err, "saved kubeconfig is not loadable — this is the corruption bug")

	assert.Equal(t, "ctx-b", reloaded.CurrentContext)
	assert.Len(t, reloaded.Clusters, 2)
	assert.Len(t, reloaded.AuthInfos, 3)
	assert.Len(t, reloaded.Contexts, 3)
}

// The on-disk file must use v1 key names, not the internal Go field names.
func TestSaveUsesVersionedKeyNames(t *testing.T) {
	path := writeKubeconfig(t, validKubeconfig)

	kc, err := ParseKubeconfig(path)
	require.NoError(t, err)
	require.NoError(t, kc.SwitchContext("ctx-b"))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	got := string(raw)

	// Internal-form markers that must never appear.
	for _, bad := range []string{"authinfos:", "locationoforigin", "currentcontext:", "insecureskiptlsverify"} {
		assert.NotContains(t, got, bad,
			"internal Go field name leaked into the kubeconfig")
	}
	// v1 markers that must be present.
	for _, want := range []string{"apiVersion: v1", "kind: Config", "users:", "current-context:", "contexts:"} {
		assert.Contains(t, got, want)
	}
}

// Credentials must survive the round trip. A save that silently dropped exec
// blocks or cert data would lock the operator out of their clusters.
func TestSavePreservesCredentials(t *testing.T) {
	path := writeKubeconfig(t, validKubeconfig)

	kc, err := ParseKubeconfig(path)
	require.NoError(t, err)
	require.NoError(t, kc.SwitchContext("ctx-b"))

	reloaded, err := clientcmd.LoadFromFile(path)
	require.NoError(t, err)

	cert := reloaded.AuthInfos["user-cert"]
	require.NotNil(t, cert)
	assert.NotEmpty(t, cert.ClientCertificateData, "client cert data lost")
	assert.NotEmpty(t, cert.ClientKeyData, "client key data lost")

	tok := reloaded.AuthInfos["user-token"]
	require.NotNil(t, tok)
	assert.Equal(t, "sha256~abcdefghijklmnop", tok.Token, "bearer token lost")

	ex := reloaded.AuthInfos["user-exec"]
	require.NotNil(t, ex)
	require.NotNil(t, ex.Exec, "exec credential plugin lost")
	assert.Equal(t, "kubelogin", ex.Exec.Command)
	assert.Equal(t, []string{"get-token", "--environment", "AzurePublicCloud"}, ex.Exec.Args)
	require.Len(t, ex.Exec.Env, 1)
	assert.Equal(t, "SOME_VAR", ex.Exec.Env[0].Name)

	// Cluster CA and TLS settings too.
	a := reloaded.Clusters["cluster-a"]
	require.NotNil(t, a)
	assert.NotEmpty(t, a.CertificateAuthorityData, "cluster CA data lost")
	b := reloaded.Clusters["cluster-b"]
	require.NotNil(t, b)
	assert.True(t, b.InsecureSkipTLSVerify)

	// Per-context namespaces must not be dropped.
	assert.Equal(t, "alpha", reloaded.Contexts["ctx-a"].Namespace)
	assert.Equal(t, "beta", reloaded.Contexts["ctx-b"].Namespace)
}

// Saving repeatedly must converge, not accumulate drift.
func TestSaveIsIdempotent(t *testing.T) {
	path := writeKubeconfig(t, validKubeconfig)

	kc, err := ParseKubeconfig(path)
	require.NoError(t, err)
	require.NoError(t, kc.SwitchContext("ctx-b"))
	first, err := os.ReadFile(path)
	require.NoError(t, err)

	kc2, err := ParseKubeconfig(path)
	require.NoError(t, err)
	require.NoError(t, kc2.SwitchContext("ctx-b"))
	second, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, string(first), string(second), "repeated saves drift")

	// And the file must not balloon: the internal form duplicated
	// locationoforigin and every empty field on each entry.
	assert.Less(t, len(second), len(validKubeconfig)*4,
		"saved file is suspiciously large; internal fields may be leaking")
}

// Save must refuse to write when there is nothing loaded, rather than replacing
// a populated kubeconfig with an empty one.
func TestSaveRefusesEmptyConfig(t *testing.T) {
	kc := &KubeConfig{ConfigFile: filepath.Join(t.TempDir(), "config")}
	err := kc.Save()
	require.Error(t, err, "Save must refuse when Raw is nil")
	assert.NoFileExists(t, kc.ConfigFile)
}

// A switch to an unknown context must not touch the file at all.
func TestSwitchContextUnknownLeavesFileIntact(t *testing.T) {
	path := writeKubeconfig(t, validKubeconfig)
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	kc, err := ParseKubeconfig(path)
	require.NoError(t, err)

	require.Error(t, kc.SwitchContext("does-not-exist"))

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "failed switch modified the file")
}

// The saved file must remain readable by a fresh parse through our own loader.
func TestSaveRoundTripsThroughParse(t *testing.T) {
	path := writeKubeconfig(t, validKubeconfig)

	kc, err := ParseKubeconfig(path)
	require.NoError(t, err)
	require.NoError(t, kc.SwitchContext("ctx-token"))

	again, err := ParseKubeconfig(path)
	require.NoError(t, err, "our own parser cannot read what we just saved")
	assert.Equal(t, "ctx-token", again.CurrentContext)
	assert.Len(t, again.Contexts, 3)

	var names []string
	for _, c := range again.Contexts {
		names = append(names, c.Name)
	}
	assert.ElementsMatch(t, []string{"ctx-a", "ctx-b", "ctx-token"}, names)
}

// Permissions must stay owner-only: a kubeconfig holds credentials.
func TestSavePreservesRestrictivePermissions(t *testing.T) {
	path := writeKubeconfig(t, validKubeconfig)

	kc, err := ParseKubeconfig(path)
	require.NoError(t, err)
	require.NoError(t, kc.SwitchContext("ctx-b"))

	info, err := os.Stat(path)
	require.NoError(t, err)
	mode := info.Mode().Perm()
	assert.Zero(t, mode&0o077,
		"kubeconfig is group/world accessible after save (mode %o)", mode)
}

// Guard the exact failure the operator hit: the internal form must never be
// something our own Save produces.
func TestSavedFileIsNotInternalForm(t *testing.T) {
	path := writeKubeconfig(t, validKubeconfig)

	kc, err := ParseKubeconfig(path)
	require.NoError(t, err)
	require.NoError(t, kc.SwitchContext("ctx-b"))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	// In the internal form, "clusters:" is followed by a name-keyed mapping
	// rather than a "- name:" list entry.
	idx := strings.Index(string(raw), "clusters:")
	require.GreaterOrEqual(t, idx, 0)
	after := string(raw)[idx:]
	firstEntry := strings.SplitN(after, "\n", 3)
	require.GreaterOrEqual(t, len(firstEntry), 2)
	assert.True(t, strings.HasPrefix(strings.TrimSpace(firstEntry[1]), "- "),
		"clusters should be a YAML list in v1 format, got %q", firstEntry[1])
}
