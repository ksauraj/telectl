package bot

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	bottg "github.com/go-telegram/bot"
	botmodels "github.com/go-telegram/bot/models"
	"github.com/ksauraj/telectl/internal/config"
	"github.com/ksauraj/telectl/internal/handlers"
	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/menus"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Bot struct {
	tgBot        *tg.RealBot
	libBot       *bottg.Bot
	config       *config.Config
	k8sClient    *k8s.Client
	logger       *zap.Logger
	handlers     map[string]handlers.CommandHandler
	rateLimiter  *types.RateLimiter
	userSessions sync.Map
	menuBuilder  *menus.MenuBuilder
	cancelFunc   context.CancelFunc
}

func New(cfg *config.Config, logger *zap.Logger) (*Bot, error) {
	if err := config.ValidateConfig(cfg); err != nil {
		return nil, err
	}

	tgBot, err := tg.NewRealBot(cfg.Telegram.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	libBot := tgBot.LibraryBot()
	me, _ := libBot.GetMe(context.Background())
	if me != nil {
		logger.Info("Bot authorized", zap.String("username", me.Username))
	}

	k8sClient, err := k8s.NewClient(
		cfg.Kubernetes.KubeconfigPath,
		cfg.Kubernetes.Context,
		cfg.Kubernetes.DryRun,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	version, err := k8sClient.GetServerVersion(ctx)
	if err != nil {
		logger.Warn("Failed to connect to Kubernetes API", zap.Error(err))
	} else {
		logger.Info("Connected to Kubernetes", zap.String("version", version))
	}

	b := &Bot{
		tgBot:       tgBot,
		libBot:      libBot,
		config:      cfg,
		k8sClient:   k8sClient,
		logger:      logger,
		handlers:    make(map[string]handlers.CommandHandler),
		rateLimiter: types.NewRateLimiter(cfg.Bot.RateLimit, time.Minute),
		menuBuilder: menus.NewMenuBuilder(cfg),
	}
	b.registerHandlers()
	return b, nil
}

func (b *Bot) registerHandlers() {
	b.handlers["start"] = handlers.NewStartHandler(b)
	b.handlers["help"] = handlers.NewHelpHandler(b)
	b.handlers["version"] = handlers.NewVersionHandler(b)
	b.handlers["get"] = handlers.NewGetHandler(b)
	b.handlers["describe"] = handlers.NewDescribeHandler(b)
	b.handlers["logs"] = handlers.NewLogsHandler(b)
	b.handlers["exec"] = handlers.NewExecHandler(b)
	b.handlers["portforward"] = handlers.NewPortForwardHandler(b)
	b.handlers["contexts"] = handlers.NewContextsHandler(b)
	b.handlers["use-context"] = handlers.NewUseContextHandler(b)
	b.handlers["config"] = handlers.NewConfigHandler(b)
	b.handlers["top"] = handlers.NewTopHandler(b)
	b.handlers["events"] = handlers.NewEventsHandler(b)
	b.handlers["watch"] = handlers.NewWatchHandler(b)
	b.handlers["restart"] = handlers.NewRestartHandler(b)
	b.handlers["scale"] = handlers.NewScaleHandler(b)
	b.handlers["resources"] = handlers.NewResourcesHandler(b)
	b.handlers["monitor"] = handlers.NewMonitorHandler(b)
	b.handlers["operations"] = handlers.NewOperationsHandler(b)
	b.handlers["settings"] = handlers.NewSettingsHandler(b)
}

func (b *Bot) Start(ctx context.Context) error {
	ctx, b.cancelFunc = context.WithCancel(ctx)

	if b.menuBuilder.IsMenuButtonEnabled() {
		tgCommands := b.menuBuilder.GetBotCommands()
		commands := make([]botmodels.BotCommand, len(tgCommands))
		for i, c := range tgCommands {
			commands[i] = botmodels.BotCommand{Command: c.Command, Description: c.Description}
		}
		if _, err := b.libBot.SetMyCommands(ctx, &bottg.SetMyCommandsParams{Commands: commands}); err != nil {
			b.logger.Warn("Failed to set bot menu commands", zap.Error(err))
		} else {
			b.logger.Info("Bot menu commands set", zap.Int("count", len(commands)))
		}
	}

	b.registerUpdateHandlers(b.libBot)

	b.libBot.Start(ctx)
	return nil
}

// registerUpdateHandlers wires update routing. Kept separate from Start so tests
// can exercise the real registration against a stub API server.
//
// Match on update shape, not on a command pattern: an empty pattern with
// MatchTypeCommand compares the parsed command name against "" and never
// matches, and HandlerTypeMessageText ignores updates without a Message.
func (b *Bot) registerUpdateHandlers(lib *bottg.Bot) {
	lib.RegisterHandlerMatchFunc(func(update *botmodels.Update) bool {
		return update.Message != nil || update.InlineQuery != nil
	}, b.handleMessage)
	lib.RegisterHandlerMatchFunc(func(update *botmodels.Update) bool {
		return update.CallbackQuery != nil
	}, b.handleCallbackQuery)
}

func (b *Bot) Stop() {
	if b.cancelFunc != nil {
		b.cancelFunc()
	}
}

func (b *Bot) handleMessage(ctx context.Context, bot *bottg.Bot, update *botmodels.Update) {
	// Handle inline queries first
	if update.InlineQuery != nil {
		b.handleInlineQuery(ctx, update.InlineQuery)
		return
	}

	msg := update.Message
	if msg == nil {
		return
	}
	// From is optional in the Telegram API (channel posts, some forwards).
	if msg.From == nil {
		return
	}

	userID := msg.From.ID
	chatID := msg.Chat.ID

	b.logger.Debug("Received message",
		zap.Int64("user_id", userID),
		zap.Int64("chat_id", chatID),
		zap.String("text", msg.Text),
		zap.String("username", msg.From.Username),
	)

	if !b.IsUserAllowed(userID) {
		b.logger.Debug("User not allowed", zap.Int64("user_id", userID))
		if _, err := b.tgBot.SendText(ctx, chatID, "❌ You are not authorized to use this bot.", "HTML", nil); err != nil {
			b.logger.Error("Failed to send unauthorized message", zap.Error(err), zap.Int64("chat_id", chatID))
		}
		return
	}
	if !b.rateLimiter.Allow(userID) {
		if _, err := b.tgBot.SendText(ctx, chatID, "⏱️ Rate limit exceeded. Please wait a moment.", "HTML", nil); err != nil {
			b.logger.Error("Failed to send rate limit message", zap.Error(err), zap.Int64("chat_id", chatID))
		}
		return
	}

	session := b.getOrCreateSession(userID)
	session.Touch()

	if b.menuBuilder.IsReplyKeyboardEnabled() && !strings.HasPrefix(msg.Text, "/") {
		if b.handleReplyKeyboard(ctx, msg, session) {
			return
		}
	}

	if strings.HasPrefix(msg.Text, "/") {
		b.logger.Debug("Processing command", zap.String("command", msg.Text))
		b.handleCommand(ctx, msg, session)
	} else if session.IsInExecMode() {
		b.handleExecInput(ctx, msg, session)
	} else {
		b.ShowMainMenu(ctx, chatID, session)
	}
}

func (b *Bot) handleInlineQuery(ctx context.Context, iq *botmodels.InlineQuery) {
	if iq == nil {
		return
	}
	userID := iq.From.ID
	if !b.IsUserAllowed(userID) {
		return
	}
	if !b.rateLimiter.Allow(userID) {
		return
	}
	handler := handlers.NewInlineQueryHandler(b)
	// Convert botmodels.InlineQuery to tg.InlineQuery
	tgIQ := &tg.InlineQuery{
		ID:       iq.ID,
		From:     &tg.User{ID: iq.From.ID, FirstName: iq.From.FirstName, Username: iq.From.Username},
		Query:    iq.Query,
		Offset:   iq.Offset,
		ChatType: string(iq.ChatType),
	}
	if err := handler.HandleInlineQuery(ctx, tgIQ); err != nil {
		b.logger.Error("Inline query failed",
			zap.Int64("user_id", userID),
			zap.String("query", iq.Query),
			zap.Error(err),
		)
	}
}

func (b *Bot) handleCommand(ctx context.Context, msg *botmodels.Message, session *types.UserSession) {
	cmd := strings.TrimPrefix(msg.Text, "/")
	cmd = strings.Split(cmd, " ")[0]
	if !b.IsCommandAllowed(cmd) {
		if _, err := b.tgBot.SendText(ctx, msg.Chat.ID, fmt.Sprintf("❌ Command /%s is not allowed.", cmd), "HTML", nil); err != nil {
			b.logger.Error("Failed to send command not allowed", zap.Error(err), zap.Int64("chat_id", msg.Chat.ID))
		}
		return
	}
	handler, ok := b.handlers[cmd]
	if !ok {
		if _, err := b.tgBot.SendText(ctx, msg.Chat.ID, fmt.Sprintf("❌ Unknown command: /%s\nUse /help to see available commands.", cmd), "HTML", nil); err != nil {
			b.logger.Error("Failed to send unknown command", zap.Error(err), zap.Int64("chat_id", msg.Chat.ID))
		}
		return
	}
	args := strings.Fields(msg.Text)[1:]
	if err := handler.Handle(ctx, &tg.Message{
		ID:     msg.ID,
		ChatID: msg.Chat.ID,
		Text:   msg.Text,
		From:   &tg.User{ID: msg.From.ID, FirstName: msg.From.FirstName, Username: msg.From.Username},
		Chat:   &tg.Chat{ID: msg.Chat.ID, Type: string(msg.Chat.Type), Title: msg.Chat.Title},
	}, args, session); err != nil {
		b.logger.Error("Command failed", zap.String("cmd", cmd), zap.Error(err))
		if _, err := b.tgBot.SendText(ctx, msg.Chat.ID, fmt.Sprintf("❌ Error: %s", err.Error()), "HTML", nil); err != nil {
			b.logger.Error("Failed to send command error", zap.Error(err), zap.Int64("chat_id", msg.Chat.ID))
		}
	}
}

func (b *Bot) handleCallbackQuery(ctx context.Context, bot *bottg.Bot, update *botmodels.Update) {
	if update.CallbackQuery == nil {
		return
	}
	callback := update.CallbackQuery
	_, _ = bot.AnswerCallbackQuery(ctx, &bottg.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
		Text:            "",
		ShowAlert:       false,
	})
	if !b.IsUserAllowed(callback.From.ID) {
		return
	}
	// MaybeInaccessibleMessage wraps the actual Message - check Type field
	var chatID int64
	if callback.Message.Type == botmodels.MaybeInaccessibleMessageTypeMessage && callback.Message.Message != nil {
		chatID = callback.Message.Message.Chat.ID
	} else {
		return
	}
	messageID := callback.Message.Message.ID
	session := b.getOrCreateSession(callback.From.ID)
	session.Touch()

	// Long callback data is stored server-side and the button carries a short
	// token, because Telegram rejects any keyboard containing callback_data
	// over 64 bytes. Expand it before parsing.
	data, ok := b.menuBuilder.ResolveCallback(callback.Data)
	if !ok {
		b.logger.Debug("Stale callback token", zap.String("data", callback.Data))
		b.SendMessage(chatID, "⏳ That menu is from an earlier session. Send /start to open a fresh one.")
		return
	}

	action := menus.ParseCallbackData(data)
	if action == nil {
		b.logger.Debug("Unparseable callback data", zap.String("data", data))
		return
	}

	// Log the resolved data, not the opaque token, so debug output stays useful.
	b.logger.Debug("Dispatching callback",
		zap.String("data", data),
		zap.String("type", action.Type),
		zap.String("action", action.Action))

	b.dispatchCallback(ctx, chatID, messageID, action, session)
}

// dispatchCallback routes a parsed inline-button action. Menu actions reuse the
// same command handlers as typed commands so behaviour cannot drift between the
// two entry points.
func (b *Bot) dispatchCallback(ctx context.Context, chatID int64, messageID int, action *menus.CallbackAction, session *types.UserSession) {
	switch action.Type {
	case "noop":
		return

	case "main":
		b.editToMainMenu(ctx, chatID, messageID, session)

	case "help":
		b.runCommand(ctx, "help", nil, chatID, session)

	case "resource":
		b.dispatchResourceCallback(ctx, chatID, messageID, action, session)

	case "monitor":
		switch action.Action {
		case "home":
			kb := b.menuBuilder.GetMonitorInlineKeyboard()
			b.editView(ctx, chatID, messageID, "📊 <b>Monitoring</b>\n\nPick a view.", &kb)
		case "top":
			resType := action.ResourceType
			if resType == "" {
				resType = "pods"
			}
			b.runCommand(ctx, "top", []string{resType}, chatID, session)
		case "events":
			b.runCommand(ctx, "events", nil, chatID, session)
		case "watch":
			b.SendMessage(chatID, "👁️ Usage: <code>/watch &lt;resource&gt; [-n namespace]</code>")
		}

	case "ops":
		switch action.Action {
		case "home":
			kb := b.menuBuilder.GetOperationsInlineKeyboard()
			b.editView(ctx, chatID, messageID, "🔧 <b>Operations</b>\n\nPick an operation.", &kb)
		case "restart":
			b.SendMessage(chatID, "🔄 Usage: <code>/restart deployment &lt;name&gt; [-n namespace]</code>")
		case "scale":
			b.SendMessage(chatID, "📈 Usage: <code>/scale deployment &lt;name&gt; &lt;replicas&gt; [-n namespace]</code>")
		case "delete":
			b.SendMessage(chatID, "🗑️ Open a resource from 📦 Resources and use its Delete button.")
		case "edit":
			b.SendMessage(chatID, "✏️ Editing is not supported yet.")
		}

	case "settings":
		b.dispatchSettingsCallback(ctx, chatID, messageID, action, session)

	case "ns":
		b.dispatchNamespaceCallback(ctx, chatID, messageID, action, session)

	case "ctx":
		switch action.Action {
		case "switch":
			b.runCommand(ctx, "use-context", []string{action.Name}, chatID, session)
		case "refresh":
			b.runCommand(ctx, "contexts", nil, chatID, session)
		}

	case "action":
		b.dispatchResourceAction(ctx, chatID, messageID, action, session)

	default:
		b.logger.Debug("Unhandled callback type", zap.String("type", action.Type))
	}
}

// runCommand invokes a registered command handler as if the user had typed it,
// so inline buttons and typed commands share one implementation.
func (b *Bot) runCommand(ctx context.Context, cmd string, args []string, chatID int64, session *types.UserSession) {
	if !b.IsCommandAllowed(cmd) {
		b.SendMessage(chatID, fmt.Sprintf("❌ Command /%s is not allowed.", cmd))
		return
	}
	handler, ok := b.handlers[cmd]
	if !ok {
		b.logger.Warn("Callback referenced unknown command", zap.String("cmd", cmd))
		b.SendMessage(chatID, fmt.Sprintf("❌ Unknown command: /%s", cmd))
		return
	}
	text := "/" + cmd
	if len(args) > 0 {
		text += " " + strings.Join(args, " ")
	}
	msg := &tg.Message{
		ChatID: chatID,
		Text:   text,
		From:   &tg.User{ID: session.UserID},
		Chat:   &tg.Chat{ID: chatID},
	}
	if err := handler.Handle(ctx, msg, args, session); err != nil {
		b.logger.Error("Callback command failed", zap.String("cmd", cmd), zap.Error(err))
		b.SendMessage(chatID, fmt.Sprintf("❌ Error: %s", err.Error()))
	}
}

// editView replaces the current menu message in place. Telegram returns a
// "message is not modified" error when the content is identical; that is benign
// for idempotent navigation, so it is logged at debug rather than surfaced.
func (b *Bot) editView(ctx context.Context, chatID int64, messageID int, text string, kb *tg.InlineKeyboardMarkup) {
	if _, err := b.tgBot.EditText(ctx, chatID, messageID, text, "HTML", kb); err != nil {
		if strings.Contains(err.Error(), "message is not modified") {
			b.logger.Debug("Menu edit was a no-op", zap.Int64("chat_id", chatID))
			return
		}
		b.logger.Error("Failed to edit menu view", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (b *Bot) editToMainMenu(ctx context.Context, chatID int64, messageID int, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "main"})
	kb := b.menuBuilder.GetMainMenuInlineKeyboard()
	b.editView(ctx, chatID, messageID, b.mainMenuText(session), &kb)
}

// dispatchResourceCallback handles the "menu:resource:*" family.
func (b *Bot) dispatchResourceCallback(ctx context.Context, chatID int64, messageID int, action *menus.CallbackAction, session *types.UserSession) {
	switch action.Action {
	case "types":
		session.SetMenuState(&types.MenuState{CurrentView: "resource_types"})
		kb := b.menuBuilder.GetResourceTypeInlineKeyboard()
		b.editView(ctx, chatID, messageID, "📦 <b>Resources</b>\n\nPick a resource type to browse.", &kb)

	case "view":
		if action.ResourceType == "" || action.Name == "" {
			return
		}
		args := []string{action.ResourceType, action.Name}
		if action.Namespace != "" {
			args = append(args, "-n", action.Namespace)
		}
		b.runCommand(ctx, "describe", args, chatID, session)

	case "refresh", "list", "page", "filter":
		// The resource-type buttons emit "menu:resource:<type>", which the
		// parser reports as Action=<type> with no ResourceType. Fall back to
		// the action verb so "menu:resource:pods" still lists pods.
		resType := action.ResourceType
		if resType == "" {
			resType = action.Action
		}
		b.listResources(ctx, chatID, messageID, resType, action, session)

	default:
		// "menu:resource:pods" style — Action holds the resource type.
		if _, known := types.ResourceMap[action.Action]; known {
			b.listResources(ctx, chatID, messageID, action.Action, action, session)
			return
		}
		b.logger.Debug("Unhandled resource callback", zap.String("action", action.Action))
	}
}

// listResources renders a resource list into the current menu message.
func (b *Bot) listResources(ctx context.Context, chatID int64, messageID int, resourceType string, action *menus.CallbackAction, session *types.UserSession) {
	gvr, known := types.ResourceMap[resourceType]
	if !known {
		b.SendMessage(chatID, fmt.Sprintf("❌ Unknown resource type: %s", formatters.EscapeHTML(resourceType)))
		return
	}

	namespace := action.Namespace
	if namespace == "" {
		namespace = session.GetNamespace()
	}
	// Cluster-scoped resources must be listed with an empty namespace.
	if types.IsClusterScoped(resourceType) {
		namespace = ""
	}

	resources, err := b.k8sClient.ListResources(ctx, gvr.GVR(), namespace, "", "")
	if err != nil {
		b.logger.Error("Failed to list resources for menu",
			zap.String("type", resourceType), zap.String("namespace", namespace), zap.Error(err))
		b.SendMessage(chatID, fmt.Sprintf("❌ Failed to list %s: %s",
			formatters.EscapeHTML(resourceType), formatters.EscapeHTML(err.Error())))
		return
	}

	page := 0
	if action.Extra != "" {
		if p, convErr := strconv.Atoi(action.Extra); convErr == nil && p >= 0 {
			page = p
		}
	}

	session.SetMenuState(&types.MenuState{
		CurrentView:  "resource_list",
		ResourceType: resourceType,
		Namespace:    namespace,
		Page:         page,
	})

	nsLabel := namespace
	if nsLabel == "" {
		nsLabel = "all namespaces"
	}
	if len(resources) == 0 {
		kb := b.menuBuilder.GetResourceTypeInlineKeyboard()
		b.editView(ctx, chatID, messageID, fmt.Sprintf("📭 No %s found in %s.",
			formatters.EscapeHTML(resourceType), formatters.EscapeHTML(nsLabel)), &kb)
		return
	}

	kb := b.menuBuilder.GetResourceListInlineKeyboard(resourceType, resources, page, b.menuBuilder.GetPageSize(), namespace)

	// Show the page's slice as a native table above the buttons, so the list is
	// readable at a glance rather than only as truncated button labels.
	pageSize := b.menuBuilder.GetPageSize()
	start := page * pageSize
	if start > len(resources) {
		start = len(resources)
	}
	end := start + pageSize
	if end > len(resources) {
		end = len(resources)
	}

	rich := fmt.Sprintf("### 📦 %s in %s — %d item(s)\n\n%s",
		resourceType, nsLabel, len(resources),
		formatters.RichResourceList(resources[start:end], false))
	fallbackText := fmt.Sprintf("📦 <b>%s</b> in <b>%s</b> — %d item(s)",
		formatters.EscapeHTML(resourceType), formatters.EscapeHTML(nsLabel), len(resources))
	b.editRichView(ctx, chatID, messageID, rich, fallbackText, &kb)
}

// editRichView edits a menu message to rich content, falling back to a plain
// HTML edit if the server rejects the rich message.
func (b *Bot) editRichView(ctx context.Context, chatID int64, messageID int, markdown, fallback string, kb *tg.InlineKeyboardMarkup) {
	if _, err := b.tgBot.EditRich(ctx, chatID, messageID, markdown, kb); err != nil {
		if strings.Contains(err.Error(), "message is not modified") {
			b.logger.Debug("Rich menu edit was a no-op", zap.Int64("chat_id", chatID))
			return
		}
		b.logger.Warn("Rich menu edit rejected, falling back to text",
			zap.Error(err), zap.Int64("chat_id", chatID))
		b.editView(ctx, chatID, messageID, fallback, kb)
	}
}

// dispatchNamespaceCallback handles the "menu:ns:*" family — the namespace
// switcher that lets users browse resources outside the default namespace.
func (b *Bot) dispatchNamespaceCallback(ctx context.Context, chatID int64, messageID int, action *menus.CallbackAction, session *types.UserSession) {
	switch action.Action {
	case "pick", "page":
		page := 0
		if action.Action == "page" {
			if p, err := strconv.Atoi(action.Name); err == nil && p >= 0 {
				page = p
			}
		} else if action.Name != "" {
			// "pick" carries the resource type to return to after choosing.
			st := session.GetMenuState()
			view := ""
			if st != nil {
				view = st.CurrentView
			}
			session.SetMenuState(&types.MenuState{
				CurrentView:  view,
				ResourceType: action.Name,
				Namespace:    session.GetNamespace(),
			})
		}
		b.showNamespacePicker(ctx, chatID, messageID, page, session)

	case "set":
		// An empty name means "all namespaces".
		session.SetNamespace(action.Name)
		b.logger.Debug("Namespace switched",
			zap.Int64("user_id", session.UserID), zap.String("namespace", action.Name))

		// Return to the resource list the user came from, if any.
		if st := session.GetMenuState(); st != nil && st.ResourceType != "" {
			b.listResources(ctx, chatID, messageID, st.ResourceType,
				&menus.CallbackAction{Namespace: action.Name}, session)
			return
		}
		kb := b.menuBuilder.GetResourceTypeInlineKeyboard()
		b.editView(ctx, chatID, messageID, fmt.Sprintf(
			"✅ Namespace set to <b>%s</b>\n\nPick a resource type to browse.",
			formatters.EscapeHTML(nsDisplay(action.Name))), &kb)

	default:
		b.showNamespacePicker(ctx, chatID, messageID, 0, session)
	}
}

// showNamespacePicker lists cluster namespaces as buttons.
func (b *Bot) showNamespacePicker(ctx context.Context, chatID int64, messageID, page int, session *types.UserSession) {
	if b.k8sClient == nil {
		b.SendMessage(chatID, "❌ Kubernetes client unavailable.")
		return
	}

	nsResources, err := b.k8sClient.ListNamespaces(ctx, "")
	if err != nil {
		b.logger.Error("Failed to list namespaces for picker", zap.Error(err))
		b.SendMessage(chatID, fmt.Sprintf("❌ Failed to list namespaces: %s",
			formatters.EscapeHTML(err.Error())))
		return
	}

	names := make([]string, 0, len(nsResources))
	for _, n := range nsResources {
		names = append(names, n.Name)
	}
	sort.Strings(names)

	backTo := "menu:settings:home"
	if st := session.GetMenuState(); st != nil && st.ResourceType != "" {
		backTo = "menu:resource:" + st.ResourceType
	}

	kb := b.menuBuilder.GetNamespaceInlineKeyboard(names, session.GetNamespace(), page, backTo)
	text := fmt.Sprintf("🌐 <b>Namespace</b>\n\nCurrent: <b>%s</b>\nPick one to browse its resources.",
		formatters.EscapeHTML(nsDisplay(session.GetNamespace())))
	b.editView(ctx, chatID, messageID, text, &kb)
}

// nsDisplay renders an empty namespace as the all-namespaces label.
func nsDisplay(ns string) string {
	if ns == "" {
		return "all namespaces"
	}
	return ns
}

// dispatchSettingsCallback handles the "menu:settings:*" family.
func (b *Bot) dispatchSettingsCallback(ctx context.Context, chatID int64, messageID int, action *menus.CallbackAction, session *types.UserSession) {
	switch action.Action {
	case "home":
		kb := b.menuBuilder.GetSettingsInlineKeyboard()
		text := fmt.Sprintf("⚙️ <b>Settings</b>\n\n<b>Context:</b> %s\n<b>Namespace:</b> %s",
			formatters.EscapeHTML(b.currentContextName(session)),
			formatters.EscapeHTML(session.GetNamespace()))
		b.editView(ctx, chatID, messageID, text, &kb)
	case "context":
		b.runCommand(ctx, "contexts", nil, chatID, session)
	case "namespace":
		b.showNamespacePicker(ctx, chatID, messageID, 0, session)
	default:
		b.SendMessage(chatID, "⚙️ That setting is not available yet.")
	}
}

// dispatchResourceAction handles the "menu:action:*" family (per-resource verbs).
func (b *Bot) dispatchResourceAction(ctx context.Context, chatID int64, messageID int, action *menus.CallbackAction, session *types.UserSession) {
	nsArgs := func() []string {
		if action.Namespace != "" {
			return []string{"-n", action.Namespace}
		}
		return nil
	}

	switch action.Action {
	case "describe":
		if action.ResourceType == "" || action.Name == "" {
			return
		}
		b.runCommand(ctx, "describe", append([]string{action.ResourceType, action.Name}, nsArgs()...), chatID, session)

	case "logs":
		// For pod logs the parser puts the pod name in Namespace/Name depending
		// on the button shape; both emit "...:<namespace>:<name>".
		if action.Name == "" {
			return
		}
		b.runCommand(ctx, "logs", append([]string{action.Name}, nsArgs()...), chatID, session)

	case "restart":
		if action.Name == "" {
			return
		}
		b.runCommand(ctx, "restart", append([]string{"deployment", action.Name}, nsArgs()...), chatID, session)

	case "delete":
		if action.ResourceType == "" || action.Name == "" {
			return
		}
		kb := b.menuBuilder.GetConfirmDeleteKeyboard(action.ResourceType, action.Namespace, action.Name)
		b.editView(ctx, chatID, messageID, fmt.Sprintf(
			"⚠️ Delete <b>%s</b> <code>%s</code> in <code>%s</code>?\n\nThis cannot be undone.",
			formatters.EscapeHTML(action.ResourceType),
			formatters.EscapeHTML(action.Name),
			formatters.EscapeHTML(action.Namespace)), &kb)

	case "confirmdelete":
		b.confirmDelete(ctx, chatID, messageID, action, session)

	case "scale":
		if action.Name == "" {
			return
		}
		b.SendMessage(chatID, fmt.Sprintf(
			"📈 Usage: <code>/scale deployment %s &lt;replicas&gt; -n %s</code>",
			formatters.EscapeHTML(action.Name), formatters.EscapeHTML(action.Namespace)))

	case "exec":
		b.SendMessage(chatID, "🖥️ Usage: <code>/exec &lt;pod&gt; [-c container] -n &lt;namespace&gt; -- &lt;command&gt;</code>")

	case "portforward":
		b.SendMessage(chatID, "🔌 Usage: <code>/portforward &lt;pod&gt; &lt;local:remote&gt; [-n namespace]</code>")

	default:
		b.logger.Debug("Unhandled resource action", zap.String("action", action.Action))
		b.SendMessage(chatID, "⚠️ That action is not available yet.")
	}
}

// confirmDelete performs a delete that the user explicitly confirmed.
func (b *Bot) confirmDelete(ctx context.Context, chatID int64, messageID int, action *menus.CallbackAction, session *types.UserSession) {
	gvr, known := types.ResourceMap[action.ResourceType]
	if !known {
		b.SendMessage(chatID, fmt.Sprintf("❌ Unknown resource type: %s", formatters.EscapeHTML(action.ResourceType)))
		return
	}
	if err := b.deleteResourceForMenu(ctx, gvr.GVR(), action.Namespace, action.Name); err != nil {
		b.logger.Error("Menu delete failed",
			zap.String("type", action.ResourceType), zap.String("name", action.Name), zap.Error(err))
		b.SendMessage(chatID, fmt.Sprintf("❌ Delete failed: %s", formatters.EscapeHTML(err.Error())))
		return
	}
	if b.k8sClient.IsDryRun() {
		b.SendMessage(chatID, fmt.Sprintf("🧪 Dry run: would delete <code>%s</code>", formatters.EscapeHTML(action.Name)))
	} else {
		b.SendMessage(chatID, fmt.Sprintf("🗑️ Deleted <code>%s</code>", formatters.EscapeHTML(action.Name)))
	}
	b.listResources(ctx, chatID, messageID, action.ResourceType, action, session)
}

func (b *Bot) handleReplyKeyboard(ctx context.Context, msg *botmodels.Message, session *types.UserSession) bool {
	text := msg.Text
	switch text {
	case "📦 Resources", "resources":
		b.ShowResourceTypes(ctx, msg.Chat.ID, session)
		return true
	case "📋 Logs", "logs":
		if _, err := b.tgBot.SendText(ctx, msg.Chat.ID, "Usage: /logs <pod> [-c container] [-n namespace] [-f] [--tail N]", "HTML", nil); err != nil {
			b.logger.Error("Failed to send logs usage", zap.Error(err), zap.Int64("chat_id", msg.Chat.ID))
		}
		return true
	case "🖥️ Exec", "exec":
		if _, err := b.tgBot.SendText(ctx, msg.Chat.ID, "Usage: /exec <pod> [-c container] -n namespace -- <command>", "HTML", nil); err != nil {
			b.logger.Error("Failed to send exec usage", zap.Error(err), zap.Int64("chat_id", msg.Chat.ID))
		}
		return true
	case "⚙️ Contexts", "🌐 Contexts", "contexts":
		b.runCommand(ctx, "contexts", nil, msg.Chat.ID, session)
		return true
	case "🔌 Port Forward", "port forward", "portforward":
		b.SendMessage(msg.Chat.ID, "🔌 Usage: <code>/portforward &lt;pod&gt; &lt;local:remote&gt; [-n namespace]</code>")
		return true
	case "❓ Help", "help":
		b.runCommand(ctx, "help", nil, msg.Chat.ID, session)
		return true
	case "📊 Monitor", "monitor":
		b.ShowMonitor(ctx, msg.Chat.ID, session)
		return true
	case "🔧 Operations", "operations":
		b.ShowOperations(ctx, msg.Chat.ID, session)
		return true
	case "⚙️ Settings", "settings":
		b.ShowSettings(ctx, msg.Chat.ID, session)
		return true
	}
	return false
}

func (b *Bot) handleExecInput(ctx context.Context, msg *botmodels.Message, session *types.UserSession) {
	pod, namespace, container := session.GetExecInfo()
	if _, err := b.tgBot.SendText(ctx, msg.Chat.ID, fmt.Sprintf("Exec in %s/%s [%s] — not fully implemented", namespace, pod, container), "HTML", nil); err != nil {
		b.logger.Error("Failed to send exec input message", zap.Error(err), zap.Int64("chat_id", msg.Chat.ID))
	}
	session.ClearExecMode()
}

// currentContextName returns the active kubeconfig context name, or "unknown".
func (b *Bot) currentContextName(session *types.UserSession) string {
	if ctxName := session.CurrentCtx; ctxName != "" {
		return ctxName
	}
	if b.k8sClient != nil {
		if kc := b.k8sClient.GetCurrentContext(); kc != nil && kc.Name != "" {
			return kc.Name
		}
	}
	return "unknown"
}

func (b *Bot) mainMenuText(session *types.UserSession) string {
	return fmt.Sprintf(`🤖 <b>telectl</b>

<b>Cluster:</b> %s
<b>Namespace:</b> %s

Choose an action below, or use /help for the full command reference.`,
		formatters.EscapeHTML(b.currentContextName(session)),
		formatters.EscapeHTML(session.GetNamespace()))
}

func (b *Bot) ShowMainMenu(ctx context.Context, chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "main"})
	// Send the persistent reply keyboard once per main-menu visit so the bottom
	// bar actually appears; handleReplyKeyboard matches on these exact labels.
	if b.menuBuilder.IsReplyKeyboardEnabled() {
		rk := b.menuBuilder.GetMainReplyKeyboard()
		if _, err := b.tgBot.SendTextReplyKeyboard(ctx, chatID, b.mainMenuText(session), "HTML", &rk); err != nil {
			b.logger.Error("Failed to send main menu keyboard", zap.Error(err), zap.Int64("chat_id", chatID))
		}
		kb := b.menuBuilder.GetMainMenuInlineKeyboard()
		if _, err := b.tgBot.SendText(ctx, chatID, "Choose a section:", "HTML", &kb); err != nil {
			b.logger.Error("Failed to send main menu", zap.Error(err), zap.Int64("chat_id", chatID))
		}
		return
	}
	kb := b.menuBuilder.GetMainMenuInlineKeyboard()
	if _, err := b.tgBot.SendText(ctx, chatID, b.mainMenuText(session), "HTML", &kb); err != nil {
		b.logger.Error("Failed to send main menu", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (b *Bot) ShowResourceTypes(ctx context.Context, chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "resource_types"})
	kb := b.menuBuilder.GetResourceTypeInlineKeyboard()
	if _, err := b.tgBot.SendText(ctx, chatID, "📦 <b>Resources</b>\n\nPick a resource type to browse.", "HTML", &kb); err != nil {
		b.logger.Error("Failed to send resource types", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (b *Bot) ShowMonitor(ctx context.Context, chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "monitor"})
	kb := b.menuBuilder.GetMonitorInlineKeyboard()
	if _, err := b.tgBot.SendText(ctx, chatID, "📊 <b>Monitoring</b>\n\nPick a view.", "HTML", &kb); err != nil {
		b.logger.Error("Failed to send monitor", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (b *Bot) ShowOperations(ctx context.Context, chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "operations"})
	kb := b.menuBuilder.GetOperationsInlineKeyboard()
	if _, err := b.tgBot.SendText(ctx, chatID, "🔧 <b>Operations</b>\n\nPick an operation.", "HTML", &kb); err != nil {
		b.logger.Error("Failed to send operations", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (b *Bot) ShowSettings(ctx context.Context, chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "settings"})
	kb := b.menuBuilder.GetSettingsInlineKeyboard()
	text := fmt.Sprintf("⚙️ <b>Settings</b>\n\n<b>Context:</b> %s\n<b>Namespace:</b> %s",
		formatters.EscapeHTML(b.currentContextName(session)),
		formatters.EscapeHTML(session.GetNamespace()))
	if _, err := b.tgBot.SendText(ctx, chatID, text, "HTML", &kb); err != nil {
		b.logger.Error("Failed to send settings", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (b *Bot) getOrCreateSession(userID int64) *types.UserSession {
	val, _ := b.userSessions.LoadOrStore(userID, &types.UserSession{
		UserID:    userID,
		CurrentNS: b.config.Kubernetes.DefaultNamespace,
		State:     make(map[string]interface{}),
	})
	return val.(*types.UserSession)
}

func (b *Bot) getGVR(resourceType string) *schema.GroupVersionResource {
	if gvr, ok := types.ResourceMap[resourceType]; ok {
		v := gvr.GVR()
		return &v
	}
	return nil
}

func (b *Bot) listResourcesForMenu(ctx context.Context, resourceType, namespace string) ([]types.MenuState, error) {
	_ = ctx
	_ = namespace
	return nil, nil
}

func (b *Bot) deleteResourceForMenu(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	return b.k8sClient.DeleteResource(ctx, gvr, namespace, name, &metav1.DeleteOptions{})
}

// SendRich sends a Rich Message (native tables, headings, collapsible sections).
// If the server rejects it — older Bot API, or rich messages unavailable for the
// chat — it falls back to the plain-text rendering so the user still gets an
// answer rather than silence. The fallback is logged, not surfaced.
func (b *Bot) SendRich(chatID int64, markdown, fallback string) {
	ctx := context.Background()
	if _, err := b.tgBot.SendRich(ctx, chatID, markdown, nil); err != nil {
		b.logger.Warn("Rich message rejected, falling back to text",
			zap.Error(err), zap.Int64("chat_id", chatID))
		b.SendLongMessage(chatID, fallback)
	}
}

// SendRichKeyboard is SendRich with an inline keyboard attached.
func (b *Bot) SendRichKeyboard(chatID int64, markdown, fallback string, keyboard *tg.InlineKeyboardMarkup) {
	ctx := context.Background()
	if _, err := b.tgBot.SendRich(ctx, chatID, markdown, keyboard); err != nil {
		b.logger.Warn("Rich message rejected, falling back to text",
			zap.Error(err), zap.Int64("chat_id", chatID))
		if _, ferr := b.tgBot.SendText(ctx, chatID, fallback, "HTML", keyboard); ferr != nil {
			b.logger.Error("Fallback send failed", zap.Error(ferr), zap.Int64("chat_id", chatID))
		}
	}
}

func (b *Bot) SendMessage(chatID int64, text string) {
	ctx := context.Background()
	if _, err := b.tgBot.SendText(ctx, chatID, text, "HTML", nil); err != nil {
		b.logger.Error("Failed to send message", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (b *Bot) SendLongMessage(chatID int64, text string) {
	ctx := context.Background()
	for i, chunk := range splitMessage(text, b.config.Bot.MaxMessageLength) {
		if _, err := b.tgBot.SendText(ctx, chatID, chunk, "HTML", nil); err != nil {
			b.logger.Error("Failed to send message chunk",
				zap.Error(err), zap.Int64("chat_id", chatID), zap.Int("chunk", i))
			return
		}
	}
}

// splitMessage breaks text into chunks that fit Telegram's per-message limit,
// preferring line boundaries. SendLongMessage previously sent oversized text as
// a single message, which Telegram rejects outright once it exceeds 4096 chars.
func splitMessage(text string, max int) []string {
	if max <= 0 || max > 4096 {
		max = 4096
	}
	if len(text) <= max {
		return []string{text}
	}
	var out []string
	for len(text) > max {
		cut := strings.LastIndex(text[:max], "\n")
		if cut <= 0 {
			cut = max
		}
		out = append(out, text[:cut])
		text = strings.TrimPrefix(text[cut:], "\n")
	}
	if text != "" {
		out = append(out, text)
	}
	return out
}

func (b *Bot) SendMarkdown(chatID int64, text string) {
	ctx := context.Background()
	if _, err := b.tgBot.SendText(ctx, chatID, text, "MarkdownV2", nil); err != nil {
		b.logger.Error("Failed to send markdown message", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

// SendHTML sends HTML-formatted text. Prefer this over SendMarkdown for static
// help/usage text: MarkdownV2 requires escaping -, ., (, ), |, ! and friends,
// and an unescaped char makes Telegram reject the whole message with a 400.
func (b *Bot) SendHTML(chatID int64, text string) {
	ctx := context.Background()
	if _, err := b.tgBot.SendText(ctx, chatID, text, "HTML", nil); err != nil {
		b.logger.Error("Failed to send HTML message", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (b *Bot) SendText(chatID int64, text string) {
	ctx := context.Background()
	if _, err := b.tgBot.SendText(ctx, chatID, text, "HTML", nil); err != nil {
		b.logger.Error("Failed to send text", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (b *Bot) SendTextFull(chatID int64, text string, parseMode string, keyboard *tg.InlineKeyboardMarkup) {
	ctx := context.Background()
	if _, err := b.tgBot.SendText(ctx, chatID, text, parseMode, keyboard); err != nil {
		b.logger.Error("Failed to send text", zap.Error(err),
			zap.Int64("chat_id", chatID), zap.String("parse_mode", parseMode))
	}
}

func (b *Bot) SendKeyboard(chatID int64, text string, keyboard *tg.InlineKeyboardMarkup) {
	ctx := context.Background()
	if _, err := b.tgBot.SendText(ctx, chatID, text, "HTML", keyboard); err != nil {
		b.logger.Error("Failed to send keyboard", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (b *Bot) SendReplyKeyboard(chatID int64, text string, keyboard *tg.ReplyKeyboardMarkup) {
	ctx := context.Background()
	if _, err := b.tgBot.SendTextReplyKeyboard(ctx, chatID, text, "HTML", keyboard); err != nil {
		b.logger.Error("Failed to send reply keyboard", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (b *Bot) IsUserAllowed(userID int64) bool {
	allowed := b.config.Telegram.AllowedUserIDs
	if len(allowed) == 0 {
		return true
	}
	for _, id := range allowed {
		if id == userID {
			return true
		}
	}
	return false
}

func (b *Bot) IsCommandAllowed(command string) bool {
	allowed := b.config.Bot.AllowedCommands
	if len(allowed) == 0 {
		return true
	}
	for _, cmd := range allowed {
		if cmd == command {
			return true
		}
	}
	return false
}

func (b *Bot) K8sClient() interface{}      { return b.k8sClient }
func (b *Bot) Config() interface{}         { return b.config }
func (b *Bot) API() interface{}            { return b.libBot }
func (b *Bot) MenuBuilder() interface{}    { return b.menuBuilder }
func (b *Bot) Logger() interface{}         { return b.logger }
func (b *Bot) RateLimiter() interface{}    { return b.rateLimiter }
func (b *Bot) GetK8sClient() interface{}   { return b.k8sClient }
func (b *Bot) GetConfig() interface{}      { return b.config }
func (b *Bot) GetAPI() interface{}         { return b.libBot }
func (b *Bot) GetMenuBuilder() interface{} { return b.menuBuilder }
func (b *Bot) GetLogger() interface{}      { return b.logger }
func (b *Bot) GetRateLimiter() interface{} { return b.rateLimiter }
