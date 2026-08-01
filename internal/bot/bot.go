package bot

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/k8s-telegram-bot/internal/config"
	"github.com/ksauraj/k8s-telegram-bot/internal/handlers"
	"github.com/ksauraj/k8s-telegram-bot/internal/k8s"
	"github.com/ksauraj/k8s-telegram-bot/internal/menus"
	"github.com/ksauraj/k8s-telegram-bot/internal/utils/formatters"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"go.uber.org/zap"
)

type Bot struct {
	api           *tgbotapi.BotAPI
	config        *config.Config
	k8sClient     *k8s.Client
	logger        *zap.Logger
	handlers      map[string]handlers.CommandHandler
	rateLimiter   *RateLimiter
	userSessions  sync.Map // userID -> *UserSession
	menuBuilder   *menus.MenuBuilder
	cancelFunc    context.CancelFunc
}

type UserSession struct {
	UserID         int64
	CurrentNS      string
	CurrentCtx     string
	LastActivity   time.Time
	State          map[string]interface{}
	MenuState      *MenuState
	mu             sync.RWMutex
}

type MenuState struct {
	CurrentView    string
	ResourceType   string
	Namespace      string
	Page           int
	Filter         string
}

type RateLimiter struct {
	requests map[int64][]time.Time
	mu       sync.Mutex
	limit    int
	window   time.Duration
}

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

	bot := &Bot{
		api:         api,
		config:      cfg,
		k8sClient:   k8sClient,
		logger:      logger,
		handlers:    make(map[string]handlers.CommandHandler),
		rateLimiter: NewRateLimiter(cfg.Bot.RateLimit, time.Minute),
		menuBuilder: menus.NewMenuBuilder(cfg),
	}

	bot.registerHandlers()

	return bot, nil
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
		commands := b.menuBuilder.GetBotCommands()
		if err := b.api.SetMyCommands(commands...); err != nil {
			b.logger.Warn("Failed to set bot menu commands", zap.Error(err))
		} else {
			b.logger.Info("Bot menu commands set", zap.Int("count", len(commands)))
		}
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	logger := b.logger.With(zap.String("bot", b.api.Self.UserName))
	logger.Info("Bot started, listening for updates")

	for {
		select {
		case <-ctx.Done():
			logger.Info("Bot stopping")
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				logger.Info("Updates channel closed")
				return nil
			}
			go b.handleUpdate(ctx, update)
		}
	}
}

func (b *Bot) Stop() {
	if b.cancelFunc != nil {
		b.cancelFunc()
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	// Handle inline queries
	if update.InlineQuery != nil {
		b.handleInlineQuery(ctx, update.InlineQuery)
		return
	}

	// Handle callback queries (inline keyboard buttons)
	if update.CallbackQuery != nil {
		b.handleCallbackQuery(ctx, update.CallbackQuery)
		return
	}

	// Handle messages
	if update.Message == nil {
		return
	}

	msg := update.Message
	userID := msg.From.ID
	chatID := msg.Chat.ID

	if !b.isUserAllowed(userID) {
		b.sendMessage(chatID, "❌ You are not authorized to use this bot.")
		return
	}

	if !b.rateLimiter.Allow(userID) {
		b.sendMessage(chatID, "⏱️ Rate limit exceeded. Please wait a moment.")
		return
	}

	session := b.getOrCreateSession(userID)
	session.mu.Lock()
	session.LastActivity = time.Now()
	session.mu.Unlock()

	if b.menuBuilder.IsReplyKeyboardEnabled() && !msg.IsCommand() {
		if b.handleReplyKeyboard(ctx, msg, session) {
			return
		}
	}

	if msg.IsCommand() {
		b.handleCommand(ctx, msg, session)
	} else if session.isInExecMode() {
		b.handleExecInput(ctx, msg, session)
	} else {
		b.showMainMenu(chatID, session)
	}
}

func (b *Bot) handleInlineQuery(ctx context.Context, inlineQuery *tgbotapi.InlineQuery) {
	userID := inlineQuery.From.ID
	if !b.isUserAllowed(userID) {
		return
	}

	// Rate limiting for inline queries
	if !b.rateLimiter.Allow(userID) {
		return
	}

	handler := handlers.NewInlineQueryHandler(b)
	err := handler.HandleInlineQuery(ctx, inlineQuery)
	if err != nil {
		b.logger.Error("Inline query failed",
			zap.Int64("user_id", userID),
			zap.String("query", inlineQuery.Query),
			zap.Error(err),
		)
	}
}

func (rl *RateLimiter) Allow(userID int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	requests := rl.requests[userID]
	valid := make([]time.Time, 0, len(requests))
	for _, t := range requests {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		return false
	}

	valid = append(valid, now)
	rl.requests[userID] = valid
	return true
}

func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	for userID, requests := range rl.requests {
		valid := make([]time.Time, 0, len(requests))
		for _, t := range requests {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.requests, userID)
		} else {
			rl.requests[userID] = valid
		}
	}
}

func SanitizeMarkdown(text string) string {
	specialChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	result := text
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}

func EscapeHTML(text string) string {
	replacer := strings.NewReplacer(
		"&", "&",
		"<", "<",
		">", ">",
		"\"", """,
		"'", "'",
	)
	return replacer.Replace(text)
}

func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func ParseResourceArg(arg string) (resource, name, namespace string) {
	if strings.Contains(arg, "/") {
		parts := strings.SplitN(arg, "/", 2)
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
			namespace = parts[0]
			name = parts[1]
		} else {
			resource = parts[0]
			name = parts[1]
		}
		return
	}

	resourceNameRegex := regexp.MustCompile(`^(\w+)/(\S+)$`)
	matches := resourceNameRegex.FindStringSubmatch(arg)
	if len(matches) == 3 {
		resource = matches[1]
		name = matches[2]
		return
	}

	name = arg
	return
}

func int64Ptr(i int64) *int64 {
	return &i
}