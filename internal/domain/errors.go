package domain

import "errors"

// Sentinel errors that adapters map to transport status codes.
var (
	ErrNotFound           = errors.New("user not found")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidID          = errors.New("invalid user id")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

// ValidationError reports bad input for a single field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + " " + e.Message
}
