package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// The config search once matched ~/.kube/config and the compiled binary, both
// of which share the config base name. Viper's own error for those is opaque
// ("control characters are not allowed"), so these messages have to name the
// file and say what it looks like.
func TestDescribeConfigReadError(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	underlying := errors.New("yaml: control characters are not allowed")

	t.Run("kubeconfig is named as such", func(t *testing.T) {
		path := write("telectl.yaml", "apiVersion: v1\nkind: Config\nclusters: []\n")
		err := describeConfigReadError(path, underlying)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "looks like a kubeconfig") {
			t.Errorf("message does not identify a kubeconfig: %v", err)
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("message does not name the file: %v", err)
		}
	})

	// The compiled binary is called telectl with no extension, and sits in the
	// repo root after `make build`.
	t.Run("extensionless file is refused", func(t *testing.T) {
		path := write("telectl", "\x7fELF binary-ish content")
		err := describeConfigReadError(path, underlying)
		if err == nil || !strings.Contains(err.Error(), "not a recognized config file") {
			t.Errorf("expected a not-a-config-file error, got %v", err)
		}
	})

	// A real config that merely has a syntax error must report the parse error,
	// not be misdiagnosed.
	t.Run("genuine parse error passes through", func(t *testing.T) {
		path := write("broken.yaml", "telegram:\n  bot_token: [unclosed\n")
		err := describeConfigReadError(path, underlying)
		if err == nil {
			t.Fatal("expected an error")
		}
		if strings.Contains(err.Error(), "kubeconfig") ||
			strings.Contains(err.Error(), "not a recognized") {
			t.Errorf("genuine parse error was misdiagnosed: %v", err)
		}
		if !errors.Is(err, underlying) {
			t.Errorf("underlying error not wrapped: %v", err)
		}
	})

	// A kubeconfig-looking file that also has a telegram section is ours.
	t.Run("our config with apiVersion is not mistaken for kubeconfig", func(t *testing.T) {
		path := write("ours.yaml", "apiVersion: v1\ntelegram:\n  bot_token: x\n")
		err := describeConfigReadError(path, underlying)
		if err != nil && strings.Contains(err.Error(), "looks like a kubeconfig") {
			t.Errorf("our own config was refused as a kubeconfig: %v", err)
		}
	})

	t.Run("no file named", func(t *testing.T) {
		err := describeConfigReadError("", underlying)
		if err == nil || !errors.Is(err, underlying) {
			t.Errorf("expected the underlying error wrapped, got %v", err)
		}
	})

	t.Run("missing file falls back to the raw error", func(t *testing.T) {
		err := describeConfigReadError(filepath.Join(dir, "absent.yaml"), underlying)
		if err == nil || !errors.Is(err, underlying) {
			t.Errorf("expected the underlying error wrapped, got %v", err)
		}
	})
}

func TestIsKubeconfig(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	cases := []struct {
		name string
		body string
		want bool
	}{
		{"kubeconfig by apiVersion", "apiVersion: v1\nkind: Config\n", true},
		{"kubeconfig by cert data", "clusters:\n- cluster:\n    certificate-authority-data: abc\n", true},
		{"our config", "telegram:\n  bot_token: abc\n", false},
		{"our config with apiVersion", "apiVersion: v1\ntelegram:\n  bot_token: abc\n", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isKubeconfig(write(tc.name+".yaml", tc.body)); got != tc.want {
				t.Errorf("isKubeconfig = %v, want %v", got, tc.want)
			}
		})
	}

	if isKubeconfig(filepath.Join(dir, "does-not-exist.yaml")) {
		t.Error("a missing file must not be reported as a kubeconfig")
	}
}

// The allowlist decides who can operate the cluster, so its parsing gets
// direct coverage. Note the deliberate asymmetry with --allowed-users: the flag
// rejects a malformed ID, this skips it (see applyUserIDEnv).
func TestApplyUserIDEnv(t *testing.T) {
	const key = "telegram.allowed_user_ids"

	cases := []struct {
		name string
		env  string
		want []int64
	}{
		{"single", "123", []int64{123}},
		{"multiple", "123,456", []int64{123, 456}},
		{"spaces tolerated", " 123 , 456 ", []int64{123, 456}},
		{"empty entries skipped", "123,,456,", []int64{123, 456}},
		{"malformed entries skipped", "123,abc,456", []int64{123, 456}},
		{"negative ids kept", "-100123", []int64{-100123}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			t.Setenv("TEST_USER_IDS", tc.env)
			applyUserIDEnv("TEST_USER_IDS", key)

			got := viper.Get(key)
			ids, ok := got.([]int64)
			if !ok {
				t.Fatalf("stored value is %T, want []int64", got)
			}
			if len(ids) != len(tc.want) {
				t.Fatalf("got %v, want %v", ids, tc.want)
			}
			for i := range ids {
				if ids[i] != tc.want[i] {
					t.Errorf("id[%d] = %d, want %d", i, ids[i], tc.want[i])
				}
			}
		})
	}

	// An unset or all-garbage value must leave the key untouched rather than
	// storing an empty allowlist — which would read as "allow nobody" or, worse,
	// be mistaken for "allow everyone".
	t.Run("unset leaves key absent", func(t *testing.T) {
		viper.Reset()
		applyUserIDEnv("TEST_UNSET_IDS", key)
		if viper.Get(key) != nil {
			t.Errorf("unset env set the key to %v", viper.Get(key))
		}
	})

	t.Run("all-malformed leaves key absent", func(t *testing.T) {
		viper.Reset()
		t.Setenv("TEST_USER_IDS", "abc,def")
		applyUserIDEnv("TEST_USER_IDS", key)
		if viper.Get(key) != nil {
			t.Errorf("all-malformed env set the key to %v", viper.Get(key))
		}
	})
}
