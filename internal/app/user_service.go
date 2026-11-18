package app

import (
	"context"
	"fmt"
	"time"

	"github.com/nnutica/7solution-backend-challenge/internal/domain"
)

// CreateUserInput is the raw input for creating a user.
type CreateUserInput struct {
	Name     string
	Email    string
	Password string
}

// UpdateUserInput is the raw input for updating a user; nil fields are left unchanged.
type UpdateUserInput struct {
	Name  *string
	Email *string
}

// UserService handles user management use cases.
type UserService struct {
	repo   UserRepository
	hasher PasswordHasher
	now    func() time.Time
}

// NewUserService returns a UserService.
func NewUserService(repo UserRepository, hasher PasswordHasher) *UserService {
	return &UserService{repo: repo, hasher: hasher, now: time.Now}
}

// Create validates the input, hashes the password and stores the user.
func (s *UserService) Create(ctx context.Context, in CreateUserInput) (*domain.User, error) {
	name, err := domain.NormalizeName(in.Name)
	if err != nil {
		return nil, err
	}
	email, err := domain.NormalizeEmail(in.Email)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePassword(in.Password); err != nil {
		return nil, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := &domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		// MongoDB stores milliseconds; truncate so the response matches what is saved.
		CreatedAt: s.now().UTC().Truncate(time.Millisecond),
	}
	// Uniqueness is enforced by the database index, not a prior lookup, to avoid a race.
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Get returns the user with the given ID.
func (s *UserService) Get(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns all users.
func (s *UserService) List(ctx context.Context) ([]domain.User, error) {
	return s.repo.List(ctx)
}

// Update changes a user's name and/or email.
func (s *UserService) Update(ctx context.Context, id string, in UpdateUserInput) (*domain.User, error) {
	if in.Name == nil && in.Email == nil {
		return nil, &domain.ValidationError{Field: "request", Message: "must include name or email"}
	}

	var upd UserUpdate
	if in.Name != nil {
		name, err := domain.NormalizeName(*in.Name)
		if err != nil {
			return nil, err
		}
		upd.Name = &name
	}
	if in.Email != nil {
		email, err := domain.NormalizeEmail(*in.Email)
		if err != nil {
			return nil, err
		}
		upd.Email = &email
	}
	return s.repo.Update(ctx, id, upd)
}

// Delete removes the user with the given ID.
func (s *UserService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// Count returns the total number of users.
func (s *UserService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
