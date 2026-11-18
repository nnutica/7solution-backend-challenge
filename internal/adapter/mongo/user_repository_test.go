package mongoadapter_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	mongoadapter "github.com/nnutica/7solution-backend-challenge/internal/adapter/mongo"
	"github.com/nnutica/7solution-backend-challenge/internal/app"
	"github.com/nnutica/7solution-backend-challenge/internal/domain"
)

// newRepo connects to a real MongoDB in a throwaway database. It skips unless MONGO_URI is set.
func newRepo(t *testing.T) *mongoadapter.UserRepository {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI not set; skipping MongoDB integration test")
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("ping %s: %v", uri, err)
	}

	db := client.Database(fmt.Sprintf("test_users_%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
	})

	repo := mongoadapter.NewUserRepository(db, 5*time.Second)
	if err := repo.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	return repo
}

func newUser(email string) *domain.User {
	return &domain.User{
		Name:         "User " + email,
		Email:        email,
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC().Truncate(time.Millisecond),
	}
}

func TestUserRepository(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	const missing = "ffffffffffffffffffffffff"

	alice := newUser("alice@x.com")
	if err := repo.Create(ctx, alice); err != nil {
		t.Fatalf("create: %v", err)
	}
	if alice.ID == "" {
		t.Fatal("create must set ID")
	}
	bob := newUser("bob@x.com")
	if err := repo.Create(ctx, bob); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	t.Run("duplicate email", func(t *testing.T) {
		if err := repo.Create(ctx, newUser("alice@x.com")); !errors.Is(err, domain.ErrEmailTaken) {
			t.Errorf("got %v, want ErrEmailTaken", err)
		}
	})

	t.Run("get by id and email", func(t *testing.T) {
		got, err := repo.GetByID(ctx, alice.ID)
		if err != nil || got.Email != alice.Email || !got.CreatedAt.Equal(alice.CreatedAt) || got.PasswordHash != "hash" {
			t.Errorf("GetByID = %+v, %v", got, err)
		}
		got, err = repo.GetByEmail(ctx, "bob@x.com")
		if err != nil || got.ID != bob.ID {
			t.Errorf("GetByEmail = %+v, %v", got, err)
		}
	})

	t.Run("not found and invalid id", func(t *testing.T) {
		if _, err := repo.GetByID(ctx, missing); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("GetByID missing: %v", err)
		}
		if _, err := repo.GetByEmail(ctx, "nobody@x.com"); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("GetByEmail missing: %v", err)
		}
		if _, err := repo.GetByID(ctx, "bad"); !errors.Is(err, domain.ErrInvalidID) {
			t.Errorf("GetByID bad: %v", err)
		}
		if _, err := repo.Update(ctx, "bad", app.UserUpdate{}); !errors.Is(err, domain.ErrInvalidID) {
			t.Errorf("Update bad: %v", err)
		}
		if err := repo.Delete(ctx, "bad"); !errors.Is(err, domain.ErrInvalidID) {
			t.Errorf("Delete bad: %v", err)
		}
	})

	t.Run("list and count", func(t *testing.T) {
		list, err := repo.List(ctx)
		if err != nil || len(list) != 2 || list[0].ID != alice.ID {
			t.Errorf("List = %v, %v", list, err)
		}
		if n, err := repo.Count(ctx); err != nil || n != 2 {
			t.Errorf("Count = %d, %v", n, err)
		}
	})

	t.Run("update", func(t *testing.T) {
		name, email := "Alicia", "alicia@x.com"
		got, err := repo.Update(ctx, alice.ID, app.UserUpdate{Name: &name, Email: &email})
		if err != nil || got.Name != name || got.Email != email {
			t.Fatalf("Update = %+v, %v", got, err)
		}

		taken := "bob@x.com"
		if _, err := repo.Update(ctx, alice.ID, app.UserUpdate{Email: &taken}); !errors.Is(err, domain.ErrEmailTaken) {
			t.Errorf("update to taken email: %v", err)
		}
		if _, err := repo.Update(ctx, missing, app.UserUpdate{Name: &name}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("update missing: %v", err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		if err := repo.Delete(ctx, bob.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if err := repo.Delete(ctx, bob.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("second delete: %v", err)
		}
	})

	t.Run("ensure indexes is idempotent", func(t *testing.T) {
		if err := repo.EnsureIndexes(ctx); err != nil {
			t.Errorf("second EnsureIndexes: %v", err)
		}
	})
}
