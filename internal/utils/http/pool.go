package http

import (
	"context"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type ConnectionPool struct {
	transport       *http.Transport
	maxIdleConns    int
	maxIdlePerHost  int
	idleConnTimeout time.Duration
	maxConnsPerHost int

	stats   PoolStats
	statsMu sync.RWMutex
	closed  atomic.Bool
	mu      sync.Mutex
}

type PoolStats struct {
	TotalConns      int64
	IdleConns       int64
	ActiveConns     int64
	WaitCount       int64
	WaitDuration    time.Duration
	MaxIdleConns    int
	MaxIdlePerHost  int
	IdleConnTimeout time.Duration
	MaxConnsPerHost int
}

type PoolConfig struct {
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	IdleConnTimeout       time.Duration
	MaxConnsPerHost       int
	DialerTimeout         time.Duration
	DialerKeepAlive       time.Duration
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
	DisableKeepAlives     bool
	DisableCompression    bool
	ForceHTTP2            bool
}

func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxIdleConns:          defaultMaxIdleConns,
		MaxIdleConnsPerHost:   defaultMaxIdleConnsPerHost,
		IdleConnTimeout:       defaultIdleConnTimeout,
		MaxConnsPerHost:       0,
		DialerTimeout:         30 * time.Second,
		DialerKeepAlive:       30 * time.Second,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: defaultExpectContinueTimeout,
		DisableKeepAlives:     false,
		DisableCompression:    false,
		ForceHTTP2:            true,
	}
}

func NewConnectionPool(config *PoolConfig) *ConnectionPool {
	if config == nil {
		config = DefaultPoolConfig()
	}

	dialer := &net.Dialer{
		Timeout:   config.DialerTimeout,
		KeepAlive: config.DialerKeepAlive,
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		DisableKeepAlives:     config.DisableKeepAlives,
		DisableCompression:    config.DisableCompression,
		ForceAttemptHTTP2:     config.ForceHTTP2,
	}

	pool := &ConnectionPool{
		transport:       transport,
		maxIdleConns:    config.MaxIdleConns,
		maxIdlePerHost:  config.MaxIdleConnsPerHost,
		idleConnTimeout: config.IdleConnTimeout,
		maxConnsPerHost: config.MaxConnsPerHost,
	}

	pool.stats.MaxIdleConns = config.MaxIdleConns
	pool.stats.MaxIdlePerHost = config.MaxIdleConnsPerHost
	pool.stats.IdleConnTimeout = config.IdleConnTimeout
	pool.stats.MaxConnsPerHost = config.MaxConnsPerHost

	return pool
}

func (cp *ConnectionPool) GetTransport() *http.Transport {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return cp.transport
}

func (cp *ConnectionPool) SetMaxIdleConns(n int) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.maxIdleConns = n
	cp.transport.MaxIdleConns = n
	cp.statsMu.Lock()
	cp.stats.MaxIdleConns = n
	cp.statsMu.Unlock()
}

func (cp *ConnectionPool) SetMaxIdleConnsPerHost(n int) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.maxIdlePerHost = n
	cp.transport.MaxIdleConnsPerHost = n
	cp.statsMu.Lock()
	cp.stats.MaxIdlePerHost = n
	cp.statsMu.Unlock()
}

func (cp *ConnectionPool) SetMaxConnsPerHost(n int) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.maxConnsPerHost = n
	cp.transport.MaxConnsPerHost = n
	cp.statsMu.Lock()
	cp.stats.MaxConnsPerHost = n
	cp.statsMu.Unlock()
}

func (cp *ConnectionPool) SetIdleConnTimeout(d time.Duration) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.idleConnTimeout = d
	cp.transport.IdleConnTimeout = d
	cp.statsMu.Lock()
	cp.stats.IdleConnTimeout = d
	cp.statsMu.Unlock()
}

func (cp *ConnectionPool) GetMaxIdleConns() int {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return cp.maxIdleConns
}

func (cp *ConnectionPool) GetMaxIdleConnsPerHost() int {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return cp.maxIdlePerHost
}

func (cp *ConnectionPool) GetMaxConnsPerHost() int {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return cp.maxConnsPerHost
}

func (cp *ConnectionPool) GetIdleConnTimeout() time.Duration {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return cp.idleConnTimeout
}

func (cp *ConnectionPool) Stats() PoolStats {
	cp.statsMu.RLock()
	defer cp.statsMu.RUnlock()

	stats := cp.stats
	// Note: http.Transport doesn't expose IdleConnCount publicly
	// We track idle connections through our own stats
	stats.IdleConns = atomic.LoadInt64(&cp.stats.IdleConns)
	stats.ActiveConns = atomic.LoadInt64(&stats.TotalConns) - stats.IdleConns
	if stats.ActiveConns < 0 {
		stats.ActiveConns = 0
	}

	return stats
}

func (cp *ConnectionPool) CloseIdleConnections() {
	cp.transport.CloseIdleConnections()
}

func (cp *ConnectionPool) Close() error {
	cp.closed.Store(true)
	cp.transport.CloseIdleConnections()
	return nil
}

func (cp *ConnectionPool) IsClosed() bool {
	return cp.closed.Load()
}

func (cp *ConnectionPool) Clone() *ConnectionPool {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	newTransport := cp.transport.Clone()
	newPool := &ConnectionPool{
		transport:       newTransport,
		maxIdleConns:    cp.maxIdleConns,
		maxIdlePerHost:  cp.maxIdlePerHost,
		idleConnTimeout: cp.idleConnTimeout,
		maxConnsPerHost: cp.maxConnsPerHost,
	}

	cp.statsMu.RLock()
	newPool.stats = cp.stats
	cp.statsMu.RUnlock()

	return newPool
}

func (cp *ConnectionPool) WithDialContext(dial func(ctx context.Context, network, addr string) (net.Conn, error)) *ConnectionPool {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.transport.DialContext = dial
	return cp
}

func (cp *ConnectionPool) WithTLSClientConfig(tlsConfig *TLSConfig) error {
	config, err := tlsConfig.BuildTLSConfig()
	if err != nil {
		return err
	}
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.transport.TLSClientConfig = config
	return nil
}

func (cp *ConnectionPool) WithProxy(proxyURL string) error {
	proxyConfig, err := NewProxyConfig(proxyURL)
	if err != nil {
		return err
	}
	proxyURLParsed, err := proxyConfig.GetProxyURL()
	if err != nil {
		return err
	}
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.transport.Proxy = http.ProxyURL(proxyURLParsed)
	return nil
}

func (cp *ConnectionPool) Reset() {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.transport.CloseIdleConnections()
	cp.statsMu.Lock()
	cp.stats = PoolStats{
		MaxIdleConns:    cp.maxIdleConns,
		MaxIdlePerHost:  cp.maxIdlePerHost,
		IdleConnTimeout: cp.idleConnTimeout,
		MaxConnsPerHost: cp.maxConnsPerHost,
	}
	cp.statsMu.Unlock()
}

type PooledClient struct {
	client  *http.Client
	pool    *ConnectionPool
	timeout time.Duration
}

func NewPooledClient(pool *ConnectionPool, timeout time.Duration) *PooledClient {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &PooledClient{
		client: &http.Client{
			Transport: pool.GetTransport(),
			Timeout:   timeout,
		},
		pool:    pool,
		timeout: timeout,
	}
}

func (pc *PooledClient) Do(req *http.Request) (*http.Response, error) {
	return pc.client.Do(req)
}

func (pc *PooledClient) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	return pc.client.Do(req)
}

func (pc *PooledClient) GetPool() *ConnectionPool {
	return pc.pool
}

func (pc *PooledClient) GetClient() *http.Client {
	return pc.client
}

func (pc *PooledClient) SetTimeout(timeout time.Duration) {
	pc.timeout = timeout
	pc.client.Timeout = timeout
}

func (pc *PooledClient) Close() error {
	return pc.pool.Close()
}

type PoolMonitor struct {
	pool     *ConnectionPool
	interval time.Duration
	stopCh   chan struct{}
	onStats  func(PoolStats)
	mu       sync.Mutex
	running  bool
}

func NewPoolMonitor(pool *ConnectionPool, interval time.Duration, onStats func(PoolStats)) *PoolMonitor {
	if interval == 0 {
		interval = 10 * time.Second
	}
	return &PoolMonitor{
		pool:     pool,
		interval: interval,
		stopCh:   make(chan struct{}),
		onStats:  onStats,
	}
}

func (pm *PoolMonitor) Start() {
	pm.mu.Lock()
	if pm.running {
		pm.mu.Unlock()
		return
	}
	pm.running = true
	pm.mu.Unlock()

	go pm.run()
}

func (pm *PoolMonitor) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if !pm.running {
		return
	}
	pm.running = false
	close(pm.stopCh)
}

func (pm *PoolMonitor) run() {
	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stopCh:
			return
		case <-ticker.C:
			stats := pm.pool.Stats()
			if pm.onStats != nil {
				pm.onStats(stats)
			}
		}
	}
}

func (pm *PoolMonitor) IsRunning() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.running
}
