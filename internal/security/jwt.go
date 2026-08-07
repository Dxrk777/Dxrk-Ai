// SPDX-License-Identifier: MIT
package security

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/golang-jwt/jwt/v5"
)

// ---- JWT Token Types ----

// TokenKind identifies the type of token.
type TokenKind int

const (
	TokenKindSessionIngress TokenKind = iota // sk-ant-si-* (session ingress JWT)
	TokenKindAccessToken                     // sk-ant-oa-* (API access token)
	TokenKindUnknown
)

func (k TokenKind) String() string {
	switch k {
	case TokenKindSessionIngress:
		return "session_ingress"
	case TokenKindAccessToken:
		return "access_token"
	default:
		return strconst.StrUnknown
	}
}

// TokenInfo holds parsed JWT metadata.
type TokenInfo struct {
	Kind      TokenKind
	Token     string
	Subject   string
	Issuer    string
	ExpiresAt time.Time
	IssuedAt  time.Time
	Claims    jwt.RegisteredClaims
	IsValid   bool
	IsExpired bool
}

// ---- JWT Utilities ----

// DecodeJwtPayload decodes the payload of a JWT without signature verification.
// Returns nil if the token is malformed.
func DecodeJwtPayload(tokenString string) map[string]any {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 2 && len(parts) != 3 {
		return nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}

	return claims
}

// ClassifyToken determines the kind of a token based on its prefix.
func ClassifyToken(token string) TokenKind {
	if strings.HasPrefix(token, "sk-ant-si-") {
		return TokenKindSessionIngress
	}
	if strings.HasPrefix(token, "sk-ant-oa-") {
		return TokenKindAccessToken
	}
	return TokenKindUnknown
}

// IsTokenExpired checks if a JWT is expired using clock skew tolerance.
func IsTokenExpired(tokenString string, skew time.Duration) bool {
	claims := DecodeJwtPayload(tokenString)
	if claims == nil {
		return false // can't determine → assume not expired (fail-open for check)
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return false // no exp claim → assume not expired
	}

	expiry := time.Unix(int64(exp), 0)
	now := time.Now()

	return now.After(expiry.Add(skew))
}

// ParseTokenSafe parses a JWT with signature verification.
func ParseTokenSafe(tokenString string, keyFunc jwt.Keyfunc) (*TokenInfo, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, keyFunc)
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	info := &TokenInfo{
		Kind:      ClassifyToken(tokenString),
		Token:     tokenString,
		Subject:   claims.Subject,
		Issuer:    claims.Issuer,
		ExpiresAt: claims.ExpiresAt.Time,
		IssuedAt:  claims.IssuedAt.Time,
		Claims:    *claims,
		IsValid:   token.Valid,
		IsExpired: time.Now().After(claims.ExpiresAt.Time),
	}

	return info, nil
}

// ---- Token Refresh Scheduler ----

// RefreshFunc is called when a token needs refreshing.
type RefreshFunc func(ctx context.Context) (newToken string, err error)

// RefreshConfig configures the token refresh scheduler.
type RefreshConfig struct {
	// PollInterval is how often to check token expiry. Default: 5 min.
	PollInterval time.Duration
	// RefreshBefore is how long before expiry to refresh. Default: 10 min.
	RefreshBefore time.Duration
	// RetryInterval is the base interval between refresh retries. Default: 30s.
	RetryInterval time.Duration
	// MaxRetries is the maximum number of retries on refresh failure. Default: 2.
	MaxRetries int
	// ClockSkew is the tolerance for clock skew. Default: 30s.
	ClockSkew time.Duration
}

// DefaultRefreshConfig returns sensible defaults.
func DefaultRefreshConfig() RefreshConfig {
	return RefreshConfig{
		PollInterval:  5 * time.Minute,
		RefreshBefore: 10 * time.Minute,
		RetryInterval: 30 * time.Second,
		MaxRetries:    2,
		ClockSkew:     30 * time.Second,
	}
}

// TokenRefreshScheduler manages automatic token refresh.
type TokenRefreshScheduler struct {
	mu          sync.Mutex
	config      RefreshConfig
	token       string
	refreshFunc RefreshFunc
	cancel      context.CancelFunc
	done        chan struct{}

	// State
	lastRefresh  time.Time
	lastError    error
	refreshCount int64
	failureCount int64
}

// NewTokenRefreshScheduler creates a scheduler that monitors and refreshes tokens.
func NewTokenRefreshScheduler(token string, refreshFunc RefreshFunc, config RefreshConfig) *TokenRefreshScheduler {
	if config.PollInterval == 0 {
		config = DefaultRefreshConfig()
	}
	return &TokenRefreshScheduler{
		config:      config,
		token:       token,
		refreshFunc: refreshFunc,
		done:        make(chan struct{}),
	}
}

// Start begins the background refresh loop.
func (s *TokenRefreshScheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	go func() {
		defer close(s.done)

		ticker := time.NewTicker(s.config.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.maybeRefresh(ctx)
			}
		}
	}()
}

// Stop halts the refresh loop and waits for it to finish.
func (s *TokenRefreshScheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.done != nil {
		<-s.done
	}
}

// Token returns the current (possibly refreshed) token.
func (s *TokenRefreshScheduler) Token() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token
}

// RefreshStats returns refresh metrics.
func (s *TokenRefreshScheduler) RefreshStats() (refreshCount, failureCount int64, lastRefresh time.Time, lastErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshCount, s.failureCount, s.lastRefresh, s.lastError
}

func (s *TokenRefreshScheduler) maybeRefresh(ctx context.Context) {
	s.mu.Lock()
	token := s.token
	s.mu.Unlock()

	if token == "" {
		return
	}

	expired := IsTokenExpired(token, s.config.ClockSkew)
	if !expired {
		// Not expired yet — check if we're within the refresh window
		claims := DecodeJwtPayload(token)
		if claims == nil {
			return
		}
		exp, ok := claims["exp"].(float64)
		if !ok {
			return
		}
		expiry := time.Unix(int64(exp), 0)
		timeUntilExpiry := time.Until(expiry)

		// Only refresh if within the refresh window
		if timeUntilExpiry > s.config.RefreshBefore {
			return
		}
	}

	// Attempt refresh with retries
	s.attemptRefresh(ctx)
}

func (s *TokenRefreshScheduler) attemptRefresh(ctx context.Context) {
	var lastErr error

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: retry interval * attempt
			retryDelay := s.config.RetryInterval * time.Duration(attempt)
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDelay):
			}
		}

		newToken, err := s.refreshFunc(ctx)
		if err != nil {
			lastErr = err
			continue
		}

		if newToken == "" {
			lastErr = fmt.Errorf("refresh returned empty token")
			continue
		}

		// Success
		s.mu.Lock()
		s.token = newToken
		s.lastRefresh = time.Now()
		s.lastError = nil
		s.refreshCount++
		s.mu.Unlock()

		return
	}

	// All retries failed
	s.mu.Lock()
	s.lastError = lastErr
	s.failureCount++
	s.mu.Unlock()
}

// ---- Trusted Device Tokens ----

// TrustedDevice holds a trusted device token and metadata.
type TrustedDevice struct {
	Token     string    `json:"token"`
	DeviceID  string    `json:"device_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IsDeviceTrusted checks if a device token is valid and not expired.
func IsDeviceTrusted(device TrustedDevice) bool {
	if device.Token == "" {
		return false
	}
	if time.Now().After(device.ExpiresAt) {
		return false
	}
	// Must be at least 10 minutes old to be considered trusted
	if time.Since(device.CreatedAt) < 10*time.Minute {
		return false
	}
	return true
}

// ---- URL Safety ----

// ValidateIngressURL checks that an ingress URL uses HTTPS (except localhost).
func ValidateIngressURL(url string, isDev bool) error {
	if isDev {
		return nil // dev mode allows any protocol
	}

	if strings.HasPrefix(url, "http://localhost") || strings.HasPrefix(url, "http://127.0.0.1") {
		return nil // localhost exception
	}

	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("insecure origin required: %s (must use HTTPS in production)", url)
	}

	return nil
}

// ValidateID checks that a server-provided ID is safe for URL paths.
func ValidateID(id string) bool {
	const safePattern = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
	for _, c := range id {
		if !strings.ContainsRune(safePattern, c) {
			return false
		}
	}
	return len(id) > 0 && len(id) <= 256
}

// RedactToken returns a partially redacted version of a token for logging.
func RedactToken(token string) string {
	if len(token) < 16 {
		return "[REDACTED]"
	}
	return token[:8] + "..." + token[len(token)-4:]
}
