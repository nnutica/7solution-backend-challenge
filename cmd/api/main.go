// Command api runs the user management REST and gRPC servers.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"

	grpcadapter "github.com/nnutica/7solution-backend-challenge/internal/adapter/grpc"
	httpadapter "github.com/nnutica/7solution-backend-challenge/internal/adapter/http"
	mongoadapter "github.com/nnutica/7solution-backend-challenge/internal/adapter/mongo"
	"github.com/nnutica/7solution-backend-challenge/internal/app"
	"github.com/nnutica/7solution-backend-challenge/internal/platform/config"
	"github.com/nnutica/7solution-backend-challenge/internal/platform/password"
	"github.com/nnutica/7solution-backend-challenge/internal/platform/token"
)

const (
	dbTimeout           = 5 * time.Second
	shutdownTimeout     = 10 * time.Second
	userCountInterval   = 10 * time.Second
	mongoConnectTimeout = 10 * time.Second
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Cancelled on SIGINT/SIGTERM; every long-running part watches this context.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return fmt.Errorf("connect mongo: %w", err)
	}
	defer func() {
		// The root context may already be cancelled, so disconnect with a fresh one.
		dctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := client.Disconnect(dctx); err != nil {
			log.Error("disconnect mongo", "error", err)
		}
	}()

	pingCtx, cancel := context.WithTimeout(ctx, mongoConnectTimeout)
	err = client.Ping(pingCtx, nil)
	cancel()
	if err != nil {
		return fmt.Errorf("ping mongo: %w", err)
	}

	repo := mongoadapter.NewUserRepository(client.Database(cfg.MongoDB), dbTimeout)
	if err := repo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}

	hasher := password.NewBcryptHasher(bcrypt.DefaultCost)
	jwt := token.NewJWT(cfg.JWTSecret, cfg.JWTTTL)
	users := app.NewUserService(repo, hasher)
	auth, err := app.NewAuthService(users, repo, hasher, jwt, jwt)
	if err != nil {
		return err
	}

	gin.SetMode(gin.ReleaseMode)
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpadapter.NewRouter(users, auth, log),
		ReadHeaderTimeout: 5 * time.Second,
	}
	grpcServer := grpcadapter.NewServer(users, auth, log)
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	var wg sync.WaitGroup
	wg.Go(func() { app.ReportUserCount(ctx, users, userCountInterval, log) })

	// Buffered so a failing server never blocks after we stop listening.
	serveErr := make(chan error, 2)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serveErr <- fmt.Errorf("http server: %w", err)
		}
	}()
	go func() {
		log.Info("grpc server listening", "addr", cfg.GRPCAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			serveErr <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case runErr = <-serveErr:
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown", "error", err)
	}
	stopGRPC(shutdownCtx, grpcServer)
	wg.Wait()

	log.Info("server stopped")
	return runErr
}

// stopGRPC waits for in-flight RPCs, but forces a stop if the deadline passes.
func stopGRPC(ctx context.Context, srv *grpc.Server) {
	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		srv.Stop()
	}
}
