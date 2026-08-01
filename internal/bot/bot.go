package bot

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/telectl/internal/config"
	"github.com/ksauraj/telectl/internal/handlers"
	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/menus"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"go.uber.org/zap"
)

// Bot is the Telegram bot for Kubernetes operations.
type Bot struct {
	api          *tgbotapi.BotAPI
	config       *config.Config
	k8sClient    *k8s.Client
	logger       *zap.Logger
	handlers     map[string]handlers.CommandHandler
	rateLimiter  *types.RateLimiter
	userSessions sync.Map // userID -> *types.UserSession
	menuBuilder  *menus.MenuBuilder
	cancelFunc   context.CancelFunc
}

// New creates a new Bot instance.
func New(cfg *config.Config, logger *zap.Logger) (*Bot, error) {
	if err := config.ValidateConfig(cfg); err != nil {
		return nil, err
	}

	api, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}
	api.Debug = cfg.Logging.Level == "debug"
	logger.Info("Bot authorized", zap.String("username", api.Self.UserName))

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
		api:          api,
		config:       cfg,
		k8sClient:    k8sClient,
		logger:       logger,
		handlers:     make(map[string]handlers.CommandHandler),
		rateLimiter:  types.NewRateLimiter(cfg.Bot.RateLimit, time.Minute),
		menuBuilder:  menus.NewMenuBuilder(cfg),
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

// Start begins the long-poll loop for Telegram updates.
func (b *Bot) Start(ctx context.Context) error {
	ctx, b.cancelFunc = context.WithCancel(ctx)

	if b.menuBuilder.IsMenuButtonEnabled() {
		commands := b.menuBuilder.GetBotCommands()
		if _, err := b.api.Request(tgbotapi.NewSetMyCommands(commands...)); err != nil {
			b.logger.Warn("Failed to set bot menu commands", zap.Error(err))
		} else {
			b.logger.Info("Bot menu commands set", zap.Int("count", len(commands)))
		}
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	log := b.logger.With(zap.String("bot", b.api.Self.UserName))
	log.Info("Bot started, listening for updates")

	for {
		select {
		case <-ctx.Done():
			log.Info("Bot stopping")
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				log.Info("Updates channel closed")
				return nil
			}
			go b.handleUpdate(ctx, update)
		}
	}
}

// Stop cancels the update loop.
func (b *Bot) Stop() {
	if b.cancelFunc != nil {
		b.cancelFunc()
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	if update.InlineQuery != nil {
		b.handleInlineQuery(ctx, update.InlineQuery)
		return
	}
	if update.CallbackQuery != nil {
		b.handleCallbackQuery(ctx, update.CallbackQuery)
		return
	}
	if update.Message == nil {
		return
	}

	msg := update.Message
	userID := msg.From.ID
	chatID := msg.Chat.ID

	if !b.IsUserAllowed(userID) {
		b.SendMessage(chatID, "❌ You are not authorized to use this bot.")
		return
	}
	if !b.rateLimiter.Allow(userID) {
		b.SendMessage(chatID, "⏱️ Rate limit exceeded. Please wait a moment.")
		return
	}

	session := b.getOrCreateSession(userID)
	session.Touch()

	if b.menuBuilder.IsReplyKeyboardEnabled() && !msg.IsCommand() {
		if b.handleReplyKeyboard(ctx, msg, session) {
			return
		}
	}

	if msg.IsCommand() {
		b.handleCommand(ctx, msg, session)
	} else if session.IsInExecMode() {
		b.handleExecInput(ctx, msg, session)
	} else {
		b.ShowMainMenu(chatID, session)
	}
}

func (b *Bot) handleInlineQuery(ctx context.Context, inlineQuery *tgbotapi.InlineQuery) {
	userID := inlineQuery.From.ID
	if !b.IsUserAllowed(userID) {
		return
	}
	if !b.rateLimiter.Allow(userID) {
		return
	}
	handler := handlers.NewInlineQueryHandler(b)
	if err := handler.HandleInlineQuery(ctx, inlineQuery); err != nil {
		b.logger.Error("Inline query failed",
			zap.Int64("user_id", userID),
			zap.String("query", inlineQuery.Query),
			zap.Error(err),
		)
	}
}

func (b *Bot) handleCommand(ctx context.Context, msg *tgbotapi.Message, session *types.UserSession) {
	cmd := msg.Command()
	if !b.IsCommandAllowed(cmd) {
		b.SendMessage(msg.Chat.ID, fmt.Sprintf("❌ Command /%s is not allowed.", cmd))
		return
	}
	handler, ok := b.handlers[cmd]
	if !ok {
		b.SendMessage(msg.Chat.ID, fmt.Sprintf("❌ Unknown command: /%s\nUse /help to see available commands.", cmd))
		return
	}
	args := strings.Fields(msg.CommandArguments())
	if err := handler.Handle(ctx, msg, args, session); err != nil {
		b.logger.Error("Command failed", zap.String("cmd", cmd), zap.Error(err))
		b.SendMessage(msg.Chat.ID, fmt.Sprintf("❌ Error: %s", err.Error()))
	}
}

func (b *Bot) handleCallbackQuery(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, ""))
	if !b.IsUserAllowed(callback.From.ID) {
		return
	}
	chatID := callback.Message.Chat.ID
	b.SendMessage(chatID, fmt.Sprintf("🔘 Callback: %s", callback.Data))
}

func (b *Bot) handleReplyKeyboard(ctx context.Context, msg *tgbotapi.Message, session *types.UserSession) bool {
	text := msg.Text
	switch text {
	case "📦 Resources", "resources":
		b.ShowResourceTypes(msg.Chat.ID, session)
		return true
	case "📋 Logs", "logs":
		b.SendMessage(msg.Chat.ID, "Usage: /logs <pod> [-c container] [-n namespace] [-f] [--tail N]")
		return true
	case "🖥️ Exec", "exec":
		b.SendMessage(msg.Chat.ID, "Usage: /exec <pod> [-c container] -n namespace -- <command>")
		return true
	case "🌐 Contexts", "contexts":
		b.handlers["contexts"].Handle(ctx, msg, nil, session)
		return true
	case "📊 Monitor", "monitor":
		b.ShowMonitor(msg.Chat.ID, session)
		return true
	case "🔧 Operations", "operations":
		b.ShowOperations(msg.Chat.ID, session)
		return true
	case "⚙️ Settings", "settings":
		b.ShowSettings(msg.Chat.ID, session)
		return true
	}
	return false
}

func (b *Bot) handleExecInput(ctx context.Context, msg *tgbotapi.Message, session *types.UserSession) {
	pod, namespace, container := session.GetExecInfo()
	b.SendMessage(msg.Chat.ID, fmt.Sprintf("Exec in %s/%s [%s] — not fully implemented", namespace, pod, container))
	session.ClearExecMode()
}

// --- Menu views ---

func (b *Bot) ShowMainMenu(chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "main"})
	currentCtx := "default"
	currentNS := session.GetNamespace()
	text := fmt.Sprintf(`🤖 *telectl*

*Cluster:* %s
*Namespace:* %s

Choose an action from the menu or use /help.`, currentCtx, currentNS)
	b.SendMessage(chatID, text)
}

func (b *Bot) ShowResourceTypes(chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "resource_types"})
	b.SendMessage(chatID, "📦 Use /get <resource> to list resources.\nTypes: pods, deployments, services, replicasets, namespaces, nodes, configmaps, secrets, pvcs, pvs, ingresses, events")
}

func (b *Bot) ShowMonitor(chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "monitor"})
	b.SendMessage(chatID, "📊 Monitoring\n• /top pods|nodes\n• /events\n• /watch <resource>")
}

func (b *Bot) ShowOperations(chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "operations"})
	b.SendMessage(chatID, "🔧 Operations\n• /restart deployment <name>\n• /scale deployment <name> <replicas>")
}

func (b *Bot) ShowSettings(chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "settings"})
	b.SendMessage(chatID, "⚙️ Settings\nUse /config to view current configuration.")
}

func (b *Bot) getOrCreateSession(userID int64) *types.UserSession {
	val, _ := b.userSessions.LoadOrStore(userID, &types.UserSession{
		UserID:    userID,
		CurrentNS: b.config.Kubernetes.DefaultNamespace,
		State:     make(map[string]interface{}),
	})
	return val.(*types.UserSession)
}

// --- k8s helper methods used by menu callbacks ---

func (b *Bot) getGVR(resourceType string) *schema.GroupVersionResource {
	if gvr, ok := types.ResourceMap[resourceType]; ok {
		v := gvr.GVR()
		return &v
	}
	return nil
}

func (b *Bot) listResourcesForMenu(ctx context.Context, resourceType, namespace string) ([]types.MenuState, error) {
	// Placeholder — the real list call goes through k8s.Client
	_ = ctx
	_ = namespace
	return nil, nil
}

func (b *Bot) deleteResourceForMenu(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	return b.k8sClient.DeleteResource(ctx, gvr, namespace, name, &metav1.DeleteOptions{})
}

// formatLogs reads logs from an io.Reader and formats them.
func formatLogs(reader io.Reader, tail int) string {
	logs, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Sprintf("error reading logs: %s", err)
	}
	return formatters.FormatPodLogs(string(logs), tail)
}

// unused guards to keep imports referenced if other files are trimmed later
var _ = strconv.Itoa
var _ = io.ReadAll
