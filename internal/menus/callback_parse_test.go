package menus

import (
	"strings"
	"testing"

	"github.com/ksauraj/telectl/internal/config"
	"github.com/ksauraj/telectl/internal/tg"
)

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Bot.MenuPageSize = 10
	cfg.Bot.EnableMenuButton = true
	cfg.Bot.EnableReplyKeyboard = true
	return cfg
}

// buttonData flattens every callback_data value out of an inline keyboard.
func buttonData(kb tg.InlineKeyboardMarkup) []string {
	var out []string
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			out = append(out, btn.CallbackData)
		}
	}
	return out
}

// Every inline button the bot renders must produce callback data that
// ParseCallbackData can decode. A button whose data does not parse is a dead
// button: the user taps it and nothing happens.
func TestAllRenderedButtonsParse(t *testing.T) {
	mb := &MenuBuilder{config: testConfig()}

	builders := map[string][]string{
		"main":          buttonData(mb.GetMainMenuInlineKeyboard()),
		"resourceTypes": buttonData(mb.GetResourceTypeInlineKeyboard()),
		"monitor":       buttonData(mb.GetMonitorInlineKeyboard()),
		"operations":    buttonData(mb.GetOperationsInlineKeyboard()),
		"settings":      buttonData(mb.GetSettingsInlineKeyboard()),
		"confirmDelete": buttonData(mb.GetConfirmDeleteKeyboard("pods", "default", "my-pod")),
		"scale":         buttonData(mb.GetScaleKeyboard("deployments", "default", "web", 3)),
		"logOptions":    buttonData(mb.GetLogOptionsKeyboard("default", "my-pod", "app")),
	}

	for name, datas := range builders {
		if len(datas) == 0 {
			t.Errorf("%s keyboard rendered no buttons", name)
		}
		for _, d := range datas {
			if d == "" {
				continue // URL buttons legitimately have no callback data
			}
			act := ParseCallbackData(d)
			if act == nil {
				t.Errorf("%s: callback data %q does not parse — dead button", name, d)
				continue
			}
			if act.Type == "" {
				t.Errorf("%s: callback data %q parsed with empty Type", name, d)
			}
		}
	}
}

func TestParseCallbackDataShapes(t *testing.T) {
	tests := []struct {
		data     string
		wantType string
		wantAct  string
		wantRes  string
		wantNS   string
		wantName string
	}{
		{"menu:main", "main", "", "", "", ""},
		{"menu:noop", "noop", "", "", "", ""},
		{"menu:help", "help", "", "", "", ""},
		{"menu:resource:types", "resource", "types", "", "", ""},
		{"menu:monitor:home", "monitor", "home", "", "", ""},
		{"menu:ops:home", "ops", "home", "", "", ""},
		{"menu:settings:home", "settings", "home", "", "", ""},
		{"menu:resource:view:pods:default:my-pod", "resource", "view", "pods", "default", "my-pod"},
		{"menu:action:describe:pods:default:my-pod", "action", "describe", "pods", "default", "my-pod"},
		{"menu:action:confirmdelete:pods:default:my-pod", "action", "confirmdelete", "pods", "default", "my-pod"},
		{"menu:ctx:switch:minikube", "ctx", "switch", "", "", "minikube"},
	}

	for _, tc := range tests {
		t.Run(tc.data, func(t *testing.T) {
			got := ParseCallbackData(tc.data)
			if got == nil {
				t.Fatalf("ParseCallbackData(%q) = nil", tc.data)
			}
			if got.Type != tc.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tc.wantType)
			}
			if got.Action != tc.wantAct {
				t.Errorf("Action = %q, want %q", got.Action, tc.wantAct)
			}
			if tc.wantRes != "" && got.ResourceType != tc.wantRes {
				t.Errorf("ResourceType = %q, want %q", got.ResourceType, tc.wantRes)
			}
			if tc.wantNS != "" && got.Namespace != tc.wantNS {
				t.Errorf("Namespace = %q, want %q", got.Namespace, tc.wantNS)
			}
			if tc.wantName != "" && got.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tc.wantName)
			}
		})
	}
}

func TestParseCallbackDataRejectsForeignData(t *testing.T) {
	for _, d := range []string{"", "notmenu", "exec:exit", "random:stuff"} {
		if got := ParseCallbackData(d); got != nil {
			t.Errorf("ParseCallbackData(%q) = %+v, want nil", d, got)
		}
	}
}

// Telegram rejects callback_data longer than 64 bytes, which would make the
// whole keyboard fail to send.
func TestCallbackDataWithinTelegramLimit(t *testing.T) {
	mb := &MenuBuilder{config: testConfig()}
	all := map[string][]string{
		"main":          buttonData(mb.GetMainMenuInlineKeyboard()),
		"resourceTypes": buttonData(mb.GetResourceTypeInlineKeyboard()),
		"monitor":       buttonData(mb.GetMonitorInlineKeyboard()),
		"operations":    buttonData(mb.GetOperationsInlineKeyboard()),
		"settings":      buttonData(mb.GetSettingsInlineKeyboard()),
	}
	for name, datas := range all {
		for _, d := range datas {
			if len(d) > 64 {
				t.Errorf("%s: callback_data %q is %d bytes, exceeds Telegram's 64-byte limit",
					name, d, len(d))
			}
		}
	}
}

// The reply keyboard labels must match what handleReplyKeyboard switches on, or
// tapping a bottom-bar button silently falls through to the main menu.
func TestReplyKeyboardLabelsAreNonEmpty(t *testing.T) {
	mb := &MenuBuilder{config: testConfig()}
	kb := mb.GetMainReplyKeyboard()
	var count int
	for _, row := range kb.Keyboard {
		for _, btn := range row {
			if strings.TrimSpace(btn.Text) == "" {
				t.Error("reply keyboard has a blank button label")
			}
			count++
		}
	}
	if count == 0 {
		t.Fatal("main reply keyboard has no buttons")
	}
}
