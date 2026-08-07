// SPDX-License-Identifier: MIT
package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// PollConfig controls the polling behavior.
type PollConfig struct {
	Interval           time.Duration
	ReclaimOlderThanMs int64
	MaxRetries         int
	BackoffMultiplier  float64
}

// DefaultPollConfig returns sensible polling defaults.
func DefaultPollConfig() PollConfig {
	return PollConfig{
		Interval:           5 * time.Second,
		ReclaimOlderThanMs: 60_000,
		MaxRetries:         5,
		BackoffMultiplier:  2.0,
	}
}

// Poller manages the work polling loop.
type Poller struct {
	mu         sync.RWMutex
	config     PollConfig
	bridge     *Bridge
	apiClient  APIClient
	abort      chan struct{}
	lastPoll   time.Time
	retryCount int
	onWork     func(WorkResponse)
	onError    func(error)
}

// APIClient defines the interface for bridge API communication.
type APIClient interface {
	RegisterBridgeEnvironment(config BridgeConfig) (environmentID, environmentSecret string, err error)
	PollForWork(environmentID, environmentSecret string, reclaimOlderThanMs int64) (*WorkResponse, error)
	AcknowledgeWork(environmentID, workID, sessionToken string) error
	StopWork(environmentID, workID string, force bool) error
	DeregisterEnvironment(environmentID string) error
	SendPermissionResponseEvent(sessionID string, event PermissionResponseEvent, sessionToken string) error
	ArchiveSession(sessionID string) error
	ReconnectSession(environmentID, sessionID string) error
	HeartbeatWork(environmentID, workID, sessionToken string) (leaseExtended bool, state string, err error)
}

// PermissionResponseEvent is a control_response event sent back to a session.
type PermissionResponseEvent struct {
	Type     string                 `json:"type"`
	Response PermissionResponseData `json:"response"`
}

type PermissionResponseData struct {
	Subtype   string                 `json:"subtype"`
	RequestID string                 `json:"request_id"`
	Response  map[string]interface{} `json:"response"`
}

// NewPoller creates a new poller.
func NewPoller(bridge *Bridge, client APIClient, config PollConfig) *Poller {
	return &Poller{
		config:    config,
		bridge:    bridge,
		apiClient: client,
		abort:     make(chan struct{}),
	}
}

// SetOnWork sets the callback for incoming work.
func (p *Poller) SetOnWork(fn func(WorkResponse)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onWork = fn
}

// SetOnError sets the callback for polling errors.
func (p *Poller) SetOnError(fn func(error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onError = fn
}

// Start begins the polling loop.
func (p *Poller) Start() {
	go p.loop()
}

// Stop stops the polling loop.
func (p *Poller) Stop() {
	close(p.abort)
}

func (p *Poller) loop() {
	interval := p.config.Interval
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-p.abort:
			return
		case <-timer.C:
			work, err := p.poll()
			if err != nil {
				p.mu.Lock()
				if p.onError != nil {
					p.onError(err)
				}
				p.retryCount++
				if p.retryCount >= p.config.MaxRetries {
					p.mu.Unlock()
					return
				}
				p.mu.Unlock()
				interval = time.Duration(float64(interval) * p.config.BackoffMultiplier)
			} else {
				p.mu.Lock()
				p.retryCount = 0
				p.lastPoll = time.Now()
				p.mu.Unlock()
				interval = p.config.Interval
			}

			if work != nil {
				p.mu.RLock()
				handler := p.onWork
				p.mu.RUnlock()
				if handler != nil {
					handler(*work)
				}
			}

			timer.Reset(interval)
		}
	}
}

func (p *Poller) poll() (*WorkResponse, error) {
	config := p.bridge.Config()
	return p.apiClient.PollForWork(
		config.EnvironmentID,
		"",
		p.config.ReclaimOlderThanMs,
	)
}

// HTTPClient implements APIClient using HTTP.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPClient creates a new HTTP API client.
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *HTTPClient) RegisterBridgeEnvironment(config BridgeConfig) (string, string, error) {
	body, _ := json.Marshal(config)
	resp, err := c.httpClient.Post(c.baseURL+"/worker/register", "application/json",
		strings.NewReader(string(body)))
	if err != nil {
		return "", "", fmt.Errorf("register environment: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("register returned %d", resp.StatusCode)
	}

	var result struct {
		EnvironmentID     string `json:"environment_id"`
		EnvironmentSecret string `json:"environment_secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("decode register response: %w", err)
	}
	return result.EnvironmentID, result.EnvironmentSecret, nil
}

func (c *HTTPClient) PollForWork(environmentID, environmentSecret string, reclaimOlderThanMs int64) (*WorkResponse, error) {
	url := fmt.Sprintf("%s/worker/poll?environment_id=%s", c.baseURL, environmentID)
	if reclaimOlderThanMs > 0 {
		url += fmt.Sprintf("&reclaim_older_than_ms=%d", reclaimOlderThanMs)
	}

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("poll: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("poll returned %d", resp.StatusCode)
	}

	var work WorkResponse
	if err := json.NewDecoder(resp.Body).Decode(&work); err != nil {
		return nil, fmt.Errorf("decode poll response: %w", err)
	}
	return &work, nil
}

func (c *HTTPClient) AcknowledgeWork(environmentID, workID, sessionToken string) error {
	url := fmt.Sprintf("%s/worker/acknowledge", c.baseURL)
	body, _ := json.Marshal(map[string]string{
		strconst.StrEnvironmentId: environmentID,
		strconst.StrWorkId:        workID,
	})
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("acknowledge: %w", err)
	}
	_ = resp.Body.Close()
	return nil
}

func (c *HTTPClient) StopWork(environmentID, workID string, force bool) error {
	url := fmt.Sprintf("%s/worker/stop", c.baseURL)
	body, _ := json.Marshal(map[string]interface{}{
		strconst.StrEnvironmentId: environmentID,
		strconst.StrWorkId:        workID,
		"force":                   force,
	})
	resp, err := c.httpClient.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	_ = resp.Body.Close()
	return nil
}

func (c *HTTPClient) DeregisterEnvironment(environmentID string) error {
	url := fmt.Sprintf("%s/worker/deregister/%s", c.baseURL, environmentID)
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("deregister: %w", err)
	}
	_ = resp.Body.Close()
	return nil
}

func (c *HTTPClient) SendPermissionResponseEvent(sessionID string, event PermissionResponseEvent, sessionToken string) error {
	url := fmt.Sprintf("%s/sessions/%s/events", c.baseURL, sessionID)
	body, _ := json.Marshal(event)
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send permission response: %w", err)
	}
	_ = resp.Body.Close()
	return nil
}

func (c *HTTPClient) ArchiveSession(sessionID string) error {
	url := fmt.Sprintf("%s/sessions/%s/archive", c.baseURL, sessionID)
	req, _ := http.NewRequest(http.MethodPost, url, nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("archive: %w", err)
	}
	_ = resp.Body.Close()
	return nil
}

func (c *HTTPClient) ReconnectSession(environmentID, sessionID string) error {
	url := fmt.Sprintf("%s/worker/reconnect", c.baseURL)
	body, _ := json.Marshal(map[string]string{
		strconst.StrEnvironmentId: environmentID,
		"session_id":              sessionID,
	})
	resp, err := c.httpClient.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("reconnect: %w", err)
	}
	_ = resp.Body.Close()
	return nil
}

func (c *HTTPClient) HeartbeatWork(environmentID, workID, sessionToken string) (bool, string, error) {
	url := fmt.Sprintf("%s/worker/heartbeat", c.baseURL)
	body, _ := json.Marshal(map[string]string{
		strconst.StrEnvironmentId: environmentID,
		strconst.StrWorkId:        workID,
	})
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("heartbeat: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		LeaseExtended bool   `json:"lease_extended"`
		State         string `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, "", fmt.Errorf("decode heartbeat: %w", err)
	}
	return result.LeaseExtended, result.State, nil
}

// Ensure io.Reader is used.
var _ io.Reader = (*strings.Reader)(nil)
