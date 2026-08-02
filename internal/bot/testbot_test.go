package bot

import (
	"testing"

	bottg "github.com/go-telegram/bot"
	"github.com/ksauraj/telectl/internal/config"
	"github.com/ksauraj/telectl/internal/handlers"
	"github.com/ksauraj/telectl/internal/menus"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// newTestBot assembles a Bot wired to a stub Telegram API. It deliberately
// bypasses New(), which requires a reachable cluster: routing, menu rendering
// and parse-mode handling are all independent of Kubernetes, so they can be
// tested without one. k8sClient stays nil; tests here only exercise commands
// that do not touch the cluster.
func newTestBot(t *testing.T, fake *fakeTelegram) (*Bot, *bottg.Bot) {
	t.Helper()

	cfg := &config.Config{}
	cfg.Telegram.BotToken = "test:token"
	cfg.Bot.RateLimit = 100
	cfg.Bot.MaxMessageLength = 4096
	cfg.Bot.EnableMenuButton = true
	cfg.Bot.EnableReplyKeyboard = true
	cfg.Bot.MenuPageSize = 10
	cfg.Kubernetes.DefaultNamespace = "default"
	cfg.Kubernetes.KubeconfigPath = "/dev/null"
	// Empty means "allow everything"; individual tests narrow this.
	cfg.Telegram.AllowedUserIDs = nil

	lib := mustLibBot(t, fake.srv.URL)

	realBot, err := tg.NewRealBot("test:token",
		bottg.WithServerURL(fake.srv.URL),
		bottg.WithSkipGetMe(),
		bottg.WithNotAsyncHandlers(),
	)
	if err != nil {
		t.Fatalf("new real bot: %v", err)
	}

	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))

	b := &Bot{
		tgBot:       realBot,
		libBot:      lib,
		config:      cfg,
		logger:      logger,
		handlers:    make(map[string]handlers.CommandHandler),
		rateLimiter: types.NewRateLimiter(cfg.Bot.RateLimit, 60_000_000_000),
		menuBuilder: menus.NewMenuBuilder(cfg),
	}
	b.registerHandlers()
	return b, lib
}
