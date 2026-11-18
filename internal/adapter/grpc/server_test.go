package grpcadapter_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	grpcadapter "github.com/nnutica/7solution-backend-challenge/internal/adapter/grpc"
	"github.com/nnutica/7solution-backend-challenge/internal/app"
	"github.com/nnutica/7solution-backend-challenge/internal/app/apptest"
	"github.com/nnutica/7solution-backend-challenge/internal/platform/token"
	"github.com/nnutica/7solution-backend-challenge/proto/userpb"
)

var secret = []byte("test-secret-that-is-at-least-32-bytes-long")

type testEnv struct {
	client userpb.UserServiceClient
	repo   *apptest.UserRepo
	token  string
}

// newTestEnv runs the gRPC server over an in-memory listener.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	repo := apptest.NewUserRepo()
	hasher := &apptest.Hasher{}
	jwt := token.NewJWT(secret, time.Hour)
	users := app.NewUserService(repo, hasher)
	auth, err := app.NewAuthService(users, repo, hasher, jwt, jwt)
	if err != nil {
		t.Fatal(err)
	}

	lis := bufconn.Listen(1 << 20)
	srv := grpcadapter.NewServer(users, auth, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	tok, _, err := jwt.Issue("000000000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	return &testEnv{client: userpb.NewUserServiceClient(conn), repo: repo, token: tok}
}

func withAuth(value string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", value)
}

func assertCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code = %v, want %v (err %v)", got, want, err)
	}
}

func TestCreateAndGetUser(t *testing.T) {
	env := newTestEnv(t)
	ctx := withAuth("Bearer " + env.token)

	created, err := env.client.CreateUser(ctx, &userpb.CreateUserRequest{Name: "Carol", Email: "Carol@X.com", Password: "password1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	u := created.GetUser()
	if u.GetId() == "" || u.GetEmail() != "carol@x.com" || u.GetCreatedAt() == nil {
		t.Errorf("unexpected user: %v", u)
	}

	got, err := env.client.GetUser(ctx, &userpb.GetUserRequest{Id: u.GetId()})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetUser().GetName() != "Carol" {
		t.Errorf("unexpected user: %v", got.GetUser())
	}
}

func TestErrorCodes(t *testing.T) {
	env := newTestEnv(t)
	ctx := withAuth("Bearer " + env.token)
	if _, err := env.client.CreateUser(ctx, &userpb.CreateUserRequest{Name: "A", Email: "a@x.com", Password: "password1"}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		call func() error
		want codes.Code
	}{
		{"duplicate email", func() error {
			_, err := env.client.CreateUser(ctx, &userpb.CreateUserRequest{Name: "B", Email: "A@x.com", Password: "password1"})
			return err
		}, codes.AlreadyExists},
		{"invalid input", func() error {
			_, err := env.client.CreateUser(ctx, &userpb.CreateUserRequest{Name: "", Email: "b@x.com", Password: "password1"})
			return err
		}, codes.InvalidArgument},
		{"invalid id", func() error {
			_, err := env.client.GetUser(ctx, &userpb.GetUserRequest{Id: "123"})
			return err
		}, codes.InvalidArgument},
		{"not found", func() error {
			_, err := env.client.GetUser(ctx, &userpb.GetUserRequest{Id: "ffffffffffffffffffffffff"})
			return err
		}, codes.NotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assertCode(t, tt.call(), tt.want) })
	}
}

func TestInternalErrorIsHidden(t *testing.T) {
	env := newTestEnv(t)
	env.repo.Err = errors.New("connection refused to 10.0.0.5")

	_, err := env.client.GetUser(withAuth("Bearer "+env.token), &userpb.GetUserRequest{Id: "000000000000000000000001"})
	assertCode(t, err, codes.Internal)
	if msg := status.Convert(err).Message(); msg != "internal server error" {
		t.Errorf("message = %q, want generic message", msg)
	}
}

func TestAuthInterceptor(t *testing.T) {
	env := newTestEnv(t)
	expired, _, _ := token.NewJWT(secret, -time.Minute).Issue("user-1")

	tests := []struct {
		name string
		ctx  context.Context
		want codes.Code
	}{
		{"valid token", withAuth("Bearer " + env.token), codes.NotFound},
		{"no metadata", context.Background(), codes.Unauthenticated},
		{"wrong scheme", withAuth("Basic " + env.token), codes.Unauthenticated},
		{"no scheme", withAuth(env.token), codes.Unauthenticated},
		{"garbage token", withAuth("Bearer abc"), codes.Unauthenticated},
		{"expired token", withAuth("Bearer " + expired), codes.Unauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A valid token reaches the handler, which returns NotFound for this unknown ID.
			_, err := env.client.GetUser(tt.ctx, &userpb.GetUserRequest{Id: "ffffffffffffffffffffffff"})
			assertCode(t, err, tt.want)
		})
	}
}
