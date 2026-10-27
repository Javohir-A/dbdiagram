// Package jwt signs and verifies the short-lived access tokens.
package jwt

import (
	"errors"
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// Claims is all an access token carries: who the caller is. Authorization is
// always looked up per request, never trusted from the token.
type Claims struct {
	// Family is the login (refresh-token family) the token was issued for, so
	// "change password" can end every other login but keep this one.
	Family string `json:"fam,omitempty"`
	gojwt.RegisteredClaims
}

// UserID is the subject: the user's guid.
func (c *Claims) UserID() string { return c.Subject }

var ErrInvalid = errors.New("invalid token")

// Sign issues an HS256 token for userGuid that expires after ttl.
func Sign(secret, userGuid, family string, ttl time.Duration, now time.Time) (string, error) {
	claims := Claims{Family: family, RegisteredClaims: gojwt.RegisteredClaims{
		Subject:   userGuid,
		IssuedAt:  gojwt.NewNumericDate(now),
		ExpiresAt: gojwt.NewNumericDate(now.Add(ttl)),
	}}

	signed, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signed, nil
}

// Parse verifies the signature and the expiry. Only HS256 is accepted, so a
// token cannot downgrade the algorithm (for example to "none").
func Parse(secret, token string, now time.Time) (*Claims, error) {
	claims := &Claims{}

	_, err := gojwt.ParseWithClaims(token, claims, func(*gojwt.Token) (any, error) {
		return []byte(secret), nil
	},
		gojwt.WithValidMethods([]string{gojwt.SigningMethodHS256.Alg()}),
		gojwt.WithTimeFunc(func() time.Time { return now }),
		gojwt.WithExpirationRequired(),
	)
	if err != nil || claims.Subject == "" {
		return nil, ErrInvalid
	}

	return claims, nil
}
