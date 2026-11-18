// Package config loads application settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const minSecretLength = 32

// Config holds all runtime settings.
type Config struct {
	JWTSecret []byte
	JWTTTL    time.Duration
	MongoURI  string
	MongoDB   string
	HTTPAddr  string
	GRPCAddr  string
}

// Load reads the environment and returns every problem at once so startup fails with a full list.
func Load() (Config, error) {
	var errs []error

	secret := os.Getenv("JWT_SECRET")
	switch {
	case secret == "":
		errs = append(errs, errors.New("JWT_SECRET is required"))
	case len(secret) < minSecretLength:
		errs = append(errs, fmt.Errorf("JWT_SECRET must be at least %d bytes", minSecretLength))
	}

	ttl, err := time.ParseDuration(getEnv("JWT_TTL", "1h"))
	if err != nil || ttl <= 0 {
		errs = append(errs, errors.New("JWT_TTL must be a positive duration such as 1h"))
	}

	cfg := Config{
		JWTSecret: []byte(secret),
		JWTTTL:    ttl,
		MongoURI:  getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:   getEnv("MONGO_DB", "user_management"),
		HTTPAddr:  getEnv("HTTP_ADDR", ":8080"),
		GRPCAddr:  getEnv("GRPC_ADDR", ":50051"),
	}
	return cfg, errors.Join(errs...)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
