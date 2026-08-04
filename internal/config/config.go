package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// appName is the binary name, and doubles as the config file base name
// (telectl.yaml) and the /etc subdirectory searched for it.
const appName = "telectl"

// envPrefix namespaces config overrides: bot.rate_limit is TELECTL_BOT_RATE_LIMIT.
// Credentials keep their conventional unprefixed names (TELEGRAM_BOT_TOKEN,
// KUBECONFIG), bound explicitly below.
const envPrefix = "TELECTL"

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
	ClusterName       string   `mapstructure:"cluster_name"`
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
		viper.SetConfigName(appName)
		viper.SetConfigType("yaml")
		home, err := os.UserHomeDir()
		if err == nil {
			// NB: deliberately do NOT add "." (cwd) as a search path.
			// A binary named telectl built into the repo root (via
			// 'make build') collides with the config name and viper tries
			// to parse the executable as YAML, producing
			// "yaml: control characters are not allowed".
			viper.AddConfigPath(filepath.Join(home, ".config", appName))
			viper.AddConfigPath(filepath.Join(home, ".config"))
			viper.AddConfigPath("/etc/" + appName)
		}
	}

	// Step 2: Prevent AddConfigPath(home) from ever finding ~/.kube/config.
	// Even though AddConfigPath is scoped above, defensively clear any
	// stale search paths if InitConfig is called more than once.
	viper.SetEnvPrefix(envPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Bind specific env vars. These are env-only bindings and do NOT
	// cause viper to read any file.
	_ = viper.BindEnv("telegram.bot_token", "TELEGRAM_BOT_TOKEN")
	_ = viper.BindEnv("kubernetes.kubeconfig_path", "KUBECONFIG")
	_ = viper.BindEnv("telegram.allowed_user_ids", "ALLOWED_USER_IDS")
	_ = viper.BindEnv("telegram.admin_user_ids", "ADMIN_USER_IDS")

	// Viper does not split a comma-separated env var into []int64, so the two
	// user-ID lists are parsed here.
	applyUserIDEnv("ALLOWED_USER_IDS", "telegram.allowed_user_ids")
	applyUserIDEnv("ADMIN_USER_IDS", "telegram.admin_user_ids")

	setDefaults()

	// Step 3: Read the config file (if any). A missing config file is OK;
	// we'll fall back to defaults + env vars. Any OTHER read error (including
	// an accidental kubeconfig parse) is reported with the file path so the
	// user can identify what was read.
	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return describeConfigReadError(viper.ConfigFileUsed(), err)
		}
		// A missing config file is fine: defaults plus environment are enough.
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// applyUserIDEnv parses a comma-separated list of Telegram user IDs from env
// and stores it under key.
//
// A malformed entry is skipped rather than failing startup, because this path
// also runs for the admin list, which is advisory. The --allowed-users flag
// takes the stricter line and rejects a bad ID outright: that list decides who
// can operate the cluster, and silently dropping an entry there would quietly
// change the answer.
func applyUserIDEnv(envVar, key string) {
	raw := os.Getenv(envVar)
	if raw == "" {
		return
	}

	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if id, err := strconv.ParseInt(part, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		viper.Set(key, ids)
	}
}

// configExtensions are the file extensions viper can actually parse. A file
// without one is not a config file, whatever viper thinks it found.
var configExtensions = map[string]bool{
	".yaml": true, ".yml": true, ".json": true,
	".toml": true, ".ini": true, ".env": true,
	".properties": true, ".props": true, ".prop": true,
	".hcl": true, ".tfvars": true, ".dotenv": true,
}

// describeConfigReadError turns a viper read failure into a message that names
// the file and, where possible, says what the file actually appears to be.
//
// The two cases worth calling out by name are the ones operators hit: viper
// picking up ~/.kube/config (which fails with an opaque "control characters are
// not allowed" from the embedded cert data), and viper picking up the compiled
// telectl binary, which shares the config base name and has no extension.
func describeConfigReadError(usedFile string, err error) error {
	if usedFile == "" {
		return fmt.Errorf("failed to read config: %w", err)
	}

	fi, statErr := os.Stat(usedFile)
	if statErr != nil || fi.Size() == 0 {
		return fmt.Errorf("failed to read config %q: %w", usedFile, err)
	}

	ext := strings.ToLower(filepath.Ext(usedFile))
	if !configExtensions[ext] {
		return fmt.Errorf("refusing to use %q as bot config: not a recognized "+
			"config file (ext=%q). Pass --config /path/to/telectl.yaml explicitly",
			usedFile, ext)
	}

	if isKubeconfig(usedFile) {
		return fmt.Errorf("refusing to use %q as bot config: it looks like a "+
			"kubeconfig file. Pass --config /path/to/telectl.yaml explicitly",
			usedFile)
	}

	return fmt.Errorf("failed to read config %q: %w", usedFile, err)
}

// isKubeconfig reports whether a file looks like a kubeconfig rather than a
// telectl config: it carries kubeconfig markers and no telegram section.
func isKubeconfig(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	body := string(raw)
	hasKubeMarkers := strings.Contains(body, "apiVersion: v1") ||
		strings.Contains(body, "certificate-authority-data")
	return hasKubeMarkers && !strings.Contains(body, "telegram")
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
	viper.SetDefault("kubernetes.cluster_name", "")

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
	// Must stay in sync with the handlers registered in bot.registerHandlers.
	// A command missing here is rejected with "not allowed" even though a
	// handler exists; note the key is "portforward", not "port-forward".
	viper.SetDefault("bot.allowed_commands", []string{
		"start", "help", "version",
		"get", "describe", "logs", "exec", "portforward",
		"contexts", "use-context", "config",
		"top", "events", "watch", "restart", "scale",
		"resources", "monitor", "operations", "settings",
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

// PrintConfig writes the effective configuration to stdout.
//
// It prints viper's merged view rather than reflecting over the Config struct,
// because that is the value actually in effect for each key — including
// anything supplied by environment variables or flag overrides. The bot token
// is redacted: this output is routinely pasted into issue reports.
func PrintConfig() error {
	// Load populates viper as a side effect; the returned struct is not needed
	// because the merged key/value view below is strictly more complete.
	if _, err := Load(); err != nil {
		return err
	}

	fmt.Println("Current Configuration:")
	fmt.Println("=====================")
	keys := viper.AllKeys()
	sort.Strings(keys)
	for _, key := range keys {
		value := viper.Get(key)
		if key == "telegram.bot_token" {
			value = redact(viper.GetString(key))
		}
		fmt.Printf("%s: %v\n", key, value)
	}
	return nil
}

// redact keeps a short prefix so an operator can tell which token is loaded
// without the output disclosing it.
func redact(s string) string {
	if s == "" {
		return "(not set)"
	}
	const keep = 4
	if len(s) <= keep {
		return "****"
	}
	return s[:keep] + "****"
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
			ClusterName:      "",
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
				"get", "describe", "logs", "exec", "portforward",
				"contexts", "use-context", "config",
				"top", "events", "watch", "restart", "scale",
				"resources", "monitor", "operations", "settings",
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

	// Do NOT set a default kubeconfig path here. Let the k8s client handle it:
	// - If kubeconfigPath is explicitly set, use it
	// - If empty, try in-cluster config first, then fall back to default loading rules
	// This avoids hardcoding /home/telectl/.kube/config in container environments

	if cfg.Kubernetes.Timeout <= 0 {
		cfg.Kubernetes.Timeout = 30
	}

	if cfg.Bot.MaxMessageLength <= 0 {
		cfg.Bot.MaxMessageLength = 4096
	}

	if cfg.Bot.RateLimit <= 0 {
		cfg.Bot.RateLimit = 30
	}

	// Expand home directory in kubeconfig path if explicitly set
	if cfg.Kubernetes.KubeconfigPath != "" && strings.HasPrefix(cfg.Kubernetes.KubeconfigPath, "~/") {
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
