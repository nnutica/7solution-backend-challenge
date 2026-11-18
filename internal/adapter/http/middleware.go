package httpadapter

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const userIDKey = "userID"

// TokenVerifier validates a bearer token and returns its user ID.
type TokenVerifier interface {
	VerifyToken(token string) (string, error)
}

// Logging logs method, path, status and duration for every request.
// The query string is left out because it may carry sensitive values.
func Logging(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", time.Since(start).String(),
		)
	}
}

// RequireAuth rejects requests without a valid "Authorization: Bearer <token>" header.
func RequireAuth(verifier TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme, token, ok := strings.Cut(c.GetHeader("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			abort(c, http.StatusUnauthorized, "missing or malformed bearer token")
			return
		}
		userID, err := verifier.VerifyToken(token)
		if err != nil {
			abort(c, http.StatusUnauthorized, err.Error())
			return
		}
		c.Set(userIDKey, userID)
		c.Next()
	}
}
