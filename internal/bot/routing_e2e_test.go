package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	bottg "github.com/go-telegram/bot"
	botmodels "github.com/go-telegram/bot/models"
)

// fakeTelegram is a stand-in Bot API server. It records every outgoing call so
// tests can assert what the bot actually sent, including reply_markup shape.
type fakeTelegram struct {
	mu       sync.Mutex
	calls    []recordedCall
	failures map[string]apiFailure
	srv      *httptest.Server
}

// apiFailure makes the stub reject a method, so tests can drive fallback paths.
type apiFailure struct {
	code        int
	description string
}

// failMethod makes subsequent calls to method return a Telegram API error.
func (f *fakeTelegram) failMethod(method string, code int, description string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failures == nil {
		f.failures = map[string]apiFailure{}
	}
	f.failures[method] = apiFailure{code: code, description: description}
}

type recordedCall struct {
	Method string
	Body   map[string]any
}

func newFakeTelegram(t *testing.T) *fakeTelegram {
	t.Helper()
	f := &fakeTelegram{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]

		// go-telegram/bot sends multipart/form-data, not JSON. Each param is a
		// form field; nested objects like reply_markup arrive as JSON strings.
		body := map[string]any{}
		ct := r.Header.Get("Content-Type")
		switch {
		case strings.HasPrefix(ct, "multipart/form-data"):
			if err := r.ParseMultipartForm(1 << 20); err == nil {
				for k, v := range r.MultipartForm.Value {
					if len(v) == 0 {
						continue
					}
					// Decode JSON-valued fields (reply_markup, entities) so
					// tests can assert on their structure.
					var nested any
					if err := json.Unmarshal([]byte(v[0]), &nested); err == nil {
						if _, isObj := nested.(map[string]any); isObj {
							body[k] = nested
							continue
						}
					}
					body[k] = v[0]
				}
			}
		case strings.HasPrefix(ct, "application/json"):
			_ = json.NewDecoder(r.Body).Decode(&body)
		}

		f.mu.Lock()
		f.calls = append(f.calls, recordedCall{Method: method, Body: body})
		f.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		f.mu.Lock()
		fail, shouldFail := f.failures[method]
		f.mu.Unlock()
		if shouldFail {
			w.WriteHeader(fail.code)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":` +
				itoa(fail.code) + `,"description":"` + fail.description + `"}`))
			return
		}

		switch method {
		case "getMe":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"username":"testbot"}}`))
		case "sendMessage", "editMessageText", "sendRichMessage":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42,"date":0,"chat":{"id":1,"type":"private"}}}`))
		case "getUpdates":
			_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeTelegram) sent() []recordedCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]recordedCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// lastText returns the text of the most recent sendMessage/editMessageText.
func (f *fakeTelegram) lastText() string {
	calls := f.sent()
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].Method == "sendMessage" || calls[i].Method == "editMessageText" {
			if s, ok := calls[i].Body["text"].(string); ok {
				return s
			}
		}
	}
	return ""
}

// allTexts returns every message body the bot sent, oldest first.
func (f *fakeTelegram) allTexts() []string {
	var out []string
	for _, c := range f.sent() {
		if c.Method == "sendMessage" || c.Method == "editMessageText" {
			if s, ok := c.Body["text"].(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func (f *fakeTelegram) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
}

// textMessage builds an update that mirrors what Telegram sends for a typed
// command, including the bot_command entity real clients attach.
func textMessage(text string, userID int64) *botmodels.Update {
	msg := &botmodels.Message{
		ID:   1,
		Text: text,
		From: &botmodels.User{ID: userID, Username: "tester"},
		Chat: botmodels.Chat{ID: userID, Type: botmodels.ChatTypePrivate},
	}
	if strings.HasPrefix(text, "/") {
		cmdEnd := strings.Index(text, " ")
		if cmdEnd == -1 {
			cmdEnd = len(text)
		}
		msg.Entities = []botmodels.MessageEntity{{
			Type:   botmodels.MessageEntityTypeBotCommand,
			Offset: 0,
			Length: cmdEnd,
		}}
	}
	return &botmodels.Update{ID: 1, Message: msg}
}

func callbackUpdate(data string, userID int64) *botmodels.Update {
	return &botmodels.Update{
		ID: 2,
		CallbackQuery: &botmodels.CallbackQuery{
			ID:   "cb1",
			From: botmodels.User{ID: userID, Username: "tester"},
			Data: data,
			Message: botmodels.MaybeInaccessibleMessage{
				Type: botmodels.MaybeInaccessibleMessageTypeMessage,
				Message: &botmodels.Message{
					ID:   42,
					Chat: botmodels.Chat{ID: userID, Type: botmodels.ChatTypePrivate},
				},
			},
		},
	}
}

// TestUpdateRoutingReachesHandlers is the regression test for the original bug:
// handlers were registered with MatchTypeCommand and an empty pattern, which can
// never match, so every update fell through to the library's default handler and
// the bot answered nothing.
func TestUpdateRoutingReachesHandlers(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)

	// Register handlers exactly as Start does.
	b.registerUpdateHandlers(lib)

	tests := []struct {
		name       string
		update     *botmodels.Update
		wantSubstr string
	}{
		// Commands that need a live cluster (/version, /get, /top) are covered by
		// the handler tests; these are the cluster-independent routing paths.
		{"start shows main menu", textMessage("/start", 7), "telectl"},
		{"help renders reference", textMessage("/help", 7), "Command Reference"},
		{"unknown command is reported", textMessage("/definitelynotacommand", 7), "Unknown command"},
		{"plain text falls back to menu", textMessage("hello there", 7), "telectl"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake.reset()
			lib.ProcessUpdate(context.Background(), tc.update)

			texts := fake.allTexts()
			if len(texts) == 0 {
				t.Fatalf("bot sent nothing for %q — update was not routed to a handler", tc.update.Message.Text)
			}
			joined := strings.Join(texts, "\n---\n")
			if !strings.Contains(joined, tc.wantSubstr) {
				t.Errorf("reply missing %q.\ngot: %s", tc.wantSubstr, joined)
			}
		})
	}
}

// TestHelpIsValidHTML guards the /help regression: the text was single-* bold
// (Markdown V1) sent as MarkdownV2, so Telegram rejected it with a 400 and the
// error was swallowed, making /help look silently dead.
func TestHelpIsValidHTML(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	lib.ProcessUpdate(context.Background(), textMessage("/help", 7))

	var helpCall *recordedCall
	for _, c := range fake.sent() {
		if c.Method == "sendMessage" {
			if s, ok := c.Body["text"].(string); ok && strings.Contains(s, "Command Reference") {
				cc := c
				helpCall = &cc
				break
			}
		}
	}
	if helpCall == nil {
		t.Fatal("/help produced no message")
	}

	if pm, _ := helpCall.Body["parse_mode"].(string); pm != "HTML" {
		t.Errorf("parse_mode = %q, want HTML (MarkdownV2 requires escaping -.()| and rejects single-* bold)", pm)
	}

	text, _ := helpCall.Body["text"].(string)
	// Angle-bracket placeholders must be entity-escaped or Telegram treats them
	// as unknown tags and rejects the message.
	for _, bad := range []string{"<resource>", "<pod>", "<name>", "<replicas>", "<context-name>"} {
		if strings.Contains(text, bad) {
			t.Errorf("unescaped placeholder %q would be parsed as an HTML tag", bad)
		}
	}
	if !strings.Contains(text, "&lt;resource&gt;") {
		t.Error("expected escaped &lt;resource&gt; in help text")
	}
	// Only <b> is allowed by Telegram among the tags we emit.
	for _, tag := range extractTags(text) {
		switch tag {
		case "b", "/b", "code", "/code", "i", "/i", "pre", "/pre":
		default:
			t.Errorf("help text contains tag <%s> that Telegram HTML does not support", tag)
		}
	}
}

func extractTags(s string) []string {
	var tags []string
	for i := 0; i < len(s); i++ {
		if s[i] != '<' {
			continue
		}
		end := strings.IndexByte(s[i:], '>')
		if end == -1 {
			break
		}
		tags = append(tags, s[i+1:i+end])
		i += end
	}
	return tags
}

// TestMainMenuCarriesButtons pins the fix for menus rendering as plain text: the
// Show* views passed a nil keyboard, and the local keyboard structs had no JSON
// tags, so reply_markup never reached Telegram.
func TestMainMenuCarriesButtons(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	lib.ProcessUpdate(context.Background(), textMessage("/start", 7))

	var sawInline, sawReply bool
	var callbackData []string
	for _, c := range fake.sent() {
		if c.Method != "sendMessage" {
			continue
		}
		rm, ok := c.Body["reply_markup"].(map[string]any)
		if !ok {
			continue
		}
		if rows, ok := rm["inline_keyboard"].([]any); ok {
			sawInline = true
			for _, row := range rows {
				for _, btn := range row.([]any) {
					bm := btn.(map[string]any)
					if cd, ok := bm["callback_data"].(string); ok {
						callbackData = append(callbackData, cd)
					}
				}
			}
		}
		if _, ok := rm["keyboard"].([]any); ok {
			sawReply = true
		}
	}

	if !sawInline {
		t.Error("main menu sent no inline_keyboard — buttons would not appear")
	}
	if !sawReply {
		t.Error("main menu sent no reply keyboard — the bottom bar would never appear")
	}
	if len(callbackData) == 0 {
		t.Fatal("inline buttons carried no callback_data")
	}
	// Every button must parse, or clicking it silently does nothing.
	for _, cd := range callbackData {
		if cd == "" {
			t.Error("empty callback_data on a button")
		}
	}
}

// TestChannelPostDoesNotPanic covers messages with no From (channel posts),
// which now reach the handler since routing was fixed.
func TestChannelPostDoesNotPanic(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	upd := &botmodels.Update{ID: 3, Message: &botmodels.Message{
		ID:   9,
		Text: "/start",
		From: nil, // channel post
		Chat: botmodels.Chat{ID: 5, Type: botmodels.ChatTypeChannel},
	}}

	// Must not panic on the msg.From dereference.
	lib.ProcessUpdate(context.Background(), upd)
}

// TestUnauthorizedUserRejected verifies the allowlist gate still applies now
// that updates actually reach the handler.
func TestUnauthorizedUserRejected(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.config.Telegram.AllowedUserIDs = []int64{7}
	b.registerUpdateHandlers(lib)

	lib.ProcessUpdate(context.Background(), textMessage("/start", 999))

	if got := fake.lastText(); !strings.Contains(got, "not authorized") {
		t.Errorf("expected authorization refusal, got %q", got)
	}
}

func mustLibBot(t *testing.T, serverURL string) *bottg.Bot {
	t.Helper()
	lib, err := bottg.New("test:token",
		bottg.WithServerURL(serverURL),
		bottg.WithSkipGetMe(),
		bottg.WithNotAsyncHandlers(), // deterministic: handlers run inline
	)
	if err != nil {
		t.Fatalf("new lib bot: %v", err)
	}
	return lib
}
