// SPDX-License-Identifier: MIT
package multitenant

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/config"
	"github.com/Dxrk777/Dxrk/internal/log"
	"github.com/Dxrk777/Dxrk/internal/memory"
	"github.com/Dxrk777/Dxrk/internal/rag"
	"github.com/Dxrk777/Dxrk/internal/vault"
)

type Project struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Config      *config.Config    `json:"config"`
	VaultPath   string            `json:"vault_path"`
	MemoryPath  string            `json:"memory_path"`
	Labels      map[string]string `json:"labels"`
	Owner       string            `json:"owner"`
	Members     []string          `json:"members"`
	Active      bool              `json:"active"`
}

type Manager struct {
	mu       sync.RWMutex
	projects map[string]*Project
	rootDir  string
	active   string
}

func NewManager(rootDir string) *Manager {
	m := &Manager{
		projects: make(map[string]*Project),
		rootDir:  rootDir,
	}
	if err := m.load(); err != nil {
		log.NewSlog(slog.Default()).Warn("multitenant state load failed, starting fresh", "err", err)
	}
	return m
}

func (m *Manager) CreateProject(ctx context.Context, name, owner string) (*Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("proj-%d", time.Now().UnixNano())
	now := time.Now()

	projDir := filepath.Join(m.rootDir, id)
	if err := os.MkdirAll(projDir, 0o750); err != nil {
		return nil, err
	}

	cfg := config.Default()
	cfg.Project = config.ProjectConfig{
		Name: name,
		Root: projDir,
	}
	cfg.Vault = &config.VaultConfig{
		Enabled:      true,
		Path:         filepath.Join(projDir, "vault.enc"),
		MasterKeyEnv: "DXRK_VAULT_KEY_" + id,
	}
	cfg.Cache = &config.CacheConfig{
		Enabled: true,
		MaxSize: 500,
	}

	vaultPath := filepath.Join(projDir, "vault.enc")
	memoryPath := filepath.Join(projDir, "memory.json")

	project := &Project{
		ID:         id,
		Name:       name,
		CreatedAt:  now,
		UpdatedAt:  now,
		Config:     cfg,
		VaultPath:  vaultPath,
		MemoryPath: memoryPath,
		Labels:     make(map[string]string),
		Owner:      owner,
		Members:    []string{owner},
		Active:     true,
	}

	m.projects[id] = project
	if m.active == "" {
		m.active = id
	}
	return project, m.save()
}

func (m *Manager) GetProject(id string) (*Project, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.projects[id]
	return p, ok
}

func (m *Manager) GetActiveProject() *Project {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.active == "" {
		return nil
	}
	return m.projects[m.active]
}

func (m *Manager) SetActiveProject(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.projects[id]; !ok {
		return fmt.Errorf("project %s not found", id)
	}
	m.active = id
	return nil
}

func (m *Manager) ListProjects() []*Project {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*Project, 0, len(m.projects))
	for _, p := range m.projects {
		list = append(list, p)
	}
	return list
}

func (m *Manager) DeleteProject(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.projects[id]; !ok {
		return fmt.Errorf("project %s not found", id)
	}
	delete(m.projects, id)
	if m.active == id {
		m.active = ""
		for k := range m.projects {
			m.active = k
			break
		}
	}
	return m.save()
}

func (m *Manager) UpdateProject(id string, fn func(*Project)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.projects[id]
	if !ok {
		return fmt.Errorf("project %s not found", id)
	}
	fn(p)
	p.UpdatedAt = time.Now()
	return m.save()
}

func (m *Manager) GetProjectConfig(id string) (*config.Config, error) {
	p, ok := m.GetProject(id)
	if !ok {
		return nil, fmt.Errorf("project %s not found", id)
	}
	return p.Config, nil
}

func (m *Manager) GetProjectVault(id string) (*vault.Vault, error) {
	p, ok := m.GetProject(id)
	if !ok {
		return nil, fmt.Errorf("project %s not found", id)
	}
	return vault.New(p.VaultPath, p.Config.Vault.MasterKeyEnv)
}

func (m *Manager) GetProjectMemory(id string, r *rag.RAG) (*memory.AgentMemory, error) {
	p, ok := m.GetProject(id)
	if !ok {
		return nil, fmt.Errorf("project %s not found", id)
	}
	return memory.NewAgentMemory(p.MemoryPath, 10000, r, nil), nil
}

func (m *Manager) save() error {
	data := make([]*Project, 0, len(m.projects))
	for _, p := range m.projects {
		data = append(data, p)
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.rootDir, "projects.json"), jsonData, 0600)
}

func (m *Manager) load() error {
	data, err := os.ReadFile(filepath.Join(m.rootDir, "projects.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var projects []*Project
	if err := json.Unmarshal(data, &projects); err != nil {
		return err
	}
	for _, p := range projects {
		m.projects[p.ID] = p
		if m.active == "" {
			m.active = p.ID
		}
	}
	return nil
}
