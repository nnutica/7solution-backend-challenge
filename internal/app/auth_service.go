package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nnutica/7solution-backend-challenge/internal/domain"
)

// LoginResult is returned after a successful login.
type LoginResult struct {
	Token     string
	ExpiresAt time.Time
}

// AuthService handles registration, login and token verification.
type AuthService struct {
	users    *UserService
	repo     UserRepository
	hasher   PasswordHasher
	issuer   TokenIssuer
	verifier TokenVerifier
	// dummyHash is compared against when the email is unknown, so both failure paths cost one bcrypt check.
	dummyHash string
}

// NewAuthService returns an AuthService.
func NewAuthService(users *UserService, repo UserRepository, hasher PasswordHasher, issuer TokenIssuer, verifier TokenVerifier) (*AuthService, error) {
	dummy, err := hasher.Hash("timing-equaliser-not-a-real-password")
	if err != nil {
		return nil, fmt.Errorf("create dummy hash: %w", err)
	}
	return &AuthService{
		users:     users,
		repo:      repo,
		hasher:    hasher,
		issuer:    issuer,
		verifier:  verifier,
		dummyHash: dummy,
	}, nil
}

// Register creates a new user account.
func (a *AuthService) Register(ctx context.Context, in CreateUserInput) (*domain.User, error) {
	return a.users.Create(ctx, in)
}

// Login checks the credentials and returns a signed token.
// Unknown email and wrong password return the same error so accounts cannot be enumerated.
func (a *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	email, err := domain.NormalizeEmail(email)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	u, err := a.repo.GetByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		a.hasher.Matches(a.dummyHash, password)
		return nil, domain.ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !a.hasher.Matches(u.PasswordHash, password) {
		return nil, domain.ErrInvalidCredentials
	}

	token, expiresAt, err := a.issuer.Issue(u.ID)
	if err != nil {
		return nil, fmt.Errorf("issue token: %w", err)
	}
	return &LoginResult{Token: token, ExpiresAt: expiresAt}, nil
}

// VerifyToken returns the user ID of a valid token.
func (a *AuthService) VerifyToken(token string) (string, error) {
	return a.verifier.Verify(token)
}
