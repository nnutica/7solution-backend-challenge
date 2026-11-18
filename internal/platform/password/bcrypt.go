// Package password hashes and checks passwords with bcrypt.
package password

import "golang.org/x/crypto/bcrypt"

// BcryptHasher implements app.PasswordHasher.
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher returns a hasher using the given bcrypt cost.
func NewBcryptHasher(cost int) *BcryptHasher {
	return &BcryptHasher{cost: cost}
}

// Hash returns a salted bcrypt hash of the password.
func (h *BcryptHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Matches reports whether password matches hash. bcrypt compares in constant time.
func (h *BcryptHasher) Matches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
