package menus

import (
	"fmt"
	"strings"
	"testing"

	f "github.com/ksauraj/telectl/internal/utils/formatters"
)

func TestNamespaceKeyboardMarksCurrent(t *testing.T) {
	mb := &MenuBuilder{config: testConfig(), tokens: NewTokenStore(4096)}
	kb := mb.GetNamespaceInlineKeyboard(
		[]string{"default", "kube-system", "prod"}, "kube-system", 0, "menu:settings:home")

	var labels []string
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			labels = append(labels, b.Text)
		}
	}
	joined := strings.Join(labels, " | ")

	if !strings.Contains(joined, f.Btn(f.GlyphSelected, "kube-system")) {
		t.Errorf("current namespace not marked: %s", joined)
	}
	if strings.Contains(joined, f.Btn(f.GlyphSelected, "default")) {
		t.Errorf("non-current namespace marked: %s", joined)
	}
	if !strings.Contains(joined, "All namespaces") {
		t.Errorf("missing all-namespaces option: %s", joined)
	}
}

// With no namespace set, "All namespaces" is the active choice.
func TestNamespaceKeyboardMarksAllWhenEmpty(t *testing.T) {
	mb := &MenuBuilder{config: testConfig(), tokens: NewTokenStore(4096)}
	kb := mb.GetNamespaceInlineKeyboard([]string{"default"}, "", 0, "")

	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			if strings.Contains(b.Text, "All namespaces") {
				if !strings.Contains(b.Text, f.GlyphSelected) {
					t.Errorf("all-namespaces should be marked active, got %q", b.Text)
				}
				return
			}
		}
	}
	t.Fatal("all-namespaces button missing")
}

// Selecting a namespace must round-trip through the parser.
func TestNamespaceSetCallbackParses(t *testing.T) {
	mb := &MenuBuilder{config: testConfig(), tokens: NewTokenStore(4096)}
	kb := mb.GetNamespaceInlineKeyboard([]string{"kube-system"}, "default", 0, "")

	var found bool
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			data, ok := mb.ResolveCallback(b.CallbackData)
			if !ok {
				t.Fatalf("unresolvable callback %q", b.CallbackData)
			}
			if data == "menu:ns:set:kube-system" {
				act := ParseCallbackData(data)
				if act == nil {
					t.Fatal("ns set callback did not parse")
				}
				if act.Type != "ns" || act.Action != "set" || act.Name != "kube-system" {
					t.Errorf("parsed %+v, want type=ns action=set name=kube-system", act)
				}
				found = true
			}
		}
	}
	if !found {
		t.Error("no menu:ns:set button generated")
	}
}

// The all-namespaces button carries an empty name, which must parse cleanly.
func TestNamespaceSetEmptyParses(t *testing.T) {
	act := ParseCallbackData("menu:ns:set:")
	if act == nil {
		t.Fatal("menu:ns:set: did not parse")
	}
	if act.Type != "ns" || act.Action != "set" || act.Name != "" {
		t.Errorf("parsed %+v, want type=ns action=set with empty name", act)
	}
}

// Namespaces containing a colon must not be truncated by the ":" split.
func TestNamespaceWithColonSurvivesParse(t *testing.T) {
	act := ParseCallbackData("menu:ns:set:weird:name")
	if act == nil {
		t.Fatal("did not parse")
	}
	if act.Name != "weird:name" {
		t.Errorf("Name = %q, want %q", act.Name, "weird:name")
	}
}

func TestNamespaceKeyboardPaginates(t *testing.T) {
	mb := &MenuBuilder{config: testConfig(), tokens: NewTokenStore(4096)}

	many := make([]string, 0, 25)
	for i := 0; i < 25; i++ {
		many = append(many, fmt.Sprintf("namespace-%02d", i))
	}

	first := mb.GetNamespaceInlineKeyboard(many, "", 0, "")
	var firstLabels, firstData []string
	for _, row := range first.InlineKeyboard {
		for _, b := range row {
			firstLabels = append(firstLabels, b.Text)
			d, _ := mb.ResolveCallback(b.CallbackData)
			firstData = append(firstData, d)
		}
	}
	joinedFirst := strings.Join(firstLabels, " ")

	if strings.Contains(joinedFirst, "namespace-20") {
		t.Error("page 0 should not contain page-2 entries")
	}
	if !strings.Contains(strings.Join(firstData, " "), "menu:ns:page:1") {
		t.Errorf("missing next-page button: %v", firstData)
	}
	if strings.Contains(strings.Join(firstData, " "), "menu:ns:page:-1") {
		t.Error("page 0 must not offer a previous page")
	}

	last := mb.GetNamespaceInlineKeyboard(many, "", 2, "")
	var lastData []string
	for _, row := range last.InlineKeyboard {
		for _, b := range row {
			d, _ := mb.ResolveCallback(b.CallbackData)
			lastData = append(lastData, d)
		}
	}
	if !strings.Contains(strings.Join(lastData, " "), "menu:ns:page:1") {
		t.Errorf("last page missing prev button: %v", lastData)
	}
}

// Out-of-range pages must not panic.
func TestNamespaceKeyboardHandlesOutOfRangePage(t *testing.T) {
	mb := &MenuBuilder{config: testConfig(), tokens: NewTokenStore(4096)}
	kb := mb.GetNamespaceInlineKeyboard([]string{"a", "b"}, "", 99, "")
	if len(kb.InlineKeyboard) == 0 {
		t.Error("expected at least the all-namespaces and back rows")
	}
}

// The resource list must expose a namespace switcher, since that is where users
// actually notice they are stuck in one namespace.
func TestResourceListHasNamespaceButton(t *testing.T) {
	mb := &MenuBuilder{config: testConfig(), tokens: NewTokenStore(4096)}
	kb := mb.GetResourceListInlineKeyboard("pods", nil, 0, 10, "kube-system")

	var sawPicker bool
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			d, _ := mb.ResolveCallback(b.CallbackData)
			if strings.HasPrefix(d, "menu:ns:pick:") {
				sawPicker = true
				if !strings.Contains(b.Text, "kube-system") {
					t.Errorf("switcher should show the active namespace, got %q", b.Text)
				}
			}
		}
	}
	if !sawPicker {
		t.Error("resource list has no namespace switcher button")
	}
}

func TestNsButtonLabel(t *testing.T) {
	if got := nsButtonLabel(""); !strings.Contains(got, "All") {
		t.Errorf("empty namespace should read as all, got %q", got)
	}
	long := nsButtonLabel("a-very-long-namespace-name-indeed")
	if len([]rune(long)) > 16 {
		t.Errorf("label not truncated: %q", long)
	}
	if !strings.Contains(long, "…") {
		t.Errorf("truncated label should be elided: %q", long)
	}
}
