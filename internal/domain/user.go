// Package domain holds the core user entity, its rules and errors.
// It must not import frameworks or database drivers.
package domain

import "time"

// User is a registered account. PasswordHash is a bcrypt hash, never plain text.
type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}
