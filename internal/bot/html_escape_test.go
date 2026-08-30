package bot

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/ksauraj/telectl/internal/menus"
)

// anyTag matches any angle-bracketed token that looks like an HTML tag.
var anyTag = regexp.MustCompile(`<[a-zA-Z/][a-zA-Z0-9/-]*>`)

// allowedTags are Telegram's HTML parse-mode tags plus the entity markers the
// code emits. Anything else in a body sent as HTML makes Telegram reject the
// whole message with "can't parse entities: Unsupported start tag" — the bug
// that left the help menu and the Logs/Exec/Resources reply buttons dead.
var allowedTags = map[string]bool{
	"<b>": true, "</b>": true,
	"<i>": true, "</i>": true,
	"<code>": true, "</code>": true,
	"<pre>": true, "</pre>": true,
	"<s>": true, "</s>": true,
	"<u>": true, "</u>": true,
	"<tg-spoiler>": true, "</tg-spoiler>": true,
}

// findUnescaped returns the first HTML tag in s that is not an allowed
// Telegram tag, or "".
func findUnescaped(s string) string {
	for _, m := range anyTag.FindAllString(s, -1) {
		if !allowedTags[m] {
			return m
		}
	}
	return ""
}

// TestHelpSectionsHaveNoUnescapedPlaceholders asserts every help-category body
// renders as valid HTML — no raw <pod>/<deployment>/<type>/<name>/<cmd> tags.
func TestHelpSectionsHaveNoUnescapedPlaceholders(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	for _, data := range []string{
		"menu:help:resources",
		"menu:help:workloads",
		"menu:help:networking",
		"menu:help:storage",
		"menu:help:monitoring",
		"menu:help:operations",
		"menu:help:settings",
		"menu:help:all",
	} {
		fake.reset()
		lib.ProcessUpdate(context.Background(), callbackUpdate(data, 7))
		for _, txt := range fake.allTexts() {
			if m := findUnescaped(txt); m != "" {
				t.Errorf("help callback %q rendered unescaped %q in HTML: %q", data, m, txt)
			}
		}
	}
}

// TestReplyKeyboardUsageHasNoUnescapedPlaceholders asserts the Logs and Exec
// reply-keyboard usage hints and the Resources type picker render valid HTML.
func TestReplyKeyboardUsageHasNoUnescapedPlaceholders(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	for _, label := range []string{
		menus.LabelResources,
		menus.LabelLogs,
		menus.LabelExec,
	} {
		fake.reset()
		lib.ProcessUpdate(context.Background(), textMessage(label, 7))
		for _, txt := range fake.allTexts() {
			if m := findUnescaped(txt); m != "" {
				t.Errorf("reply tap %q rendered unescaped %q in HTML: %q", label, m, txt)
			}
		}
	}
}

// TestEscapedPlaceholderRendersLiteral guard-asserts that the fixed usage text
// still reads as the user expects: the placeholder word remains present in the
// underlying string (escaped), not silently stripped.
func TestEscapedPlaceholderRendersLiteral(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	lib.ProcessUpdate(context.Background(), textMessage(menus.LabelLogs, 7))
	joined := strings.Join(fake.allTexts(), "\n")
	if !strings.Contains(joined, "&lt;pod&gt;") {
		t.Errorf("Logs usage lost its escaped placeholder; got: %q", joined)
	}
}
