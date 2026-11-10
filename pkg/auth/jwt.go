// Package auth issues and verifies JWT access tokens.
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims are the fields stored inside our JWT.
type Claims struct {
	Email                string `json:"email"`
	jwt.RegisteredClaims        // embedded: sub, exp, iat, iss, jti
}

// TokenManager signs and parses tokens with one HMAC secret.
type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// NewTokenManager creates a TokenManager. secret must be long and random.
func NewTokenManager(secret, issuer string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), issuer: issuer, ttl: ttl}
}

// Generate returns a signed token for the user and its expiry time.
func (m *TokenManager) Generate(userID, email string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(m.ttl)
	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        uuid.NewString(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

// Parse verifies signature, algorithm, issuer and expiry, and returns the claims.
func (m *TokenManager) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), // blocks "alg: none" attacks
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	return claims, nil
}
