// Package cmd implements the telectl command line: flag parsing, config
// loading, and the subcommands that run without starting the bot.
package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

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

// Build metadata, set from main via SetBuildInfo.
var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

// SetBuildInfo records the values main received from -ldflags, so `telectl
// version` and the startup log report the real build rather than a hardcoded
// string.
func SetBuildInfo(version, commit, date string) {
	buildVersion, buildCommit, buildDate = version, commit, date
}

var rootCmd = &cobra.Command{
	Use:   "telectl",
	Short: "Manage Kubernetes clusters from Telegram",
	Long: `telectl is a Telegram bot for operating Kubernetes clusters from chat.

It talks to the API server directly through client-go — kubectl is not required
and is not bundled. Access is defined by the kubeconfig context it runs against,
so telectl can do exactly what that context's RBAC permits, and no more.

Capabilities:
  - Browse pods, deployments, services, replicasets, nodes, namespaces and more
  - Per-resource detail panes: describe, labels, events, selector, manifest
  - Read logs, exec into containers, port-forward
  - Restart and scale workloads; cordon, uncordon and drain nodes
  - Switch kubeconfig contexts and namespaces per chat session
  - Resource usage and cluster events

Access is restricted to the Telegram user IDs in --allowed-users; with none set,
the bot answers anyone who finds it. Always set it.`,
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		return config.InitConfig(cfgFile)
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runBot(cmd.Context(), cmd)
	},
	SilenceUsage: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("telectl %s\ncommit: %s\nbuilt:  %s\n",
			buildVersion, buildCommit, buildDate)
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print the effective configuration",
	RunE: func(_ *cobra.Command, _ []string) error {
		return config.PrintConfig()
	},
}

var contextsCmd = &cobra.Command{
	Use:   "contexts",
	Short: "List kubeconfig contexts",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return listContexts(cmd.Context())
	},
}

func init() {
	cobra.OnInitialize(applyFlagEnvOverrides)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"Config file (default: search $HOME/.config/telectl/, $HOME/.config/, /etc/telectl/)")
	rootCmd.PersistentFlags().StringVar(&botToken, "token", "",
		"Telegram bot token (or set TELEGRAM_BOT_TOKEN)")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "",
		"Path to kubeconfig file (or set KUBECONFIG)")
	rootCmd.PersistentFlags().StringVar(&allowedIDs, "allowed-users", "",
		"Comma-separated Telegram user IDs permitted to use the bot")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info",
		"Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false,
		"Log mutating operations instead of performing them")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(contextsCmd)
}

// applyFlagEnvOverrides publishes flag values as environment variables so
// viper's env bindings pick them up during InitConfig, which runs before the
// flags are otherwise consulted.
func applyFlagEnvOverrides() {
	for env, val := range map[string]string{
		"TELEGRAM_BOT_TOKEN": botToken,
		"KUBECONFIG":         kubeconfig,
		"ALLOWED_USER_IDS":   allowedIDs,
	} {
		if val != "" {
			// A failure here would mean the process environment is unusable;
			// config validation reports the resulting missing value.
			_ = os.Setenv(env, val)
		}
	}
}

func runBot(ctx context.Context, cmd *cobra.Command) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Flags outrank config file and environment.
	if botToken != "" {
		cfg.Telegram.BotToken = botToken
	}
	if kubeconfig != "" {
		cfg.Kubernetes.KubeconfigPath = kubeconfig
	}
	if allowedIDs != "" {
		ids, parseErr := parseUserIDs(allowedIDs)
		if parseErr != nil {
			return parseErr
		}
		cfg.Telegram.AllowedUserIDs = ids
	}
	if dryRun {
		cfg.Kubernetes.DryRun = true
	}
	// Only override the configured level when the flag was actually passed, so
	// a `logging.level` from the config file is not clobbered by the flag's own
	// "info" default.
	if cmd != nil && cmd.Flags().Changed("log-level") {
		cfg.Logging.Level = logLevel
	}

	if cfg.Telegram.BotToken == "" {
		return fmt.Errorf("telegram bot token is required (set TELEGRAM_BOT_TOKEN or use --token)")
	}

	logger, err := newLogger(cfg.Logging.Level)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	if len(cfg.Telegram.AllowedUserIDs) == 0 {
		logger.Warn("No allowed users configured — every Telegram user who finds " +
			"this bot can operate the cluster. Set --allowed-users or " +
			"telegram.allowed_user_ids.")
	}

	logger.Info("Starting telectl",
		zap.String("version", buildVersion),
		zap.String("commit", buildCommit),
		zap.String("kubeconfig", cfg.Kubernetes.KubeconfigPath),
		zap.Bool("dry_run", cfg.Kubernetes.DryRun),
		zap.Int("allowed_users", len(cfg.Telegram.AllowedUserIDs)),
	)

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

	logger, err := newLogger("info")
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

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
	for _, kubeCtx := range contexts {
		current := ""
		if kubeCtx.Current {
			current = " (current)"
		}
		fmt.Printf("  %s%s\n", kubeCtx.Name, current)
		fmt.Printf("    Cluster: %s (%s)\n", kubeCtx.Cluster, kubeCtx.ClusterServer)
		fmt.Printf("    User: %s\n", kubeCtx.User)
		fmt.Printf("    Namespace: %s\n", kubeCtx.Namespace)
		fmt.Println()
	}
	return nil
}

// parseUserIDs converts a comma-separated Telegram user ID list.
//
// A malformed entry is an error rather than a skip: silently dropping an ID
// from an allowlist would quietly widen or narrow who can operate the cluster.
func parseUserIDs(s string) ([]int64, error) {
	parts := strings.Split(s, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID %q in --allowed-users: must be a number", part)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// newLogger builds the application logger at the requested level.
func newLogger(level string) (*zap.Logger, error) {
	var cfg zap.Config
	if level == "debug" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}
	// Choosing the base config is not enough: production config pins the level
	// to Info, so "warn"/"error" were silently ignored. Set it explicitly.
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", level, err)
	}
	cfg.Level = lvl
	return cfg.Build()
}

// Execute runs the root command.
func Execute(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}
