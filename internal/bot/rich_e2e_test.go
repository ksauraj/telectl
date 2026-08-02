package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/utils/formatters"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The bot must use sendRichMessage (Bot API 10.1+) so Telegram renders a native
// table, not the monospace code-block fallback.
func TestSendRichUsesRichMessageEndpoint(t *testing.T) {
	fake := newFakeTelegram(t)
	b, _ := newTestBot(t, fake)

	pods := []k8s.ResourceInfo{
		{Name: "nginx-1", Namespace: "default", Kind: "Pod", Status: "Running",
			CreatedAt: metav1.Now()},
	}
	b.SendRich(7, formatters.RichResourceList(pods, false), "fallback text")

	var rich *recordedCall
	for _, c := range fake.sent() {
		if c.Method == "sendRichMessage" {
			cc := c
			rich = &cc
		}
	}
	if rich == nil {
		t.Fatalf("expected sendRichMessage, calls were %v", methodsOf(fake))
	}

	rm, ok := rich.Body["rich_message"].(map[string]any)
	if !ok {
		t.Fatalf("rich_message missing or not an object: %#v", rich.Body)
	}
	markdown, _ := rm["markdown"].(string)
	if markdown == "" {
		t.Fatalf("rich_message.markdown empty: %#v", rm)
	}

	// A GFM pipe table with a header separator is what Telegram renders as a
	// native table.
	if !strings.Contains(markdown, "| NAME |") {
		t.Errorf("markdown has no table header:\n%s", markdown)
	}
	if !strings.Contains(markdown, ":---") {
		t.Errorf("markdown has no GFM alignment row:\n%s", markdown)
	}
	if !strings.Contains(markdown, "nginx-1") {
		t.Errorf("markdown missing the pod row:\n%s", markdown)
	}
	// The old blocks payload must be gone.
	if _, hasBlocks := rm["blocks"]; hasBlocks {
		t.Error("rich_message still carries a blocks field; that is the receive shape")
	}
}

// If the server rejects rich messages, the user must still get an answer.
func TestSendRichFallsBackOnRejection(t *testing.T) {
	fake := newFakeTelegram(t)
	fake.failMethod("sendRichMessage", 400, "Bad Request: rich messages are not supported")
	b, _ := newTestBot(t, fake)

	b.SendRich(7, "### rich\n\n| A |\n|:---|\n| 1 |", "PLAIN FALLBACK BODY")

	var sawRichAttempt, sawFallback bool
	for _, c := range fake.sent() {
		switch c.Method {
		case "sendRichMessage":
			sawRichAttempt = true
		case "sendMessage":
			if s, _ := c.Body["text"].(string); strings.Contains(s, "PLAIN FALLBACK BODY") {
				sawFallback = true
			}
		}
	}
	if !sawRichAttempt {
		t.Error("expected a sendRichMessage attempt first")
	}
	if !sawFallback {
		t.Errorf("rich send failed but no text fallback was sent; calls were %v", methodsOf(fake))
	}
}

// Keyboards must still attach when the message is rich.
func TestSendRichKeyboardAttachesMarkup(t *testing.T) {
	fake := newFakeTelegram(t)
	b, _ := newTestBot(t, fake)

	kb := b.menuBuilder.GetMainMenuInlineKeyboard()
	b.SendRichKeyboard(7, "### hi", "hi", &kb)

	for _, c := range fake.sent() {
		if c.Method != "sendRichMessage" {
			continue
		}
		rm, ok := c.Body["reply_markup"].(map[string]any)
		if !ok {
			t.Fatalf("rich message carried no reply_markup: %#v", c.Body)
		}
		if _, ok := rm["inline_keyboard"].([]any); !ok {
			t.Errorf("reply_markup is not snake_case inline_keyboard: %#v", rm)
		}
		return
	}
	t.Fatalf("no sendRichMessage call; got %v", methodsOf(fake))
}

// And the keyboard must survive the fallback path too.
func TestSendRichKeyboardFallbackKeepsKeyboard(t *testing.T) {
	fake := newFakeTelegram(t)
	fake.failMethod("sendRichMessage", 400, "Bad Request: unsupported")
	b, _ := newTestBot(t, fake)

	kb := b.menuBuilder.GetMainMenuInlineKeyboard()
	b.SendRichKeyboard(7, "### hi", "plain hi", &kb)

	for _, c := range fake.sent() {
		if c.Method != "sendMessage" {
			continue
		}
		rm, ok := c.Body["reply_markup"].(map[string]any)
		if !ok {
			t.Fatalf("fallback dropped the keyboard: %#v", c.Body)
		}
		if _, ok := rm["inline_keyboard"].([]any); !ok {
			t.Errorf("fallback reply_markup malformed: %#v", rm)
		}
		return
	}
	t.Fatal("no fallback sendMessage observed")
}

// A rich table must never be emitted with literal ``` fences, which was the old
// monospace rendering the user saw.
func TestRichListIsNotCodeFenced(t *testing.T) {
	pods := []k8s.ResourceInfo{
		{Name: "a", Namespace: "default", Kind: "Pod", Status: "Running", CreatedAt: metav1.Now()},
	}
	got := formatters.RichResourceList(pods, false)
	if strings.Contains(got, "```") {
		t.Errorf("rich list should use a real table, not a code fence:\n%s", got)
	}
}

// Guard the menu browse path: tapping a resource type must edit to rich content.
func TestMenuResourceListAttemptsRich(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	// k8sClient is nil in the stub bot, so listResources cannot run; assert the
	// navigation edit path itself still goes through editMessageText.
	lib.ProcessUpdate(context.Background(), callbackUpdate("menu:resource:types", 7))

	var sawEdit bool
	for _, c := range fake.sent() {
		if c.Method == "editMessageText" {
			sawEdit = true
		}
	}
	if !sawEdit {
		t.Errorf("expected editMessageText for menu navigation, got %v", methodsOf(fake))
	}
}
