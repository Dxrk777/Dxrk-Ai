// SPDX-License-Identifier: MIT
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// ---- Hierarchical Config Types ----

// ModelConfig holds AI model configuration.
type ModelConfig struct {
	Provider     string  `yaml:"provider" json:"provider"`
	ModelName    string  `yaml:"model_name" json:"model_name"`
	MaxTokens    int     `yaml:"max_tokens" json:"max_tokens"`
	Temperature  float64 `yaml:"temperature" json:"temperature"`
	TopP         float64 `yaml:"top_p" json:"top_p"`
	SystemPrompt string  `yaml:"system_prompt" json:"system_prompt"`
}

// APIConfig holds API connection settings.
type APIConfig struct {
	BaseURL   string `yaml:"base_url" json:"base_url"`
	APIKey    string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	Timeout   int    `yaml:"timeout" json:"timeout"`
	Retries   int    `yaml:"retries" json:"retries"`
	RateLimit int    `yaml:"rate_limit" json:"rate_limit"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Provider  string   `yaml:"provider" json:"provider"`
	ClientID  string   `yaml:"client_id" json:"client_id"`
	Scopes    []string `yaml:"scopes" json:"scopes"`
	TokenPath string   `yaml:"token_path" json:"token_path"`
}

// SessionConfig holds session management settings.
type SessionConfig struct {
	MaxHistory   int  `yaml:"max_history" json:"max_history"`
	AutoSave     bool `yaml:"auto_save" json:"auto_save"`
	ArchiveAfter int  `yaml:"archive_after" json:"archive_after"`
	RestoreLast  bool `yaml:"restore_last" json:"restore_last"`
}

// ToolsConfig holds tool configuration.
type ToolsConfig struct {
	Enabled       []string `yaml:"enabled" json:"enabled"`
	Disabled      []string `yaml:"disabled" json:"disabled"`
	Timeout       int      `yaml:"timeout" json:"timeout"`
	MaxConcurrent int      `yaml:"max_concurrent" json:"max_concurrent"`
}

// UIConfig holds UI preferences.
type UIConfig struct {
	Theme       string `yaml:"theme" json:"theme"`
	FontSize    int    `yaml:"font_size" json:"font_size"`
	ShowTokens  bool   `yaml:"show_tokens" json:"show_tokens"`
	ShowCost    bool   `yaml:"show_cost" json:"show_cost"`
	CompactMode bool   `yaml:"compact_mode" json:"compact_mode"`
}

// AdvancedConfig holds advanced/experimental settings.
type AdvancedConfig struct {
	Debug      bool   `yaml:"debug" json:"debug"`
	LogLevel   string `yaml:"log_level" json:"log_level"`
	Telemetry  bool   `yaml:"telemetry" json:"telemetry"`
	AutoUpdate bool   `yaml:"auto_update" json:"auto_update"`
	YoloMode   bool   `yaml:"yolo_mode" json:"yolo_mode"`
}

// HierarchicalConfig is the top-level hierarchical configuration.
type HierarchicalConfig struct {
	Model    ModelConfig    `yaml:"model" json:"model"`
	API      APIConfig      `yaml:"api" json:"api"`
	Auth     AuthConfig     `yaml:"auth" json:"auth"`
	Session  SessionConfig  `yaml:"session" json:"session"`
	Tools    ToolsConfig    `yaml:"tools" json:"tools"`
	UI       UIConfig       `yaml:"ui" json:"ui"`
	Advanced AdvancedConfig `yaml:"advanced" json:"advanced"`
}

// ConfigError represents a configuration validation error.
type ConfigError struct {
	Path       string `json:"path"`
	Message    string `json:"message"`
	Severity   string `json:"severity"`
	Suggestion string `json:"suggestion,omitempty"`
}

func (e ConfigError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Severity, e.Path, e.Message)
}

// Watcher is a callback for config change notifications.
type Watcher func(path string, value any)

// Option is a functional option for ConfigManager.
type Option func(*ConfigManager)

// WithGlobalPath sets the global config file path.
func WithGlobalPath(path string) Option {
	return func(cm *ConfigManager) { cm.globalPath = path }
}

// WithUserPath sets the user config file path.
func WithUserPath(path string) Option {
	return func(cm *ConfigManager) { cm.userPath = path }
}

// WithProjectPath sets the project config file path.
func WithProjectPath(path string) Option {
	return func(cm *ConfigManager) { cm.projectPath = path }
}

// WithEnvPrefix sets the environment variable prefix.
func WithEnvPrefix(prefix string) Option {
	return func(cm *ConfigManager) { cm.envPrefix = prefix }
}

// ConfigManager manages hierarchical configuration loading, access, and persistence.
type ConfigManager struct {
	mu          sync.RWMutex
	config      *HierarchicalConfig
	defaults    *HierarchicalConfig
	globalPath  string
	userPath    string
	projectPath string
	envPrefix   string
	watchers    map[string][]Watcher
}

// NewConfigManager creates a new ConfigManager with functional options.
func NewConfigManager(opts ...Option) *ConfigManager {
	home, _ := os.UserHomeDir()
	cm := &ConfigManager{
		config:      defaultHierarchicalConfig(),
		defaults:    defaultHierarchicalConfig(),
		globalPath:  "/etc/dxrk/config.yaml",
		userPath:    filepath.Join(home, ".dxrk", "config.yaml"),
		projectPath: ".dxrk/config.yaml",
		envPrefix:   "DXRK",
		watchers:    make(map[string][]Watcher),
	}
	for _, opt := range opts {
		opt(cm)
	}
	return cm
}

// defaultHierarchicalConfig returns built-in defaults.
func defaultHierarchicalConfig() *HierarchicalConfig {
	return &HierarchicalConfig{
		Model: ModelConfig{
			Provider:    "claude",
			ModelName:   "claude-sonnet-4-20250514",
			MaxTokens:   8192,
			Temperature: 0.7,
			TopP:        0.9,
		},
		API: APIConfig{
			BaseURL:   "https://api.anthropic.com",
			Timeout:   30,
			Retries:   3,
			RateLimit: 60,
		},
		Auth: AuthConfig{
			Provider:  "oauth",
			Scopes:    []string{"read", "write"},
			TokenPath: "~/.dxrk/tokens",
		},
		Session: SessionConfig{
			MaxHistory:   100,
			AutoSave:     true,
			ArchiveAfter: 24,
			RestoreLast:  true,
		},
		Tools: ToolsConfig{
			Timeout:       30,
			MaxConcurrent: 5,
		},
		UI: UIConfig{
			Theme:       "dark",
			FontSize:    14,
			ShowTokens:  true,
			ShowCost:    true,
			CompactMode: false,
		},
		Advanced: AdvancedConfig{
			Debug:      false,
			LogLevel:   "info",
			Telemetry:  true,
			AutoUpdate: true,
			YoloMode:   false,
		},
	}
}

// Load loads configuration from all sources in priority order.
// Priority: CLI flags > env vars > project > user > global > defaults.
func (cm *ConfigManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Start with defaults
	cm.config = cm.defaults

	// Layer global config
	if err := cm.loadFile(cm.globalPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load global config: %w", err)
	}

	// Layer user config
	if err := cm.loadFile(cm.userPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load user config: %w", err)
	}

	// Layer project config
	if err := cm.loadFile(cm.projectPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load project config: %w", err)
	}

	// Layer environment variables
	cm.loadFromEnv()

	return nil
}

// loadFile merges a YAML/JSON file into the current config.
func (cm *ConfigManager) loadFile(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return err
	}

	var overlay HierarchicalConfig
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &overlay); err != nil {
			return fmt.Errorf("parse json %s: %w", path, err)
		}
	default:
		// Try YAML, then JSON as fallback
		if err := yamlUnmarshal(data, &overlay); err != nil {
			if err2 := json.Unmarshal(data, &overlay); err2 != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
		}
	}

	cm.merge(&overlay)
	return nil
}

// yamlUnmarshal wraps yaml.Unmarshal for config file parsing.
var yamlUnmarshal = yaml.Unmarshal

// merge applies overlay values on top of the current config (non-zero wins).
func (cm *ConfigManager) merge(overlay *HierarchicalConfig) {
	if overlay.Model.Provider != "" {
		cm.config.Model.Provider = overlay.Model.Provider
	}
	if overlay.Model.ModelName != "" {
		cm.config.Model.ModelName = overlay.Model.ModelName
	}
	if overlay.Model.MaxTokens != 0 {
		cm.config.Model.MaxTokens = overlay.Model.MaxTokens
	}
	if overlay.Model.Temperature != 0 {
		cm.config.Model.Temperature = overlay.Model.Temperature
	}
	if overlay.Model.TopP != 0 {
		cm.config.Model.TopP = overlay.Model.TopP
	}
	if overlay.Model.SystemPrompt != "" {
		cm.config.Model.SystemPrompt = overlay.Model.SystemPrompt
	}
	if overlay.API.BaseURL != "" {
		cm.config.API.BaseURL = overlay.API.BaseURL
	}
	if overlay.API.APIKey != "" {
		cm.config.API.APIKey = overlay.API.APIKey
	}
	if overlay.API.Timeout != 0 {
		cm.config.API.Timeout = overlay.API.Timeout
	}
	if overlay.API.Retries != 0 {
		cm.config.API.Retries = overlay.API.Retries
	}
	if overlay.API.RateLimit != 0 {
		cm.config.API.RateLimit = overlay.API.RateLimit
	}
	if overlay.Auth.Provider != "" {
		cm.config.Auth.Provider = overlay.Auth.Provider
	}
	if overlay.Auth.ClientID != "" {
		cm.config.Auth.ClientID = overlay.Auth.ClientID
	}
	if len(overlay.Auth.Scopes) > 0 {
		cm.config.Auth.Scopes = overlay.Auth.Scopes
	}
	if overlay.Auth.TokenPath != "" {
		cm.config.Auth.TokenPath = overlay.Auth.TokenPath
	}
	if overlay.Session.MaxHistory != 0 {
		cm.config.Session.MaxHistory = overlay.Session.MaxHistory
	}
	if overlay.Tools.Timeout != 0 {
		cm.config.Tools.Timeout = overlay.Tools.Timeout
	}
	if overlay.Tools.MaxConcurrent != 0 {
		cm.config.Tools.MaxConcurrent = overlay.Tools.MaxConcurrent
	}
	if len(overlay.Tools.Enabled) > 0 {
		cm.config.Tools.Enabled = overlay.Tools.Enabled
	}
	if len(overlay.Tools.Disabled) > 0 {
		cm.config.Tools.Disabled = overlay.Tools.Disabled
	}
	if overlay.UI.Theme != "" {
		cm.config.UI.Theme = overlay.UI.Theme
	}
	if overlay.UI.FontSize != 0 {
		cm.config.UI.FontSize = overlay.UI.FontSize
	}
	if overlay.Advanced.LogLevel != "" {
		cm.config.Advanced.LogLevel = overlay.Advanced.LogLevel
	}
	// Booleans: only override if explicitly set via a marker
	// (handled by env vars and viper directly)
}

// loadFromEnv reads DXRK_* environment variables and applies them.
func (cm *ConfigManager) loadFromEnv() {
	prefix := cm.envPrefix + "_"
	envMap := map[string]*string{
		prefix + "MODEL_PROVIDER":  &cm.config.Model.Provider,
		prefix + "MODEL_NAME":      &cm.config.Model.ModelName,
		prefix + "API_BASE_URL":    &cm.config.API.BaseURL,
		prefix + "API_KEY":         &cm.config.API.APIKey,
		prefix + "AUTH_PROVIDER":   &cm.config.Auth.Provider,
		prefix + "AUTH_CLIENT_ID":  &cm.config.Auth.ClientID,
		prefix + "AUTH_TOKEN_PATH": &cm.config.Auth.TokenPath,
		prefix + "UI_THEME":        &cm.config.UI.Theme,
		prefix + "LOG_LEVEL":       &cm.config.Advanced.LogLevel,
	}

	for envKey, target := range envMap {
		if val, ok := os.LookupEnv(envKey); ok {
			*target = val
		}
	}

	// Numeric env vars
	numMap := map[string]*int{
		prefix + "MODEL_MAX_TOKENS":     &cm.config.Model.MaxTokens,
		prefix + "API_TIMEOUT":          &cm.config.API.Timeout,
		prefix + "API_RETRIES":          &cm.config.API.Retries,
		prefix + "API_RATE_LIMIT":       &cm.config.API.RateLimit,
		prefix + "SESSION_MAX_HISTORY":  &cm.config.Session.MaxHistory,
		prefix + "TOOLS_TIMEOUT":        &cm.config.Tools.Timeout,
		prefix + "TOOLS_MAX_CONCURRENT": &cm.config.Tools.MaxConcurrent,
		prefix + "UI_FONT_SIZE":         &cm.config.UI.FontSize,
	}

	for envKey, target := range numMap {
		if val, ok := os.LookupEnv(envKey); ok {
			var n int
			if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
				*target = n
			}
		}
	}

	// Float env vars
	floatMap := map[string]*float64{
		prefix + "MODEL_TEMPERATURE": &cm.config.Model.Temperature,
		prefix + "MODEL_TOP_P":       &cm.config.Model.TopP,
	}

	for envKey, target := range floatMap {
		if val, ok := os.LookupEnv(envKey); ok {
			var f float64
			if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
				*target = f
			}
		}
	}

	// Boolean env vars
	boolMap := map[string]*bool{
		prefix + "SESSION_AUTO_SAVE":    &cm.config.Session.AutoSave,
		prefix + "SESSION_RESTORE_LAST": &cm.config.Session.RestoreLast,
		prefix + "UI_SHOW_TOKENS":       &cm.config.UI.ShowTokens,
		prefix + "UI_SHOW_COST":         &cm.config.UI.ShowCost,
		prefix + "UI_COMPACT_MODE":      &cm.config.UI.CompactMode,
		prefix + "ADVANCED_DEBUG":       &cm.config.Advanced.Debug,
		prefix + "ADVANCED_TELEMETRY":   &cm.config.Advanced.Telemetry,
		prefix + "ADVANCED_AUTO_UPDATE": &cm.config.Advanced.AutoUpdate,
		prefix + "ADVANCED_YOLO_MODE":   &cm.config.Advanced.YoloMode,
	}

	for envKey, target := range boolMap {
		if val, ok := os.LookupEnv(envKey); ok {
			*target = strings.EqualFold(val, "true") || val == "1"
		}
	}
}

// Get retrieves a config value by dot-notation path (e.g., "model.provider").
func (cm *ConfigManager) Get(path string) any {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return nil
	}

	switch parts[0] {
	case "model":
		return cm.getField(parts[1:], &cm.config.Model)
	case "api":
		return cm.getField(parts[1:], &cm.config.API)
	case "auth":
		return cm.getField(parts[1:], &cm.config.Auth)
	case "session":
		return cm.getField(parts[1:], &cm.config.Session)
	case "tools":
		return cm.getField(parts[1:], &cm.config.Tools)
	case "ui":
		return cm.getField(parts[1:], &cm.config.UI)
	case "advanced":
		return cm.getField(parts[1:], &cm.config.Advanced)
	}
	return nil
}

// getField uses reflection-free field access via JSON tags.
func (cm *ConfigManager) getField(parts []string, obj any) any {
	if len(parts) == 0 {
		return nil
	}

	// Marshal to map for dot-notation access
	data, err := json.Marshal(obj)
	if err != nil {
		return nil
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}

	val := m[parts[0]]
	for _, key := range parts[1:] {
		switch v := val.(type) {
		case map[string]any:
			val = v[key]
		default:
			return nil
		}
	}
	return val
}

// Set sets a config value by dot-notation path and notifies watchers.
func (cm *ConfigManager) Set(path string, value any) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return fmt.Errorf("invalid config path: %s", path)
	}

	// Load current config as map, set value, marshal back
	data, err := json.Marshal(cm.config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	// Navigate to parent and set
	current := m
	for i := 0; i < len(parts)-1; i++ {
		if next, ok := current[parts[i]].(map[string]any); ok {
			current = next
		} else {
			return fmt.Errorf("invalid config path: %s", path)
		}
	}
	current[parts[len(parts)-1]] = value

	// Marshal back
	newData, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal updated config: %w", err)
	}
	if err := json.Unmarshal(newData, cm.config); err != nil {
		return fmt.Errorf("unmarshal updated config: %w", err)
	}

	// Notify watchers
	cm.notifyWatchers(path, value)
	return nil
}

// Save writes the current config to the user config file.
func (cm *ConfigManager) Save() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	dir := filepath.Dir(cm.userPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(cm.userPath, data, 0o600) //nolint:gosec
}

// Reset restores a config section to defaults.
func (cm *ConfigManager) Reset(path string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return fmt.Errorf("empty config path")
	}

	defaultCfg := defaultHierarchicalConfig()
	switch parts[0] {
	case "model":
		cm.config.Model = defaultCfg.Model
	case "api":
		cm.config.API = defaultCfg.API
	case "auth":
		cm.config.Auth = defaultCfg.Auth
	case "session":
		cm.config.Session = defaultCfg.Session
	case "tools":
		cm.config.Tools = defaultCfg.Tools
	case "ui":
		cm.config.UI = defaultCfg.UI
	case "advanced":
		cm.config.Advanced = defaultCfg.Advanced
	default:
		return fmt.Errorf("unknown config section: %s", parts[0])
	}

	cm.notifyWatchers(path, nil)
	return nil
}

// Merge deep-merges another config on top of the current one.
func (cm *ConfigManager) Merge(other *HierarchicalConfig) error {
	if other == nil {
		return nil
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.merge(other)
	return nil
}

// Validate runs all validators against the current config.
func (cm *ConfigManager) Validate() []ConfigError {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return ValidateConfig(cm.config)
}

// Watch registers a callback for changes to a config path.
func (cm *ConfigManager) Watch(path string, callback func(any)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.watchers[path] = append(cm.watchers[path], func(_ string, v any) { callback(v) })
}

// notifyWatchers fires all registered watchers for a path.
func (cm *ConfigManager) notifyWatchers(path string, value any) {
	for pattern, cbs := range cm.watchers {
		if strings.HasPrefix(path, pattern) || pattern == "*" {
			for _, cb := range cbs {
				cb(path, value)
			}
		}
	}
}

// Config returns the current hierarchical config (read-only snapshot).
func (cm *ConfigManager) Config() *HierarchicalConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	cp := *cm.config
	return &cp
}

// LoadFromViper loads config from an existing Viper instance.
func (cm *ConfigManager) LoadFromViper(v *viper.Viper) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if v == nil {
		return fmt.Errorf("nil viper instance")
	}

	if v.IsSet("model.provider") {
		cm.config.Model.Provider = v.GetString("model.provider")
	}
	if v.IsSet("model.model_name") {
		cm.config.Model.ModelName = v.GetString("model.model_name")
	}
	if v.IsSet("model.max_tokens") {
		cm.config.Model.MaxTokens = v.GetInt("model.max_tokens")
	}
	if v.IsSet("model.temperature") {
		cm.config.Model.Temperature = v.GetFloat64("model.temperature")
	}
	if v.IsSet("model.top_p") {
		cm.config.Model.TopP = v.GetFloat64("model.top_p")
	}
	if v.IsSet("model.system_prompt") {
		cm.config.Model.SystemPrompt = v.GetString("model.system_prompt")
	}
	if v.IsSet("api.base_url") {
		cm.config.API.BaseURL = v.GetString("api.base_url")
	}
	if v.IsSet("api.api_key") {
		cm.config.API.APIKey = v.GetString("api.api_key")
	}
	if v.IsSet("api.timeout") {
		cm.config.API.Timeout = v.GetInt("api.timeout")
	}
	if v.IsSet("api.retries") {
		cm.config.API.Retries = v.GetInt("api.retries")
	}
	if v.IsSet("api.rate_limit") {
		cm.config.API.RateLimit = v.GetInt("api.rate_limit")
	}
	if v.IsSet("auth.provider") {
		cm.config.Auth.Provider = v.GetString("auth.provider")
	}
	if v.IsSet("auth.client_id") {
		cm.config.Auth.ClientID = v.GetString("auth.client_id")
	}
	if v.IsSet("auth.scopes") {
		cm.config.Auth.Scopes = v.GetStringSlice("auth.scopes")
	}
	if v.IsSet("auth.token_path") {
		cm.config.Auth.TokenPath = v.GetString("auth.token_path")
	}
	if v.IsSet("session.max_history") {
		cm.config.Session.MaxHistory = v.GetInt("session.max_history")
	}
	if v.IsSet("session.archive_after") {
		cm.config.Session.ArchiveAfter = v.GetInt("session.archive_after")
	}
	if v.IsSet("tools.timeout") {
		cm.config.Tools.Timeout = v.GetInt("tools.timeout")
	}
	if v.IsSet("tools.max_concurrent") {
		cm.config.Tools.MaxConcurrent = v.GetInt("tools.max_concurrent")
	}
	if v.IsSet("ui.theme") {
		cm.config.UI.Theme = v.GetString("ui.theme")
	}
	if v.IsSet("ui.font_size") {
		cm.config.UI.FontSize = v.GetInt("ui.font_size")
	}
	if v.IsSet("advanced.log_level") {
		cm.config.Advanced.LogLevel = v.GetString("advanced.log_level")
	}

	return nil
}

// ---- Legacy Config Types ----

// ProjectLevelConfig holds project-level configuration (legacy, kept for compatibility).
type ProjectLevelConfig struct {
	Project   ProjectConfig    `yaml:"project"`
	Providers []ProviderConfig `yaml:"providers"`
	Sandbox   *SandboxConfig   `yaml:"sandbox,omitempty"`
	Git       *GitConfig       `yaml:"git,omitempty"`
	TUI       *TUIOpts         `yaml:"tui,omitempty"`
	WebUI     *WebUIConfig     `yaml:"web_ui,omitempty"`
	RAG       *RAGConfig       `yaml:"rag,omitempty"`
	Autonomy  *AutonomyConfig  `yaml:"autonomy,omitempty"`
	Vault     *VaultConfig     `yaml:"vault,omitempty"`
	Cache     *CacheConfig     `yaml:"cache,omitempty"`
}

// Config is an alias for ProjectLevelConfig (backward compatibility).
type Config = ProjectLevelConfig

type ProjectConfig struct {
	Name            string `yaml:"name"`
	Root            string `yaml:"root"`
	DefaultProvider string `yaml:"default_provider"`
}

type ProviderConfig struct {
	Name      string `yaml:"name"`
	Model     string `yaml:"model"`
	APIKeyEnv string `yaml:"api_key_env"`
	BaseURL   string `yaml:"base_url,omitempty"`
}

type SandboxConfig struct {
	DefaultImage  string `yaml:"default_image"`
	MemoryLimit   string `yaml:"memory_limit"`
	CPULimit      string `yaml:"cpu_limit"`
	TimeoutSec    int    `yaml:"timeout_sec"`
	MaxContainers int    `yaml:"max_containers"`
}

type GitConfig struct {
	AutoCommit bool `yaml:"auto_commit"`
	AutoPush   bool `yaml:"auto_push"`
	RequirePR  bool `yaml:"require_pr"`
}

type TUIOpts struct {
	Enabled       bool `yaml:"enabled"`
	ShowFilenames bool `yaml:"show_filenames"`
}

type WebUIConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Port       int    `yaml:"port"`
	Host       string `yaml:"host"`
	Theme      string `yaml:"theme"`
	LogLevel   string `yaml:"log_level"`
	AutoUpdate bool   `yaml:"auto_update"`
}

type RAGConfig struct {
	Enabled        bool   `yaml:"enabled"`
	EmbeddingModel string `yaml:"embedding_model"`
	ChunkSize      int    `yaml:"chunk_size"`
	ChunkOverlap   int    `yaml:"chunk_overlap"`
	MaxResults     int    `yaml:"max_results"`
}

type AutonomyConfig struct {
	Enabled     bool `yaml:"enabled"`
	IntervalSec int  `yaml:"interval_sec"`

	SelfUpdate bool `yaml:"self_update"`
	SelfVerify bool `yaml:"self_verify"`
	SelfLearn  bool `yaml:"self_learn"`
	AutoFix    bool `yaml:"auto_fix"`
	Evolution  bool `yaml:"evolution"`

	LearnDir       string `yaml:"learn_dir"`
	MemoriesFile   string `yaml:"memories_file"`
	MaxMemoryItems int    `yaml:"max_memory_items"`

	IQMetricsFile string `yaml:"iq_metrics_file"`
	IQReportEvery int    `yaml:"iq_report_every"`

	Capabilities []string `yaml:"capabilities"`
	AskBefore    []string `yaml:"ask_before"`
}

type VaultConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Path         string `yaml:"path"`
	MasterKeyEnv string `yaml:"master_key_env"`
}

type CacheConfig struct {
	Enabled           bool    `yaml:"enabled"`
	MaxSize           int     `yaml:"max_size"`
	TTLSeconds        int     `yaml:"ttl_seconds"`
	SemanticEnabled   bool    `yaml:"semantic_enabled"`
	SemanticThreshold float64 `yaml:"semantic_threshold"`
}

func Default() *Config {
	return &Config{
		Project: ProjectConfig{
			Name:            "my-project",
			Root:            ".",
			DefaultProvider: "claude",
		},
		Providers: []ProviderConfig{
			{Name: "claude", Model: "claude-sonnet-4-20250514", APIKeyEnv: "ANTHROPIC_API_KEY"}, //nolint:gosec
			{Name: "openai", Model: "gpt-4o", APIKeyEnv: "OPENAI_API_KEY"},                      //nolint:gosec
			{Name: "gemini", Model: "gemini-2.0-flash", APIKeyEnv: "GEMINI_API_KEY"},            //nolint:gosec
			{Name: "ollama", Model: "llama3.1:8b", BaseURL: "http://localhost:11434"},
		},
		Sandbox: &SandboxConfig{
			DefaultImage:  "ubuntu:22.04",
			MemoryLimit:   "4g",
			CPULimit:      "2",
			TimeoutSec:    120,
			MaxContainers: 5,
		},
		Git: &GitConfig{
			AutoCommit: true,
			AutoPush:   false,
			RequirePR:  true,
		},
		Autonomy: &AutonomyConfig{
			Enabled:        true,
			IntervalSec:    300,
			SelfUpdate:     true,
			SelfVerify:     true,
			SelfLearn:      true,
			AutoFix:        true,
			Evolution:      false,
			LearnDir:       ".dxrk/learn",
			MemoriesFile:   ".dxrk/memories.json",
			MaxMemoryItems: 1000,
			IQMetricsFile:  ".dxrk/iq.json",
			IQReportEvery:  10,
			Capabilities:   []string{"fs.read", "fs.write", "git", "net.http"},
			AskBefore:      []string{"fs.write", "sudo", "pkg.install", "docker"},
		},
		WebUI: &WebUIConfig{
			Enabled: false,
			Port:    8080,
			Host:    "127.0.0.1",
		},
		RAG: &RAGConfig{
			Enabled:        false,
			EmbeddingModel: "text-embedding-3-small",
			ChunkSize:      512,
			ChunkOverlap:   64,
			MaxResults:     5,
		},
		Vault: &VaultConfig{
			Enabled:      false,
			Path:         ".dxrk/vault.enc",
			MasterKeyEnv: "DXRK_VAULT_KEY",
		},
		Cache: &CacheConfig{
			Enabled:           false,
			MaxSize:           1000,
			TTLSeconds:        300,
			SemanticEnabled:   false,
			SemanticThreshold: 0.95,
		},
	}
}
