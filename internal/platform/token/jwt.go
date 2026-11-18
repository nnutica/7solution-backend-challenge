// Package token issues and verifies HS256 JWTs.
package token

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/nnutica/7solution-backend-challenge/internal/domain"
)

// JWT implements app.TokenIssuer and app.TokenVerifier.
type JWT struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewJWT returns a JWT issuer/verifier signing with secret.
func NewJWT(secret []byte, ttl time.Duration) *JWT {
	return &JWT{secret: secret, ttl: ttl, now: time.Now}
}

// Issue signs a token whose subject is the user ID.
func (j *JWT) Issue(userID string) (string, time.Time, error) {
	// JWT times are whole seconds; truncate so expiresAt matches the "exp" claim exactly.
	now := j.now().UTC().Truncate(time.Second)
	expiresAt := now.Add(j.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// Verify checks signature, algorithm and expiry, then returns the user ID.
func (j *JWT) Verify(tokenString string) (string, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(tokenString, &claims,
		func(*jwt.Token) (any, error) { return j.secret, nil },
		// Pinning the method blocks "alg: none" and algorithm-confusion attacks.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(j.now),
	)
	if err != nil || claims.Subject == "" {
		return "", domain.ErrInvalidToken
	}
	return claims.Subject, nil
}
