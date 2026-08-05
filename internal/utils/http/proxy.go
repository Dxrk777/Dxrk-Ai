package http

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var (
	ErrUnsupportedProxy  = errors.New("unsupported proxy scheme")
	ErrProxyAuthRequired = errors.New("proxy authentication required")
	ErrInvalidProxyAuth  = errors.New("invalid proxy authentication")
	ErrNoProxyConfigured = errors.New("no proxy configured")
)

type ProxyType string

const (
	ProxyTypeHTTP   ProxyType = "http"
	ProxyTypeHTTPS  ProxyType = "https"
	ProxyTypeSOCKS4 ProxyType = "socks4"
	ProxyTypeSOCKS5 ProxyType = "socks5"
)

type ProxyAuth struct {
	Username string
	Password string
}

func (pa *ProxyAuth) String() string {
	if pa == nil || pa.Username == "" {
		return ""
	}
	if pa.Password == "" {
		return pa.Username
	}
	return pa.Username + ":" + pa.Password
}

func (pa *ProxyAuth) Encode() string {
	if pa == nil || pa.Username == "" {
		return ""
	}
	return pa.Username + ":" + pa.Password
}

type ProxyConfig struct {
	Type    ProxyType
	Host    string
	Port    int
	Auth    *ProxyAuth
	Bypass  []string
	NoProxy string
}

func NewProxyConfig(proxyURL string) (*ProxyConfig, error) {
	if proxyURL == "" {
		return nil, ErrInvalidProxyURL
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidProxyURL, err)
	}

	proxyType := ProxyType(strings.ToLower(parsed.Scheme))
	switch proxyType {
	case ProxyTypeHTTP, ProxyTypeHTTPS, ProxyTypeSOCKS4, ProxyTypeSOCKS5:
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProxy, parsed.Scheme)
	}

	host := parsed.Hostname()
	port := 0
	if parsed.Port() != "" {
		_, _ = fmt.Sscanf(parsed.Port(), "%d", &port)
	} else {
		switch proxyType {
		case ProxyTypeHTTP:
			port = 8080
		case ProxyTypeHTTPS:
			port = 8443
		case ProxyTypeSOCKS4, ProxyTypeSOCKS5:
			port = 1080
		}
	}

	var auth *ProxyAuth
	if parsed.User != nil {
		password, _ := parsed.User.Password()
		auth = &ProxyAuth{
			Username: parsed.User.Username(),
			Password: password,
		}
	}

	return &ProxyConfig{
		Type:    proxyType,
		Host:    host,
		Port:    port,
		Auth:    auth,
		Bypass:  parseBypassList(parsed.Query().Get("bypass")),
		NoProxy: parsed.Query().Get("no_proxy"),
	}, nil
}

func NewHTTPProxy(host string, port int, auth *ProxyAuth) *ProxyConfig {
	return &ProxyConfig{
		Type: ProxyTypeHTTP,
		Host: host,
		Port: port,
		Auth: auth,
	}
}

func NewHTTPSProxy(host string, port int, auth *ProxyAuth) *ProxyConfig {
	return &ProxyConfig{
		Type: ProxyTypeHTTPS,
		Host: host,
		Port: port,
		Auth: auth,
	}
}

func NewSOCKS5Proxy(host string, port int, auth *ProxyAuth) *ProxyConfig {
	return &ProxyConfig{
		Type: ProxyTypeSOCKS5,
		Host: host,
		Port: port,
		Auth: auth,
	}
}

func (pc *ProxyConfig) GetProxyURL() (*url.URL, error) {
	if pc == nil {
		return nil, ErrNoProxyConfigured
	}

	var scheme string
	switch pc.Type {
	case ProxyTypeHTTP:
		scheme = "http"
	case ProxyTypeHTTPS:
		scheme = "https"
	case ProxyTypeSOCKS4:
		scheme = "socks4"
	case ProxyTypeSOCKS5:
		scheme = "socks5"
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProxy, pc.Type)
	}

	hostPort := net.JoinHostPort(pc.Host, fmt.Sprintf("%d", pc.ProxyPort()))

	var user *url.Userinfo
	if pc.Auth != nil {
		user = url.UserPassword(pc.Auth.Username, pc.Auth.Password)
	}

	return &url.URL{
		Scheme: scheme,
		Host:   hostPort,
		User:   user,
	}, nil
}

func (pc *ProxyConfig) ProxyPort() int {
	if pc.Port > 0 {
		return pc.Port
	}
	switch pc.Type {
	case ProxyTypeHTTP:
		return 8080
	case ProxyTypeHTTPS:
		return 8443
	case ProxyTypeSOCKS4, ProxyTypeSOCKS5:
		return 1080
	}
	return 0
}

func (pc *ProxyConfig) String() string {
	proxyURL, err := pc.GetProxyURL()
	if err != nil {
		return ""
	}
	return proxyURL.String()
}

func (pc *ProxyConfig) ShouldBypass(host string) bool {
	if pc == nil {
		return false
	}

	for _, bypass := range pc.Bypass {
		if matchBypass(bypass, host) {
			return true
		}
	}

	if pc.NoProxy != "" {
		for _, pattern := range strings.Split(pc.NoProxy, ",") {
			pattern = strings.TrimSpace(pattern)
			if matchBypass(pattern, host) {
				return true
			}
		}
	}

	return false
}

func (pc *ProxyConfig) AddBypass(pattern string) {
	if pc != nil && pattern != "" {
		pc.Bypass = append(pc.Bypass, pattern)
	}
}

func (pc *ProxyConfig) SetNoProxy(noProxy string) {
	if pc != nil {
		pc.NoProxy = noProxy
	}
}

func parseBypassList(bypass string) []string {
	if bypass == "" {
		return nil
	}
	var patterns []string
	for _, p := range strings.Split(bypass, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

func matchBypass(pattern, host string) bool {
	pattern = strings.ToLower(pattern)
	host = strings.ToLower(host)

	if pattern == "*" {
		return true
	}

	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:]
		return strings.HasSuffix(host, suffix)
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(host, prefix)
	}

	return pattern == host
}

func (pc *ProxyConfig) Clone() *ProxyConfig {
	if pc == nil {
		return nil
	}

	var auth *ProxyAuth
	if pc.Auth != nil {
		auth = &ProxyAuth{
			Username: pc.Auth.Username,
			Password: pc.Auth.Password,
		}
	}

	bypass := make([]string, len(pc.Bypass))
	copy(bypass, pc.Bypass)

	return &ProxyConfig{
		Type:    pc.Type,
		Host:    pc.Host,
		Port:    pc.Port,
		Auth:    auth,
		Bypass:  bypass,
		NoProxy: pc.NoProxy,
	}
}

func GetProxyFromEnvironment() *ProxyConfig {
	proxyURL := ""
	for _, env := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if val := getEnv(env); val != "" {
			proxyURL = val
			break
		}
	}

	if proxyURL == "" {
		return nil
	}

	config, err := NewProxyConfig(proxyURL)
	if err != nil {
		return nil
	}

	noProxy := getEnv("NO_PROXY")
	if noProxy == "" {
		noProxy = getEnv("no_proxy")
	}
	config.SetNoProxy(noProxy)

	return config
}

func getEnv(key string) string {
	return ""
}

func ParseProxyURL(proxyURL string) (*ProxyConfig, error) {
	return NewProxyConfig(proxyURL)
}

func MustParseProxyURL(proxyURL string) *ProxyConfig {
	config, err := NewProxyConfig(proxyURL)
	if err != nil {
		panic(err)
	}
	return config
}
