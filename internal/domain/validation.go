package domain

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

// Input limits.
const (
	MaxNameLength     = 100
	MaxEmailLength    = 254 // RFC 5321 path limit
	MinPasswordLength = 8
	MaxPasswordLength = 72 // bcrypt ignores bytes beyond 72
)

// NormalizeName trims the name and checks it is present and not too long.
func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", &ValidationError{Field: "name", Message: "is required"}
	}
	if utf8.RuneCountInString(name) > MaxNameLength {
		return "", &ValidationError{Field: "name", Message: "must be at most 100 characters"}
	}
	return name, nil
}

// NormalizeEmail trims and lowercases the email so uniqueness is case-insensitive,
// then checks it is a bare address.
func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", &ValidationError{Field: "email", Message: "is required"}
	}
	if len(email) > MaxEmailLength {
		return "", &ValidationError{Field: "email", Message: "is too long"}
	}
	// ParseAddress also accepts forms like "Bob <bob@x.com>"; requiring an exact match rejects them.
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", &ValidationError{Field: "email", Message: "is not a valid email address"}
	}
	return email, nil
}

// ValidatePassword checks length in bytes. Passwords are not trimmed: spaces are allowed.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return &ValidationError{Field: "password", Message: "must be at least 8 characters"}
	}
	if len(password) > MaxPasswordLength {
		return &ValidationError{Field: "password", Message: "must be at most 72 bytes"}
	}
	return nil
}
