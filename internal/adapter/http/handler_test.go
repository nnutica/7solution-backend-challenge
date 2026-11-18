package httpadapter_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	httpadapter "github.com/nnutica/7solution-backend-challenge/internal/adapter/http"
	"github.com/nnutica/7solution-backend-challenge/internal/app"
	"github.com/nnutica/7solution-backend-challenge/internal/app/apptest"
	"github.com/nnutica/7solution-backend-challenge/internal/platform/token"
)

var secret = []byte("test-secret-that-is-at-least-32-bytes-long")

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type testAPI struct {
	t      *testing.T
	router http.Handler
	repo   *apptest.UserRepo
	logs   *bytes.Buffer
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	repo := apptest.NewUserRepo()
	hasher := &apptest.Hasher{}
	jwt := token.NewJWT(secret, time.Hour)
	users := app.NewUserService(repo, hasher)
	auth, err := app.NewAuthService(users, repo, hasher, jwt, jwt)
	if err != nil {
		t.Fatal(err)
	}
	logs := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logs, nil))
	return &testAPI{t: t, router: httpadapter.NewRouter(users, auth, log), repo: repo, logs: logs}
}

func (a *testAPI) do(method, path, body, bearer string) *httptest.ResponseRecorder {
	a.t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return rec
}

// login registers a user and returns a valid token.
func (a *testAPI) login() string {
	a.t.Helper()
	a.do(http.MethodPost, "/auth/register", `{"name":"Admin","email":"admin@x.com","password":"password1"}`, "")
	rec := a.do(http.MethodPost, "/auth/login", `{"email":"admin@x.com","password":"password1"}`, "")
	if rec.Code != http.StatusOK {
		a.t.Fatalf("login failed: %d %s", rec.Code, rec.Body)
	}
	var res struct{ Token string }
	decode(a.t, rec, &res)
	return res.Token
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode %q: %v", rec.Body, err)
	}
}

// assertError checks status and the shared {"error": "..."} shape.
func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, msg string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body)
	}
	var body map[string]string
	decode(t, rec, &body)
	if len(body) != 1 || !strings.Contains(body["error"], msg) {
		t.Errorf("body = %v, want only error containing %q", body, msg)
	}
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status int
		errMsg string
	}{
		{"created", `{"name":"Alice","email":"Alice@X.com","password":"password1"}`, http.StatusCreated, ""},
		{"duplicate email", `{"name":"Dup","email":"existing@x.com","password":"password1"}`, http.StatusConflict, "email already registered"},
		{"malformed json", `{"name":`, http.StatusBadRequest, "invalid JSON body"},
		{"wrong type", `{"name":123}`, http.StatusBadRequest, "invalid JSON body"},
		{"empty body", ``, http.StatusBadRequest, "invalid JSON body"},
		{"missing name", `{"email":"a@x.com","password":"password1"}`, http.StatusBadRequest, "name is required"},
		{"bad email", `{"name":"A","email":"nope","password":"password1"}`, http.StatusBadRequest, "email is not a valid"},
		{"short password", `{"name":"A","email":"a@x.com","password":"short"}`, http.StatusBadRequest, "password must be at least"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newTestAPI(t)
			api.do(http.MethodPost, "/auth/register", `{"name":"E","email":"existing@x.com","password":"password1"}`, "")

			rec := api.do(http.MethodPost, "/auth/register", tt.body, "")
			if tt.errMsg != "" {
				assertError(t, rec, tt.status, tt.errMsg)
				return
			}
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.status, rec.Body)
			}
			var body map[string]any
			decode(t, rec, &body)
			if body["email"] != "alice@x.com" || body["id"] == "" || body["created_at"] == "" {
				t.Errorf("unexpected body: %v", body)
			}
			for k := range body {
				if strings.Contains(strings.ToLower(k), "password") {
					t.Errorf("response must not expose %q", k)
				}
			}
		})
	}
}

func TestLogin(t *testing.T) {
	api := newTestAPI(t)
	api.do(http.MethodPost, "/auth/register", `{"name":"A","email":"a@x.com","password":"password1"}`, "")

	rec := api.do(http.MethodPost, "/auth/login", `{"email":"a@x.com","password":"password1"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var res struct {
		Token     string    `json:"token"`
		TokenType string    `json:"token_type"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	decode(t, rec, &res)
	if res.Token == "" || res.TokenType != "Bearer" || res.ExpiresAt.IsZero() {
		t.Errorf("unexpected login response: %+v", res)
	}

	for _, body := range []string{
		`{"email":"a@x.com","password":"wrong-pass"}`,
		`{"email":"nobody@x.com","password":"password1"}`,
	} {
		assertError(t, api.do(http.MethodPost, "/auth/login", body, ""), http.StatusUnauthorized, "invalid email or password")
	}
	assertError(t, api.do(http.MethodPost, "/auth/login", `nope`, ""), http.StatusBadRequest, "invalid JSON body")
}

func TestRequireAuth(t *testing.T) {
	api := newTestAPI(t)
	valid := api.login()
	expired, _, _ := token.NewJWT(secret, -time.Minute).Issue("user-1")
	otherSecret, _, _ := token.NewJWT([]byte("a-different-secret-of-32-bytes-or-more"), time.Hour).Issue("user-1")

	tests := []struct {
		name   string
		header string
		status int
		errMsg string
	}{
		{"valid", "Bearer " + valid, http.StatusOK, ""},
		{"scheme is case-insensitive", "bearer " + valid, http.StatusOK, ""},
		{"missing header", "", http.StatusUnauthorized, "missing or malformed"},
		{"no token", "Bearer ", http.StatusUnauthorized, "missing or malformed"},
		{"wrong scheme", "Basic " + valid, http.StatusUnauthorized, "missing or malformed"},
		{"token without scheme", valid, http.StatusUnauthorized, "missing or malformed"},
		{"garbage token", "Bearer abc.def.ghi", http.StatusUnauthorized, "invalid or expired token"},
		{"expired token", "Bearer " + expired, http.StatusUnauthorized, "invalid or expired token"},
		{"signed with other secret", "Bearer " + otherSecret, http.StatusUnauthorized, "invalid or expired token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			api.router.ServeHTTP(rec, req)

			if tt.errMsg != "" {
				assertError(t, rec, tt.status, tt.errMsg)
				return
			}
			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
		})
	}
}

func TestProtectedRoutesNeedToken(t *testing.T) {
	api := newTestAPI(t)
	routes := []struct{ method, path string }{
		{http.MethodPost, "/users"},
		{http.MethodGet, "/users"},
		{http.MethodGet, "/users/000000000000000000000001"},
		{http.MethodPatch, "/users/000000000000000000000001"},
		{http.MethodDelete, "/users/000000000000000000000001"},
	}
	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			assertError(t, api.do(r.method, r.path, `{}`, ""), http.StatusUnauthorized, "missing or malformed")
		})
	}
}

func TestUserCRUD(t *testing.T) {
	api := newTestAPI(t)
	tok := api.login()

	rec := api.do(http.MethodPost, "/users", `{"name":"Bob","email":"bob@x.com","password":"password1"}`, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var bob struct{ ID, Name, Email string }
	decode(t, rec, &bob)

	assertError(t, api.do(http.MethodPost, "/users", `{"name":"Bob2","email":"BOB@x.com","password":"password1"}`, tok),
		http.StatusConflict, "email already registered")

	rec = api.do(http.MethodGet, "/users/"+bob.ID, "", tok)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"bob@x.com"`) {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}

	rec = api.do(http.MethodGet, "/users", "", tok)
	var list []map[string]any
	decode(t, rec, &list)
	if rec.Code != http.StatusOK || len(list) != 2 {
		t.Fatalf("list: %d %v", rec.Code, list)
	}

	rec = api.do(http.MethodPatch, "/users/"+bob.ID, `{"name":"Bobby","email":"Bobby@x.com"}`, tok)
	var updated struct{ Name, Email string }
	decode(t, rec, &updated)
	if rec.Code != http.StatusOK || updated.Name != "Bobby" || updated.Email != "bobby@x.com" {
		t.Fatalf("patch: %d %+v", rec.Code, updated)
	}

	rec = api.do(http.MethodDelete, "/users/"+bob.ID, "", tok)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete: %d %q", rec.Code, rec.Body)
	}
	assertError(t, api.do(http.MethodGet, "/users/"+bob.ID, "", tok), http.StatusNotFound, "user not found")
}

func TestUserErrors(t *testing.T) {
	const missing = "ffffffffffffffffffffffff"
	tests := []struct {
		name, method, path, body string
		status                   int
		errMsg                   string
	}{
		{"get invalid id", http.MethodGet, "/users/123", "", http.StatusBadRequest, "invalid user id"},
		{"get unknown id", http.MethodGet, "/users/" + missing, "", http.StatusNotFound, "user not found"},
		{"patch empty body", http.MethodPatch, "/users/" + missing, `{}`, http.StatusBadRequest, "must include name or email"},
		{"patch malformed json", http.MethodPatch, "/users/" + missing, `{`, http.StatusBadRequest, "invalid JSON body"},
		{"patch bad email", http.MethodPatch, "/users/" + missing, `{"email":"x"}`, http.StatusBadRequest, "email is not a valid"},
		{"patch unknown id", http.MethodPatch, "/users/" + missing, `{"name":"X"}`, http.StatusNotFound, "user not found"},
		{"patch email taken", http.MethodPatch, "/users/OWN", `{"email":"admin@x.com"}`, http.StatusConflict, "email already registered"},
		{"delete invalid id", http.MethodDelete, "/users/abc", "", http.StatusBadRequest, "invalid user id"},
		{"delete unknown id", http.MethodDelete, "/users/" + missing, "", http.StatusNotFound, "user not found"},
		{"create invalid input", http.MethodPost, "/users", `{"name":"","email":"","password":""}`, http.StatusBadRequest, "name is required"},
		{"unknown route", http.MethodGet, "/nope", "", http.StatusNotFound, "route not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newTestAPI(t)
			tok := api.login()
			rec := api.do(http.MethodPost, "/users", `{"name":"Own","email":"own@x.com","password":"password1"}`, tok)
			var own struct{ ID string }
			decode(t, rec, &own)

			path := strings.Replace(tt.path, "OWN", own.ID, 1)
			assertError(t, api.do(tt.method, path, tt.body, tok), tt.status, tt.errMsg)
		})
	}
}

func TestInternalErrorIsHidden(t *testing.T) {
	api := newTestAPI(t)
	tok := api.login()
	api.repo.Err = errors.New("connection refused to 10.0.0.5")

	rec := api.do(http.MethodGet, "/users", "", tok)
	assertError(t, rec, http.StatusInternalServerError, "internal server error")
	if strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Error("internal details leaked to client")
	}
	if !strings.Contains(api.logs.String(), "10.0.0.5") {
		t.Error("internal error should be logged")
	}
}

func TestLoggingMiddleware(t *testing.T) {
	api := newTestAPI(t)
	api.do(http.MethodGet, "/users?token=secret-value", "", "")

	var entry map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(api.logs.String()), "\n") {
		if strings.Contains(line, `"http request"`) {
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				t.Fatal(err)
			}
		}
	}
	if entry == nil {
		t.Fatalf("no request log in %q", api.logs)
	}
	if entry["method"] != "GET" || entry["path"] != "/users" || entry["status"] != float64(401) || entry["duration"] == "" {
		t.Errorf("unexpected log entry: %v", entry)
	}
	if strings.Contains(api.logs.String(), "secret-value") {
		t.Error("query string must not be logged")
	}
}
