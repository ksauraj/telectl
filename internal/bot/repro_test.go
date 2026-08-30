package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/ksauraj/telectl/internal/menus"
)

// TestReplyKeyboardLabelsReachHandlers reproduces the reported regression where
// the bottom-bar reply-keyboard buttons (Resources / Logs / Exec) stopped doing
// anything when tapped. A reply keyboard sends its label back as message text,
// so handleReplyKeyboard must match the exact rendered string.
func TestReplyKeyboardLabelsReachHandlers(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	cases := []struct {
		label string
		want  string
	}{
		{menus.LabelResources, "Resources"},
		{menus.LabelLogs, "Usage"},
		{menus.LabelExec, "Usage"},
		{menus.LabelMonitor, "Monitoring"},
		{menus.LabelOperations, "Operations"},
		{menus.LabelSettings, "Settings"},
		{menus.LabelHelp, "command reference"},
	}

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			fake.reset()
			lib.ProcessUpdate(context.Background(), textMessage(c.label, 7))
			if !fake.sawText(c.want) {
				t.Errorf("tap %q did not produce %q; texts %v", c.label, c.want, fake.allTexts())
			}
		})
	}
}

// TestHelpMenuButtonsReachHandlers reproduces the reported regression where
// most help-menu inline buttons stopped working. Each category button must
// render its section into the pane.
func TestHelpMenuButtonsReachHandlers(t *testing.T) {
	fake := newFakeTelegram(t)
	b, lib := newTestBot(t, fake)
	b.registerUpdateHandlers(lib)

	cases := []struct {
		data string
		want string
	}{
		{"menu:help:resources", "Resource Commands"},
		{"menu:help:workloads", "Workload Commands"},
		{"menu:help:networking", "Networking Commands"},
		{"menu:help:storage", "Storage Commands"},
		{"menu:help:monitoring", "Monitoring Commands"},
		{"menu:help:operations", "Operations Commands"},
		{"menu:help:settings", "Settings Commands"},
		{"menu:help:all", "command reference"},
	}

	for _, c := range cases {
		t.Run(c.data, func(t *testing.T) {
			fake.reset()
			lib.ProcessUpdate(context.Background(), callbackUpdate(c.data, 7))
			if !fake.sawText(c.want) {
				t.Errorf("callback %q did not produce %q; texts %v", c.data, c.want, fake.allTexts())
			}
		})
	}
}

func (f *fakeTelegram) sawText(want string) bool {
	for _, s := range f.allTexts() {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}
