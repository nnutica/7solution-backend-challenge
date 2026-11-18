package token

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/nnutica/7solution-backend-challenge/internal/domain"
)

var secret = []byte("test-secret-that-is-at-least-32-bytes-long")

func sign(t *testing.T, method jwt.SigningMethod, key any, claims jwt.Claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func validClaims() jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Subject:   "user-1",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
}

func TestIssueAndVerify(t *testing.T) {
	j := NewJWT(secret, time.Hour)

	tok, expiresAt, err := j.Issue("user-1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if got := time.Until(expiresAt); got < 59*time.Minute || got > time.Hour {
		t.Errorf("expiresAt %v not about one hour from now", expiresAt)
	}
	if expiresAt.Nanosecond() != 0 {
		t.Errorf("expiresAt should be whole seconds to match the exp claim, got %v", expiresAt)
	}

	sub, err := j.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if sub != "user-1" {
		t.Errorf("subject = %q, want user-1", sub)
	}
}

func TestVerifyRejects(t *testing.T) {
	noExp := validClaims()
	noExp.ExpiresAt = nil
	noSub := validClaims()
	noSub.Subject = ""

	issued, _, _ := NewJWT(secret, time.Hour).Issue("user-1")

	tests := []struct {
		name  string
		token string
	}{
		{"garbage", "not.a.token"},
		{"empty", ""},
		{"wrong secret", sign(t, jwt.SigningMethodHS256, []byte("another-secret-that-is-32-bytes-long!"), validClaims())},
		{"HS512 with same secret", sign(t, jwt.SigningMethodHS512, secret, validClaims())},
		{"alg none", sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, validClaims())},
		{"missing exp", sign(t, jwt.SigningMethodHS256, secret, noExp)},
		{"missing subject", sign(t, jwt.SigningMethodHS256, secret, noSub)},
		{"tampered signature", issued[:len(issued)-2] + "xx"},
	}
	j := NewJWT(secret, time.Hour)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := j.Verify(tt.token); !errors.Is(err, domain.ErrInvalidToken) {
				t.Errorf("got %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestVerifyExpired(t *testing.T) {
	j := NewJWT(secret, time.Minute)
	issuedAt := time.Now().Add(-2 * time.Minute)
	j.now = func() time.Time { return issuedAt }
	tok, _, err := j.Issue("user-1")
	if err != nil {
		t.Fatal(err)
	}

	j.now = time.Now
	if _, err := j.Verify(tok); !errors.Is(err, domain.ErrInvalidToken) {
		t.Errorf("got %v, want ErrInvalidToken for expired token", err)
	}
}
