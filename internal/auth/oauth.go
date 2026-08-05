package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// TokenSet holds OAuth 2.0 tokens and metadata.
type TokenSet struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
	Scopes       []string  `json:"scopes,omitempty"`
	Provider     string    `json:"provider"`
}

// IsExpired reports whether the access token has expired, with a 30-second buffer.
func (t *TokenSet) IsExpired() bool {
	return time.Now().After(t.ExpiresAt.Add(-30 * time.Second))
}

// OAuthConfig holds the configuration for an OAuth 2.0 provider.
type OAuthConfig struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret,omitempty"`
	RedirectURI  string   `json:"redirect_uri"`
	Scopes       []string `json:"scopes"`
	AuthURL      string   `json:"auth_url"`
	TokenURL     string   `json:"token_url"`
	APIURL       string   `json:"api_url,omitempty"`
	ProviderName string   `json:"provider_name"`
}

// OAuthProvider defines the interface for OAuth provider operations.
type OAuthProvider interface {
	GetAuthorizationURL(config OAuthConfig, state, codeChallenge string) string
	ExchangeCode(ctx context.Context, config OAuthConfig, code, codeVerifier string) (*TokenSet, error)
	RefreshToken(ctx context.Context, config OAuthConfig, refreshToken string) (*TokenSet, error)
	RevokeToken(ctx context.Context, config OAuthConfig, token string) error
}

// tokenResponse is the standard OAuth 2.0 token endpoint response.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

// OAuthManager manages OAuth 2.0 flows with PKCE for a single provider.
type OAuthManager struct {
	config  OAuthConfig
	storage SecureStorage
	prov    OAuthProvider
	tokenMu sync.RWMutex
	token   *TokenSet
}

// NewOAuthManager creates an OAuthManager with the given config and storage.
func NewOAuthManager(config OAuthConfig, storage SecureStorage) *OAuthManager {
	return &OAuthManager{
		config:  config,
		storage: storage,
		prov:    &defaultOAuthProvider{},
	}
}

// NewOAuthManagerWithProvider creates an OAuthManager with a custom provider.
func NewOAuthManagerWithProvider(config OAuthConfig, storage SecureStorage, provider OAuthProvider) *OAuthManager {
	return &OAuthManager{
		config:  config,
		storage: storage,
		prov:    provider,
	}
}

// storageKey returns the storage key for this provider's token set.
func (m *OAuthManager) storageKey() string {
	return fmt.Sprintf("oauth_token_%s", m.config.ProviderName)
}

// StartFlow generates a PKCE authorization URL and returns the URL and state.
func (m *OAuthManager) StartFlow() (string, string, error) {
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", "", fmt.Errorf("generate state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	verifier, challenge, err := generatePKCE()
	if err != nil {
		return "", "", fmt.Errorf("generate PKCE: %w", err)
	}

	flowData := map[string]string{
		"state":                  state,
		strconst.StrCodeVerifier: verifier,
		"code_challenge":         challenge,
	}
	data, _ := json.Marshal(flowData)
	if err := m.storage.Store("oauth_flow_state", data); err != nil {
		return "", "", fmt.Errorf("store flow state: %w", err)
	}

	authURL := m.prov.GetAuthorizationURL(m.config, state, challenge)
	return authURL, state, nil
}

// HandleCallback validates the state and exchanges the authorization code for tokens.
func (m *OAuthManager) HandleCallback(code, state string) (*TokenSet, error) {
	// Retrieve stored flow state
	data, err := m.storage.Load("oauth_flow_state")
	if err != nil {
		return nil, fmt.Errorf("load flow state: %w", err)
	}

	var flowData map[string]string
	if err := json.Unmarshal(data, &flowData); err != nil {
		return nil, fmt.Errorf("unmarshal flow state: %w", err)
	}

	if flowData["state"] != state {
		return nil, fmt.Errorf("state mismatch: expected %q, got %q", flowData["state"], state)
	}

	// Clean up flow state
	_ = m.storage.Delete("oauth_flow_state")

	// Exchange code for tokens
	tokenSet, err := m.prov.ExchangeCode(context.Background(), m.config, code, flowData[strconst.StrCodeVerifier])
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	tokenSet.Provider = m.config.ProviderName

	// Persist token set
	if err := m.storeTokenSet(tokenSet); err != nil {
		return nil, fmt.Errorf("store token set: %w", err)
	}

	m.tokenMu.Lock()
	m.token = tokenSet
	m.tokenMu.Unlock()

	return tokenSet, nil
}

// RefreshAccessToken refreshes the access token using the stored refresh token.
func (m *OAuthManager) RefreshAccessToken(tokenSet *TokenSet) (*TokenSet, error) {
	if tokenSet.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	newTokenSet, err := m.prov.RefreshToken(context.Background(), m.config, tokenSet.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}

	newTokenSet.Provider = m.config.ProviderName
	if newTokenSet.RefreshToken == "" {
		newTokenSet.RefreshToken = tokenSet.RefreshToken
	}
	if len(newTokenSet.Scopes) == 0 {
		newTokenSet.Scopes = tokenSet.Scopes
	}

	if err := m.storeTokenSet(newTokenSet); err != nil {
		return nil, fmt.Errorf("store refreshed token: %w", err)
	}

	m.tokenMu.Lock()
	m.token = newTokenSet
	m.tokenMu.Unlock()

	return newTokenSet, nil
}

// RevokeToken revokes the given token set with the provider.
func (m *OAuthManager) RevokeToken(tokenSet *TokenSet) error {
	if tokenSet.AccessToken == "" {
		return nil
	}

	if err := m.prov.RevokeToken(context.Background(), m.config, tokenSet.AccessToken); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	_ = m.storage.Delete(m.storageKey())

	m.tokenMu.Lock()
	m.token = nil
	m.tokenMu.Unlock()

	return nil
}

// GetValidToken returns a valid token, refreshing it automatically if expired.
func (m *OAuthManager) GetValidToken() (*TokenSet, error) {
	m.tokenMu.RLock()
	token := m.token
	m.tokenMu.RUnlock()

	if token == nil {
		data, err := m.storage.Load(m.storageKey())
		if err != nil {
			return nil, fmt.Errorf("no stored token: %w", err)
		}

		token = &TokenSet{}
		if err := json.Unmarshal(data, token); err != nil {
			return nil, fmt.Errorf("unmarshal token: %w", err)
		}

		m.tokenMu.Lock()
		m.token = token
		m.tokenMu.Unlock()
	}

	if token.IsExpired() && token.RefreshToken != "" {
		refreshed, err := m.RefreshAccessToken(token)
		if err != nil {
			return nil, fmt.Errorf("auto-refresh failed: %w", err)
		}
		return refreshed, nil
	}

	return token, nil
}

// IsAuthenticated reports whether a valid (non-expired) token is available.
func (m *OAuthManager) IsAuthenticated() bool {
	token, err := m.GetValidToken()
	if err != nil {
		return false
	}
	return token != nil && !token.IsExpired()
}

// Logout clears the stored token and revokes it if possible.
func (m *OAuthManager) Logout() error {
	m.tokenMu.RLock()
	token := m.token
	m.tokenMu.RUnlock()

	if token != nil {
		_ = m.RevokeToken(token)
	}

	_ = m.storage.Delete(m.storageKey())
	_ = m.storage.Delete("oauth_flow_state")

	m.tokenMu.Lock()
	m.token = nil
	m.tokenMu.Unlock()

	return nil
}

// GetAccessToken returns the raw access token string, or empty if not authenticated.
func (m *OAuthManager) GetAccessToken() string {
	token, err := m.GetValidToken()
	if err != nil || token == nil {
		return ""
	}
	return token.AccessToken
}

func (m *OAuthManager) storeTokenSet(tokenSet *TokenSet) error {
	data, err := json.Marshal(tokenSet)
	if err != nil {
		return fmt.Errorf("marshal token set: %w", err)
	}
	return m.storage.Store(m.storageKey(), data)
}

// ---- PKCE ----

// generatePKCE creates a PKCE code verifier and code challenge pair.
func generatePKCE() (codeVerifier, codeChallenge string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}

	codeVerifier = base64.RawURLEncoding.EncodeToString(buf)
	if len(codeVerifier) < 43 {
		codeVerifier += strings.Repeat("a", 43-len(codeVerifier))
	}

	h := sha256.Sum256([]byte(codeVerifier))
	codeChallenge = base64.RawURLEncoding.EncodeToString(h[:])

	return codeVerifier, codeChallenge, nil
}

// ---- defaultOAuthProvider ----

// defaultOAuthProvider implements OAuthProvider using the standard token endpoint.
type defaultOAuthProvider struct{}

func (p *defaultOAuthProvider) GetAuthorizationURL(config OAuthConfig, state, codeChallenge string) string {
	params := url.Values{
		strconst.StrClientId:    {config.ClientID},
		strconst.StrRedirectUri: {config.RedirectURI},
		"response_type":         {"code"},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	if len(config.Scopes) > 0 {
		params.Set("scope", strings.Join(config.Scopes, " "))
	}
	return config.AuthURL + "?" + params.Encode()
}

func (p *defaultOAuthProvider) ExchangeCode(ctx context.Context, config OAuthConfig, code, codeVerifier string) (*TokenSet, error) {
	data := url.Values{
		strconst.StrGrantType:    {"authorization_code"},
		strconst.StrClientId:     {config.ClientID},
		"code":                   {code},
		strconst.StrRedirectUri:  {config.RedirectURI},
		strconst.StrCodeVerifier: {codeVerifier},
	}
	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}

	return p.doTokenRequest(ctx, config.TokenURL, data)
}

func (p *defaultOAuthProvider) RefreshToken(ctx context.Context, config OAuthConfig, refreshToken string) (*TokenSet, error) {
	data := url.Values{
		strconst.StrGrantType:    {strconst.StrRefreshToken},
		strconst.StrClientId:     {config.ClientID},
		strconst.StrRefreshToken: {refreshToken},
	}
	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}

	return p.doTokenRequest(ctx, config.TokenURL, data)
}

func (p *defaultOAuthProvider) RevokeToken(ctx context.Context, config OAuthConfig, token string) error {
	data := url.Values{
		"token": {token},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.TokenURL+"/revoke", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if config.ClientID != "" && config.ClientSecret != "" {
		req.SetBasicAuth(config.ClientID, config.ClientSecret)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoke request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("revoke failed (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (p *defaultOAuthProvider) doTokenRequest(ctx context.Context, tokenURL string, data url.Values) (*TokenSet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("unmarshal token response: %w", err)
	}

	tokenSet := &TokenSet{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		TokenType:    tokenResp.TokenType,
	}
	if tokenResp.Scope != "" {
		tokenSet.Scopes = strings.Split(tokenResp.Scope, " ")
	}

	return tokenSet, nil
}
