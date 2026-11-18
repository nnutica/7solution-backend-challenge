// Package app contains the use cases and the ports (interfaces) they depend on.
package app

import (
	"context"
	"time"

	"github.com/nnutica/7solution-backend-challenge/internal/domain"
)

// UserRepository persists users. Implementations return domain errors
// (ErrNotFound, ErrEmailTaken, ErrInvalidID) so callers stay storage-agnostic.
type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
	Update(ctx context.Context, id string, upd UserUpdate) (*domain.User, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}

// UserUpdate holds the fields to change; nil means "leave as is".
type UserUpdate struct {
	Name  *string
	Email *string
}

// PasswordHasher hashes and checks passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Matches(hash, password string) bool
}

// TokenIssuer creates access tokens for a user.
type TokenIssuer interface {
	Issue(userID string) (token string, expiresAt time.Time, err error)
}

// TokenVerifier validates an access token and returns its user ID.
type TokenVerifier interface {
	Verify(token string) (userID string, err error)
}
