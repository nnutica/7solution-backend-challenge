package password

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestBcryptHasher(t *testing.T) {
	// MinCost keeps the test fast; production uses DefaultCost.
	h := NewBcryptHasher(bcrypt.MinCost)

	hash, err := h.Hash("password1")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "password1" || !strings.HasPrefix(hash, "$2") {
		t.Errorf("not a bcrypt hash: %q", hash)
	}

	tests := []struct {
		name     string
		hash     string
		password string
		want     bool
	}{
		{"correct password", hash, "password1", true},
		{"wrong password", hash, "password2", false},
		{"empty password", hash, "", false},
		{"malformed hash", "not-a-hash", "password1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := h.Matches(tt.hash, tt.password); got != tt.want {
				t.Errorf("Matches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBcryptHasher_Salted(t *testing.T) {
	h := NewBcryptHasher(bcrypt.MinCost)
	a, _ := h.Hash("same")
	b, _ := h.Hash("same")
	if a == b {
		t.Error("two hashes of the same password must differ (random salt)")
	}
}

func TestBcryptHasher_TooLong(t *testing.T) {
	h := NewBcryptHasher(bcrypt.MinCost)
	if _, err := h.Hash(strings.Repeat("a", 73)); err == nil {
		t.Error("want error for passwords over 72 bytes")
	}
}
