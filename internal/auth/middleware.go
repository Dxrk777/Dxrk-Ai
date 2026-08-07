package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware returns a Gin middleware that validates Bearer tokens from
// the Authorization header. It checks both OAuth tokens and API keys stored
// in the given SecureStorage.
func AuthMiddleware(storage SecureStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				strconst.StrError: "missing Authorization header",
			})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				strconst.StrError: "invalid Authorization header format, expected: Bearer <token>",
			})
			return
		}

		token := parts[1]

		// Check if it's an API key
		if strings.HasPrefix(token, KeyPrefix) {
			apiKeyManager, err := NewAPIKeyManager(storage)
			if err == nil {
				apiKey, err := apiKeyManager.Validate(token)
				if err == nil {
					_ = apiKeyManager.RecordUsage(apiKey.Name, apiKey.Provider)
					c.Set("auth_type", "api_key")
					c.Set("auth_provider", apiKey.Provider)
					c.Set("auth_key_name", apiKey.Name)
					c.Next()
					return
				}
			}
		}

		// Check stored OAuth tokens
		data, err := storage.Load("oauth_token_github")
		if err == nil {
			var tokenSet TokenSet
			if json.Unmarshal(data, &tokenSet) == nil {
				if tokenSet.AccessToken == token && !tokenSet.IsExpired() {
					c.Set("auth_type", "oauth")
					c.Set("auth_provider", tokenSet.Provider)
					c.Set("auth_token", tokenSet.AccessToken)
					c.Next()
					return
				}
			}
		}

		// Try all oauth_token_* keys for other providers
		keys, err := storage.List()
		if err == nil {
			for _, key := range keys {
				if !strings.HasPrefix(key, "oauth_token_") {
					continue
				}
				data, err := storage.Load(key)
				if err != nil {
					continue
				}
				var tokenSet TokenSet
				if json.Unmarshal(data, &tokenSet) != nil {
					continue
				}
				if tokenSet.AccessToken == token && !tokenSet.IsExpired() {
					c.Set("auth_type", "oauth")
					c.Set("auth_provider", tokenSet.Provider)
					c.Set("auth_token", tokenSet.AccessToken)
					c.Next()
					return
				}
			}
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			strconst.StrError: "invalid or expired token",
		})
	}
}

// OAuthCallbackHandler returns a Gin handler that processes OAuth callbacks.
// It validates the state parameter, exchanges the code for tokens, and
// redirects to the success URL or returns an error.
func OAuthCallbackHandler(manager *OAuthManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Query("code")
		state := c.Query("state")
		errParam := c.Query(strconst.StrError)

		if errParam != "" {
			desc := c.Query("error_description")
			c.JSON(http.StatusBadRequest, gin.H{
				strconst.StrError:       errParam,
				strconst.StrDescription: desc,
			})
			return
		}

		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				strconst.StrError: "missing authorization code",
			})
			return
		}

		if state == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				strconst.StrError: "missing state parameter",
			})
			return
		}

		tokenSet, err := manager.HandleCallback(code, state)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				strconst.StrError: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":    "authentication successful",
			"provider":   tokenSet.Provider,
			"expires_at": tokenSet.ExpiresAt,
			"scopes":     tokenSet.Scopes,
		})
	}
}

// RequireAuth returns a Gin handler that checks for authentication context
// set by AuthMiddleware. Use this on routes that require authentication.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authType, exists := c.Get("auth_type")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				strconst.StrError: "authentication required",
			})
			return
		}

		_ = authType
		c.Next()
	}
}

// GetAuthInfo extracts authentication info from the Gin context.
func GetAuthInfo(c *gin.Context) (authType, provider string, ok bool) {
	t, tOk := c.Get("auth_type")
	p, pOk := c.Get("auth_provider")
	if !tOk || !pOk {
		return "", "", false
	}
	return t.(string), p.(string), true
}
