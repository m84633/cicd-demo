package jwt

import (
	"crypto/rsa"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
)

// asymmetricSigner implements the Signer interface for asymmetric algorithms like RS256.
type asymmetricSigner struct {
	privateKey *rsa.PrivateKey
}

// Sign signs the claims using the RS256 algorithm.
func (s *asymmetricSigner) Sign(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.privateKey)
}

// NewAsymmetric creates a new Manager that uses an asymmetric key pair (e.g., RS256).
func NewAsymmetric(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey, issuer string) (*Manager, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("private key cannot be nil")
	}
	if publicKey == nil {
		return nil, fmt.Errorf("public key cannot be nil")
	}

	keyFunc := func(token *jwt.Token) (interface{}, error) {
		// Check the signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	}

	return &Manager{
		signer:  &asymmetricSigner{privateKey: privateKey},
		issuer:  issuer,
		keyFunc: keyFunc,
	}, nil
}
