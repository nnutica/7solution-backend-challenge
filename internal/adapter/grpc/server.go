// Package grpcadapter exposes CreateUser and GetUser over gRPC.
package grpcadapter

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nnutica/7solution-backend-challenge/internal/app"
	"github.com/nnutica/7solution-backend-challenge/internal/domain"
	"github.com/nnutica/7solution-backend-challenge/proto/userpb"
)

// TokenVerifier validates a bearer token and returns its user ID.
type TokenVerifier interface {
	VerifyToken(token string) (string, error)
}

// Server implements userpb.UserServiceServer on top of UserService.
type Server struct {
	userpb.UnimplementedUserServiceServer
	users *app.UserService
	log   *slog.Logger
}

// NewServer builds a gRPC server with logging and auth interceptors.
func NewServer(users *app.UserService, verifier TokenVerifier, log *slog.Logger) *grpc.Server {
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(logging(log), auth(verifier)))
	userpb.RegisterUserServiceServer(srv, &Server{users: users, log: log})
	return srv
}

// CreateUser creates a user.
func (s *Server) CreateUser(ctx context.Context, req *userpb.CreateUserRequest) (*userpb.CreateUserResponse, error) {
	u, err := s.users.Create(ctx, app.CreateUserInput{
		Name:     req.GetName(),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &userpb.CreateUserResponse{User: toProto(u)}, nil
}

// GetUser returns a user by ID.
func (s *Server) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
	u, err := s.users.Get(ctx, req.GetId())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &userpb.GetUserResponse{User: toProto(u)}, nil
}

func toProto(u *domain.User) *userpb.User {
	return &userpb.User{
		Id:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: timestamppb.New(u.CreatedAt),
	}
}

// toStatus maps domain errors to gRPC codes, mirroring the HTTP mapping.
func (s *Server) toStatus(err error) error {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		return status.Error(codes.InvalidArgument, ve.Error())
	case errors.Is(err, domain.ErrInvalidID):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrEmailTaken):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		s.log.Error("grpc request failed", "error", err)
		return status.Error(codes.Internal, "internal server error")
	}
}

// auth reads "authorization: Bearer <token>" from metadata and rejects invalid tokens.
func auth(verifier TokenVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing bearer token")
		}
		scheme, token, ok := strings.Cut(values[0], " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			return nil, status.Error(codes.Unauthenticated, "malformed bearer token")
		}
		if _, err := verifier.VerifyToken(token); err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		return handler(ctx, req)
	}
}

func logging(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		log.Info("grpc request",
			"method", info.FullMethod,
			"code", status.Code(err).String(),
			"duration", time.Since(start).String(),
		)
		return resp, err
	}
}
