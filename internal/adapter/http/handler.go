// Package httpadapter exposes the use cases over REST using Gin.
package httpadapter

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/nnutica/7solution-backend-challenge/internal/app"
)

// Handler serves the auth and user endpoints.
type Handler struct {
	users *app.UserService
	auth  *app.AuthService
	log   *slog.Logger
}

// NewRouter builds the Gin engine with all routes and middleware.
func NewRouter(users *app.UserService, auth *app.AuthService, log *slog.Logger) *gin.Engine {
	h := &Handler{users: users, auth: auth, log: log}

	r := gin.New()
	r.Use(Logging(log), gin.Recovery())
	r.NoRoute(func(c *gin.Context) { abort(c, http.StatusNotFound, "route not found") })

	r.POST("/auth/register", h.register)
	r.POST("/auth/login", h.login)

	protected := r.Group("/users", RequireAuth(auth))
	protected.POST("", h.createUser)
	protected.GET("", h.listUsers)
	protected.GET("/:id", h.getUser)
	protected.PATCH("/:id", h.updateUser)
	protected.DELETE("/:id", h.deleteUser)

	return r
}

func (h *Handler) register(c *gin.Context) {
	var req createUserRequest
	if !bindJSON(c, &req) {
		return
	}
	u, err := h.auth.Register(c.Request.Context(), app.CreateUserInput(req))
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, toUserResponse(u))
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, loginResponse{Token: res.Token, TokenType: "Bearer", ExpiresAt: res.ExpiresAt})
}

func (h *Handler) createUser(c *gin.Context) {
	var req createUserRequest
	if !bindJSON(c, &req) {
		return
	}
	u, err := h.users.Create(c.Request.Context(), app.CreateUserInput(req))
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, toUserResponse(u))
}

func (h *Handler) listUsers(c *gin.Context) {
	list, err := h.users.List(c.Request.Context())
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	resp := make([]userResponse, 0, len(list))
	for i := range list {
		resp = append(resp, toUserResponse(&list[i]))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) getUser(c *gin.Context) {
	u, err := h.users.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, toUserResponse(u))
}

func (h *Handler) updateUser(c *gin.Context) {
	var req updateUserRequest
	if !bindJSON(c, &req) {
		return
	}
	u, err := h.users.Update(c.Request.Context(), c.Param("id"), app.UpdateUserInput(req))
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, toUserResponse(u))
}

func (h *Handler) deleteUser(c *gin.Context) {
	if err := h.users.Delete(c.Request.Context(), c.Param("id")); err != nil {
		writeError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// bindJSON decodes the body and writes a 400 on failure.
func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		abort(c, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}
