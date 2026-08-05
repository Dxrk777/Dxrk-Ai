// SPDX-License-Identifier: MIT
package remote

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// SessionState represents the current state of a remote session.
type SessionState int

const (
	StateDisconnected SessionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateFailed
)

func (s SessionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateFailed:
		return strconst.StrFailed
	default:
		return strconst.StrUnknown
	}
}

// MarshalJSON implements json.Marshaler for SessionState.
func (s SessionState) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON implements json.Unmarshaler for SessionState.
func (s *SessionState) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "disconnected":
		*s = StateDisconnected
	case "connecting":
		*s = StateConnecting
	case "connected":
		*s = StateConnected
	case "reconnecting":
		*s = StateReconnecting
	case strconst.StrFailed:
		*s = StateFailed
	default:
		return fmt.Errorf("unknown session state: %q", str)
	}
	return nil
}

// TLSConfig holds TLS configuration for remote connections.
type TLSConfig struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	CertFile           string `yaml:"cert_file" json:"cert_file"`
	KeyFile            string `yaml:"key_file" json:"key_file"`
	CACertFile         string `yaml:"ca_cert_file" json:"ca_cert_file"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify" json:"insecure_skip_verify"`
	ServerName         string `yaml:"server_name" json:"server_name"`
}

// AuthConfig holds authentication settings for remote connections.
type AuthConfig struct {
	Token     string `yaml:"token" json:"token"`
	TokenPath string `yaml:"token_path" json:"token_path"`
	Username  string `yaml:"username" json:"username"`
	Password  string `yaml:"password,omitempty" json:"password,omitempty"`
}

// RemoteConfig holds configuration for a remote connection.
type RemoteConfig struct {
	Host           string            `yaml:"host" json:"host"`
	Port           int               `yaml:"port" json:"port"`
	Protocol       string            `yaml:"protocol" json:"protocol"`
	ConnectTimeout int               `yaml:"connect_timeout" json:"connect_timeout"`
	ReadTimeout    int               `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout   int               `yaml:"write_timeout" json:"write_timeout"`
	KeepAlive      int               `yaml:"keep_alive" json:"keep_alive"`
	MaxRetries     int               `yaml:"max_retries" json:"max_retries"`
	RetryDelay     int               `yaml:"retry_delay" json:"retry_delay"`
	MaxMessageSize int64             `yaml:"max_message_size" json:"max_message_size"`
	TLS            TLSConfig         `yaml:"tls" json:"tls"`
	Auth           AuthConfig        `yaml:"auth" json:"auth"`
	Headers        map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// RemoteOption is a functional option for RemoteConfig.
type RemoteOption func(*RemoteConfig)

// WithHost sets the remote host address.
func WithHost(host string) RemoteOption {
	return func(c *RemoteConfig) { c.Host = host }
}

// WithPort sets the remote port.
func WithPort(port int) RemoteOption {
	return func(c *RemoteConfig) { c.Port = port }
}

// WithProtocol sets the connection protocol.
func WithProtocol(protocol string) RemoteOption {
	return func(c *RemoteConfig) { c.Protocol = protocol }
}

// WithConnectTimeout sets the connection timeout in seconds.
func WithConnectTimeout(seconds int) RemoteOption {
	return func(c *RemoteConfig) { c.ConnectTimeout = seconds }
}

// WithReadTimeout sets the read timeout in seconds.
func WithReadTimeout(seconds int) RemoteOption {
	return func(c *RemoteConfig) { c.ReadTimeout = seconds }
}

// WithWriteTimeout sets the write timeout in seconds.
func WithWriteTimeout(seconds int) RemoteOption {
	return func(c *RemoteConfig) { c.WriteTimeout = seconds }
}

// WithKeepAlive sets the keep-alive interval in seconds.
func WithKeepAlive(seconds int) RemoteOption {
	return func(c *RemoteConfig) { c.KeepAlive = seconds }
}

// WithMaxRetries sets the maximum number of connection retries.
func WithMaxRetries(n int) RemoteOption {
	return func(c *RemoteConfig) { c.MaxRetries = n }
}

// WithRetryDelay sets the delay between retries in seconds.
func WithRetryDelay(seconds int) RemoteOption {
	return func(c *RemoteConfig) { c.RetryDelay = seconds }
}

// WithMaxMessageSize sets the maximum message size in bytes.
func WithMaxMessageSize(size int64) RemoteOption {
	return func(c *RemoteConfig) { c.MaxMessageSize = size }
}

// WithTLS enables and configures TLS for the connection.
func WithTLS(enabled bool) RemoteOption {
	return func(c *RemoteConfig) { c.TLS.Enabled = enabled }
}

// WithTLSCert sets the TLS certificate and key files.
func WithTLSCert(certFile, keyFile string) RemoteOption {
	return func(c *RemoteConfig) {
		c.TLS.CertFile = certFile
		c.TLS.KeyFile = keyFile
	}
}

// WithTLSCACert sets the CA certificate for TLS verification.
func WithTLSCACert(caFile string) RemoteOption {
	return func(c *RemoteConfig) { c.TLS.CACertFile = caFile }
}

// WithTLSInsecureSkipVerify disables TLS certificate verification.
func WithTLSInsecureSkipVerify(skip bool) RemoteOption {
	return func(c *RemoteConfig) { c.TLS.InsecureSkipVerify = skip }
}

// WithTLSServerName sets the expected server name for TLS verification.
func WithTLSServerName(name string) RemoteOption {
	return func(c *RemoteConfig) { c.TLS.ServerName = name }
}

// WithAuthToken sets the authentication token.
func WithAuthToken(token string) RemoteOption {
	return func(c *RemoteConfig) { c.Auth.Token = token }
}

// WithAuthTokenPath sets the path to the authentication token file.
func WithAuthTokenPath(path string) RemoteOption {
	return func(c *RemoteConfig) { c.Auth.TokenPath = path }
}

// WithAuthCredentials sets username/password authentication.
func WithAuthCredentials(username, password string) RemoteOption {
	return func(c *RemoteConfig) {
		c.Auth.Username = username
		c.Auth.Password = password
	}
}

// WithHeader adds a custom header to the connection.
func WithHeader(key, value string) RemoteOption {
	return func(c *RemoteConfig) {
		if c.Headers == nil {
			c.Headers = make(map[string]string)
		}
		c.Headers[key] = value
	}
}

// NewRemoteConfig creates a new RemoteConfig with functional options.
func NewRemoteConfig(opts ...RemoteOption) *RemoteConfig {
	cfg := &RemoteConfig{
		Host:           "127.0.0.1",
		Port:           7700,
		Protocol:       "tcp",
		ConnectTimeout: 10,
		ReadTimeout:    30,
		WriteTimeout:   30,
		KeepAlive:      60,
		MaxRetries:     3,
		RetryDelay:     5,
		MaxMessageSize: 10 * 1024 * 1024,
		TLS: TLSConfig{
			Enabled: false,
		},
		Auth:    AuthConfig{},
		Headers: make(map[string]string),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// Address returns the full host:port address.
func (c *RemoteConfig) Address() string {
	return net.JoinHostPort(c.Host, fmt.Sprintf("%d", c.Port))
}

// ConnectTimeoutDuration returns the connect timeout as time.Duration.
func (c *RemoteConfig) ConnectTimeoutDuration() time.Duration {
	if c.ConnectTimeout <= 0 {
		return 10 * time.Second
	}
	return time.Duration(c.ConnectTimeout) * time.Second
}

// ReadTimeoutDuration returns the read timeout as time.Duration.
func (c *RemoteConfig) ReadTimeoutDuration() time.Duration {
	if c.ReadTimeout <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.ReadTimeout) * time.Second
}

// WriteTimeoutDuration returns the write timeout as time.Duration.
func (c *RemoteConfig) WriteTimeoutDuration() time.Duration {
	if c.WriteTimeout <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.WriteTimeout) * time.Second
}

// KeepAliveDuration returns the keep-alive interval as time.Duration.
func (c *RemoteConfig) KeepAliveDuration() time.Duration {
	if c.KeepAlive <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.KeepAlive) * time.Second
}

// RetryDelayDuration returns the retry delay as time.Duration.
func (c *RemoteConfig) RetryDelayDuration() time.Duration {
	if c.RetryDelay <= 0 {
		return 5 * time.Second
	}
	return time.Duration(c.RetryDelay) * time.Second
}

// Validate checks the config for common errors.
func (c *RemoteConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("remote host must not be empty")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("remote port must be between 1 and 65535, got %d", c.Port)
	}
	if c.ConnectTimeout < 0 {
		return fmt.Errorf("connect timeout must be non-negative, got %d", c.ConnectTimeout)
	}
	if c.ReadTimeout < 0 {
		return fmt.Errorf("read timeout must be non-negative, got %d", c.ReadTimeout)
	}
	if c.WriteTimeout < 0 {
		return fmt.Errorf("write timeout must be non-negative, got %d", c.WriteTimeout)
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative, got %d", c.MaxRetries)
	}
	_ = c.TLS // TLS without operator cert/key is permitted for the client; only the server requires them
	return nil
}

// BuildTLSConfig constructs a crypto/tls.Config from the remote TLS config.
func (c *RemoteConfig) BuildTLSConfig() (*tls.Config, error) {
	if !c.TLS.Enabled {
		return nil, nil
	}

	tc := &tls.Config{
		InsecureSkipVerify: c.TLS.InsecureSkipVerify, //nolint:gosec
		ServerName:         c.TLS.ServerName,
	}

	if c.TLS.CertFile != "" && c.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.TLS.CertFile, c.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load TLS keypair: %w", err)
		}
		tc.Certificates = []tls.Certificate{cert}
	}

	return tc, nil
}
