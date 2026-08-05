package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPClient_Basic(t *testing.T) {
	client, err := NewHTTPClient(&ClientOptions{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewHTTPClient failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestHTTPClient_Post(t *testing.T) {
	client, err := NewHTTPClient(&ClientOptions{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewHTTPClient failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}
}

func TestHTTPClient_Timeout(t *testing.T) {
	client, err := NewHTTPClient(&ClientOptions{
		Timeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewHTTPClient failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestHTTPClient_Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewHTTPClient(&ClientOptions{
		Timeout: 5 * time.Second,
		RetryPolicy: &RetryPolicy{
			MaxRetries:           3,
			RetryBackoff:         10 * time.Millisecond,
			RetryableStatusCodes: []int{http.StatusInternalServerError},
		},
	})
	if err != nil {
		t.Fatalf("NewHTTPClient failed: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 after retries, got %d", resp.StatusCode)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestHTTPClient_Proxy(t *testing.T) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("proxied"))
	}))
	defer proxyServer.Close()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("direct"))
	}))
	defer targetServer.Close()

	proxyURL := proxyServer.URL
	proxyConfig, err := NewProxyConfig(proxyURL)
	if err != nil {
		t.Fatalf("NewProxyConfig failed: %v", err)
	}

	client, err := NewHTTPClient(&ClientOptions{
		Timeout:     5 * time.Second,
		ProxyConfig: proxyConfig,
	})
	if err != nil {
		t.Fatalf("NewHTTPClient failed: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, targetServer.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do with proxy failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestTLSConfig(t *testing.T) {
	tlsConfig := NewTLSConfig()
	tlsConfig.InsecureSkipVerify = true

	cfg, err := tlsConfig.BuildTLSConfig()
	if err != nil {
		t.Fatalf("BuildTLSConfig() failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("BuildTLSConfig() returned nil")
	}
	if !cfg.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify=true")
	}
}

func TestProxyConfig_Parse(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
		wantHost string
		wantPort int
	}{
		{"http://proxy:8080", "http", "proxy", 8080},
		{"https://proxy:8443", "https", "proxy", 8443},
		{"socks5://proxy:1080", "socks5", "proxy", 1080},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			cfg, err := NewProxyConfig(tc.input)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			if string(cfg.Type) != tc.wantType {
				t.Errorf("Type = %s, want %s", cfg.Type, tc.wantType)
			}
			if cfg.Host != tc.wantHost {
				t.Errorf("Host = %s, want %s", cfg.Host, tc.wantHost)
			}
			if cfg.Port != tc.wantPort {
				t.Errorf("Port = %d, want %d", cfg.Port, tc.wantPort)
			}
		})
	}
}

func TestConnectionPool(t *testing.T) {
	opts := &ClientOptions{
		Timeout:             5 * time.Second,
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
	}

	client, err := NewHTTPClient(opts)
	if err != nil {
		t.Fatalf("NewHTTPClient failed: %v", err)
	}

	client.GetTransport().CloseIdleConnections()
}

func TestClientOptions_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  ClientOptions
		wantErr bool
	}{
		{"valid", ClientOptions{Timeout: time.Second}, false},
		{"zero timeout", ClientOptions{}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewHTTPClient(&tc.config)
			if (err != nil) != tc.wantErr {
				t.Errorf("NewHTTPClient() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestHTTPClient_ContextCancellation(t *testing.T) {
	client, err := NewHTTPClient(&ClientOptions{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewHTTPClient failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Error("Expected context cancellation error")
	}
}

func TestHTTPClient_Clone(t *testing.T) {
	client, err := NewHTTPClient(&ClientOptions{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewHTTPClient failed: %v", err)
	}

	cloned := client.Clone()
	if cloned == nil {
		t.Error("Clone() returned nil")
	}
	if cloned.GetClient() == client.GetClient() {
		t.Error("Cloned client should be different instance")
	}
}

func TestRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()
	if policy.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries=3, got %d", policy.MaxRetries)
	}

	if !policy.IsRetryable(http.StatusInternalServerError) {
		t.Error("500 should be retryable")
	}
	if !policy.IsRetryable(http.StatusServiceUnavailable) {
		t.Error("503 should be retryable")
	}
	if policy.IsRetryable(http.StatusOK) {
		t.Error("200 should not be retryable")
	}
}

func TestProxyConfig_String(t *testing.T) {
	cfg, err := NewProxyConfig("http://proxy:8080")
	if err != nil {
		t.Fatalf("NewProxyConfig failed: %v", err)
	}

	str := cfg.String()
	if str == "" {
		t.Error("String() should not be empty")
	}
}

func TestProxyConfig_ShouldBypass(t *testing.T) {
	cfg := &ProxyConfig{
		Type:    ProxyTypeHTTP,
		Host:    "proxy",
		Port:    8080,
		Bypass:  []string{"localhost", "*.internal"},
		NoProxy: "127.0.0.1",
	}

	if !cfg.ShouldBypass("localhost") {
		t.Error("Should bypass localhost")
	}
	if !cfg.ShouldBypass("service.internal") {
		t.Error("Should bypass *.internal")
	}
	if cfg.ShouldBypass("external.com") {
		t.Error("Should not bypass external.com")
	}
}

func TestProxyConfig_Clone(t *testing.T) {
	cfg := &ProxyConfig{
		Type: ProxyTypeHTTP,
		Host: "proxy",
		Port: 8080,
		Auth: &ProxyAuth{
			Username: "user",
			Password: "pass",
		},
		Bypass: []string{"localhost"},
	}

	cloned := cfg.Clone()
	if cloned == nil {
		t.Error("Clone() returned nil")
	}
	if cloned.Host != cfg.Host {
		t.Error("Host not cloned")
	}
	if cloned.Auth == nil || cloned.Auth.Username != cfg.Auth.Username {
		t.Error("Auth not cloned")
	}
}
