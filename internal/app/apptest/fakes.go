// Package apptest provides in-memory fakes of the app ports for tests.
package apptest

import (
	"context"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/nnutica/7solution-backend-challenge/internal/app"
	"github.com/nnutica/7solution-backend-challenge/internal/domain"
)

// UserRepo is an in-memory app.UserRepository that mimics the Mongo adapter's errors.
type UserRepo struct {
	mu     sync.Mutex
	users  map[string]domain.User
	nextID int
	// Err, when set, is returned by every method to simulate a database failure.
	Err error
}

var _ app.UserRepository = (*UserRepo)(nil)

// NewUserRepo returns an empty repository.
func NewUserRepo() *UserRepo {
	return &UserRepo{users: map[string]domain.User{}}
}

// IDs look like ObjectIDs (24 hex chars) so invalid-ID handling matches production.
func validID(id string) bool {
	_, err := hex.DecodeString(id)
	return len(id) == 24 && err == nil
}

func (r *UserRepo) emailTaken(email, exceptID string) bool {
	for id, u := range r.users {
		if u.Email == email && id != exceptID {
			return true
		}
	}
	return false
}

// Create stores a copy of u and assigns its ID.
func (r *UserRepo) Create(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	if r.emailTaken(u.Email, "") {
		return domain.ErrEmailTaken
	}
	r.nextID++
	u.ID = fmt.Sprintf("%024x", r.nextID)
	r.users[u.ID] = *u
	return nil
}

// GetByID returns a copy of the user.
func (r *UserRepo) GetByID(_ context.Context, id string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	if !validID(id) {
		return nil, domain.ErrInvalidID
	}
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &u, nil
}

// GetByEmail returns a copy of the user with that email.
func (r *UserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	for _, u := range r.users {
		if u.Email == email {
			return &u, nil
		}
	}
	return nil, domain.ErrNotFound
}

// List returns all users in creation order.
func (r *UserRepo) List(_ context.Context) ([]domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	list := make([]domain.User, 0, len(r.users))
	for _, u := range r.users {
		list = append(list, u)
	}
	// IDs are zero-padded counters, so string order is creation order.
	slices.SortFunc(list, func(a, b domain.User) int { return strings.Compare(a.ID, b.ID) })
	return list, nil
}

// Update applies the non-nil fields.
func (r *UserRepo) Update(_ context.Context, id string, upd app.UserUpdate) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	if !validID(id) {
		return nil, domain.ErrInvalidID
	}
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if upd.Email != nil && r.emailTaken(*upd.Email, id) {
		return nil, domain.ErrEmailTaken
	}
	if upd.Name != nil {
		u.Name = *upd.Name
	}
	if upd.Email != nil {
		u.Email = *upd.Email
	}
	r.users[id] = u
	return &u, nil
}

// Delete removes the user.
func (r *UserRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	if !validID(id) {
		return domain.ErrInvalidID
	}
	if _, ok := r.users[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.users, id)
	return nil
}

// Count returns the number of users.
func (r *UserRepo) Count(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return 0, r.Err
	}
	return int64(len(r.users)), nil
}

// Hasher is a fast, insecure app.PasswordHasher for tests.
type Hasher struct {
	mu         sync.Mutex
	matchCalls int
	// HashErr, when set, is returned by Hash.
	HashErr error
}

var _ app.PasswordHasher = (*Hasher)(nil)

// Hash prefixes the password so tests can tell it was hashed.
func (h *Hasher) Hash(password string) (string, error) {
	if h.HashErr != nil {
		return "", h.HashErr
	}
	return "hashed:" + password, nil
}

// Matches compares against the prefixed form and counts the call.
func (h *Hasher) Matches(hash, password string) bool {
	h.mu.Lock()
	h.matchCalls++
	h.mu.Unlock()
	return hash == "hashed:"+password
}

// MatchCalls reports how many times Matches was called.
func (h *Hasher) MatchCalls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.matchCalls
}
