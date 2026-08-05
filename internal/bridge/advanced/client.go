package advanced

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type ClientOption func(*Client)

type Client struct {
	mu         sync.RWMutex
	config     *Config
	httpClient *http.Client
	baseURL    string
	token      string
	connected  bool
	lastPing   time.Time
}

func NewClient(config *Config, opts ...ClientOption) *Client {
	c := &Client{
		config:  config,
		baseURL: fmt.Sprintf("http://%s", config.Address()),
	}
	if config.TLSEnabled {
		c.baseURL = fmt.Sprintf("https://%s", config.Address())
	}
	c.httpClient = &http.Client{
		Timeout: config.ConnectTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        config.MaxIdleConns,
			MaxIdleConnsPerHost: config.MaxIdleConns,
			IdleConnTimeout:     config.IdleTimeout,
			TLSClientConfig:     config.buildTLSConfig(),
			DisableKeepAlives:   false,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if config.DisableRedirect {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func WithClientToken(token string) ClientOption {
	return func(c *Client) { c.token = token }
}

func WithClientBaseURL(baseURL string) ClientOption {
	return func(c *Client) { c.baseURL = baseURL }
}

func (c *Client) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

func (c *Client) Token() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *Client) LastPing() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastPing
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bridge/connect", nil)
	if err != nil {
		return fmt.Errorf("create connect request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("connect: status %d", resp.StatusCode)
	}

	c.connected = true
	c.lastPing = time.Now()
	return nil
}

func (c *Client) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bridge/disconnect", nil)
	if err != nil {
		return fmt.Errorf("create disconnect request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("disconnect: %w", err)
	}
	_ = resp.Body.Close()

	c.connected = false
	return nil
}

func (c *Client) Ping(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/bridge/ping", nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.connected = false
		return err
	}
	_ = resp.Body.Close()

	c.lastPing = time.Now()
	c.connected = true
	return nil
}

type SendRequest struct {
	SessionID string
	Method    string
	Path      string
	Body      interface{}
	Headers   map[string]string
}

type SendResponse struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

func (c *Client) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	var bodyReader io.Reader
	if req.Body != nil {
		data, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	u := c.baseURL + req.Path
	if req.SessionID != "" {
		u = fmt.Sprintf("%s/sessions/%s%s", c.baseURL, url.PathEscape(req.SessionID), req.Path)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, u, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return &SendResponse{
		StatusCode: resp.StatusCode,
		Body:       body,
		Header:     resp.Header,
	}, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.httpClient.Do(req)
}
