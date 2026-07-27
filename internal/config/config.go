// SPDX-License-Identifier: MIT
package config

type Config struct {
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
