package menus

import (
	"github.com/ksauraj/telectl/internal/config"
	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/pkg/kubeconfig"
)

// MenuBuilder builds various Telegram keyboards for the bot
type MenuBuilder struct {
	config *config.Config
}

func NewMenuBuilder(cfg *config.Config) *MenuBuilder {
	return &MenuBuilder{config: cfg}
}

// ============================================================================
// Bot Menu Commands (Persistent header menu)
// ============================================================================

// GetBotCommands returns the list of commands for the bot menu button
func (mb *MenuBuilder) GetBotCommands() []tg.BotCommand {
	return []tg.BotCommand{
		tg.BotCommand{Command: "resources", Description: "📦 Browse Resources"},
		tg.BotCommand{Command: "logs", Description: "📋 View Logs"},
		tg.BotCommand{Command: "exec", Description: "🖥️ Execute Commands"},
		tg.BotCommand{Command: "contexts", Description: "⚙️ Manage Contexts"},
		tg.BotCommand{Command: "monitor", Description: "📊 Monitoring"},
		tg.BotCommand{Command: "operations", Description: "🔧 Operations"},
		tg.BotCommand{Command: "settings", Description: "⚙️ Settings"},
		tg.BotCommand{Command: "help", Description: "❓ Help & Commands"},
	}
}

// ============================================================================
// Reply Keyboard (Persistent bottom bar)
// ============================================================================

// GetMainReplyKeyboard returns the main persistent reply keyboard
func (mb *MenuBuilder) GetMainReplyKeyboard() tg.ReplyKeyboardMarkup {
	keyboard := tg.ReplyKeyboard(
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText("📦 Resources"),
			tg.KeyboardButtonText("📋 Logs"),
			tg.KeyboardButtonText("🖥️ Exec"),
		),
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText("🔌 Port Forward"),
			tg.KeyboardButtonText("⚙️ Contexts"),
			tg.KeyboardButtonText("📊 Monitor"),
		),
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText("🔧 Operations"),
			tg.KeyboardButtonText("⚙️ Settings"),
			tg.KeyboardButtonText("❓ Help"),
		),
	)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false
	keyboard.InputFieldPlaceholder = "Choose an action or type a command..."
	return keyboard
}

// GetResourceTypeReplyKeyboard returns keyboard for resource type selection
func (mb *MenuBuilder) GetResourceTypeReplyKeyboard() tg.ReplyKeyboardMarkup {
	keyboard := tg.ReplyKeyboard(
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText("📦 Pods"),
			tg.KeyboardButtonText("🚀 Deployments"),
			tg.KeyboardButtonText("🔌 Services"),
		),
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText("📋 ReplicaSets"),
			tg.KeyboardButtonText("📁 Namespaces"),
			tg.KeyboardButtonText("🖥️ Nodes"),
		),
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText("⚙️ ConfigMaps"),
			tg.KeyboardButtonText("🔐 Secrets"),
			tg.KeyboardButtonText("💾 PVCs"),
		),
		tg.KeyboardButtonRow(
			tg.KeyboardButtonText("🌐 Ingresses"),
			tg.KeyboardButtonText("📅 Events"),
			tg.KeyboardButtonText("🔙 Back"),
		),
	)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false
	keyboard.InputFieldPlaceholder = "Select resource type..."
	return keyboard
}

// GetNamespaceReplyKeyboard returns keyboard for namespace selection
func (mb *MenuBuilder) GetNamespaceReplyKeyboard(namespaces []string, currentNS string) tg.ReplyKeyboardMarkup {
	var rows [][]tg.KeyboardButton
	var currentRow []tg.KeyboardButton

	// Add "All Namespaces" button
	currentRow = append(currentRow, tg.KeyboardButtonText("🌐 All Namespaces"))

	for _, ns := range namespaces {
		label := ns
		if ns == currentNS {
			label = "✅ " + ns
		}
		currentRow = append(currentRow, tg.KeyboardButtonText(label))

		if len(currentRow) >= 3 {
			rows = append(rows, currentRow)
			currentRow = []tg.KeyboardButton{}
		}
	}

	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}

	rows = append(rows, tg.KeyboardButtonRow(
		tg.KeyboardButtonText("🔙 Back"),
		tg.KeyboardButtonText("➕ New Namespace"),
	))

	keyboard := tg.ReplyKeyboard(rows...)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false
	keyboard.InputFieldPlaceholder = "Select namespace..."
	return keyboard
}

// ============================================================================
// Inline Keyboards (Navigation & Actions)
// ============================================================================

// GetResourceTypeInlineKeyboard returns inline keyboard for resource type selection
func (mb *MenuBuilder) GetResourceTypeInlineKeyboard() tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			tg.InlineButtonData("📦 Pods", "menu:resource:pods"),
			tg.InlineButtonData("🚀 Deployments", "menu:resource:deployments"),
			tg.InlineButtonData("🔌 Services", "menu:resource:services"),
		),
		tg.InlineKeyboardRow(
			tg.InlineButtonData("📋 ReplicaSets", "menu:resource:replicasets"),
			tg.InlineButtonData("📁 Namespaces", "menu:resource:namespaces"),
			tg.InlineButtonData("🖥️ Nodes", "menu:resource:nodes"),
		),
		tg.InlineKeyboardRow(
			tg.InlineButtonData("⚙️ ConfigMaps", "menu:resource:configmaps"),
			tg.InlineButtonData("🔐 Secrets", "menu:resource:secrets"),
			tg.InlineButtonData("💾 PVCs", "menu:resource:pvcs"),
		),
		tg.InlineKeyboardRow(
			tg.InlineButtonData("🌐 Ingresses", "menu:resource:ingresses"),
			tg.InlineButtonData("📅 Events", "menu:resource:events"),
			tg.InlineButtonData("💾 PVs", "menu:resource:pvs"),
		),
		tg.InlineKeyboardRow(
			tg.InlineButtonData("🔙 Main Menu", "menu:main"),
		),
	)
}

// GetResourceListInlineKeyboard returns paginated inline keyboard for resource list
func (mb *MenuBuilder) GetResourceListInlineKeyboard(resourceType string, resources []k8s.ResourceInfo, page, pageSize int, namespace string) tg.InlineKeyboardMarkup {
	var rows [][]tg.InlineKeyboardButton

	start := page * pageSize
	end := start + pageSize
	if end > len(resources) {
		end = len(resources)
	}

	// Resource rows (2 per row for better mobile UX)
	for i := start; i < end; i += 2 {
		var row []tg.InlineKeyboardButton
		r1 := resources[i]
		btn1 := tg.InlineButtonData(
			mb.formatResourceButton(r1),
			"menu:resource:view:"+resourceType+":"+r1.Namespace+":"+r1.Name,
		)
		row = append(row, btn1)

		if i+1 < end {
			r2 := resources[i+1]
			btn2 := tg.InlineButtonData(
				mb.formatResourceButton(r2),
				"menu:resource:view:"+resourceType+":"+r2.Namespace+":"+r2.Name,
			)
			row = append(row, btn2)
		}
		rows = append(rows, row)
	}

	// Pagination row
	var paginationRow []tg.InlineKeyboardButton
	totalPages := (len(resources) + pageSize - 1) / pageSize

	if page > 0 {
		paginationRow = append(paginationRow, tg.InlineButtonData("⬅️ Prev", "menu:resource:page:"+resourceType+":"+namespace+":"+intToString(page-1)))
	}

	paginationRow = append(paginationRow, tg.InlineButtonData(
		"📄 "+intToString(page+1)+"/"+intToString(totalPages),
		"menu:noop",
	))

	if page+1 < totalPages {
		paginationRow = append(paginationRow, tg.InlineButtonData("Next ➡️", "menu:resource:page:"+resourceType+":"+namespace+":"+intToString(page+1)))
	}

	if len(paginationRow) > 1 {
		rows = append(rows, paginationRow)
	}

	// Action row
	rows = append(rows, tg.InlineKeyboardRow(
		tg.InlineButtonData("🔄 Refresh", "menu:resource:refresh:"+resourceType+":"+namespace),
		tg.InlineButtonData("🏷️ Filter", "menu:resource:filter:"+resourceType+":"+namespace),
		tg.InlineButtonData("🔙 Types", "menu:resource:types"),
	))

	return tg.InlineKeyboard(rows...)
}

func (mb *MenuBuilder) formatResourceButton(r k8s.ResourceInfo) string {
	statusIcon := "⚪"
	switch r.Status {
	case "Running", "Active", "Ready":
		statusIcon = "🟢"
	case "Pending", "Creating":
		statusIcon = "🟡"
	case "Failed", "Error", "CrashLoopBackOff":
		statusIcon = "🔴"
	case "Succeeded", "Completed":
		statusIcon = "🔵"
	case "Terminating":
		statusIcon = "🟠"
	}

	name := r.Name
	if len(name) > 20 {
		name = name[:17] + "..."
	}
	return statusIcon + " " + name
}

// GetResourceActionInlineKeyboard returns action buttons for a specific resource
func (mb *MenuBuilder) GetResourceActionInlineKeyboard(resourceType, namespace, name string, resource *k8s.ResourceInfo) tg.InlineKeyboardMarkup {
	var rows [][]tg.InlineKeyboardButton

	// Common actions for all resources
	rows = append(rows, tg.InlineKeyboardRow(
		tg.InlineButtonData("📝 Describe", "menu:action:describe:"+resourceType+":"+namespace+":"+name),
		tg.InlineButtonData("🗑️ Delete", "menu:action:delete:"+resourceType+":"+namespace+":"+name),
	))

	// Resource-specific actions
	switch resourceType {
	case "pods":
		rows = append(rows, tg.InlineKeyboardRow(
			tg.InlineButtonData("📋 Logs", "menu:action:logs:"+namespace+":"+name),
			tg.InlineButtonData("🖥️ Exec", "menu:action:exec:"+namespace+":"+name),
			tg.InlineButtonData("🔌 Port Forward", "menu:action:portforward:"+namespace+":"+name),
		))
		if resource != nil {
			// Add container selection if multiple containers
			if containers, ok := resource.Details["spec"].(map[string]interface{})["containers"].([]interface{}); ok && len(containers) > 1 {
				var containerRow []tg.InlineKeyboardButton
				for _, c := range containers {
					if container, ok := c.(map[string]interface{}); ok {
						if cName, ok := container["name"].(string); ok {
							containerRow = append(containerRow, tg.InlineButtonData("📋 "+cName, "menu:action:logs:"+namespace+":"+name+":"+cName))
						}
					}
				}
				if len(containerRow) > 0 {
					rows = append(rows, containerRow)
				}
			}
		}

	case "deployments":
		rows = append(rows, tg.InlineKeyboardRow(
			tg.InlineButtonData("🔄 Restart", "menu:action:restart:"+namespace+":"+name),
			tg.InlineButtonData("📈 Scale", "menu:action:scale:"+namespace+":"+name),
			tg.InlineButtonData("📋 Pods", "menu:action:pods:"+namespace+":"+name),
		))
		rows = append(rows, tg.InlineKeyboardRow(
			tg.InlineButtonData("📝 Edit", "menu:action:edit:"+resourceType+":"+namespace+":"+name),
			tg.InlineButtonData("📜 History", "menu:action:history:"+namespace+":"+name),
		))

	case "services":
		rows = append(rows, tg.InlineKeyboardRow(
			tg.InlineButtonData("🔌 Port Forward", "menu:action:portforward:"+namespace+":"+name),
			tg.InlineButtonData("📋 Endpoints", "menu:action:endpoints:"+namespace+":"+name),
		))

	case "nodes":
		rows = append(rows, tg.InlineKeyboardRow(
			tg.InlineButtonData("📊 Top", "menu:action:top:node:"+name),
			tg.InlineButtonData("📋 Pods", "menu:action:nodepods:"+name),
			tg.InlineButtonData("🔧 Cordon", "menu:action:cordon:"+name),
		))
		rows = append(rows, tg.InlineKeyboardRow(
			tg.InlineButtonData("🔓 Uncordon", "menu:action:uncordon:"+name),
			tg.InlineButtonData("💤 Drain", "menu:action:drain:"+name),
		))

	case "namespaces":
		rows = append(rows, tg.InlineKeyboardRow(
			tg.InlineButtonData("📋 Resources", "menu:action:nsresources:"+name),
			tg.InlineButtonData("🗑️ Delete", "menu:action:delete:namespace::"+name),
		))

	case "replicasets":
		rows = append(rows, tg.InlineKeyboardRow(
			tg.InlineButtonData("📋 Pods", "menu:action:rspods:"+namespace+":"+name),
			tg.InlineButtonData("📈 Scale", "menu:action:rsscale:"+namespace+":"+name),
		))
	}

	// Navigation
	rows = append(rows, tg.InlineKeyboardRow(
		tg.InlineButtonData("🔄 Refresh", "menu:resource:view:"+resourceType+":"+namespace+":"+name),
		tg.InlineButtonData("🔙 List", "menu:resource:list:"+resourceType+":"+namespace),
		tg.InlineButtonData("🏠 Main", "menu:main"),
	))

	return tg.InlineKeyboard(rows...)
}

// GetContextsInlineKeyboard returns inline keyboard for context management
func (mb *MenuBuilder) GetContextsInlineKeyboard(contexts []kubeconfig.ContextInfo) tg.InlineKeyboardMarkup {
	var rows [][]tg.InlineKeyboardButton

	for _, ctx := range contexts {
		label := ctx.Name
		if ctx.Current {
			label = "✅ " + label
		}
		rows = append(rows, tg.InlineKeyboardRow(
			tg.InlineButtonData(label, "menu:ctx:switch:"+ctx.Name),
		))
	}

	rows = append(rows, tg.InlineKeyboardRow(
		tg.InlineButtonData("🔄 Refresh", "menu:ctx:refresh"),
		tg.InlineButtonData("🏠 Main", "menu:main"),
	))

	return tg.InlineKeyboard(rows...)
}

// GetMonitorInlineKeyboard returns inline keyboard for monitoring
func (mb *MenuBuilder) GetMonitorInlineKeyboard() tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			tg.InlineButtonData("📊 Top Pods", "menu:monitor:top:pods"),
			tg.InlineButtonData("🖥️ Top Nodes", "menu:monitor:top:nodes"),
		),
		tg.InlineKeyboardRow(
			tg.InlineButtonData("📅 Events", "menu:monitor:events"),
			tg.InlineButtonData("👁️ Watch", "menu:monitor:watch"),
		),
		tg.InlineKeyboardRow(
			tg.InlineButtonData("🔙 Main", "menu:main"),
		),
	)
}

// GetOperationsInlineKeyboard returns inline keyboard for operations
func (mb *MenuBuilder) GetOperationsInlineKeyboard() tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			tg.InlineButtonData("🔄 Restart Deployment", "menu:ops:restart"),
			tg.InlineButtonData("📈 Scale Deployment", "menu:ops:scale"),
		),
		tg.InlineKeyboardRow(
			tg.InlineButtonData("🗑️ Delete Resource", "menu:ops:delete"),
			tg.InlineButtonData("✏️ Edit Resource", "menu:ops:edit"),
		),
		tg.InlineKeyboardRow(
			tg.InlineButtonData("🏠 Main", "menu:main"),
		),
	)
}

// GetSettingsInlineKeyboard returns inline keyboard for settings
func (mb *MenuBuilder) GetSettingsInlineKeyboard() tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			tg.InlineButtonData("🌐 Namespace", "menu:settings:namespace"),
			tg.InlineButtonData("⚙️ Context", "menu:settings:context"),
		),
		tg.InlineKeyboardRow(
			tg.InlineButtonData("🎨 Theme", "menu:settings:theme"),
			tg.InlineButtonData("🔔 Notifications", "menu:settings:notifications"),
		),
		tg.InlineKeyboardRow(
			tg.InlineButtonData("🏠 Main", "menu:main"),
		),
	)
}

// GetConfirmDeleteKeyboard returns confirmation keyboard for delete actions
func (mb *MenuBuilder) GetConfirmDeleteKeyboard(resourceType, namespace, name string) tg.InlineKeyboardMarkup {
	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			tg.InlineButtonData("✅ Yes, Delete", "menu:action:confirmdelete:"+resourceType+":"+namespace+":"+name),
			tg.InlineButtonData("❌ Cancel", "menu:resource:view:"+resourceType+":"+namespace+":"+name),
		),
	)
}

// GetScaleKeyboard returns keyboard for scaling deployments
func (mb *MenuBuilder) GetScaleKeyboard(namespace, name string, currentReplicas int32) tg.InlineKeyboardMarkup {
	var rows [][]tg.InlineKeyboardButton

	// Quick scale buttons
	quickScales := []int32{0, 1, 2, 3, 5, 10}
	var scaleRow []tg.InlineKeyboardButton
	for _, r := range quickScales {
		label := intToString(int(r))
		if r == currentReplicas {
			label = "✅ " + label
		}
		scaleRow = append(scaleRow, tg.InlineButtonData(label, "menu:action:scaleset:"+namespace+":"+name+":"+intToString(int(r))))
		if len(scaleRow) == 3 {
			rows = append(rows, scaleRow)
			scaleRow = []tg.InlineKeyboardButton{}
		}
	}
	if len(scaleRow) > 0 {
		rows = append(rows, scaleRow)
	}

	// Custom scale
	rows = append(rows, tg.InlineKeyboardRow(
		tg.InlineButtonData("✏️ Custom", "menu:action:scalecustom:"+namespace+":"+name),
		tg.InlineButtonData("🔙 Back", "menu:resource:view:deployments:"+namespace+":"+name),
	))

	return tg.InlineKeyboard(rows...)
}

// GetLogOptionsKeyboard returns keyboard for log options
func (mb *MenuBuilder) GetLogOptionsKeyboard(namespace, name, container string) tg.InlineKeyboardMarkup {
	followText := "👁️ Follow"
	// We can't track follow state here, but the handler can

	return tg.InlineKeyboard(
		tg.InlineKeyboardRow(
			tg.InlineButtonData("📋 Last 50", "menu:action:logs:"+namespace+":"+name+":"+container+":50"),
			tg.InlineButtonData("📋 Last 100", "menu:action:logs:"+namespace+":"+name+":"+container+":100"),
			tg.InlineButtonData("📋 Last 500", "menu:action:logs:"+namespace+":"+name+":"+container+":500"),
		),
		tg.InlineKeyboardRow(
			tg.InlineButtonData(followText, "menu:action:logsfollow:"+namespace+":"+name+":"+container),
			tg.InlineButtonData("⏮️ Previous", "menu:action:logsprevious:"+namespace+":"+name+":"+container),
		),
		tg.InlineKeyboardRow(
			tg.InlineButtonData("🔙 Back", "menu:resource:view:pods:"+namespace+":"+name),
		),
	)
}

// ============================================================================
// Callback Data Parsing
// ============================================================================

// CallbackAction represents a parsed callback action
type CallbackAction struct {
	Type         string
	ResourceType string
	Namespace    string
	Name         string
	Container    string
	Page         int
	Action       string
	Extra        string
}

// ParseCallbackData parses callback data from inline keyboard buttons
func ParseCallbackData(data string) *CallbackAction {
	// Format: "menu:type:action:resource:namespace:name:extra"
	parts := splitCallbackData(data)
	if len(parts) < 2 {
		return nil
	}

	if parts[0] != "menu" {
		return nil
	}

	action := &CallbackAction{
		Type: parts[1],
	}

	switch parts[1] {
	case "resource":
		if len(parts) >= 3 {
			action.Action = parts[2] // view, page, list, refresh, filter, types
		}
		if len(parts) >= 4 {
			action.ResourceType = parts[3]
		}
		if len(parts) >= 5 {
			action.Namespace = parts[4]
		}
		if len(parts) >= 6 {
			// For "page" action, the trailing field is the page number (Extra).
			// For "view" action, it's the resource name.
			if action.Action == "page" {
				action.Extra = parts[5]
			} else {
				action.Name = parts[5]
			}
		}
		if len(parts) >= 7 {
			action.Extra = parts[6]
		}

	case "action":
		if len(parts) >= 3 {
			action.Action = parts[2] // describe, delete, logs, exec, portforward, restart, scale, etc.
		}
		if len(parts) >= 4 {
			action.ResourceType = parts[3]
		}
		if len(parts) >= 5 {
			action.Namespace = parts[4]
		}
		if len(parts) >= 6 {
			action.Name = parts[5]
		}
		if len(parts) >= 7 {
			action.Container = parts[6]
		}
		if len(parts) >= 8 {
			action.Extra = parts[7]
		}

	case "ctx":
		if len(parts) >= 3 {
			action.Action = parts[2] // switch, refresh
		}
		if len(parts) >= 4 {
			action.Name = parts[3]
		}

	case "monitor":
		if len(parts) >= 3 {
			action.Action = parts[2] // top, events, watch
		}
		if len(parts) >= 4 {
			action.ResourceType = parts[3]
		}

	case "ops":
		if len(parts) >= 3 {
			action.Action = parts[2] // restart, scale, delete, edit
		}

	case "settings":
		if len(parts) >= 3 {
			action.Action = parts[2] // namespace, context, theme, notifications
		}

	case "main", "noop":
		// No additional data needed
	}

	return action
}

func splitCallbackData(data string) []string {
	var parts []string
	current := ""
	for _, c := range data {
		if c == ':' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	parts = append(parts, current)
	return parts
}

func intToString(i int) string {
	if i == 0 {
		return "0"
	}
	var result string
	negative := false
	if i < 0 {
		negative = true
		i = -i
	}
	for i > 0 {
		result = string(rune('0'+i%10)) + result
		i /= 10
	}
	if negative {
		result = "-" + result
	}
	return result
}

// ============================================================================
// Helper Functions
// ============================================================================

// GetPageSize returns the configured page size
func (mb *MenuBuilder) GetPageSize() int {
	if mb.config.Bot.MenuPageSize > 0 {
		return mb.config.Bot.MenuPageSize
	}
	return 10
}

// IsMenuEnabled returns true if menu system is enabled
func (mb *MenuBuilder) IsMenuEnabled() bool {
	return mb.config.Bot.EnableMenuButton || mb.config.Bot.EnableReplyKeyboard
}

// IsMenuButtonEnabled returns true if menu button is enabled
func (mb *MenuBuilder) IsMenuButtonEnabled() bool {
	return mb.config.Bot.EnableMenuButton
}

// IsReplyKeyboardEnabled returns true if reply keyboard is enabled
func (mb *MenuBuilder) IsReplyKeyboardEnabled() bool {
	return mb.config.Bot.EnableReplyKeyboard
}
