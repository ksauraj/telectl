package menus

import (
	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/k8s-telegram-bot/internal/config"
	"github.com/ksauraj/k8s-telegram-bot/internal/k8s"
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
func (mb *MenuBuilder) GetBotCommands() []tgbotapi.BotCommand {
	return []tgbotapi.BotCommand{
		{Command: "resources", Description: "📦 Browse Resources"},
		{Command: "logs", Description: "📋 View Logs"},
		{Command: "exec", Description: "🖥️ Execute Commands"},
		{Command: "contexts", Description: "⚙️ Manage Contexts"},
		{Command: "monitor", Description: "📊 Monitoring"},
		{Command: "operations", Description: "🔧 Operations"},
		{Command: "settings", Description: "⚙️ Settings"},
		{Command: "help", Description: "❓ Help & Commands"},
	}
}

// ============================================================================
// Reply Keyboard (Persistent bottom bar)
// ============================================================================

// GetMainReplyKeyboard returns the main persistent reply keyboard
func (mb *MenuBuilder) GetMainReplyKeyboard() tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📦 Resources"),
			tgbotapi.NewKeyboardButton("📋 Logs"),
			tgbotapi.NewKeyboardButton("🖥️ Exec"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🔌 Port Forward"),
			tgbotapi.NewKeyboardButton("⚙️ Contexts"),
			tgbotapi.NewKeyboardButton("📊 Monitor"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🔧 Operations"),
			tgbotapi.NewKeyboardButton("⚙️ Settings"),
			tgbotapi.NewKeyboardButton("❓ Help"),
		),
	)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false
	keyboard.InputFieldPlaceholder = "Choose an action or type a command..."
	return keyboard
}

// GetResourceTypeReplyKeyboard returns keyboard for resource type selection
func (mb *MenuBuilder) GetResourceTypeReplyKeyboard() tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📦 Pods"),
			tgbotapi.NewKeyboardButton("🚀 Deployments"),
			tgbotapi.NewKeyboardButton("🔌 Services"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📋 ReplicaSets"),
			tgbotapi.NewKeyboardButton("📁 Namespaces"),
			tgbotapi.NewKeyboardButton("🖥️ Nodes"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⚙️ ConfigMaps"),
			tgbotapi.NewKeyboardButton("🔐 Secrets"),
			tgbotapi.NewKeyboardButton("💾 PVCs"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🌐 Ingresses"),
			tgbotapi.NewKeyboardButton("📅 Events"),
			tgbotapi.NewKeyboardButton("🔙 Back"),
		),
	)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false
	keyboard.InputFieldPlaceholder = "Select resource type..."
	return keyboard
}

// GetNamespaceReplyKeyboard returns keyboard for namespace selection
func (mb *MenuBuilder) GetNamespaceReplyKeyboard(namespaces []string, currentNS string) tgbotapi.ReplyKeyboardMarkup {
	var rows [][]tgbotapi.KeyboardButton
	var currentRow []tgbotapi.KeyboardButton

	// Add "All Namespaces" button
	currentRow = append(currentRow, tgbotapi.NewKeyboardButton("🌐 All Namespaces"))

	for _, ns := range namespaces {
		label := ns
		if ns == currentNS {
			label = "✅ " + ns
		}
		currentRow = append(currentRow, tgbotapi.NewKeyboardButton(label))

		if len(currentRow) >= 3 {
			rows = append(rows, currentRow)
			currentRow = []tgbotapi.KeyboardButton{}
		}
	}

	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}

	rows = append(rows, tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("🔙 Back"),
		tgbotapi.NewKeyboardButton("➕ New Namespace"),
	))

	keyboard := tgbotapi.NewReplyKeyboard(rows...)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false
	keyboard.InputFieldPlaceholder = "Select namespace..."
	return keyboard
}

// ============================================================================
// Inline Keyboards (Navigation & Actions)
// ============================================================================

// GetResourceTypeInlineKeyboard returns inline keyboard for resource type selection
func (mb *MenuBuilder) GetResourceTypeInlineKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📦 Pods", "menu:resource:pods"),
			tgbotapi.NewInlineKeyboardButtonData("🚀 Deployments", "menu:resource:deployments"),
			tgbotapi.NewInlineKeyboardButtonData("🔌 Services", "menu:resource:services"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 ReplicaSets", "menu:resource:replicasets"),
			tgbotapi.NewInlineKeyboardButtonData("📁 Namespaces", "menu:resource:namespaces"),
			tgbotapi.NewInlineKeyboardButtonData("🖥️ Nodes", "menu:resource:nodes"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚙️ ConfigMaps", "menu:resource:configmaps"),
			tgbotapi.NewInlineKeyboardButtonData("🔐 Secrets", "menu:resource:secrets"),
			tgbotapi.NewInlineKeyboardButtonData("💾 PVCs", "menu:resource:pvcs"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🌐 Ingresses", "menu:resource:ingresses"),
			tgbotapi.NewInlineKeyboardButtonData("📅 Events", "menu:resource:events"),
			tgbotapi.NewInlineKeyboardButtonData("💾 PVs", "menu:resource:pvs"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Main Menu", "menu:main"),
		),
	)
}

// GetResourceListInlineKeyboard returns paginated inline keyboard for resource list
func (mb *MenuBuilder) GetResourceListInlineKeyboard(resourceType string, resources []k8s.ResourceInfo, page, pageSize int, namespace string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	start := page * pageSize
	end := start + pageSize
	if end > len(resources) {
		end = len(resources)
	}

	// Resource rows (2 per row for better mobile UX)
	for i := start; i < end; i += 2 {
		var row []tgbotapi.InlineKeyboardButton
		r1 := resources[i]
		btn1 := tgbotapi.NewInlineKeyboardButtonData(
			mb.formatResourceButton(r1),
			"menu:resource:view:"+resourceType+":"+r1.Namespace+":"+r1.Name,
		)
		row = append(row, btn1)

		if i+1 < end {
			r2 := resources[i+1]
			btn2 := tgbotapi.NewInlineKeyboardButtonData(
				mb.formatResourceButton(r2),
				"menu:resource:view:"+resourceType+":"+r2.Namespace+":"+r2.Name,
			)
			row = append(row, btn2)
		}
		rows = append(rows, row)
	}

	// Pagination row
	var paginationRow []tgbotapi.InlineKeyboardButton
	totalPages := (len(resources) + pageSize - 1) / pageSize

	if page > 0 {
		paginationRow = append(paginationRow, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", "menu:resource:page:"+resourceType+":"+namespace+":"+intToString(page-1)))
	}

	paginationRow = append(paginationRow, tgbotapi.NewInlineKeyboardButtonData(
		"📄 "+intToString(page+1)+"/"+intToString(totalPages),
		"menu:noop",
	))

	if page+1 < totalPages {
		paginationRow = append(paginationRow, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", "menu:resource:page:"+resourceType+":"+namespace+":"+intToString(page+1)))
	}

	if len(paginationRow) > 1 {
		rows = append(rows, paginationRow)
	}

	// Action row
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "menu:resource:refresh:"+resourceType+":"+namespace),
		tgbotapi.NewInlineKeyboardButtonData("🏷️ Filter", "menu:resource:filter:"+resourceType+":"+namespace),
		tgbotapi.NewInlineKeyboardButtonData("🔙 Types", "menu:resource:types"),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
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
func (mb *MenuBuilder) GetResourceActionInlineKeyboard(resourceType, namespace, name string, resource *k8s.ResourceInfo) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Common actions for all resources
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📝 Describe", "menu:action:describe:"+resourceType+":"+namespace+":"+name),
		tgbotapi.NewInlineKeyboardButtonData("🗑️ Delete", "menu:action:delete:"+resourceType+":"+namespace+":"+name),
	))

	// Resource-specific actions
	switch resourceType {
	case "pods":
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Logs", "menu:action:logs:"+namespace+":"+name),
			tgbotapi.NewInlineKeyboardButtonData("🖥️ Exec", "menu:action:exec:"+namespace+":"+name),
			tgbotapi.NewInlineKeyboardButtonData("🔌 Port Forward", "menu:action:portforward:"+namespace+":"+name),
		))
		if resource != nil {
			// Add container selection if multiple containers
			if containers, ok := resource.Details["spec"].(map[string]interface{})["containers"].([]interface{}); ok && len(containers) > 1 {
				var containerRow []tgbotapi.InlineKeyboardButton
				for _, c := range containers {
					if container, ok := c.(map[string]interface{}); ok {
						if cName, ok := container["name"].(string); ok {
							containerRow = append(containerRow, tgbotapi.NewInlineKeyboardButtonData("📋 "+cName, "menu:action:logs:"+namespace+":"+name+":"+cName))
						}
					}
				}
				if len(containerRow) > 0 {
					rows = append(rows, containerRow)
				}
			}
		}

	case "deployments":
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Restart", "menu:action:restart:"+namespace+":"+name),
			tgbotapi.NewInlineKeyboardButtonData("📈 Scale", "menu:action:scale:"+namespace+":"+name),
			tgbotapi.NewInlineKeyboardButtonData("📋 Pods", "menu:action:pods:"+namespace+":"+name),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Edit", "menu:action:edit:"+resourceType+":"+namespace+":"+name),
			tgbotapi.NewInlineKeyboardButtonData("📜 History", "menu:action:history:"+namespace+":"+name),
		))

	case "services":
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔌 Port Forward", "menu:action:portforward:"+namespace+":"+name),
			tgbotapi.NewInlineKeyboardButtonData("📋 Endpoints", "menu:action:endpoints:"+namespace+":"+name),
		))

	case "nodes":
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Top", "menu:action:top:node:"+name),
			tgbotapi.NewInlineKeyboardButtonData("📋 Pods", "menu:action:nodepods:"+name),
			tgbotapi.NewInlineKeyboardButtonData("🔧 Cordon", "menu:action:cordon:"+name),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔓 Uncordon", "menu:action:uncordon:"+name),
			tgbotapi.NewInlineKeyboardButtonData("💤 Drain", "menu:action:drain:"+name),
		))

	case "namespaces":
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Resources", "menu:action:nsresources:"+name),
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Delete", "menu:action:delete:namespace::"+name),
		))

	case "replicasets":
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Pods", "menu:action:rspods:"+namespace+":"+name),
			tgbotapi.NewInlineKeyboardButtonData("📈 Scale", "menu:action:rsscale:"+namespace+":"+name),
		))
	}

	// Navigation
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "menu:resource:view:"+resourceType+":"+namespace+":"+name),
		tgbotapi.NewInlineKeyboardButtonData("🔙 List", "menu:resource:list:"+resourceType+":"+namespace),
		tgbotapi.NewInlineKeyboardButtonData("🏠 Main", "menu:main"),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// GetContextsInlineKeyboard returns inline keyboard for context management
func (mb *MenuBuilder) GetContextsInlineKeyboard(contexts []k8s.ContextInfo) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, ctx := range contexts {
		label := ctx.Name
		if ctx.Current {
			label = "✅ " + label
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, "menu:ctx:switch:"+ctx.Name),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "menu:ctx:refresh"),
		tgbotapi.NewInlineKeyboardButtonData("🏠 Main", "menu:main"),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// GetMonitorInlineKeyboard returns inline keyboard for monitoring
func (mb *MenuBuilder) GetMonitorInlineKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Top Pods", "menu:monitor:top:pods"),
			tgbotapi.NewInlineKeyboardButtonData("🖥️ Top Nodes", "menu:monitor:top:nodes"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 Events", "menu:monitor:events"),
			tgbotapi.NewInlineKeyboardButtonData("👁️ Watch", "menu:monitor:watch"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Main", "menu:main"),
		),
	)
}

// GetOperationsInlineKeyboard returns inline keyboard for operations
func (mb *MenuBuilder) GetOperationsInlineKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Restart Deployment", "menu:ops:restart"),
			tgbotapi.NewInlineKeyboardButtonData("📈 Scale Deployment", "menu:ops:scale"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Delete Resource", "menu:ops:delete"),
			tgbotapi.NewInlineKeyboardButtonData("✏️ Edit Resource", "menu:ops:edit"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Main", "menu:main"),
		),
	)
}

// GetSettingsInlineKeyboard returns inline keyboard for settings
func (mb *MenuBuilder) GetSettingsInlineKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🌐 Namespace", "menu:settings:namespace"),
			tgbotapi.NewInlineKeyboardButtonData("⚙️ Context", "menu:settings:context"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎨 Theme", "menu:settings:theme"),
			tgbotapi.NewInlineKeyboardButtonData("🔔 Notifications", "menu:settings:notifications"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Main", "menu:main"),
		),
	)
}

// GetConfirmDeleteKeyboard returns confirmation keyboard for delete actions
func (mb *MenuBuilder) GetConfirmDeleteKeyboard(resourceType, namespace, name string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Yes, Delete", "menu:action:confirmdelete:"+resourceType+":"+namespace+":"+name),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "menu:resource:view:"+resourceType+":"+namespace+":"+name),
		),
	)
}

// GetScaleKeyboard returns keyboard for scaling deployments
func (mb *MenuBuilder) GetScaleKeyboard(namespace, name string, currentReplicas int32) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Quick scale buttons
	quickScales := []int32{0, 1, 2, 3, 5, 10}
	var scaleRow []tgbotapi.InlineKeyboardButton
	for _, r := range quickScales {
		label := intToString(int(r))
		if r == currentReplicas {
			label = "✅ " + label
		}
		scaleRow = append(scaleRow, tgbotapi.NewInlineKeyboardButtonData(label, "menu:action:scaleset:"+namespace+":"+name+":"+intToString(int(r))))
		if len(scaleRow) == 3 {
			rows = append(rows, scaleRow)
			scaleRow = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(scaleRow) > 0 {
		rows = append(rows, scaleRow)
	}

	// Custom scale
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✏️ Custom", "menu:action:scalecustom:"+namespace+":"+name),
		tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "menu:resource:view:deployments:"+namespace+":"+name),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// GetLogOptionsKeyboard returns keyboard for log options
func (mb *MenuBuilder) GetLogOptionsKeyboard(namespace, name, container string) tgbotapi.InlineKeyboardMarkup {
	followText := "👁️ Follow"
	// We can't track follow state here, but the handler can

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Last 50", "menu:action:logs:"+namespace+":"+name+":"+container+":50"),
			tgbotapi.NewInlineKeyboardButtonData("📋 Last 100", "menu:action:logs:"+namespace+":"+name+":"+container+":100"),
			tgbotapi.NewInlineKeyboardButtonData("📋 Last 500", "menu:action:logs:"+namespace+":"+name+":"+container+":500"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(followText, "menu:action:logsfollow:"+namespace+":"+name+":"+container),
			tgbotapi.NewInlineKeyboardButtonData("⏮️ Previous", "menu:action:logsprevious:"+namespace+":"+name+":"+container),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "menu:resource:view:pods:"+namespace+":"+name),
		),
	)
}

// ============================================================================
// Callback Data Parsing
// ============================================================================

// CallbackAction represents a parsed callback action
type CallbackAction struct {
	Type       string
	ResourceType string
	Namespace  string
	Name       string
	Container  string
	Page       int
	Action     string
	Extra      string
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
		if len(parts) >= 4 {
			action.Action = parts[2] // view, page, list, refresh, filter, types
			action.ResourceType = parts[3]
		}
		if len(parts) >= 5 {
			action.Namespace = parts[4]
		}
		if len(parts) >= 6 {
			action.Name = parts[5]
		}
		if len(parts) >= 7 {
			action.Extra = parts[6]
		}

	case "action":
		if len(parts) >= 4 {
			action.Action = parts[2] // describe, delete, logs, exec, portforward, restart, scale, etc.
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