package menus

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ksauraj/telectl/internal/k8s"
)

// Regression test for BUTTON_DATA_INVALID. Telegram rejects the *entire*
// keyboard when any callback_data exceeds 64 bytes, so a single long pod name
// made "Pods" render nothing at all. These are real names from the reporter's
// cluster.
func TestGeneratedButtonsRespectCallbackLimit(t *testing.T) {
	mb := &MenuBuilder{config: testConfig(), tokens: NewTokenStore(4096)}

	longNames := []string{
		"sample-service-api-5b7d9f4c82-klmno",
		"sample-controller-manager-worker-7d4b9c8f65-abcde",
		"sample-webhook-listener-6c8d5b7f4a-fghij",
		"sh.helm.release.v1.sample-service.v1",
		"extension-apiserver-authentication",
		strings.Repeat("a", 253), // max DNS subdomain length
	}

	resources := make([]k8s.ResourceInfo, 0, len(longNames))
	for _, n := range longNames {
		resources = append(resources, k8s.ResourceInfo{
			Name: n, Namespace: "kube-system", Kind: "Pod", Status: "Running",
		})
	}

	keyboards := map[string][]string{
		"resourceList": buttonData(mb.GetResourceListInlineKeyboard(
			"pods", resources, 0, 10, "kube-system")),
		"resourceActionPod": buttonData(mb.GetResourceActionInlineKeyboard(
			"pods", "kube-system", longNames[5], nil)),
		"resourceActionDeploy": buttonData(mb.GetResourceActionInlineKeyboard(
			"deployments", "kube-system", longNames[1], nil)),
		"confirmDelete": buttonData(mb.GetConfirmDeleteKeyboard(
			"secrets", "kube-system", longNames[3])),
		"scale":      buttonData(mb.GetScaleKeyboard("kube-system", longNames[1], 3)),
		"logOptions": buttonData(mb.GetLogOptionsKeyboard("kube-system", longNames[0], "manager")),
	}

	for name, datas := range keyboards {
		if len(datas) == 0 {
			t.Errorf("%s produced no buttons", name)
		}
		for _, d := range datas {
			if n := len(d); n > maxCallbackData {
				t.Errorf("%s: callback_data is %d bytes (limit %d): %q",
					name, n, maxCallbackData, d)
			}
		}
	}
}

// Shortened data must round-trip to exactly the original.
func TestTokenRoundTrip(t *testing.T) {
	s := NewTokenStore(64)
	original := "menu:resource:view:pods:kube-system:sample-controller-manager-worker-7d4b9c8f65-abcde"

	short := s.Shorten(original)
	if len(short) > maxCallbackData {
		t.Fatalf("shortened data still too long: %d bytes", len(short))
	}
	if !strings.HasPrefix(short, tokenPrefix) {
		t.Fatalf("expected token prefix, got %q", short)
	}

	got, ok := s.Resolve(short)
	if !ok {
		t.Fatal("Resolve reported unknown token for data it just minted")
	}
	if got != original {
		t.Errorf("round trip mismatch:\n got %q\nwant %q", got, original)
	}
}

// Short data must pass through untouched so logs stay readable.
func TestShortDataNotTokenised(t *testing.T) {
	s := NewTokenStore(64)
	for _, d := range []string{"menu:main", "menu:resource:types", "menu:ctx:switch:minikube"} {
		if got := s.Shorten(d); got != d {
			t.Errorf("Shorten(%q) = %q, want unchanged", d, got)
		}
		if got, ok := s.Resolve(d); !ok || got != d {
			t.Errorf("Resolve(%q) = (%q,%v), want (%q,true)", d, got, ok, d)
		}
	}
}

// Re-rendering the same list must not mint a new token each time.
func TestTokenReusedForIdenticalData(t *testing.T) {
	s := NewTokenStore(64)
	d := strings.Repeat("x", 100)

	first, second := s.Shorten(d), s.Shorten(d)
	if first != second {
		t.Errorf("same data minted two tokens: %q vs %q", first, second)
	}
	if len(s.byKey) != 1 {
		t.Errorf("expected 1 stored entry, got %d", len(s.byKey))
	}
}

// Distinct data must never collide onto one token.
func TestTokensDoNotCollide(t *testing.T) {
	s := NewTokenStore(8192)
	seen := map[string]string{}

	for i := 0; i < 2000; i++ {
		data := fmt.Sprintf("menu:resource:view:pods:kube-system:%s-%d",
			strings.Repeat("pod", 12), i)
		tok := s.Shorten(data)
		if prev, dup := seen[tok]; dup && prev != data {
			t.Fatalf("token %q maps to both %q and %q", tok, prev, data)
		}
		seen[tok] = data

		got, ok := s.Resolve(tok)
		if !ok || got != data {
			t.Fatalf("resolve failed for entry %d", i)
		}
	}
}

// An unknown token must be reported, not silently treated as valid data.
func TestUnknownTokenReported(t *testing.T) {
	s := NewTokenStore(64)
	if _, ok := s.Resolve(tokenPrefix + "deadbeef"); ok {
		t.Error("unknown token should report false so the caller can prompt a refresh")
	}
}

// The table must not grow without bound in a long-running bot.
func TestTokenStoreEvictsOldest(t *testing.T) {
	s := NewTokenStore(10)
	for i := 0; i < 50; i++ {
		s.Shorten(fmt.Sprintf("%s-%d", strings.Repeat("y", 80), i))
	}
	if len(s.byKey) > 10 {
		t.Errorf("store holds %d entries, limit was 10", len(s.byKey))
	}
	if len(s.byData) != len(s.byKey) {
		t.Errorf("index desync: byKey=%d byData=%d", len(s.byKey), len(s.byData))
	}
}

// Tokenised data must still parse to the same action as the original.
func TestTokenisedDataParsesIdentically(t *testing.T) {
	mb := &MenuBuilder{config: testConfig(), tokens: NewTokenStore(64)}
	full := "menu:resource:view:pods:kube-system:sample-service-api-5b7d9f4c82-klmno"

	b := mb.btn("x", full)
	resolved, ok := mb.ResolveCallback(b.CallbackData)
	if !ok {
		t.Fatal("could not resolve freshly minted token")
	}

	fromToken := ParseCallbackData(resolved)
	direct := ParseCallbackData(full)
	if fromToken == nil || direct == nil {
		t.Fatal("parse returned nil")
	}
	if *fromToken != *direct {
		t.Errorf("token path parsed differently:\n token %+v\ndirect %+v", fromToken, direct)
	}
}
