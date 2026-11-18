package config

import (
	"strings"
	"testing"
	"time"
)

const validSecret = "0123456789abcdef0123456789abcdef"

// setEnv clears every variable first so the host environment cannot leak into the test.
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, k := range []string{"JWT_SECRET", "JWT_TTL", "MONGO_URI", "MONGO_DB", "HTTP_ADDR", "GRPC_ADDR"} {
		t.Setenv(k, vars[k])
	}
}

func TestLoad_Defaults(t *testing.T) {
	setEnv(t, map[string]string{"JWT_SECRET": validSecret})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(cfg.JWTSecret) != validSecret || cfg.JWTTTL != time.Hour ||
		cfg.MongoURI != "mongodb://localhost:27017" || cfg.MongoDB != "user_management" ||
		cfg.HTTPAddr != ":8080" || cfg.GRPCAddr != ":50051" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}

func TestLoad_Overrides(t *testing.T) {
	setEnv(t, map[string]string{
		"JWT_SECRET": validSecret,
		"JWT_TTL":    "15m",
		"MONGO_URI":  "mongodb://mongo:27017",
		"MONGO_DB":   "test",
		"HTTP_ADDR":  ":9090",
		"GRPC_ADDR":  ":9091",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.JWTTTL != 15*time.Minute || cfg.MongoURI != "mongodb://mongo:27017" ||
		cfg.MongoDB != "test" || cfg.HTTPAddr != ":9090" || cfg.GRPCAddr != ":9091" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name    string
		vars    map[string]string
		wantMsg []string
	}{
		{"missing secret", map[string]string{}, []string{"JWT_SECRET is required"}},
		{"short secret", map[string]string{"JWT_SECRET": "short"}, []string{"at least 32 bytes"}},
		{"bad ttl", map[string]string{"JWT_SECRET": validSecret, "JWT_TTL": "soon"}, []string{"JWT_TTL"}},
		{"negative ttl", map[string]string{"JWT_SECRET": validSecret, "JWT_TTL": "-1h"}, []string{"JWT_TTL"}},
		{"reports all problems", map[string]string{"JWT_TTL": "0s"}, []string{"JWT_SECRET", "JWT_TTL"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, tt.vars)
			_, err := Load()
			if err == nil {
				t.Fatal("want error")
			}
			for _, msg := range tt.wantMsg {
				if !strings.Contains(err.Error(), msg) {
					t.Errorf("error %q does not mention %q", err, msg)
				}
			}
		})
	}
}
