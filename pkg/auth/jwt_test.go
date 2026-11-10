package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
)

const secret = "test-secret-test-secret-test-secret"

func TestGenerateAndParse(t *testing.T) {
	tm := auth.NewTokenManager(secret, "iss", time.Hour)
	tok, exp, err := tm.Generate("user-1", "a@x.com")
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(time.Hour), exp, 5*time.Second)

	claims, err := tm.Parse(tok)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.Subject)
	assert.Equal(t, "a@x.com", claims.Email)
	assert.NotEmpty(t, claims.ID)
}

func TestParseRejects(t *testing.T) {
	good := auth.NewTokenManager(secret, "iss", time.Hour)

	expired, _, err := auth.NewTokenManager(secret, "iss", -time.Minute).Generate("u", "e")
	require.NoError(t, err)
	otherSecret, _, err := auth.NewTokenManager("another-secret-another-secret", "iss", time.Hour).Generate("u", "e")
	require.NoError(t, err)
	otherIssuer, _, err := auth.NewTokenManager(secret, "evil", time.Hour).Generate("u", "e")
	require.NoError(t, err)

	// "alg: none" token: unsigned, must never be accepted.
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Subject: "u", Issuer: "iss", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	for name, tok := range map[string]string{
		"expired": expired, "other secret": otherSecret, "other issuer": otherIssuer,
		"alg none": none, "garbage": "not.a.jwt",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := good.Parse(tok)
			assert.Error(t, err)
		})
	}
}
