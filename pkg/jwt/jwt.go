package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenMalformed        = errors.New("token is malformed")
	ErrTokenExpired          = errors.New("token is expired")
	ErrTokenNotValidYet      = errors.New("token is not yet valid")
	ErrTokenInvalid          = errors.New("token is invalid")
	ErrTokenSignatureInvalid = errors.New("token signature is invalid")
)

// --- 核心結構 ---

// Manager is a JWT token generator and parser.
type Manager struct {
	signer  Signer
	issuer  string
	keyFunc jwt.Keyfunc
}

// Claims represents the JWT claims, embedding standard claims and allowing for a custom payload.
type Claims struct {
	jwt.RegisteredClaims
	Payload map[string]interface{} `json:"payload"`
}

// Signer defines the interface for signing JWT claims.
type Signer interface {
	Sign(claims jwt.Claims) (string, error)
}

// --- 主要方法 ---

// Option defines a function that can modify JWT claims.
type Option func(*Claims)

// WithExpiresAt sets a specific expiration time for the token.
func WithExpiresAt(t time.Time) Option {
	return func(c *Claims) {
		c.ExpiresAt = jwt.NewNumericDate(t)
	}
}

// WithNotBefore sets a specific not-before time for the token.
func WithNotBefore(t time.Time) Option {
	return func(c *Claims) {
		c.NotBefore = jwt.NewNumericDate(t)
	}
}

// Generate generates a new JWT token with the given payload and options.
func (g *Manager) Generate(payload map[string]interface{}, opts ...Option) (string, error) {
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   g.issuer,
			IssuedAt: jwt.NewNumericDate(now),
		},
		Payload: payload,
	}

	// Apply all options
	for _, opt := range opts {
		opt(claims)
	}

	return g.signer.Sign(claims)
}

// Parse validates the token and returns the claims.
func (g *Manager) Parse(tokenString string) (map[string]interface{}, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, g.keyFunc)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, ErrTokenMalformed
		} else if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		} else if errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, ErrTokenNotValidYet
		} else if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, ErrTokenSignatureInvalid
		}
		return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims.Payload, nil
	}

	return nil, ErrTokenInvalid
}
