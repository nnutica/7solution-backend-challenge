// Package mongoadapter implements app.UserRepository with the official MongoDB driver.
package mongoadapter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/nnutica/7solution-backend-challenge/internal/app"
	"github.com/nnutica/7solution-backend-challenge/internal/domain"
)

const collectionName = "users"

type userDocument struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	Name         string        `bson:"name"`
	Email        string        `bson:"email"`
	PasswordHash string        `bson:"password_hash"`
	CreatedAt    time.Time     `bson:"created_at"`
}

func (d userDocument) toDomain() *domain.User {
	return &domain.User{
		ID:           d.ID.Hex(),
		Name:         d.Name,
		Email:        d.Email,
		PasswordHash: d.PasswordHash,
		CreatedAt:    d.CreatedAt,
	}
}

// UserRepository stores users in a MongoDB collection.
type UserRepository struct {
	coll    *mongo.Collection
	timeout time.Duration
}

var _ app.UserRepository = (*UserRepository)(nil)

// NewUserRepository returns a repository; timeout bounds every database call.
func NewUserRepository(db *mongo.Database, timeout time.Duration) *UserRepository {
	return &UserRepository{coll: db.Collection(collectionName), timeout: timeout}
}

// EnsureIndexes creates the unique email index. It is idempotent.
func (r *UserRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	_, err := r.coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("email_unique"),
	})
	return err
}

// Create inserts u and sets its ID.
func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	doc := userDocument{
		ID:           bson.NewObjectID(),
		Name:         u.Name,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
	}
	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		return mapWriteError(err)
	}
	u.ID = doc.ID.Hex()
	return nil
}

// GetByID returns the user with the given hex ID.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrInvalidID
	}
	return r.findOne(ctx, bson.D{{Key: "_id", Value: oid}})
}

// GetByEmail returns the user with the given (already normalised) email.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.findOne(ctx, bson.D{{Key: "email", Value: email}})
}

// List returns all users, oldest first.
func (r *UserRepository) List(ctx context.Context) ([]domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cur, err := r.coll.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("find users: %w", err)
	}
	var docs []userDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}

	users := make([]domain.User, 0, len(docs))
	for _, d := range docs {
		users = append(users, *d.toDomain())
	}
	return users, nil
}

// Update sets the given fields and returns the updated user in one atomic call.
func (r *UserRepository) Update(ctx context.Context, id string, upd app.UserUpdate) (*domain.User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrInvalidID
	}

	set := bson.D{}
	if upd.Name != nil {
		set = append(set, bson.E{Key: "name", Value: *upd.Name})
	}
	if upd.Email != nil {
		set = append(set, bson.E{Key: "email", Value: *upd.Email})
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var doc userDocument
	err = r.coll.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: oid}},
		bson.D{{Key: "$set", Value: set}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, mapWriteError(err)
	}
	return doc.toDomain(), nil
}

// Delete removes the user with the given hex ID.
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return domain.ErrInvalidID
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}})
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if res.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Count returns the number of users.
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.coll.CountDocuments(ctx, bson.D{})
}

func (r *UserRepository) findOne(ctx context.Context, filter bson.D) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var doc userDocument
	err := r.coll.FindOne(ctx, filter).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return doc.toDomain(), nil
}

func mapWriteError(err error) error {
	// The only unique index is on email, so a duplicate key always means the email is taken.
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrEmailTaken
	}
	return fmt.Errorf("write user: %w", err)
}
