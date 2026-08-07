package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// ---- Provider Registry ----

// ProviderRegistry holds registered OAuth providers by name.
type ProviderRegistry struct {
	providers map[string]OAuthProvider
	configs   map[string]OAuthConfig
}

// NewProviderRegistry creates an empty registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]OAuthProvider),
		configs:   make(map[string]OAuthConfig),
	}
}

// Register adds a provider and its config to the registry.
func (r *ProviderRegistry) Register(name string, provider OAuthProvider, config OAuthConfig) {
	r.providers[name] = provider
	r.configs[name] = config
}

// Get retrieves a provider by name.
func (r *ProviderRegistry) Get(name string) (OAuthProvider, OAuthConfig, bool) {
	p, ok := r.providers[name]
	c := r.configs[name]
	return p, c, ok
}

// List returns all registered provider names.
func (r *ProviderRegistry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// ---- GitHub Provider ----

const (
	GitHubAuthURL  = "https://github.com/login/oauth/authorize"
	GitHubTokenURL = "https://github.com/login/oauth/access_token"
)

// NewGitHubOAuth creates an OAuthConfig for GitHub.
func NewGitHubOAuth(clientID, clientSecret, redirectURI string) OAuthConfig {
	return OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		Scopes:       []string{"repo", "read:user", "user:email"},
		AuthURL:      GitHubAuthURL,
		TokenURL:     GitHubTokenURL,
		APIURL:       "https://api.github.com",
		ProviderName: "github",
	}
}

// GitHubProvider implements OAuthProvider for GitHub.
type GitHubProvider struct{}

// NewGitHubProvider creates a new GitHub OAuth provider.
func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{}
}

// GetAuthorizationURL builds the GitHub OAuth authorization URL.
func (p *GitHubProvider) GetAuthorizationURL(config OAuthConfig, state, codeChallenge string) string {
	params := fmt.Sprintf(
		"?client_id=%s&redirect_uri=%s&scope=%s&state=%s&code_challenge=%s&code_challenge_method=S256",
		config.ClientID,
		config.RedirectURI,
		strings.Join(config.Scopes, "+"),
		state,
		codeChallenge,
	)
	return config.AuthURL + params
}

// ExchangeCode exchanges an authorization code for tokens via GitHub.
func (p *GitHubProvider) ExchangeCode(ctx context.Context, config OAuthConfig, code, codeVerifier string) (*TokenSet, error) {
	return defaultTokenExchange(ctx, config, code, codeVerifier)
}

// RefreshToken refreshes an access token via GitHub.
func (p *GitHubProvider) RefreshToken(ctx context.Context, config OAuthConfig, refreshToken string) (*TokenSet, error) {
	return defaultTokenRefresh(ctx, config, refreshToken)
}

// RevokeToken revokes a GitHub token (best-effort; GitHub may not support revocation).
func (p *GitHubProvider) RevokeToken(_ context.Context, _ OAuthConfig, _ string) error {
	return nil
}

// ---- Google Provider ----

const (
	GoogleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL = "https://oauth2.googleapis.com/token"
)

// NewGoogleOAuth creates an OAuthConfig for Google.
func NewGoogleOAuth(clientID, clientSecret, redirectURI string) OAuthConfig {
	return OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		Scopes:       []string{"email", "profile", "openid"},
		AuthURL:      GoogleAuthURL,
		TokenURL:     GoogleTokenURL,
		APIURL:       "https://www.googleapis.com/oauth2/v3",
		ProviderName: "google",
	}
}

// GoogleProvider implements OAuthProvider for Google.
type GoogleProvider struct{}

// NewGoogleProvider creates a new Google OAuth provider.
func NewGoogleProvider() *GoogleProvider {
	return &GoogleProvider{}
}

// GetAuthorizationURL builds the Google OAuth authorization URL.
func (p *GoogleProvider) GetAuthorizationURL(config OAuthConfig, state, codeChallenge string) string {
	params := fmt.Sprintf(
		"?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s&code_challenge=%s&code_challenge_method=S256&access_type=offline&prompt=consent",
		config.ClientID,
		config.RedirectURI,
		strings.Join(config.Scopes, "+"),
		state,
		codeChallenge,
	)
	return config.AuthURL + params
}

// ExchangeCode exchanges an authorization code for tokens via Google.
func (p *GoogleProvider) ExchangeCode(ctx context.Context, config OAuthConfig, code, codeVerifier string) (*TokenSet, error) {
	return defaultTokenExchange(ctx, config, code, codeVerifier)
}

// RefreshToken refreshes an access token via Google.
func (p *GoogleProvider) RefreshToken(ctx context.Context, config OAuthConfig, refreshToken string) (*TokenSet, error) {
	return defaultTokenRefresh(ctx, config, refreshToken)
}

// RevokeToken revokes a Google token.
func (p *GoogleProvider) RevokeToken(ctx context.Context, config OAuthConfig, token string) error {
	req, err := newFormPost(ctx, "https://oauth2.googleapis.com/revoke", map[string]string{
		"token": token,
	})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return doSimpleRequest(req)
}

// ---- Anthropic Provider ----

// AnthropicConfig represents an Anthropic API key configuration.
// Anthropic does not support OAuth; API keys are used directly.
type AnthropicConfig struct {
	APIKey string `json:"api_key"`
}

// NewAnthropicAPIKey validates an Anthropic API key format.
// Anthropic keys start with "sk-ant-" and do not use OAuth.
func NewAnthropicAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("anthropic API key is required")
	}
	if !strings.HasPrefix(apiKey, "sk-ant-") {
		return fmt.Errorf("invalid Anthropic API key format: must start with sk-ant-")
	}
	return nil
}

// ---- Token exchange helpers ----

func defaultTokenExchange(ctx context.Context, config OAuthConfig, code, codeVerifier string) (*TokenSet, error) {
	data := map[string]string{
		strconst.StrGrantType:    "authorization_code",
		strconst.StrClientId:     config.ClientID,
		"code":                   code,
		strconst.StrRedirectUri:  config.RedirectURI,
		strconst.StrCodeVerifier: codeVerifier,
	}
	if config.ClientSecret != "" {
		data["client_secret"] = config.ClientSecret
	}

	req, err := newFormPost(ctx, config.TokenURL, data)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := doTokenRequest(req)
	if err != nil {
		return nil, err
	}

	return parseTokenResponse(resp), nil
}

func defaultTokenRefresh(ctx context.Context, config OAuthConfig, refreshToken string) (*TokenSet, error) {
	data := map[string]string{
		strconst.StrGrantType:    strconst.StrRefreshToken,
		strconst.StrClientId:     config.ClientID,
		strconst.StrRefreshToken: refreshToken,
	}
	if config.ClientSecret != "" {
		data["client_secret"] = config.ClientSecret
	}

	req, err := newFormPost(ctx, config.TokenURL, data)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := doTokenRequest(req)
	if err != nil {
		return nil, err
	}

	return parseTokenResponse(resp), nil
}

// ---- HTTP helpers ----

func newFormPost(ctx context.Context, endpoint string, data map[string]string) (*http.Request, error) {
	form := url.Values{}
	for k, v := range data {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func doTokenRequest(req *http.Request) (map[string]any, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return result, nil
}

func doSimpleRequest(req *http.Request) error {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func parseTokenResponse(resp map[string]any) *TokenSet {
	tokenSet := &TokenSet{
		TokenType: getString(resp, "token_type"),
	}

	if v, ok := resp["access_token"].(string); ok {
		tokenSet.AccessToken = v
	}
	if v, ok := resp[strconst.StrRefreshToken].(string); ok {
		tokenSet.RefreshToken = v
	}
	if v, ok := resp["expires_in"].(float64); ok {
		tokenSet.ExpiresAt = time.Now().Add(time.Duration(v) * time.Second)
	}

	if scope, ok := resp["scope"].(string); ok && scope != "" {
		tokenSet.Scopes = strings.Split(scope, " ")
	}

	return tokenSet
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
