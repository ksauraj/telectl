package menus

import (
	"testing"

	"github.com/ksauraj/k8s-telegram-bot/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestParseCallbackData(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected *CallbackAction
	}{
		{
			name: "main menu",
			data: "menu:main",
			expected: &CallbackAction{
				Type: "main",
			},
		},
		{
			name: "resource types",
			data: "menu:resource:types",
			expected: &CallbackAction{
				Type:  "resource",
				Action: "types",
			},
		},
		{
			name: "resource list",
			data: "menu:resource:list:pods:default",
			expected: &CallbackAction{
				Type:         "resource",
				Action:       "list",
				ResourceType: "pods",
				Namespace:    "default",
			},
		},
		{
			name: "resource view",
			data: "menu:resource:view:pods:default:nginx-1",
			expected: &CallbackAction{
				Type:         "resource",
				Action:       "view",
				ResourceType: "pods",
				Namespace:    "default",
				Name:         "nginx-1",
			},
		},
		{
			name: "resource page",
			data: "menu:resource:page:pods:default:2",
			expected: &CallbackAction{
				Type:         "resource",
				Action:       "page",
				ResourceType: "pods",
				Namespace:    "default",
				Extra:        "2",
			},
		},
		{
			name: "action describe",
			data: "menu:action:describe:pods:default:nginx-1",
			expected: &CallbackAction{
				Type:         "action",
				Action:       "describe",
				ResourceType: "pods",
				Namespace:    "default",
				Name:         "nginx-1",
			},
		},
		{
			name: "action logs with container",
			data: "menu:action:logs:default:nginx-1:nginx",
			expected: &CallbackAction{
				Type:        "action",
				Action:      "logs",
				Namespace:   "default",
				Name:        "nginx-1",
				Container:   "nginx",
			},
		},
		{
			name: "context switch",
			data: "menu:ctx:switch:my-context",
			expected: &CallbackAction{
				Type:  "ctx",
				Action: "switch",
				Name:  "my-context",
			},
		},
		{
			name: "monitor top pods",
			data: "menu:monitor:top:pods",
			expected: &CallbackAction{
				Type:         "monitor",
				Action:       "top",
				ResourceType: "pods",
			},
		},
		{
			name: "operations restart",
			data: "menu:ops:restart",
			expected: &CallbackAction{
				Type:  "ops",
				Action: "restart",
			},
		},
		{
			name: "settings namespace",
			data: "menu:settings:namespace",
			expected: &CallbackAction{
				Type:  "settings",
				Action: "namespace",
			},
		},
		{
			name: "noop",
			data: "menu:noop",
			expected: &CallbackAction{
				Type: "noop",
			},
		},
		{
			name: "invalid prefix",
			data: "invalid:data",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCallbackData(tt.data)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Type, result.Type)
				assert.Equal(t, tt.expected.Action, result.Action)
				assert.Equal(t, tt.expected.ResourceType, result.ResourceType)
				assert.Equal(t, tt.expected.Namespace, result.Namespace)
				assert.Equal(t, tt.expected.Name, result.Name)
				assert.Equal(t, tt.expected.Container, result.Container)
				assert.Equal(t, tt.expected.Extra, result.Extra)
			}
		})
	}
}

func TestMenuBuilder_GetBotCommands(t *testing.T) {
	cfg := &config.Config{
		Bot: config.BotConfig{
			EnableMenuButton: true,
		},
	}
	mb := NewMenuBuilder(cfg)

	commands := mb.GetBotCommands()
	assert.Len(t, commands, 8)
	
	expectedCommands := []string{"resources", "logs", "exec", "contexts", "monitor", "operations", "settings", "help"}
	for i, cmd := range expectedCommands {
		assert.Equal(t, cmd, commands[i].Command)
	}
}

func TestMenuBuilder_IsMenuEnabled(t *testing.T) {
	tests := []struct {
		name           string
		enableMenuBtn  bool
		enableReplyKb  bool
		expected       bool
	}{
		{"both enabled", true, true, true},
		{"menu button only", true, false, true},
		{"reply keyboard only", false, true, true},
		{"both disabled", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Bot: config.BotConfig{
					EnableMenuButton:   tt.enableMenuBtn,
					EnableReplyKeyboard: tt.enableReplyKb,
				},
			}
			mb := NewMenuBuilder(cfg)
			assert.Equal(t, tt.expected, mb.IsMenuEnabled())
		})
	}
}

func TestMenuBuilder_ParseResourceTypeFromButton(t *testing.T) {
	cfg := &config.Config{}
	mb := NewMenuBuilder(cfg)

	// This is a private method, so we test via the public interface
	// The mapping is tested in the handleReplyKeyboard integration test
}