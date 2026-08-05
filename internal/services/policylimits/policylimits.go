package policylimits

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type PolicyConfig struct {
	Enabled             bool
	PolicyURL           string
	PollIntervalSeconds int
	FailOpen            bool
	FeatureFlags        map[string]bool
}

type PolicyRestrictions struct {
	mu                sync.Mutex
	MCPServersAllowed bool
	WebSearchAllowed  bool
	WebFetchAllowed   bool
	BashAllowed       bool
	FileWriteAllowed  bool
	CustomRules       []string
}

type policyResponse struct {
	Restrictions map[string]interface{} `json:"restrictions"`
}

type PolicyService struct {
	config        PolicyConfig
	restrictions  PolicyRestrictions
	etag          string
	lastPoll      time.Time
	polling       bool
	cancelPolling context.CancelFunc
	mu            sync.Mutex
}

func NewPolicyService(config PolicyConfig) *PolicyService {
	if config.PollIntervalSeconds <= 0 {
		config.PollIntervalSeconds = 300
	}
	if !config.FailOpen {
		config.FailOpen = true
	}
	if config.FeatureFlags == nil {
		config.FeatureFlags = make(map[string]bool)
	}

	return &PolicyService{
		config: config,
		restrictions: PolicyRestrictions{
			MCPServersAllowed: true,
			WebSearchAllowed:  true,
			WebFetchAllowed:   true,
			BashAllowed:       true,
			FileWriteAllowed:  true,
		},
	}
}

func (s *PolicyService) IsFeatureEnabled(feature string) bool {
	s.mu.Lock()
	flags := make(map[string]bool)
	for k, v := range s.config.FeatureFlags {
		flags[k] = v
	}
	s.mu.Unlock()

	if enabled, ok := flags[feature]; ok {
		return enabled
	}
	return true
}

func (s *PolicyService) CanUseTool(toolName string) bool {
	s.restrictions.mu.Lock()
	defer s.restrictions.mu.Unlock()

	switch toolName {
	case "mcp":
		return s.restrictions.MCPServersAllowed
	case "web_search":
		return s.restrictions.WebSearchAllowed
	case "web_fetch":
		return s.restrictions.WebFetchAllowed
	case "bash":
		return s.restrictions.BashAllowed
	case "file_write":
		return s.restrictions.FileWriteAllowed
	default:
		return true
	}
}

func (s *PolicyService) FetchPolicy(ctx context.Context) error {
	if !s.config.Enabled || s.config.PolicyURL == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.config.PolicyURL, nil)
	if err != nil {
		if s.config.FailOpen {
			return nil
		}
		return fmt.Errorf("failed to create request: %w", err)
	}

	s.mu.Lock()
	if s.etag != "" {
		req.Header.Set("If-None-Match", s.etag)
	}
	s.mu.Unlock()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if s.config.FailOpen {
			return nil
		}
		return fmt.Errorf("failed to fetch policy: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		s.lastPoll = time.Now()
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		if s.config.FailOpen {
			return nil
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if s.config.FailOpen {
			return nil
		}
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var policyResp policyResponse
	if err := json.Unmarshal(body, &policyResp); err != nil {
		if s.config.FailOpen {
			return nil
		}
		return fmt.Errorf("failed to parse policy response: %w", err)
	}

	s.restrictions.mu.Lock()
	defer s.restrictions.mu.Unlock()

	s.restrictions.MCPServersAllowed = true
	s.restrictions.WebSearchAllowed = true
	s.restrictions.WebFetchAllowed = true
	s.restrictions.BashAllowed = true
	s.restrictions.FileWriteAllowed = true
	s.restrictions.CustomRules = nil

	if allowed, ok := policyResp.Restrictions["mcp"].(bool); ok {
		s.restrictions.MCPServersAllowed = allowed
	}
	if allowed, ok := policyResp.Restrictions["web_search"].(bool); ok {
		s.restrictions.WebSearchAllowed = allowed
	}
	if allowed, ok := policyResp.Restrictions["web_fetch"].(bool); ok {
		s.restrictions.WebFetchAllowed = allowed
	}
	if allowed, ok := policyResp.Restrictions["bash"].(bool); ok {
		s.restrictions.BashAllowed = allowed
	}
	if allowed, ok := policyResp.Restrictions["file_write"].(bool); ok {
		s.restrictions.FileWriteAllowed = allowed
	}
	if rules, ok := policyResp.Restrictions["custom_rules"].([]interface{}); ok {
		for _, rule := range rules {
			if r, ok := rule.(string); ok {
				s.restrictions.CustomRules = append(s.restrictions.CustomRules, r)
			}
		}
	}

	if etag := resp.Header.Get("ETag"); etag != "" {
		s.etag = etag
	}
	s.lastPoll = time.Now()

	return nil
}

func (s *PolicyService) StartPolling(ctx context.Context) {
	s.mu.Lock()
	if s.polling {
		s.mu.Unlock()
		return
	}
	s.polling = true
	s.mu.Unlock()

	pollCtx, cancel := context.WithCancel(ctx)
	s.cancelPolling = cancel

	go func() {
		ticker := time.NewTicker(time.Duration(s.config.PollIntervalSeconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-pollCtx.Done():
				return
			case <-ticker.C:
				_ = s.FetchPolicy(pollCtx)
			}
		}
	}()
}

func (s *PolicyService) StopPolling() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancelPolling != nil {
		s.cancelPolling()
		s.cancelPolling = nil
	}
	s.polling = false
}

func (s *PolicyService) GetRestrictions() PolicyRestrictions {
	s.restrictions.mu.Lock()
	defer s.restrictions.mu.Unlock()

	return PolicyRestrictions{
		MCPServersAllowed: s.restrictions.MCPServersAllowed,
		WebSearchAllowed:  s.restrictions.WebSearchAllowed,
		WebFetchAllowed:   s.restrictions.WebFetchAllowed,
		BashAllowed:       s.restrictions.BashAllowed,
		FileWriteAllowed:  s.restrictions.FileWriteAllowed,
		CustomRules:       append([]string{}, s.restrictions.CustomRules...),
	}
}

func (s *PolicyService) ApplyFeatureFlags(flags map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config.FeatureFlags == nil {
		s.config.FeatureFlags = make(map[string]bool)
	}

	for k, v := range flags {
		s.config.FeatureFlags[k] = v
	}
}
