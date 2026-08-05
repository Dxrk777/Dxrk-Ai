package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"sync"
)

// SecureStorage defines the interface for encrypted credential persistence.
type SecureStorage interface {
	Store(key string, data []byte) error
	Load(key string) ([]byte, error)
	Delete(key string) error
	List() ([]string, error)
	Clear() error
}

// EncryptionHelper provides AES-256-GCM encryption and decryption.
type EncryptionHelper struct {
	gcm cipher.AEAD
}

// NewEncryptionHelper creates an EncryptionHelper from a raw 32-byte key.
func NewEncryptionHelper(key []byte) (*EncryptionHelper, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	return &EncryptionHelper{gcm: gcm}, nil
}

// DeriveMachineKey generates a 32-byte key from hostname + username + salt.
func DeriveMachineKey() ([]byte, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("get hostname: %w", err)
	}

	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}

	salt := []byte("dxrk-ai-auth-salt-v1")
	input := append([]byte(hostname+u.Username), salt...)
	hash := sha256.Sum256(input)
	return hash[:], nil
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext with nonce prefix.
func (e *EncryptionHelper) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := e.gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt base64-decodes ciphertext and decrypts it, stripping the nonce prefix.
func (e *EncryptionHelper) Decrypt(encoded string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// ---- FileStorage ----

// FileStorage stores encrypted credentials as individual files on disk.
type FileStorage struct {
	dir string
	enc *EncryptionHelper
	mu  sync.RWMutex
}

// NewFileStorage creates a FileStorage at the given directory path.
// The directory is created if it does not exist. An encryption key is
// derived from machine-specific identifiers.
func NewFileStorage(dir string) (*FileStorage, error) {
	expanded, err := expandPath(dir)
	if err != nil {
		return nil, fmt.Errorf("expand path: %w", err)
	}

	if err := os.MkdirAll(expanded, 0700); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}

	key, err := DeriveMachineKey()
	if err != nil {
		return nil, fmt.Errorf("derive machine key: %w", err)
	}

	enc, err := NewEncryptionHelper(key)
	if err != nil {
		return nil, fmt.Errorf("create encryption helper: %w", err)
	}

	return &FileStorage{dir: expanded, enc: enc}, nil
}

// NewFileStorageWithKey creates a FileStorage with an explicit 32-byte encryption key.
func NewFileStorageWithKey(dir string, key []byte) (*FileStorage, error) {
	expanded, err := expandPath(dir)
	if err != nil {
		return nil, fmt.Errorf("expand path: %w", err)
	}

	if err := os.MkdirAll(expanded, 0700); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}

	enc, err := NewEncryptionHelper(key)
	if err != nil {
		return nil, fmt.Errorf("create encryption helper: %w", err)
	}

	return &FileStorage{dir: expanded, enc: enc}, nil
}

func (f *FileStorage) filePath(key string) string {
	h := sha256.Sum256([]byte(key))
	return filepath.Join(f.dir, fmt.Sprintf("%x.enc", h[:8]))
}

// Store encrypts data and writes it to a file.
func (f *FileStorage) Store(key string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	encrypted, err := f.enc.Encrypt(data)
	if err != nil {
		return fmt.Errorf("encrypt data: %w", err)
	}

	payload, err := json.Marshal(map[string]string{
		"key":        key,
		"ciphertext": encrypted,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	path := f.filePath(key)
	if err := os.WriteFile(path, payload, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// Load reads and decrypts data stored under the given key.
func (f *FileStorage) Load(key string) ([]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	path := f.filePath(key)
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key %q not found", key)
		}
		return nil, fmt.Errorf("read file: %w", err)
	}

	var envelope struct {
		Key        string `json:"key"`
		Ciphertext string `json:"ciphertext"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	if envelope.Key != key {
		return nil, fmt.Errorf("key mismatch: expected %q, got %q", key, envelope.Key)
	}

	data, err := f.enc.Decrypt(envelope.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt data: %w", err)
	}

	return data, nil
}

// Delete removes the stored credential file.
func (f *FileStorage) Delete(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := f.filePath(key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

// List returns all stored keys by reading .enc files in the directory.
func (f *FileStorage) List() ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var keys []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".enc" {
			continue
		}

		path := filepath.Join(f.dir, entry.Name())
		payload, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var envelope struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			continue
		}
		if envelope.Key != "" {
			keys = append(keys, envelope.Key)
		}
	}

	sort.Strings(keys)
	return keys, nil
}

// Clear removes all encrypted files from the storage directory.
func (f *FileStorage) Clear() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".enc" {
			path := filepath.Join(f.dir, entry.Name())
			_ = os.Remove(path)
		}
	}
	return nil
}

// ---- MemoryStorage ----

// MemoryStorage is an in-memory SecureStorage for testing.
type MemoryStorage struct {
	mu    sync.RWMutex
	store map[string][]byte
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{store: make(map[string][]byte)}
}

// Store saves data in memory.
func (m *MemoryStorage) Store(key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := make([]byte, len(data))
	copy(cp, data)
	m.store[key] = cp
	return nil
}

// Load retrieves data from memory.
func (m *MemoryStorage) Load(key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, ok := m.store[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}

	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

// Delete removes a key from memory.
func (m *MemoryStorage) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.store, key)
	return nil
}

// List returns all stored keys.
func (m *MemoryStorage) List() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.store))
	for k := range m.store {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// Clear removes all entries from memory.
func (m *MemoryStorage) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.store = make(map[string][]byte)
	return nil
}

// ---- helpers ----

func expandPath(path string) (string, error) {
	if len(path) == 0 {
		return "", fmt.Errorf("empty path")
	}

	if path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	return filepath.Join(home, path[1:]), nil
}
