package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	defaultMaxIdleConns          = 100
	defaultMaxIdleConnsPerHost   = 10
	defaultIdleConnTimeout       = 90 * time.Second
	defaultTLSHandshakeTimeout   = 10 * time.Second
	defaultExpectContinueTimeout = 1 * time.Second
	defaultMaxRetries            = 3
	defaultRetryBackoff          = 100 * time.Millisecond
)

var (
	ErrInvalidProxyURL    = errors.New("invalid proxy URL")
	ErrInvalidTLSConfig   = errors.New("invalid TLS configuration")
	ErrMaxRetriesExceeded = errors.New("maximum retries exceeded")
)

type RetryPolicy struct {
	MaxRetries           int
	RetryBackoff         time.Duration
	RetryableStatusCodes []int
}

func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:   defaultMaxRetries,
		RetryBackoff: defaultRetryBackoff,
		RetryableStatusCodes: []int{
			http.StatusRequestTimeout,
			http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
		},
	}
}

func (rp *RetryPolicy) IsRetryable(statusCode int) bool {
	for _, code := range rp.RetryableStatusCodes {
		if code == statusCode {
			return true
		}
	}
	return false
}

type HTTPClient struct {
	client      *http.Client
	transport   *http.Transport
	retryPolicy *RetryPolicy
	proxyConfig *ProxyConfig
	tlsConfig   *TLSConfig
	mu          sync.RWMutex
}

type ClientOptions struct {
	Timeout               time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	IdleConnTimeout       time.Duration
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
	DisableKeepAlives     bool
	DisableCompression    bool
	MaxConnsPerHost       int
	RetryPolicy           *RetryPolicy
	ProxyConfig           *ProxyConfig
	TLSConfig             *TLSConfig
}

func NewHTTPClient(opts *ClientOptions) (*HTTPClient, error) {
	if opts == nil {
		opts = &ClientOptions{}
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	maxIdleConns := opts.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = defaultMaxIdleConns
	}

	maxIdleConnsPerHost := opts.MaxIdleConnsPerHost
	if maxIdleConnsPerHost == 0 {
		maxIdleConnsPerHost = defaultMaxIdleConnsPerHost
	}

	idleConnTimeout := opts.IdleConnTimeout
	if idleConnTimeout == 0 {
		idleConnTimeout = defaultIdleConnTimeout
	}

	tlsHandshakeTimeout := opts.TLSHandshakeTimeout
	if tlsHandshakeTimeout == 0 {
		tlsHandshakeTimeout = defaultTLSHandshakeTimeout
	}

	expectContinueTimeout := opts.ExpectContinueTimeout
	if expectContinueTimeout == 0 {
		expectContinueTimeout = defaultExpectContinueTimeout
	}

	retryPolicy := opts.RetryPolicy
	if retryPolicy == nil {
		retryPolicy = DefaultRetryPolicy()
	}

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		MaxConnsPerHost:       opts.MaxConnsPerHost,
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
		DisableKeepAlives:     opts.DisableKeepAlives,
		DisableCompression:    opts.DisableCompression,
		ForceAttemptHTTP2:     true,
	}

	hc := &HTTPClient{
		transport:   transport,
		retryPolicy: retryPolicy,
		proxyConfig: opts.ProxyConfig,
		tlsConfig:   opts.TLSConfig,
	}

	if opts.ProxyConfig != nil {
		if err := hc.applyProxyConfig(opts.ProxyConfig); err != nil {
			return nil, fmt.Errorf("failed to apply proxy config: %w", err)
		}
	}

	if opts.TLSConfig != nil {
		if err := hc.applyTLSConfig(opts.TLSConfig); err != nil {
			return nil, fmt.Errorf("failed to apply TLS config: %w", err)
		}
	}

	hc.client = &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return hc, nil
}

func (hc *HTTPClient) applyProxyConfig(pc *ProxyConfig) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	proxyURL, err := pc.GetProxyURL()
	if err != nil {
		return err
	}

	hc.transport.Proxy = http.ProxyURL(proxyURL)
	hc.proxyConfig = pc
	return nil
}

func (hc *HTTPClient) applyTLSConfig(tc *TLSConfig) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	tlsConfig, err := tc.BuildTLSConfig()
	if err != nil {
		return err
	}

	hc.transport.TLSClientConfig = tlsConfig
	hc.tlsConfig = tc
	return nil
}

func (hc *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	return hc.DoWithRetry(req, hc.retryPolicy)
}

func (hc *HTTPClient) DoWithRetry(req *http.Request, policy *RetryPolicy) (*http.Response, error) {
	if policy == nil {
		policy = hc.retryPolicy
	}

	var lastResp *http.Response
	var lastErr error

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		resp, err := hc.client.Do(req)
		if err != nil {
			lastErr = err
			if !isRetryableError(err) {
				return nil, err
			}
		} else {
			lastResp = resp
			if !policy.IsRetryable(resp.StatusCode) {
				return resp, nil
			}
			_ = resp.Body.Close()
		}

		if attempt < policy.MaxRetries {
			backoff := policy.RetryBackoff * time.Duration(attempt+1)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(backoff):
			}
		}
	}

	if lastResp != nil {
		return lastResp, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, ErrMaxRetriesExceeded
}

func (hc *HTTPClient) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	return hc.Do(req)
}

func (hc *HTTPClient) GetClient() *http.Client {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.client
}

func (hc *HTTPClient) GetTransport() *http.Transport {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.transport
}

func (hc *HTTPClient) SetProxy(proxyURL string) error {
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return ErrInvalidProxyURL
	}
	hc.transport.Proxy = http.ProxyURL(parsed)
	return nil
}

func (hc *HTTPClient) CloseIdleConnections() {
	hc.transport.CloseIdleConnections()
}

func isRetryableError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isRetryableError(urlErr.Err)
	}
	return false
}

func (hc *HTTPClient) Clone() *HTTPClient {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	newTransport := hc.transport.Clone()
	newClient := &http.Client{
		Transport:     newTransport,
		Timeout:       hc.client.Timeout,
		CheckRedirect: hc.client.CheckRedirect,
	}

	return &HTTPClient{
		client:      newClient,
		transport:   newTransport,
		retryPolicy: hc.retryPolicy,
		proxyConfig: hc.proxyConfig,
		tlsConfig:   hc.tlsConfig,
	}
}

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (hc *HTTPClient) WithMiddleware(middleware func(http.RoundTripper) http.RoundTripper) *HTTPClient {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.transport = &http.Transport{
		Proxy:                 hc.transport.Proxy,
		DialContext:           hc.transport.DialContext,
		MaxIdleConns:          hc.transport.MaxIdleConns,
		MaxIdleConnsPerHost:   hc.transport.MaxIdleConnsPerHost,
		MaxConnsPerHost:       hc.transport.MaxConnsPerHost,
		IdleConnTimeout:       hc.transport.IdleConnTimeout,
		TLSHandshakeTimeout:   hc.transport.TLSHandshakeTimeout,
		ExpectContinueTimeout: hc.transport.ExpectContinueTimeout,
		DisableKeepAlives:     hc.transport.DisableKeepAlives,
		DisableCompression:    hc.transport.DisableCompression,
		ForceAttemptHTTP2:     hc.transport.ForceAttemptHTTP2,
		TLSClientConfig:       hc.transport.TLSClientConfig,
	}

	hc.client.Transport = middleware(hc.transport)
	return hc
}
