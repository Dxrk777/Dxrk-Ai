package auth

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"
)

// KeyPrefix is the prefix applied to all generated API keys.
const KeyPrefix = "dxrk-sk-"

// APIKey represents a stored API key with metadata.
type APIKey struct {
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	Provider  string    `json:"provider"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	LastUsed  time.Time `json:"last_used,omitempty"`
	Scopes    []string  `json:"scopes,omitempty"`
}

// IsExpired reports whether the API key has expired.
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(k.ExpiresAt)
}

// Redacted returns a partially redacted key safe for logging.
func (k *APIKey) Redacted() string {
	if len(k.Key) < 16 {
		return "[REDACTED]"
	}
	return k.Key[:12] + "..." + k.Key[len(k.Key)-4:]
}

// APIKeyManager provides CRUD operations for API key management.
type APIKeyManager struct {
	storage SecureStorage
	mu      sync.RWMutex
	keys    map[string]*APIKey
}

// storageKeyAPIKeys is the key used to store the API key collection.
const storageKeyAPIKeys = "api_keys"

// NewAPIKeyManager creates a manager backed by the given secure storage.
func NewAPIKeyManager(storage SecureStorage) (*APIKeyManager, error) {
	m := &APIKeyManager{
		storage: storage,
		keys:    make(map[string]*APIKey),
	}
	if err := m.loadKeys(); err != nil {
		// Ignore not-found errors — start with empty key set
		if !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("load keys: %w", err)
		}
	}
	return m, nil
}

// Create generates a new API key with the given name, provider, and optional expiry.
func (m *APIKeyManager) Create(name, provider string, scopes []string, ttl time.Duration) (*APIKey, error) {
	if name == "" {
		return nil, fmt.Errorf("key name is required")
	}
	if provider == "" {
		return nil, fmt.Errorf("provider is required")
	}

	m.mu.RLock()
	for _, existing := range m.keys {
		if existing.Name == name && existing.Provider == provider {
			m.mu.RUnlock()
			return nil, fmt.Errorf("key %q already exists for provider %q", name, provider)
		}
	}
	m.mu.RUnlock()

	key, err := GenerateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	apiKey := &APIKey{
		Name:      name,
		Key:       key,
		Provider:  provider,
		CreatedAt: time.Now(),
		Scopes:    scopes,
	}

	if ttl > 0 {
		apiKey.ExpiresAt = time.Now().Add(ttl)
	}

	m.mu.Lock()
	m.keys[name+"@"+provider] = apiKey
	if err := m.saveKeys(); err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("save keys: %w", err)
	}
	m.mu.Unlock()

	return apiKey, nil
}

// Get retrieves an API key by name and provider.
func (m *APIKeyManager) Get(name, provider string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.keys[name+"@"+provider]
	if !ok {
		return nil, fmt.Errorf("key %q not found for provider %q", name, provider)
	}
	return key, nil
}

// Validate checks whether an API key string is valid and not expired.
func (m *APIKeyManager) Validate(keyString string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, key := range m.keys {
		if key.Key == keyString {
			if key.IsExpired() {
				return nil, fmt.Errorf("key %q has expired", key.Name)
			}
			return key, nil
		}
	}
	return nil, fmt.Errorf("invalid API key")
}

// List returns all stored API keys sorted by name.
func (m *APIKeyManager) List() []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]*APIKey, 0, len(m.keys))
	for _, k := range m.keys {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Name < keys[j].Name
	})
	return keys
}

// Delete removes an API key by name and provider.
func (m *APIKeyManager) Delete(name, provider string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.keys[name+"@"+provider]; !ok {
		return fmt.Errorf("key %q not found for provider %q", name, provider)
	}

	delete(m.keys, name+"@"+provider)
	return m.saveKeys()
}

// Rotate replaces an existing API key with a new one, preserving metadata.
func (m *APIKeyManager) Rotate(name, provider string) (*APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.keys[name+"@"+provider]
	if !ok {
		return nil, fmt.Errorf("key %q not found for provider %q", name, provider)
	}

	newKey, err := GenerateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	existing.Key = newKey
	existing.CreatedAt = time.Now()
	existing.LastUsed = time.Time{}

	if err := m.saveKeys(); err != nil {
		return nil, fmt.Errorf("save keys: %w", err)
	}

	return existing, nil
}

// RecordUsage updates the last used timestamp for a key.
func (m *APIKeyManager) RecordUsage(name, provider string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.keys[name+"@"+provider]
	if !ok {
		return fmt.Errorf("key %q not found for provider %q", name, provider)
	}

	key.LastUsed = time.Now()
	return m.saveKeys()
}

// CleanupExpired removes all expired keys from storage.
func (m *APIKeyManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for k, key := range m.keys {
		if key.IsExpired() {
			delete(m.keys, k)
			count++
		}
	}

	if count > 0 {
		_ = m.saveKeys()
	}
	return count
}

func (m *APIKeyManager) loadKeys() error {
	data, err := m.storage.Load(storageKeyAPIKeys)
	if err != nil {
		return err
	}
	var keys []*APIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("unmarshal keys: %w", err)
	}
	for _, k := range keys {
		m.keys[k.Name+"@"+k.Provider] = k
	}
	return nil
}

func (m *APIKeyManager) saveKeys() error {
	keys := make([]*APIKey, 0, len(m.keys))
	for _, k := range m.keys {
		keys = append(keys, k)
	}
	data, err := json.Marshal(keys)
	if err != nil {
		return fmt.Errorf("marshal keys: %w", err)
	}
	return m.storage.Store(storageKeyAPIKeys, data)
}

// GenerateAPIKey generates a cryptographically random key with the dxrk-sk- prefix.
func GenerateAPIKey() (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const keyLen = 48

	b := make([]byte, keyLen)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", fmt.Errorf("generate random char: %w", err)
		}
		b[i] = chars[n.Int64()]
	}

	return KeyPrefix + string(b), nil
}
