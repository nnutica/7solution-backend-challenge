package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nnutica/7solution-backend-challenge/internal/app"
	"github.com/nnutica/7solution-backend-challenge/internal/app/apptest"
	"github.com/nnutica/7solution-backend-challenge/internal/domain"
)

func ptr(s string) *string { return &s }

func newUserService(t *testing.T) (*app.UserService, *apptest.UserRepo, *apptest.Hasher) {
	t.Helper()
	repo := apptest.NewUserRepo()
	hasher := &apptest.Hasher{}
	return app.NewUserService(repo, hasher), repo, hasher
}

func mustCreate(t *testing.T, svc *app.UserService, name, email string) *domain.User {
	t.Helper()
	u, err := svc.Create(context.Background(), app.CreateUserInput{Name: name, Email: email, Password: "password1"})
	if err != nil {
		t.Fatalf("create %s: %v", email, err)
	}
	return u
}

func assertValidationErr(t *testing.T, err error, field string) {
	t.Helper()
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %v", err)
	}
	if ve.Field != field {
		t.Errorf("field = %q, want %q", ve.Field, field)
	}
}

func TestUserService_Create(t *testing.T) {
	svc, _, _ := newUserService(t)
	ctx := context.Background()

	u, err := svc.Create(ctx, app.CreateUserInput{Name: " Alice ", Email: " Alice@Example.COM ", Password: "password1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID == "" || u.CreatedAt.IsZero() {
		t.Errorf("ID and CreatedAt must be set, got %+v", u)
	}
	if u.Name != "Alice" || u.Email != "alice@example.com" {
		t.Errorf("input not normalised: %+v", u)
	}
	if u.PasswordHash != "hashed:password1" {
		t.Errorf("password not hashed: %q", u.PasswordHash)
	}
	if u.CreatedAt.Location().String() != "UTC" {
		t.Errorf("CreatedAt should be UTC, got %v", u.CreatedAt.Location())
	}
}

func TestUserService_CreateErrors(t *testing.T) {
	tests := []struct {
		name      string
		in        app.CreateUserInput
		wantField string
		wantErr   error
	}{
		{"missing name", app.CreateUserInput{Email: "b@x.com", Password: "password1"}, "name", nil},
		{"bad email", app.CreateUserInput{Name: "B", Email: "nope", Password: "password1"}, "email", nil},
		{"short password", app.CreateUserInput{Name: "B", Email: "b@x.com", Password: "short"}, "password", nil},
		{"duplicate email, different case", app.CreateUserInput{Name: "B", Email: "ALICE@x.com", Password: "password1"}, "", domain.ErrEmailTaken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newUserService(t)
			mustCreate(t, svc, "Alice", "alice@x.com")

			_, err := svc.Create(context.Background(), tt.in)
			if tt.wantField != "" {
				assertValidationErr(t, err, tt.wantField)
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserService_CreateHashFailure(t *testing.T) {
	svc, repo, hasher := newUserService(t)
	hasher.HashErr = errors.New("boom")

	_, err := svc.Create(context.Background(), app.CreateUserInput{Name: "A", Email: "a@x.com", Password: "password1"})
	if !errors.Is(err, hasher.HashErr) {
		t.Fatalf("got %v, want wrapped hash error", err)
	}
	if n, _ := repo.Count(context.Background()); n != 0 {
		t.Errorf("user must not be stored when hashing fails, count = %d", n)
	}
}

func TestUserService_Get(t *testing.T) {
	svc, _, _ := newUserService(t)
	alice := mustCreate(t, svc, "Alice", "alice@x.com")

	tests := []struct {
		name    string
		id      string
		wantErr error
	}{
		{"found", alice.ID, nil},
		{"not found", "ffffffffffffffffffffffff", domain.ErrNotFound},
		{"invalid id", "123", domain.ErrInvalidID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := svc.Get(context.Background(), tt.id)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
			if err == nil && u.Email != alice.Email {
				t.Errorf("got %+v", u)
			}
		})
	}
}

func TestUserService_Update(t *testing.T) {
	tests := []struct {
		name      string
		id        string // empty means Alice's ID
		in        app.UpdateUserInput
		wantName  string
		wantEmail string
		wantField string
		wantErr   error
	}{
		{name: "name only", in: app.UpdateUserInput{Name: ptr(" Alicia ")}, wantName: "Alicia", wantEmail: "alice@x.com"},
		{name: "email only", in: app.UpdateUserInput{Email: ptr("NEW@x.com")}, wantName: "Alice", wantEmail: "new@x.com"},
		{name: "both", in: app.UpdateUserInput{Name: ptr("A"), Email: ptr("a2@x.com")}, wantName: "A", wantEmail: "a2@x.com"},
		{name: "same email is not a conflict", in: app.UpdateUserInput{Email: ptr("alice@x.com")}, wantName: "Alice", wantEmail: "alice@x.com"},
		{name: "no fields", in: app.UpdateUserInput{}, wantField: "request"},
		{name: "empty name", in: app.UpdateUserInput{Name: ptr("  ")}, wantField: "name"},
		{name: "invalid email", in: app.UpdateUserInput{Email: ptr("bad")}, wantField: "email"},
		{name: "email taken", in: app.UpdateUserInput{Email: ptr("Bob@x.com")}, wantErr: domain.ErrEmailTaken},
		{name: "not found", id: "ffffffffffffffffffffffff", in: app.UpdateUserInput{Name: ptr("X")}, wantErr: domain.ErrNotFound},
		{name: "invalid id", id: "xyz", in: app.UpdateUserInput{Name: ptr("X")}, wantErr: domain.ErrInvalidID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newUserService(t)
			alice := mustCreate(t, svc, "Alice", "alice@x.com")
			mustCreate(t, svc, "Bob", "bob@x.com")
			id := tt.id
			if id == "" {
				id = alice.ID
			}

			u, err := svc.Update(context.Background(), id, tt.in)
			switch {
			case tt.wantField != "":
				assertValidationErr(t, err, tt.wantField)
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got %v, want %v", err, tt.wantErr)
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if u.Name != tt.wantName || u.Email != tt.wantEmail {
					t.Errorf("got name=%q email=%q, want %q %q", u.Name, u.Email, tt.wantName, tt.wantEmail)
				}
			}
		})
	}
}

func TestUserService_ListCountDelete(t *testing.T) {
	svc, _, _ := newUserService(t)
	ctx := context.Background()
	alice := mustCreate(t, svc, "Alice", "alice@x.com")
	mustCreate(t, svc, "Bob", "bob@x.com")

	list, err := svc.List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %v, %v; want 2 users", list, err)
	}
	if list[0].Email != "alice@x.com" || list[1].Email != "bob@x.com" {
		t.Errorf("unexpected order: %v", list)
	}

	if err := svc.Delete(ctx, alice.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := svc.Delete(ctx, alice.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("second delete: got %v, want ErrNotFound", err)
	}
	if err := svc.Delete(ctx, "bad"); !errors.Is(err, domain.ErrInvalidID) {
		t.Errorf("delete bad id: got %v, want ErrInvalidID", err)
	}

	if n, err := svc.Count(ctx); err != nil || n != 1 {
		t.Errorf("count = %d, %v; want 1", n, err)
	}
}

func TestUserService_RepositoryErrorPropagates(t *testing.T) {
	svc, repo, _ := newUserService(t)
	repo.Err = errors.New("db down")
	if _, err := svc.List(context.Background()); !errors.Is(err, repo.Err) {
		t.Errorf("got %v, want repository error", err)
	}
}
