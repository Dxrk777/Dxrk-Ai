// Package auth provides JWT token utilities.
package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

// Claims represents a JWT claim set.
type Claims = jwt.RegisteredClaims

// ParseToken parses and validates a JWT token string.
func ParseToken(tokenString string, keyFunc jwt.Keyfunc) (*jwt.Token, error) {
	return jwt.Parse(tokenString, keyFunc)
}
