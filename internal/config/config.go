package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Config struct {
	Telegram   TelegramConfig   `mapstructure:"telegram"`
	Kubernetes KubernetesConfig `mapstructure:"kubernetes"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	Bot        BotConfig        `mapstructure:"bot"`
}

type TelegramConfig struct {
	BotToken       string  `mapstructure:"bot_token"`
	AllowedUserIDs []int64 `mapstructure:"allowed_user_ids"`
	AdminUserIDs   []int64 `mapstructure:"admin_user_ids"`
	ParseMode      string  `mapstructure:"parse_mode"`
	WebhookURL     string  `mapstructure:"webhook_url"`
	WebhookPort    int     `mapstructure:"webhook_port"`
}

type KubernetesConfig struct {
	KubeconfigPath    string   `mapstructure:"kubeconfig_path"`
	DefaultNamespace  string   `mapstructure:"default_namespace"`
	Context           string   `mapstructure:"context"`
	Timeout           int      `mapstructure:"timeout"`
	DryRun            bool     `mapstructure:"dry_run"`
	ImpersonateUser   string   `mapstructure:"impersonate_user"`
	ImpersonateGroups []string `mapstructure:"impersonate_groups"`
	Burst             int      `mapstructure:"burst"`
	QPS               float32  `mapstructure:"qps"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

type BotConfig struct {
	MaxMessageLength int      `mapstructure:"max_message_length"`
	CommandPrefix    string   `mapstructure:"command_prefix"`
	EnableMarkdown   bool     `mapstructure:"enable_markdown"`
	RateLimit        int      `mapstructure:"rate_limit"`
	AllowedCommands  []string `mapstructure:"allowed_commands"`

	// Menu system
	EnableMenuButton    bool `mapstructure:"enable_menu_button"`
	EnableReplyKeyboard bool `mapstructure:"enable_reply_keyboard"`
	MenuPageSize        int  `mapstructure:"menu_page_size"`
}

var cfg *Config

func InitConfig(configFile string) error {
	// Step 1: If a config file is provided, use it. Otherwise search
	// well-defined paths. We deliberately do NOT add the home directory
	// as a config search path because users keep unrelated YAML files
	// there (most notably ~/.kube/config) and viper will happily try to
	// parse those as the bot config, producing confusing errors like
	// "yaml: control characters are not allowed" when the kubeconfig
	// contains binary cert data.
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("k8s-telegram-bot")
		viper.SetConfigType("yaml")
		home, err := os.UserHomeDir()
		if err == nil {
			// NB: deliberately do NOT add "." (cwd) as a search path.
			// A binary named k8s-telegram-bot built into the repo root
			// (via 'make build') collides with the config name and viper
			// tries to parse the executable as YAML, producing
			// "yaml: control characters are not allowed".
			viper.AddConfigPath(filepath.Join(home, ".config", "k8s-telegram-bot"))
			viper.AddConfigPath(filepath.Join(home, ".config"))
			viper.AddConfigPath("/etc/k8s-telegram-bot")
		}
	}

	// Step 2: Prevent AddConfigPath(home) from ever finding ~/.kube/config.
	// Even though AddConfigPath is scoped above, defensively clear any
	// stale search paths if InitConfig is called more than once.
	viper.SetEnvPrefix("K8SBOT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Bind specific env vars. These are env-only bindings and do NOT
	// cause viper to read any file.
	_ = viper.BindEnv("telegram.bot_token", "TELEGRAM_BOT_TOKEN")
	_ = viper.BindEnv("kubernetes.kubeconfig_path", "KUBECONFIG")
	_ = viper.BindEnv("telegram.allowed_user_ids", "ALLOWED_USER_IDS")
	_ = viper.BindEnv("telegram.admin_user_ids", "ADMIN_USER_IDS")

	setDefaults()

	// Step 3: Read the config file (if any). A missing config file is OK;
	// we'll fall back to defaults + env vars. Any OTHER read error (including
	// an accidental kubeconfig parse) is reported with the file path so the
	// user can identify what was read.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found is OK, we'll use defaults + env vars
		} else {
			usedFile := viper.ConfigFileUsed()
			// detect the most common misconfiguration: viper accidentally
			// picked up a kubeconfig file (or the compiled binary itself)
			// as the bot config.
			if usedFile != "" {
				if fi, statErr := os.Stat(usedFile); statErr == nil && fi.Size() > 0 {
					// Refuse a file with no recognized config extension —
					// this catches the case where the compiled binary
					// (k8s-telegram-bot, no extension) is found as config.
					ext := strings.ToLower(filepath.Ext(usedFile))
					validExts := map[string]bool{
						".yaml": true, ".yml": true, ".json": true,
						".toml": true, ".ini": true, ".env": true,
						".properties": true, ".props": true, ".prop": true,
						".hcl": true, ".tfvars": true, ".dotenv": true,
					}
					if ext == "" || !validExts[ext] {
						return fmt.Errorf("refusing to use %q as bot config: not a recognized config file (ext=%q). Pass --config /path/to/k8s-telegram-bot.yaml explicitly", usedFile, ext)
					}
					raw, readErr := os.ReadFile(usedFile)
					if readErr == nil && (strings.Contains(string(raw), "apiVersion: v1") || strings.Contains(string(raw), "certificate-authority-data")) && !strings.Contains(string(raw), "telegram") {
						return fmt.Errorf("refusing to use %q as bot config: it looks like a kubeconfig file. Pass --config /path/to/k8s-telegram-bot.yaml explicitly", usedFile)
					}
				}
			}
			return fmt.Errorf("failed to read config %q: %w", usedFile, err)
		}
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

func setDefaults() {
	// Telegram defaults
	viper.SetDefault("telegram.parse_mode", "MarkdownV2")
	viper.SetDefault("telegram.webhook_port", 8443)

	// Kubernetes defaults
	viper.SetDefault("kubernetes.default_namespace", "default")
	viper.SetDefault("kubernetes.timeout", 30)
	viper.SetDefault("kubernetes.dry_run", false)
	viper.SetDefault("kubernetes.burst", 10)
	viper.SetDefault("kubernetes.qps", 5.0)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")

	// Bot defaults
	viper.SetDefault("bot.max_message_length", 4096)
	viper.SetDefault("bot.command_prefix", "/")
	viper.SetDefault("bot.enable_markdown", true)
	viper.SetDefault("bot.rate_limit", 30)
	viper.SetDefault("bot.enable_menu_button", true)
	viper.SetDefault("bot.enable_reply_keyboard", true)
	viper.SetDefault("bot.menu_page_size", 10)
	viper.SetDefault("bot.allowed_commands", []string{
		"start", "help", "version",
		"get", "describe", "logs", "exec", "port-forward",
		"contexts", "use-context", "config",
		"top", "events", "watch",
	})
}

func Load() (*Config, error) {
	if cfg == nil {
		if err := InitConfig(""); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func Get() *Config {
	return cfg
}

func PrintConfig() error {
	c, err := Load()
	if err != nil {
		return err
	}

	fmt.Println("Current Configuration:")
	fmt.Println("=====================")
	printConfig("", c)
	return nil
}

func printConfig(prefix string, v interface{}) {
	// This would need reflection to print all fields nicely
	// For now, just print the viper settings
	for _, key := range viper.AllKeys() {
		fmt.Printf("%s%s: %v\n", prefix, key, viper.Get(key))
	}
}

func CreateDefaultConfig(path string) error {
	c := &Config{
		Telegram: TelegramConfig{
			ParseMode:   "MarkdownV2",
			WebhookPort: 8443,
		},
		Kubernetes: KubernetesConfig{
			DefaultNamespace: "default",
			Timeout:          30,
			DryRun:           false,
			Burst:            10,
			QPS:              5.0,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Bot: BotConfig{
			MaxMessageLength:    4096,
			CommandPrefix:       "/",
			EnableMarkdown:      true,
			RateLimit:           30,
			EnableMenuButton:    true,
			EnableReplyKeyboard: true,
			MenuPageSize:        10,
			AllowedCommands: []string{
				"start", "help", "version",
				"get", "describe", "logs", "exec", "port-forward",
				"contexts", "use-context", "config",
				"top", "events", "watch",
			},
		},
	}

	viper.Set("telegram", c.Telegram)
	viper.Set("kubernetes", c.Kubernetes)
	viper.Set("logging", c.Logging)
	viper.Set("bot", c.Bot)

	return viper.WriteConfigAs(path)
}

func ValidateConfig(cfg *Config) error {
	if cfg.Telegram.BotToken == "" {
		return fmt.Errorf("telegram bot token is required")
	}

	if cfg.Kubernetes.KubeconfigPath == "" {
		home, _ := os.UserHomeDir()
		cfg.Kubernetes.KubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	if cfg.Kubernetes.Timeout <= 0 {
		cfg.Kubernetes.Timeout = 30
	}

	if cfg.Bot.MaxMessageLength <= 0 {
		cfg.Bot.MaxMessageLength = 4096
	}

	if cfg.Bot.RateLimit <= 0 {
		cfg.Bot.RateLimit = 30
	}

	// Expand home directory in kubeconfig path
	if strings.HasPrefix(cfg.Kubernetes.KubeconfigPath, "~/") {
		home, _ := os.UserHomeDir()
		cfg.Kubernetes.KubeconfigPath = filepath.Join(home, cfg.Kubernetes.KubeconfigPath[2:])
	}

	return nil
}

func SetupLogger(cfg *LoggingConfig) (*zap.Logger, error) {
	var zapCfg zap.Config
	switch cfg.Level {
	case "debug":
		zapCfg = zap.NewDevelopmentConfig()
	case "info", "warn", "error":
		zapCfg = zap.NewProductionConfig()
	default:
		zapCfg = zap.NewProductionConfig()
	}

	if cfg.Format == "console" {
		zapCfg.Encoding = "console"
	}

	switch cfg.Output {
	case "stderr":
		zapCfg.OutputPaths = []string{"stderr"}
	case "stdout":
		zapCfg.OutputPaths = []string{"stdout"}
	default:
		zapCfg.OutputPaths = []string{"stdout"}
	}

	return zapCfg.Build()
}
