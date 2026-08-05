package lsptool

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// ── Language configuration ───────────────────────────────────────────────────

// LanguageConfig describes a language server to manage.
type LanguageConfig struct {
	Name         string   `json:"name"`
	ServerCmd    string   `json:"serverCmd"`
	Args         []string `json:"args,omitempty"`
	Extensions   []string `json:"extensions"`
	FilePatterns []string `json:"filePatterns,omitempty"`
}

// DefaultLanguageConfigs returns the built-in language server configurations.
func DefaultLanguageConfigs() []LanguageConfig {
	return []LanguageConfig{
		{
			Name:       "go",
			ServerCmd:  "gopls",
			Extensions: []string{".go"},
		},
		{
			Name:       "python",
			ServerCmd:  "pylsp",
			Extensions: []string{".py"},
		},
		{
			Name:       "typescript",
			ServerCmd:  "typescript-language-server",
			Args:       []string{"--stdio"},
			Extensions: []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"},
		},
		{
			Name:       "rust",
			ServerCmd:  "rust-analyzer",
			Extensions: []string{".rs"},
		},
		{
			Name:       "java",
			ServerCmd:  "jdtls",
			Extensions: []string{".java"},
		},
		{
			Name:       "c-cpp",
			ServerCmd:  "clangd",
			Extensions: []string{".c", ".h", ".cpp", ".cc", ".cxx", ".hpp"},
		},
	}
}

// ── LSPManager ───────────────────────────────────────────────────────────────

// LSPManager maintains one LSP client per language and auto-detects languages
// from file extensions.
type LSPManager struct {
	mu      sync.Mutex
	clients map[string]*LSPClient
	configs map[string]LanguageConfig
	extMap  map[string]string // extension → language name
	rootDir string
}

// NewLSPManager creates a manager pre-loaded with default language configs.
func NewLSPManager() *LSPManager {
	m := &LSPManager{
		clients: make(map[string]*LSPClient),
		configs: make(map[string]LanguageConfig),
		extMap:  make(map[string]string),
	}
	for _, cfg := range DefaultLanguageConfigs() {
		m.configs[cfg.Name] = cfg
		for _, ext := range cfg.Extensions {
			m.extMap[ext] = cfg.Name
		}
	}
	return m
}

// SetRootDir sets the workspace root for newly started servers.
func (m *LSPManager) SetRootDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rootDir = dir
}

// RegisterServer adds or replaces a language configuration.
func (m *LSPManager) RegisterServer(cfg LanguageConfig) error {
	if cfg.Name == "" || cfg.ServerCmd == "" {
		return fmt.Errorf("language name and server command are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[cfg.Name] = cfg
	for _, ext := range cfg.Extensions {
		m.extMap[ext] = cfg.Name
	}
	return nil
}

// GetClientForFile auto-detects the language for a file and returns (or starts)
// the corresponding LSP client.
func (m *LSPManager) GetClientForFile(filePath string) (*LSPClient, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	lang, ok := m.extMap[ext]
	if !ok {
		return nil, fmt.Errorf("no language server configured for extension %q", ext)
	}
	return m.GetClientForLanguage(lang)
}

// GetClientForLanguage returns the client for the named language, starting
// the server if necessary.
func (m *LSPManager) GetClientForLanguage(language string) (*LSPClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[language]; ok {
		return client, nil
	}

	cfg, ok := m.configs[language]
	if !ok {
		return nil, fmt.Errorf("no server configured for language %q", language)
	}

	client := NewLSPClient(cfg.ServerCmd, cfg.Args)
	root := m.rootDir
	if root == "" {
		root = "."
	}
	if err := client.Initialize(filePathToURI(root)); err != nil {
		return nil, fmt.Errorf("start %s server: %w", language, err)
	}
	m.clients[language] = client
	return client, nil
}

// SupportedLanguages returns the names of all registered languages.
func (m *LSPManager) SupportedLanguages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	langs := make([]string, 0, len(m.configs))
	for name := range m.configs {
		langs = append(langs, name)
	}
	return langs
}

// StopAll shuts down every running language server.
func (m *LSPManager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var firstErr error
	for lang, client := range m.clients {
		if err := client.Shutdown(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("stop %s: %w", lang, err)
		}
		delete(m.clients, lang)
	}
	return firstErr
}

// DetectLanguage returns the language name for a file path, or empty string.
func (m *LSPManager) DetectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.extMap[ext]
}
