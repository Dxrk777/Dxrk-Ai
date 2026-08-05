package advanced

import (
	"crypto/tls"
	"fmt"
	"time"
)

type Config struct {
	Host            string
	Port            int
	TLSEnabled      bool
	TLSCertFile     string
	TLSKeyFile      string
	ConnectTimeout  time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	MaxHeaderBytes  int
	RetryAttempts   int
	RetryBaseDelay  time.Duration
	RetryMaxDelay   time.Duration
	RateLimitRPS    float64
	RateLimitBurst  int
	MaxConns        int
	MaxIdleConns    int
	KeepAlive       time.Duration
	DisableRedirect bool
}

func DefaultConfig() *Config {
	return &Config{
		Host:           "0.0.0.0",
		Port:           8443,
		ConnectTimeout: 10 * time.Second,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
		RetryAttempts:  3,
		RetryBaseDelay: 500 * time.Millisecond,
		RetryMaxDelay:  10 * time.Second,
		RateLimitRPS:   100,
		RateLimitBurst: 200,
		MaxConns:       100,
		MaxIdleConns:   10,
		KeepAlive:      30 * time.Second,
	}
}

func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type Option func(*Config)

func WithHost(host string) Option {
	return func(c *Config) { c.Host = host }
}

func WithPort(port int) Option {
	return func(c *Config) { c.Port = port }
}

func WithTLS(certFile, keyFile string) Option {
	return func(c *Config) {
		c.TLSEnabled = true
		c.TLSCertFile = certFile
		c.TLSKeyFile = keyFile
	}
}

func WithConnectTimeout(d time.Duration) Option {
	return func(c *Config) { c.ConnectTimeout = d }
}

func WithReadTimeout(d time.Duration) Option {
	return func(c *Config) { c.ReadTimeout = d }
}

func WithWriteTimeout(d time.Duration) Option {
	return func(c *Config) { c.WriteTimeout = d }
}

func WithIdleTimeout(d time.Duration) Option {
	return func(c *Config) { c.IdleTimeout = d }
}

func WithRetry(attempts int, baseDelay, maxDelay time.Duration) Option {
	return func(c *Config) {
		c.RetryAttempts = attempts
		c.RetryBaseDelay = baseDelay
		c.RetryMaxDelay = maxDelay
	}
}

func WithRateLimit(rps float64, burst int) Option {
	return func(c *Config) {
		c.RateLimitRPS = rps
		c.RateLimitBurst = burst
	}
}

func WithMaxConns(max int) Option {
	return func(c *Config) { c.MaxConns = max }
}

func WithKeepAlive(d time.Duration) Option {
	return func(c *Config) { c.KeepAlive = d }
}

func ApplyOptions(cfg *Config, opts ...Option) *Config {
	for _, o := range opts {
		o(cfg)
	}
	return cfg
}

func (c *Config) buildTLSConfig() *tls.Config {
	if !c.TLSEnabled {
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return &tls.Config{MinVersion: tls.VersionTLS12}
}
