package jwt

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// symmetricSigner implements the Signer interface for symmetric algorithms like HS256.
type symmetricSigner struct {
	secret []byte
}

// Sign signs the claims using the HS256 algorithm.
func (s *symmetricSigner) Sign(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// NewSymmetric creates a new Manager that uses a symmetric key (e.g., HS256).
func NewSymmetric(secret []byte, issuer string) (*Manager, error) {
	if len(secret) == 0 {
		return nil, fmt.Errorf("JWT secret cannot be empty")
	}

	keyFunc := func(token *jwt.Token) (interface{}, error) {
		// Check the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	}

	return &Manager{
		signer:  &symmetricSigner{secret: secret},
		issuer:  issuer,
		keyFunc: keyFunc,
	}, nil
}
