package bot

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

// Before the fix, handleCallbackQuery replied "Callback: <data>" for every
// button. These tests assert real navigation instead.
//
// Every case edits: a callback renders into the pane its button lives on. Help
// is in this list deliberately — it was the last navigation target that sent a
// new message, which pushed the menu off screen.
func TestCallbackNavigationEditsInPlace(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	tests := []struct {
		name       string
		data       string
		wantMethod string
		wantSubstr string
	}{
		{"main menu", "menu:main", "editMessageText", "telectl"},
		{"resource types", "menu:resource:types", "editMessageText", "Resources"},
		{"monitor home", "menu:monitor:home", "editMessageText", "Monitoring"},
		{"operations home", "menu:ops:home", "editMessageText", "Operations"},
		{"settings home", "menu:settings:home", "editMessageText", "Settings"},
		{"help button", "menu:help", "editMessageText", "command reference"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake.reset()
			lib.ProcessUpdate(context.Background(), callbackUpdate(tc.data, 7))

			var sawMethod bool
			var texts []string
			for _, c := range fake.sent() {
				if c.Method == tc.wantMethod {
					sawMethod = true
				}
				if s, ok := c.Body["text"].(string); ok {
					texts = append(texts, s)
				}
			}
			if !sawMethod {
				t.Errorf("expected a %s call for %q; calls were %v",
					tc.wantMethod, tc.data, methodsOf(fake))
			}
			joined := strings.Join(texts, "\n---\n")
			if !strings.Contains(joined, tc.wantSubstr) {
				t.Errorf("reply for %q missing %q.\ngot: %s", tc.data, tc.wantSubstr, joined)
			}
			// The old placeholder behaviour must be gone.
			if strings.Contains(joined, "Callback: menu:") {
				t.Errorf("callback %q still hits the placeholder echo", tc.data)
			}
		})
	}
}

// Every callback must be acknowledged, or Telegram shows a spinner on the
// button until it times out.
func TestCallbackIsAcknowledged(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:main", 7))

	var acked bool
	for _, c := range fake.sent() {
		if c.Method == "answerCallbackQuery" {
			acked = true
		}
	}
	if !acked {
		t.Errorf("callback was not acknowledged; calls were %v", methodsOf(fake))
	}
}

func TestNoopCallbackSendsNothing(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:noop", 7))

	for _, c := range fake.sent() {
		if c.Method == "sendMessage" || c.Method == "editMessageText" {
			t.Errorf("noop callback should not send a message, got %s: %v", c.Method, c.Body["text"])
		}
	}
}

// Navigation keyboards must survive the round trip: an edit that drops
// reply_markup would strip the buttons and dead-end the user.
func TestNavigationKeepsKeyboard(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	for _, data := range []string{"menu:resource:types", "menu:monitor:home", "menu:ops:home", "menu:main"} {
		fake.reset()
		lib.ProcessUpdate(context.Background(), callbackUpdate(data, 7))

		var hasKB bool
		for _, c := range fake.sent() {
			if c.Method != "editMessageText" {
				continue
			}
			if rm, ok := c.Body["reply_markup"].(map[string]any); ok {
				if rows, ok := rm["inline_keyboard"].([]any); ok && len(rows) > 0 {
					hasKB = true
				}
			}
		}
		if !hasKB {
			t.Errorf("%s produced an edit with no inline keyboard — user would be stuck", data)
		}
	}
}

// Unauthorized users must not be able to drive menus via crafted callbacks.
func TestCallbackRejectsUnauthorizedUser(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.config.Telegram.AllowedUserIDs = []int64{7}
	b.registerUpdateHandlers(lib)

	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:resource:types", 999))

	for _, c := range fake.sent() {
		if c.Method == "editMessageText" {
			t.Error("unauthorized callback was allowed to navigate menus")
		}
	}
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func methodsOf(f *fakeTelegram) []string {
	out := make([]string, 0, len(f.sent()))
	for _, c := range f.sent() {
		out = append(out, c.Method)
	}
	return out
}
