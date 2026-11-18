package httpadapter

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/nnutica/7solution-backend-challenge/internal/domain"
)

// writeError maps domain errors to HTTP status codes. Unknown errors become 500
// with a generic message; the real cause is only logged.
func writeError(c *gin.Context, log *slog.Logger, err error) {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		abort(c, http.StatusBadRequest, ve.Error())
	case errors.Is(err, domain.ErrInvalidID):
		abort(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		abort(c, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrEmailTaken):
		abort(c, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrInvalidToken):
		abort(c, http.StatusUnauthorized, err.Error())
	default:
		log.Error("request failed", "method", c.Request.Method, "path", c.Request.URL.Path, "error", err)
		abort(c, http.StatusInternalServerError, "internal server error")
	}
}

func abort(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, errorResponse{Error: msg})
}
