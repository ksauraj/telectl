package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/ksauraj/telectl/internal/bot"
	"github.com/ksauraj/telectl/internal/config"
	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	cfgFile    string
	botToken   string
	kubeconfig string
	allowedIDs string
	logLevel   string
	dryRun     bool
)

var rootCmd = &cobra.Command{
	Use:   "k8s-telegram-bot",
	Short: "A Telegram bot for Kubernetes cluster management",
	Long: `k8s-telegram-bot is a production-ready Telegram bot that provides
full Kubernetes cluster management capabilities including:

- List pods, deployments, services, replica sets, namespaces, nodes, etc.
- View and follow pod/container logs
- Execute commands in containers
- Port forwarding
- Kubeconfig context management
- Cluster resource monitoring
- And much more...

Built with Go for performance and reliability.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return config.InitConfig(cfgFile)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBot(cmd.Context())
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("k8s-telegram-bot v0.1.0")
		fmt.Println("Built with Go for Kubernetes")
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage bot configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		return config.PrintConfig()
	},
}

var contextsCmd = &cobra.Command{
	Use:   "contexts",
	Short: "List and manage kubeconfig contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listContexts(cmd.Context())
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default: $HOME/.k8s-telegram-bot.yaml)")
	rootCmd.PersistentFlags().StringVar(&botToken, "token", "", "Telegram bot token (or set TELEGRAM_BOT_TOKEN env)")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (or set KUBECONFIG env)")
	rootCmd.PersistentFlags().StringVar(&allowedIDs, "allowed-users", "", "Comma-separated allowed user IDs")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Run in dry-run mode (no API calls)")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(contextsCmd)
}

func initConfig() {
	if botToken != "" {
		os.Setenv("TELEGRAM_BOT_TOKEN", botToken)
	}
	if kubeconfig != "" {
		os.Setenv("KUBECONFIG", kubeconfig)
	}
	if allowedIDs != "" {
		os.Setenv("ALLOWED_USER_IDS", allowedIDs)
	}
}

func runBot(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override with flags
	if botToken != "" {
		cfg.Telegram.BotToken = botToken
	}
	if kubeconfig != "" {
		cfg.Kubernetes.KubeconfigPath = kubeconfig
	}
	if allowedIDs != "" {
		cfg.Telegram.AllowedUserIDs = parseUserIDs(allowedIDs)
	}
	if dryRun {
		cfg.Kubernetes.DryRun = true
	}

	// Validate required config
	if cfg.Telegram.BotToken == "" {
		return fmt.Errorf("telegram bot token is required (set TELEGRAM_BOT_TOKEN or use --token)")
	}

	// Initialize logger
	logger, err := newLogger(cfg.Logging.Level)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.Sync()

	logger.Info("Starting k8s-telegram-bot",
		zap.String("version", "0.1.0"),
		zap.String("kubeconfig", cfg.Kubernetes.KubeconfigPath),
		zap.Bool("dry_run", cfg.Kubernetes.DryRun),
	)

	// Create and start bot
	b, err := bot.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create bot: %w", err)
	}

	return b.Start(ctx)
}

func listContexts(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if kubeconfig != "" {
		cfg.Kubernetes.KubeconfigPath = kubeconfig
	}

	logger, _ := newLogger("info")
	k8sClient, err := k8s.NewClient(cfg.Kubernetes.KubeconfigPath, cfg.Kubernetes.Context, cfg.Kubernetes.DryRun, logger)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	contexts, err := k8sClient.ListContexts(ctx)
	if err != nil {
		return fmt.Errorf("failed to list contexts: %w", err)
	}

	fmt.Println("Available Kubernetes Contexts:")
	fmt.Println("==============================")
	for _, ctx := range contexts {
		current := ""
		if ctx.Current {
			current = " (current)"
		}
		fmt.Printf("  %s%s\n", ctx.Name, current)
		fmt.Printf("    Cluster: %s (%s)\n", ctx.Cluster, ctx.ClusterServer)
		fmt.Printf("    User: %s\n", ctx.User)
		fmt.Printf("    Namespace: %s\n", ctx.Namespace)
		fmt.Println()
	}
	return nil
}

func parseUserIDs(s string) []int64 {
	var ids []int64
	for _, part := range splitAndTrim(s, ",") {
		var id int64
		fmt.Sscanf(part, "%d", &id)
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func splitAndTrim(s, sep string) []string {
	var result []string
	for _, part := range split(s, sep) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func split(s, sep string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func newLogger(level string) (*zap.Logger, error) {
	var cfg zap.Config
	switch level {
	case "debug":
		cfg = zap.NewDevelopmentConfig()
	case "info", "warn", "error":
		cfg = zap.NewProductionConfig()
	default:
		cfg = zap.NewProductionConfig()
	}
	return cfg.Build()
}

func Execute(ctx context.Context, logger *zap.Logger) error {
	// Override logger for commands that need it
	return rootCmd.ExecuteContext(ctx)
}
