package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nnutica/7solution-backend-challenge/internal/app"
	"github.com/nnutica/7solution-backend-challenge/internal/app/apptest"
	"github.com/nnutica/7solution-backend-challenge/internal/domain"
	"github.com/nnutica/7solution-backend-challenge/internal/platform/token"
)

var testSecret = []byte("test-secret-that-is-at-least-32-bytes-long")

func newAuthService(t *testing.T) (*app.AuthService, *apptest.UserRepo, *apptest.Hasher) {
	t.Helper()
	repo := apptest.NewUserRepo()
	hasher := &apptest.Hasher{}
	jwt := token.NewJWT(testSecret, time.Hour)
	auth, err := app.NewAuthService(app.NewUserService(repo, hasher), repo, hasher, jwt, jwt)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	return auth, repo, hasher
}

func TestAuthService_RegisterAndLogin(t *testing.T) {
	auth, _, _ := newAuthService(t)
	ctx := context.Background()

	u, err := auth.Register(ctx, app.CreateUserInput{Name: "Alice", Email: "alice@x.com", Password: "password1"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Email lookup must be case-insensitive.
	res, err := auth.Login(ctx, " ALICE@x.com ", "password1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if !res.ExpiresAt.After(time.Now()) {
		t.Errorf("expiry %v should be in the future", res.ExpiresAt)
	}

	userID, err := auth.VerifyToken(res.Token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if userID != u.ID {
		t.Errorf("token subject = %q, want %q", userID, u.ID)
	}
}

func TestAuthService_LoginFailures(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
	}{
		{"wrong password", "alice@x.com", "wrong-password"},
		{"unknown email", "nobody@x.com", "password1"},
		{"malformed email", "not-an-email", "password1"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, _, _ := newAuthService(t)
			if _, err := auth.Register(context.Background(), app.CreateUserInput{Name: "A", Email: "alice@x.com", Password: "password1"}); err != nil {
				t.Fatal(err)
			}
			_, err := auth.Login(context.Background(), tt.email, tt.password)
			if !errors.Is(err, domain.ErrInvalidCredentials) {
				t.Fatalf("got %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestAuthService_LoginUnknownEmailStillComparesHash(t *testing.T) {
	auth, _, hasher := newAuthService(t)
	before := hasher.MatchCalls()

	_, _ = auth.Login(context.Background(), "nobody@x.com", "password1")

	if hasher.MatchCalls() != before+1 {
		t.Errorf("unknown email must still run one hash comparison to keep timing equal")
	}
}

func TestAuthService_LoginRepositoryError(t *testing.T) {
	auth, repo, _ := newAuthService(t)
	repo.Err = errors.New("db down")

	_, err := auth.Login(context.Background(), "alice@x.com", "password1")
	if !errors.Is(err, repo.Err) {
		t.Fatalf("got %v, want the repository error (not ErrInvalidCredentials)", err)
	}
}

func TestAuthService_VerifyInvalidToken(t *testing.T) {
	auth, _, _ := newAuthService(t)
	if _, err := auth.VerifyToken("not-a-token"); !errors.Is(err, domain.ErrInvalidToken) {
		t.Errorf("got %v, want ErrInvalidToken", err)
	}
}

func TestNewAuthService_HashFailure(t *testing.T) {
	repo := apptest.NewUserRepo()
	hasher := &apptest.Hasher{HashErr: errors.New("boom")}
	jwt := token.NewJWT(testSecret, time.Hour)
	if _, err := app.NewAuthService(app.NewUserService(repo, hasher), repo, hasher, jwt, jwt); err == nil {
		t.Fatal("want error when dummy hash cannot be created")
	}
}
