package bot

import (
	"context"
	"fmt"
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

	b.libBot.RegisterHandler(bottg.HandlerTypeMessageText, "", bottg.MatchTypeCommand, b.handleMessage)
	b.libBot.RegisterHandler(bottg.HandlerTypeCallbackQueryData, "", bottg.MatchTypePrefix, b.handleCallbackQuery)

	b.libBot.Start(ctx)
	return nil
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

	userID := msg.From.ID
	chatID := msg.Chat.ID

	if !b.IsUserAllowed(userID) {
		b.tgBot.SendText(ctx, chatID, "❌ You are not authorized to use this bot.", "HTML", nil)
		return
	}
	if !b.rateLimiter.Allow(userID) {
		b.tgBot.SendText(ctx, chatID, "⏱️ Rate limit exceeded. Please wait a moment.", "HTML", nil)
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
		b.tgBot.SendText(ctx, msg.Chat.ID, fmt.Sprintf("❌ Command /%s is not allowed.", cmd), "HTML", nil)
		return
	}
	handler, ok := b.handlers[cmd]
	if !ok {
		b.tgBot.SendText(ctx, msg.Chat.ID, fmt.Sprintf("❌ Unknown command: /%s\nUse /help to see available commands.", cmd), "HTML", nil)
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
		b.tgBot.SendText(ctx, msg.Chat.ID, fmt.Sprintf("❌ Error: %s", err.Error()), "HTML", nil)
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
	b.tgBot.SendText(ctx, chatID, fmt.Sprintf("🔘 Callback: %s", callback.Data), "HTML", nil)
}

func (b *Bot) handleReplyKeyboard(ctx context.Context, msg *botmodels.Message, session *types.UserSession) bool {
	text := msg.Text
	switch text {
	case "📦 Resources", "resources":
		b.ShowResourceTypes(ctx, msg.Chat.ID, session)
		return true
	case "📋 Logs", "logs":
		b.tgBot.SendText(ctx, msg.Chat.ID, "Usage: /logs <pod> [-c container] [-n namespace] [-f] [--tail N]", "HTML", nil)
		return true
	case "🖥️ Exec", "exec":
		b.tgBot.SendText(ctx, msg.Chat.ID, "Usage: /exec <pod> [-c container] -n namespace -- <command>", "HTML", nil)
		return true
	case "🌐 Contexts", "contexts":
		b.handlers["contexts"].Handle(ctx, &tg.Message{
			ID:     msg.ID,
			ChatID: msg.Chat.ID,
			Text:   msg.Text,
			From:   &tg.User{ID: msg.From.ID, FirstName: msg.From.FirstName, Username: msg.From.Username},
			Chat:   &tg.Chat{ID: msg.Chat.ID, Type: string(msg.Chat.Type), Title: msg.Chat.Title},
		}, nil, session)
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
	b.tgBot.SendText(ctx, msg.Chat.ID, fmt.Sprintf("Exec in %s/%s [%s] — not fully implemented", namespace, pod, container), "HTML", nil)
	session.ClearExecMode()
}

func (b *Bot) ShowMainMenu(ctx context.Context, chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "main"})
	currentCtx := "default"
	currentNS := session.GetNamespace()
	text := fmt.Sprintf(`🤖 <b>telectl</b>

<b>Cluster:</b> %s
<b>Namespace:</b> %s

Choose an action from the menu or use /help.`, currentCtx, currentNS)
	b.tgBot.SendText(ctx, chatID, text, "HTML", nil)
}

func (b *Bot) ShowResourceTypes(ctx context.Context, chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "resource_types"})
	b.tgBot.SendText(ctx, chatID, "📦 Use /get <resource> to list resources.\nTypes: pods, deployments, services, replicasets, namespaces, nodes, configmaps, secrets, pvcs, pvs, ingresses, events", "HTML", nil)
}

func (b *Bot) ShowMonitor(ctx context.Context, chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "monitor"})
	b.tgBot.SendText(ctx, chatID, "📊 Monitoring\n• /top pods|nodes\n• /events\n• /watch <resource>", "HTML", nil)
}

func (b *Bot) ShowOperations(ctx context.Context, chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "operations"})
	b.tgBot.SendText(ctx, chatID, "🔧 Operations\n• /restart deployment <name>\n• /scale deployment <name> <replicas>", "HTML", nil)
}

func (b *Bot) ShowSettings(ctx context.Context, chatID int64, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "settings"})
	b.tgBot.SendText(ctx, chatID, "⚙️ Settings\nUse /config to view current configuration.", "HTML", nil)
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

func (b *Bot) SendMessage(chatID int64, text string) {
	ctx := context.Background()
	b.tgBot.SendText(ctx, chatID, text, "HTML", nil)
}

func (b *Bot) SendLongMessage(chatID int64, text string) {
	ctx := context.Background()
	b.tgBot.SendText(ctx, chatID, text, "HTML", nil)
}

func (b *Bot) SendMarkdown(chatID int64, text string) {
	ctx := context.Background()
	b.tgBot.SendText(ctx, chatID, text, "MarkdownV2", nil)
}

func (b *Bot) SendText(chatID int64, text string) {
	ctx := context.Background()
	b.tgBot.SendText(ctx, chatID, text, "HTML", nil)
}

func (b *Bot) SendTextFull(chatID int64, text string, parseMode string, keyboard *tg.InlineKeyboardMarkup) {
	ctx := context.Background()
	b.tgBot.SendText(ctx, chatID, text, parseMode, keyboard)
}

func (b *Bot) SendKeyboard(chatID int64, text string, keyboard *tg.InlineKeyboardMarkup) {
	ctx := context.Background()
	b.tgBot.SendText(ctx, chatID, text, "HTML", keyboard)
}

func (b *Bot) SendReplyKeyboard(chatID int64, text string, keyboard *tg.ReplyKeyboardMarkup) {
	ctx := context.Background()
	b.tgBot.SendText(ctx, chatID, text, "HTML", nil)
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
