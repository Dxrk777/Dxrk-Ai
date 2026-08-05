package remotesettings

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type SettingValue struct {
	Value       interface{} `json:"value"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Editable    bool        `json:"editable"`
}

type RemoteConfig struct {
	Version      string                  `json:"version"`
	LastUpdated  time.Time               `json:"last_updated"`
	Settings     map[string]SettingValue `json:"settings"`
	FeatureFlags map[string]bool         `json:"feature_flags"`
	Overrides    map[string]SettingValue `json:"overrides"`
}

type pollState struct {
	interval time.Duration
	cancel   context.CancelFunc
}

type RemoteSettingsService struct {
	serverURL      string
	orgID          string
	config         map[string]interface{}
	remoteConfig   *RemoteConfig
	localOverrides map[string]SettingValue
	mu             sync.Mutex
	pollState      *pollState
	localPath      string
}

func NewRemoteSettingsService(serverURL, orgID string) *RemoteSettingsService {
	home, _ := os.UserHomeDir()
	return &RemoteSettingsService{
		serverURL:      serverURL,
		orgID:          orgID,
		config:         make(map[string]interface{}),
		remoteConfig:   &RemoteConfig{Settings: make(map[string]SettingValue), FeatureFlags: make(map[string]bool), Overrides: make(map[string]SettingValue)},
		localOverrides: make(map[string]SettingValue),
		localPath:      filepath.Join(home, ".dxrk", "remote-settings.json"),
	}
}

type fetchResponse struct {
	Version      string                     `json:"version"`
	LastUpdated  time.Time                  `json:"last_updated"`
	Settings     map[string]json.RawMessage `json:"settings"`
	FeatureFlags map[string]bool            `json:"feature_flags"`
	Overrides    map[string]json.RawMessage `json:"overrides"`
}

func (s *RemoteSettingsService) FetchSettings(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.serverURL+"/api/settings", nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Org-ID", s.orgID)
	req.Header.Set("User-Agent", "dxrk-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch settings: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var raw fetchResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	settings := make(map[string]SettingValue)
	for k, v := range raw.Settings {
		var sv SettingValue
		if err := json.Unmarshal(v, &sv); err != nil {
			continue
		}
		settings[k] = sv
	}

	overrides := make(map[string]SettingValue)
	for k, v := range raw.Overrides {
		var sv SettingValue
		if err := json.Unmarshal(v, &sv); err != nil {
			continue
		}
		overrides[k] = sv
	}

	featureFlags := raw.FeatureFlags
	if featureFlags == nil {
		featureFlags = make(map[string]bool)
	}

	s.mu.Lock()
	s.remoteConfig = &RemoteConfig{
		Version:      raw.Version,
		LastUpdated:  raw.LastUpdated,
		Settings:     settings,
		FeatureFlags: featureFlags,
		Overrides:    overrides,
	}
	s.mu.Unlock()

	return nil
}

func (s *RemoteSettingsService) GetSetting(key string) (SettingValue, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ov, ok := s.localOverrides[key]; ok {
		return ov, true
	}
	sv, ok := s.remoteConfig.Settings[key]
	return sv, ok
}

func (s *RemoteSettingsService) SetLocalOverride(key string, value SettingValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.localOverrides[key] = value
}

func (s *RemoteSettingsService) RemoveLocalOverride(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.localOverrides, key)
}

func (s *RemoteSettingsService) GetFeatureFlag(flag string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.remoteConfig.FeatureFlags[flag]
}

func (s *RemoteSettingsService) ApplySettings(settings map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range settings {
		s.config[k] = v
	}
}

type SettingChange struct {
	Key      string
	OldValue interface{}
	NewValue interface{}
}

func (s *RemoteSettingsService) WatchSettings(ctx context.Context, interval time.Duration) <-chan []SettingChange {
	ch := make(chan []SettingChange, 1)
	pCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	s.pollState = &pollState{interval: interval, cancel: cancel}
	s.mu.Unlock()

	go func() {
		defer close(ch)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-pCtx.Done():
				return
			case <-ticker.C:
				if err := s.FetchSettings(pCtx); err != nil {
					continue
				}
				changes := s.GetChangedSettings()
				if len(changes) > 0 {
					select {
					case ch <- changes:
					default:
					}
				}
			}
		}
	}()

	return ch
}

func (s *RemoteSettingsService) GetChangedSettings() []SettingChange {
	s.mu.Lock()
	defer s.mu.Unlock()

	var changes []SettingChange
	if s.remoteConfig == nil {
		return changes
	}

	for k, sv := range s.remoteConfig.Settings {
		if ov, ok := s.config[k]; ok {
			if fmt.Sprintf("%v", ov) != fmt.Sprintf("%v", sv.Value) {
				changes = append(changes, SettingChange{Key: k, OldValue: ov, NewValue: sv.Value})
			}
		} else {
			changes = append(changes, SettingChange{Key: k, OldValue: nil, NewValue: sv.Value})
		}
	}
	return changes
}

func (s *RemoteSettingsService) SaveToLocal() error {
	s.mu.Lock()
	data, err := json.MarshalIndent(s.remoteConfig, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	dir := filepath.Dir(s.localPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(s.localPath, data, 0o600)
}

func (s *RemoteSettingsService) LoadFromLocal() error {
	data, err := os.ReadFile(s.localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read file: %w", err)
	}

	var cfg RemoteConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	s.mu.Lock()
	s.remoteConfig = &cfg
	s.mu.Unlock()
	return nil
}

func MergeConfig(base, override map[string]SettingValue) map[string]SettingValue {
	merged := make(map[string]SettingValue, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}

func (s *RemoteSettingsService) StopWatching() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pollState != nil && s.pollState.cancel != nil {
		s.pollState.cancel()
		s.pollState = nil
	}
}
