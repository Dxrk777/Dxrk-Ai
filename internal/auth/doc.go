// Package auth provides OAuth 2.0 authentication, secure token storage,
// API key management, and HTTP middleware for the Dxrk-Ai platform.
//
// Key features:
//   - OAuth 2.0 authorization code flow with PKCE
//   - Multi-provider support (GitHub, Google, Anthropic API keys, custom)
//   - AES-256-GCM encrypted file-based token storage
//   - API key generation, validation, rotation, and expiry
//   - Gin-compatible HTTP middleware for bearer token auth
//
// Usage:
//
//	storage := auth.NewFileStorage("~/.dxrk/credentials")
//	manager := auth.NewOAuthManager(config, storage)
//	authURL, state, _ := manager.StartFlow()
//	// redirect user to authURL...
//	tokenSet, _ := manager.HandleCallback(code, state)
//
// The package avoids external keyring dependencies; instead it derives
// encryption keys from machine-specific identifiers (hostname + username)
// and stores encrypted credentials on disk.
package auth
