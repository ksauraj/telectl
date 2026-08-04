package bot

import (
	"context"
	"strings"
	"testing"

	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// The single-pane invariant, and the reason this file exists.
//
// Navigation already edited in place, but every *verb* sent a new message: four
// taps produced five messages, only one of which carried a keyboard, and the
// menu ended up several screens above where the user was reading. These tests
// fail if any callback path goes back to sending.
//
// Each was checked against the pre-change code and fails there.

// sendCalls returns the calls that add a message to the chat, which is what the
// pane model forbids for a callback. Rich messages count: sendRichMessage adds a
// message just as sendMessage does.
func sendCalls(f *fakeTelegram) []string {
	var out []string
	for _, c := range f.sent() {
		switch c.Method {
		case "sendMessage", "sendRichMessage":
			out = append(out, c.Method)
		}
	}
	return out
}

func editCalls(f *fakeTelegram) []string {
	var out []string
	for _, c := range f.sent() {
		if c.Method == "editMessageText" {
			out = append(out, c.Method)
		}
	}
	return out
}

// Every verb reachable from a detail pane must edit the pane rather than post to
// the chat. This is the anti-spam invariant stated directly.
func TestNoCallbackSendsANewMessage(t *testing.T) {
	objs := []runtime.Object{
		podObj("api-abc", "default", "node-1"),
		deployObj("api", "default", 3),
		svcObj("api", "default"),
		nodeObj("node-1", false),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	}

	// The full set of callbacks a user can reach by tapping, including the
	// navigation targets and the verbs that used to send.
	callbacks := []string{
		"menu:main",
		"menu:help",
		"menu:resource:types",
		"menu:resource:pods",
		"menu:resource:view:pods:default:api-abc",
		"menu:monitor:home",
		"menu:monitor:top:pods",
		"menu:monitor:top:nodes",
		"menu:monitor:events",
		"menu:monitor:watch",
		"menu:ops:home",
		"menu:ops:restart",
		"menu:ops:scale",
		"menu:ops:delete",
		"menu:ops:edit",
		"menu:settings:home",
		"menu:settings:namespace",
		"menu:settings:context",
		"menu:ns:pick:pods",
		"menu:ns:set:default",
		"menu:action:describe:pods:default:api-abc",
		"menu:action:labels:pods:default:api-abc",
		"menu:action:events:pods:default:api-abc",
		"menu:action:logsopts:pods:default:api-abc",
		"menu:action:logs:pods:default:api-abc:app:100",
		"menu:action:logsfollow:pods:default:api-abc:app",
		"menu:action:logsprevious:pods:default:api-abc:app",
		"menu:action:exec:pods:default:api-abc",
		"menu:action:portforward:pods:default:api-abc",
		"menu:action:help:pods:default:api-abc",
		"menu:action:delete:pods:default:api-abc",
		"menu:action:selector:deployments:default:api",
		"menu:action:pods:deployments:default:api",
		"menu:action:history:deployments:default:api",
		"menu:action:edit:deployments:default:api",
		"menu:action:scale:deployments:default:api",
		"menu:action:scaleset:deployments:default:api:2",
		"menu:action:scalecustom:deployments:default:api",
		"menu:action:restart:deployments:default:api",
		"menu:action:endpoints:services:default:api",
		"menu:action:top:nodes::node-1",
		"menu:action:nodepods:nodes::node-1",
		"menu:action:cordon:nodes::node-1",
		"menu:action:uncordon:nodes::node-1",
		"menu:action:drain:nodes::node-1",
		"menu:action:confirmdrain:nodes::node-1",
		"menu:action:nsresources:namespaces::default",
	}

	for _, data := range callbacks {
		t.Run(data, func(t *testing.T) {
			_, lib, fake := detailBot(t, objs...)
			fake.reset()

			lib.ProcessUpdate(context.Background(), callbackUpdate(data, 7))

			if sends := sendCalls(fake); len(sends) > 0 {
				t.Errorf("callback %q sent %v instead of editing the pane — "+
					"this is the message spam the single pane exists to prevent",
					data, sends)
			}
		})
	}
}

// A verb that renders output must also render a way out of it. Output with no
// keyboard is the dead end that forced scrolling back to an older message.
func TestEveryVerbPaneCarriesAKeyboard(t *testing.T) {
	objs := []runtime.Object{
		podObj("api-abc", "default", "node-1"),
		deployObj("api", "default", 3),
		svcObj("api", "default"),
		nodeObj("node-1", false),
	}

	verbs := []string{
		"menu:action:describe:pods:default:api-abc",
		"menu:action:labels:pods:default:api-abc",
		"menu:action:events:pods:default:api-abc",
		"menu:action:logs:pods:default:api-abc:app:100",
		"menu:action:help:pods:default:api-abc",
		"menu:action:exec:pods:default:api-abc",
		"menu:action:selector:deployments:default:api",
		"menu:action:history:deployments:default:api",
		"menu:action:edit:deployments:default:api",
		"menu:action:endpoints:services:default:api",
		"menu:action:nodepods:nodes::node-1",
		"menu:action:top:nodes::node-1",
		// A failure must be as escapable as a success.
		"menu:action:labels:pods:default:does-not-exist",
	}

	for _, data := range verbs {
		t.Run(data, func(t *testing.T) {
			_, lib, fake := detailBot(t, objs...)
			fake.reset()

			lib.ProcessUpdate(context.Background(), callbackUpdate(data, 7))

			if !hasInlineKeyboard(fake) {
				t.Errorf("verb %q rendered output with no keyboard — the user would "+
					"have to scroll back to an earlier message to continue; calls: %v",
					data, methodsOf(fake))
			}
		})
	}
}

// hasInlineKeyboard reports whether any call carried a non-empty inline keyboard.
func hasInlineKeyboard(f *fakeTelegram) bool {
	for _, c := range f.sent() {
		rm, ok := c.Body["reply_markup"].(map[string]any)
		if !ok {
			continue
		}
		if rows, ok := rm["inline_keyboard"].([]any); ok && len(rows) > 0 {
			return true
		}
	}
	return false
}

// Walking several verbs in sequence must leave the chat at one message. This is
// the user's actual complaint expressed as a test: before, this walk produced
// one message per tap and the menu scrolled away.
func TestVerbWalkStaysInOnePane(t *testing.T) {
	_, lib, fake := detailBot(t,
		podObj("api-abc", "default", "node-1"),
		deployObj("api", "default", 3),
	)
	fake.reset()

	walk := []string{
		"menu:resource:pods",
		"menu:resource:view:pods:default:api-abc",
		"menu:action:labels:pods:default:api-abc",
		"menu:action:events:pods:default:api-abc",
		"menu:action:describe:pods:default:api-abc",
		"menu:action:logsopts:pods:default:api-abc",
		"menu:action:help:pods:default:api-abc",
	}
	for _, data := range walk {
		lib.ProcessUpdate(context.Background(), callbackUpdate(data, 7))
	}

	if sends := sendCalls(fake); len(sends) > 0 {
		t.Errorf("a %d-tap walk added %d message(s) to the chat: %v",
			len(walk), len(sends), sends)
	}
	if edits := editCalls(fake); len(edits) == 0 {
		t.Fatalf("the walk rendered nothing at all; calls: %v", methodsOf(fake))
	}
}

// A stale token — a button from before a restart — must replace the dead pane
// rather than post an explanation underneath it, and must leave a button that
// still works. "Main" is short enough to bypass the token store, so it resolves
// after a restart when nothing else does.
func TestStaleTokenReplacesThePane(t *testing.T) {
	_, lib, fake := detailBot(t)
	fake.reset()

	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:t:deadbeef", 7))

	if sends := sendCalls(fake); len(sends) > 0 {
		t.Errorf("stale token sent %v instead of replacing the dead pane", sends)
	}
	if len(editCalls(fake)) == 0 {
		t.Fatalf("stale token produced no edit; calls: %v", methodsOf(fake))
	}
	if !hasInlineKeyboard(fake) {
		t.Error("stale-token pane carried no keyboard — every button on screen is " +
			"now dead and the user has no way forward")
	}
}

// A pane body must fit one message. Telegram rejects an edit over 4096
// characters outright, and a rejected edit leaves the user looking at stale
// content with no indication anything happened.
func TestPaneBodyFitsOneMessage(t *testing.T) {
	// A manifest is the largest thing a pane renders.
	deploy := deployObj("api", "default", 3)
	deploy.Annotations["big"] = strings.Repeat("x", 8000)

	_, lib, fake := detailBot(t, deploy)
	fake.reset()

	lib.ProcessUpdate(context.Background(),
		callbackUpdate("menu:action:edit:deployments:default:api", 7))

	const telegramLimit = 4096
	var checked bool
	for _, c := range fake.sent() {
		for _, field := range []string{"text", "rich_message"} {
			s, ok := c.Body[field].(string)
			if !ok {
				continue
			}
			checked = true
			if n := len([]rune(s)); n > telegramLimit {
				t.Errorf("%s body is %d runes, over Telegram's %d limit — "+
					"the edit would be rejected and the pane would go stale",
					c.Method, n, telegramLimit)
			}
		}
	}
	if !checked {
		t.Fatalf("no body was rendered; calls: %v", methodsOf(fake))
	}
}

// truncateForPane must cut on a rune boundary: slicing bytes can split a
// multi-byte character, and Telegram rejects invalid UTF-8 outright.
func TestTruncateForPaneKeepsValidUTF8(t *testing.T) {
	for _, in := range []string{
		strings.Repeat("a", paneLimit*2),
		strings.Repeat("日", paneLimit*2),
		strings.Repeat("a日\n", paneLimit),
	} {
		got := truncateForPane(in)
		if !utf8.ValidString(got) {
			t.Errorf("truncation produced invalid UTF-8 for input of %d bytes", len(in))
		}
		if len([]rune(got)) > paneLimit+64 { // +note
			t.Errorf("truncated body is %d runes, want <= %d", len([]rune(got)), paneLimit+64)
		}
	}

	// Short input is returned untouched, with no truncation note.
	if got := truncateForPane("short"); got != "short" {
		t.Errorf("short body was altered: %q", got)
	}
}
