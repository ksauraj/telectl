package tg

import (
	"encoding/json"
	"testing"
)

// Telegram requires snake_case keys (inline_keyboard, callback_data, ...). The
// local tg structs carry no JSON tags, so marshalling them directly yields
// {"InlineKeyboard":...} and Telegram silently drops the markup — the bug that
// made every menu render as buttonless text. These tests pin the conversion.
func TestInlineKeyboardMarshalsSnakeCase(t *testing.T) {
	kb := InlineKeyboard(
		InlineKeyboardRow(
			InlineButtonData("📦 Pods", "menu:resource:pods"),
			InlineButtonURL("Docs", "https://example.com"),
		),
	)

	raw, err := json.Marshal(toModelInlineKeyboard(&kb))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(raw)

	var decoded struct {
		InlineKeyboard [][]struct {
			Text         string `json:"text"`
			CallbackData string `json:"callback_data"`
			URL          string `json:"url"`
		} `json:"inline_keyboard"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.InlineKeyboard) != 1 || len(decoded.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 1 row of 2 buttons, got %s", got)
	}
	if decoded.InlineKeyboard[0][0].CallbackData != "menu:resource:pods" {
		t.Errorf("callback_data lost: %s", got)
	}
	if decoded.InlineKeyboard[0][0].Text != "📦 Pods" {
		t.Errorf("text lost: %s", got)
	}
	if decoded.InlineKeyboard[0][1].URL != "https://example.com" {
		t.Errorf("url lost: %s", got)
	}
}

func TestReplyKeyboardMarshalsSnakeCase(t *testing.T) {
	rk := ReplyKeyboard(KeyboardButtonRow(KeyboardButtonText("📦 Resources")))
	rk.ResizeKeyboard = true
	rk.InputFieldPlaceholder = "pick one"

	raw, err := json.Marshal(toModelReplyKeyboard(&rk))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded struct {
		Keyboard [][]struct {
			Text string `json:"text"`
		} `json:"keyboard"`
		ResizeKeyboard        bool   `json:"resize_keyboard"`
		InputFieldPlaceholder string `json:"input_field_placeholder"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Keyboard) != 1 || decoded.Keyboard[0][0].Text != "📦 Resources" {
		t.Fatalf("keyboard button lost: %s", raw)
	}
	if !decoded.ResizeKeyboard {
		t.Errorf("resize_keyboard lost: %s", raw)
	}
	if decoded.InputFieldPlaceholder != "pick one" {
		t.Errorf("input_field_placeholder lost: %s", raw)
	}
}

func TestNilKeyboardConvertsToNil(t *testing.T) {
	if toModelInlineKeyboard(nil) != nil {
		t.Error("nil inline keyboard should convert to nil, not an empty struct")
	}
	if toModelReplyKeyboard(nil) != nil {
		t.Error("nil reply keyboard should convert to nil, not an empty struct")
	}
}

func TestParseModeMapping(t *testing.T) {
	cases := map[string]string{
		"HTML":       "HTML",
		"MarkdownV2": "MarkdownV2",
		"Markdown":   "Markdown",
		"":           "HTML", // default must be a valid mode, not empty
	}
	for in, want := range cases {
		if got := string(toModelParseMode(in)); got != want {
			t.Errorf("toModelParseMode(%q) = %q, want %q", in, got, want)
		}
	}
}
