// SPDX-License-Identifier: MIT
package vault

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
	"path/filepath"
	"sync"
	"time"
)

type SecretEntry struct {
	Value    string    `json:"value"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
	Rotation string    `json:"rotation,omitempty"` // never, daily, weekly, monthly
	EnvVar   string    `json:"env_var,omitempty"`
}

type Vault struct {
	mu        sync.RWMutex
	path      string
	masterKey []byte
	secrets   map[string]SecretEntry
}

func New(path string, masterKeyEnv string) (*Vault, error) {
	var key []byte
	if masterKeyEnv != "" {
		ek := os.Getenv(masterKeyEnv)
		if ek != "" {
			h := sha256.Sum256([]byte(ek))
			key = h[:]
		}
	}
	if key == nil {
		k := make([]byte, 32)
		if _, err := rand.Read(k); err != nil {
			return nil, fmt.Errorf("generate master key: %w", err)
		}
		key = k
	}

	v := &Vault{
		path:      path,
		masterKey: key,
		secrets:   make(map[string]SecretEntry),
	}

	if path != "" {
		if err := v.load(); err != nil {
			return nil, fmt.Errorf("load vault: %w", err)
		}
	}

	return v, nil
}

func (v *Vault) Get(name string) (string, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	entry, ok := v.secrets[name]
	if !ok {
		return "", false
	}
	return entry.Value, true
}

func (v *Vault) Set(name, value string, opts ...SecretOption) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	entry := SecretEntry{
		Value:   value,
		Created: time.Now(),
		Updated: time.Now(),
	}

	for _, opt := range opts {
		opt(&entry)
	}

	v.secrets[name] = entry
	return v.save()
}

func (v *Vault) Delete(name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	delete(v.secrets, name)
	return v.save()
}

func (v *Vault) Rotate(name, newValue string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	entry, ok := v.secrets[name]
	if !ok {
		return fmt.Errorf("secret %q not found", name)
	}
	entry.Value = newValue
	entry.Updated = time.Now()
	v.secrets[name] = entry
	return v.save()
}

func (v *Vault) List() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	names := make([]string, 0, len(v.secrets))
	for n := range v.secrets {
		names = append(names, n)
	}
	return names
}

func (v *Vault) Resolve(name string) string {
	if val, ok := v.Get(name); ok {
		return val
	}
	return os.Getenv(name)
}

func (v *Vault) BindToEnv(name, envVar string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	entry, ok := v.secrets[name]
	if ok {
		entry.EnvVar = envVar
		v.secrets[name] = entry
	} else {
		val := os.Getenv(envVar)
		if val == "" {
			return fmt.Errorf("env var %s not set and no vault entry for %s", envVar, name)
		}
		v.secrets[name] = SecretEntry{
			Value:   val,
			Created: time.Now(),
			Updated: time.Now(),
			EnvVar:  envVar,
		}
	}
	return v.save()
}

func (v *Vault) load() error {
	data, err := os.ReadFile(v.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
	n, err := base64.StdEncoding.Decode(decoded, data)
	if err != nil {
		return fmt.Errorf("decode vault: %w", err)
	}
	decoded = decoded[:n]

	plaintext, err := v.decrypt(decoded)
	if err != nil {
		return fmt.Errorf("decrypt vault: %w", err)
	}

	return json.Unmarshal(plaintext, &v.secrets)
}

func (v *Vault) save() error {
	if v.path == "" {
		return nil
	}

	plaintext, err := json.Marshal(v.secrets)
	if err != nil {
		return fmt.Errorf("marshal vault: %w", err)
	}

	ciphertext, err := v.encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypt vault: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	if err := os.MkdirAll(filepath.Dir(v.path), 0700); err != nil {
		return err
	}
	return os.WriteFile(v.path, []byte(encoded), 0600)
}

func (v *Vault) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return aesGCM.Seal(nonce, nonce, plaintext, nil), nil
}

func (v *Vault) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return aesGCM.Open(nil, nonce, ciphertext, nil)
}

type SecretOption func(*SecretEntry)

func WithRotation(rotation string) SecretOption {
	return func(e *SecretEntry) { e.Rotation = rotation }
}

func WithEnvVar(envVar string) SecretOption {
	return func(e *SecretEntry) { e.EnvVar = envVar }
}
