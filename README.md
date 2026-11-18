# 7 Solutions Backend Challenge

- **Part 1:** User Management API in Go: REST (Gin) and gRPC, MongoDB, JWT (HS256).
- **Part 2:** Lottery Search System design, see [docs/lottery-design.md](docs/lottery-design.md).

## Requirements

- Docker with Docker Compose, **or**
- Go 1.26+ and a running MongoDB

## Quick start (Docker Compose)

```bash
docker compose up --build
```

This starts MongoDB and the API using the values in [.env.example](.env.example).

| Service | Address |
| --- | --- |
| REST API | `http://localhost:8080` |
| gRPC | `localhost:50051` |
| MongoDB | `localhost:27017` |

Stop with `Ctrl+C` or `docker compose down` (add `-v` to delete the data volume).

## Run locally (without Docker for the API)

```bash
docker compose up -d mongo          # or use your own MongoDB
cp .env.example .env                # edit if needed

# bash
set -a; . ./.env; set +a
go run ./cmd/api

# PowerShell
Get-Content .env | Where-Object { $_ -match '^\w+=' } | ForEach-Object { $k, $v = $_ -split '=', 2; Set-Item "env:$k" $v }
go run ./cmd/api
```

## Configuration

| Variable | Default | Notes |
| --- | --- | --- |
| `JWT_SECRET` | – | **Required**, at least 32 bytes. The server refuses to start without it. |
| `JWT_TTL` | `1h` | Token lifetime (Go duration). |
| `MONGO_URI` | `mongodb://localhost:27017` | |
| `MONGO_DB` | `user_management` | |
| `HTTP_ADDR` | `:8080` | |
| `GRPC_ADDR` | `:50051` | |

## Tests

```bash
go test ./...
```

Unit tests use the standard `testing` package with hand-written in-memory fakes ([internal/app/apptest](internal/app/apptest)) behind the port interfaces. No mocking library is used.

| Package | What is tested |
| --- | --- |
| `domain` | Validation and normalisation rules |
| `platform/token` | Issue/verify, expired token, wrong secret, `alg: none`, HS512, missing `exp`/`sub` |
| `platform/password`, `platform/config` | bcrypt hashing, env loading and errors |
| `app` | User and auth use cases, login timing protection, background reporter |
| `adapter/http` | Every route and error path via `httptest`, JWT and logging middleware |
| `adapter/grpc` | CreateUser/GetUser, error codes and auth interceptor via in-memory `bufconn` |
| `adapter/mongo` | Integration test against a real MongoDB; **skipped unless `MONGO_URI` is set** |

Run the integration test as well:

```bash
docker compose up -d mongo
MONGO_URI=mongodb://localhost:27017 go test ./...
```

## Authentication (JWT)

1. **Register** with `POST /auth/register` (public).
2. **Log in** with `POST /auth/login` (public). The response has a token signed with HS256 using `JWT_SECRET`.
3. **Send the token** on every other request:

   ```
   Authorization: Bearer <token>
   ```

The token's `sub` claim is the user ID; `iat` and `exp` are set at login. The server only accepts HS256 tokens, so `alg: none` or other algorithms are rejected, and it rejects expired tokens or tokens without `exp`. When a token expires, log in again (there are no refresh tokens).

```bash
TOKEN=$(curl -s -X POST localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"secret123"}' | jq -r .token)
```

## REST API

| Method | Path | Auth | Success |
| --- | --- | --- | --- |
| POST | `/auth/register` | – | 201 |
| POST | `/auth/login` | – | 200 |
| POST | `/users` | Bearer | 201 |
| GET | `/users` | Bearer | 200 |
| GET | `/users/{id}` | Bearer | 200 |
| PATCH | `/users/{id}` | Bearer | 200 |
| DELETE | `/users/{id}` | Bearer | 204 |

Every error uses the same shape:

```json
{ "error": "message" }
```

| Status | When |
| --- | --- |
| 400 | Invalid JSON, failed validation, malformed ID |
| 401 | Missing/invalid/expired token, wrong email or password |
| 404 | User or route not found |
| 409 | Email already registered |
| 500 | Unexpected error (details are only logged) |

### Register

```bash
curl -X POST localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"  Alice@Example.com ","password":"secret123"}'
```

```json
201 Created
{"id":"6afd2f774200dbf4a898f474","name":"Alice","email":"alice@example.com","created_at":"2026-11-18T04:54:15.434Z"}
```

The email is trimmed and lowercased. Registering `alice@example.com` again returns:

```json
409 Conflict
{"error":"email already registered"}
```

### Login

```bash
curl -X POST localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"secret123"}'
```

```json
200 OK
{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...","token_type":"Bearer","expires_at":"2026-11-18T05:55:38Z"}
```

Wrong password or unknown email returns:

```json
401 Unauthorized
{"error":"invalid email or password"}
```

### Create user

```bash
curl -X POST localhost:8080/users \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Bob","email":"bob@example.com","password":"password1"}'
```

```json
201 Created
{"id":"6afd2f784200dbf4a898f476","name":"Bob","email":"bob@example.com","created_at":"2026-11-18T04:54:16.777Z"}
```

### List users

```bash
curl localhost:8080/users -H "Authorization: Bearer $TOKEN"
```

```json
200 OK
[{"id":"6afd2f774200dbf4a898f474","name":"Alice","email":"alice@example.com","created_at":"2026-11-18T04:54:15.434Z"},
 {"id":"6afd2f784200dbf4a898f476","name":"Bob","email":"bob@example.com","created_at":"2026-11-18T04:54:16.777Z"}]
```

### Get user

```bash
curl localhost:8080/users/6afd2f784200dbf4a898f476 -H "Authorization: Bearer $TOKEN"
```

```json
200 OK
{"id":"6afd2f784200dbf4a898f476","name":"Bob","email":"bob@example.com","created_at":"2026-11-18T04:54:16.777Z"}
```

`/users/123` returns `400 {"error":"invalid user id"}`; a well-formed but unknown ID returns `404 {"error":"user not found"}`.

### Update user

Send `name`, `email` or both. Other fields are ignored.

```bash
curl -X PATCH localhost:8080/users/6afd2f784200dbf4a898f476 \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Bobby"}'
```

```json
200 OK
{"id":"6afd2f784200dbf4a898f476","name":"Bobby","email":"bob@example.com","created_at":"2026-11-18T04:54:16.777Z"}
```

An empty body `{}` returns `400 {"error":"request must include name or email"}`.

### Delete user

```bash
curl -X DELETE localhost:8080/users/6afd2f784200dbf4a898f476 -H "Authorization: Bearer $TOKEN"
```

```
204 No Content
```

### Missing or invalid token

```bash
curl localhost:8080/users
```

```json
401 Unauthorized
{"error":"missing or malformed bearer token"}
```

## gRPC

[proto/user.proto](proto/user.proto) defines `CreateUser` and `GetUser`. Both need the token in the `authorization` metadata. Get a token from the REST login first.

With [grpcurl](https://github.com/fullstorydev/grpcurl):

```bash
grpcurl -plaintext -import-path proto -proto user.proto \
  -H "authorization: Bearer $TOKEN" \
  -d '{"name":"Carol","email":"carol@example.com","password":"password1"}' \
  localhost:50051 user.v1.UserService/CreateUser
```

```json
{
  "user": {
    "id": "6afd2fc1b3a491434e2ab403",
    "name": "Carol",
    "email": "carol@example.com",
    "createdAt": "2026-11-18T04:55:29.289Z"
  }
}
```

```bash
grpcurl -plaintext -import-path proto -proto user.proto \
  -H "authorization: Bearer $TOKEN" \
  -d '{"id":"6afd2fc1b3a491434e2ab403"}' \
  localhost:50051 user.v1.UserService/GetUser
```

Domain errors map to gRPC codes: validation → `InvalidArgument`, not found → `NotFound`, duplicate email → `AlreadyExists`, bad token → `Unauthenticated`.

The generated code in `proto/userpb` is committed. To regenerate it (no local `protoc` needed):

```bash
docker run --rm -v "$(pwd):/workspace" -w /workspace bufbuild/buf:1.47.2 generate
```

## Project structure

Hexagonal (ports and adapters). Dependencies point inward: adapters → app → domain.

```
cmd/api/                 wiring, background job, graceful shutdown
internal/domain/         User entity, validation rules, domain errors (no framework imports)
internal/app/            use cases (UserService, AuthService) and ports (interfaces)
internal/adapter/mongo/  UserRepository on the official MongoDB driver
internal/adapter/http/   Gin router, handlers, DTOs, logging and JWT middleware
internal/adapter/grpc/   gRPC server, auth and logging interceptors
internal/platform/       config, bcrypt hasher, JWT issuer/verifier
proto/                   .proto and generated Go code
```

## Design decisions and assumptions

- **Hexagonal layout.** The domain and app layers do not import Gin, gRPC or the Mongo driver. REST and gRPC share the same services, so they apply the same rules.
- **Validation lives in the domain.** It does not use Gin binding tags, so gRPC requests get the same checks. Name: required, max 100 characters. Email: trimmed, lowercased, a bare address parsed by `net/mail`; addresses without a TLD such as `a@localhost` are accepted. Password: 8–72 bytes, because bcrypt ignores anything past 72 bytes.
- **Email uniqueness is enforced by a unique index**, created at startup. A duplicate-key error becomes `409`. There is no "check, then insert", which would race.
- **Passwords** are hashed with bcrypt (default cost). The hash is never returned or logged: the only way a user leaves the API is through a response DTO without that field.
- **Login does not reveal which emails exist.** Unknown email and wrong password return the same error, and an unknown email still runs one bcrypt comparison against a dummy hash so response times match.
- **JWT.** HS256 only, `exp` required, secret of at least 32 bytes from the environment. No refresh tokens or revocation, which the challenge does not ask for.
- **Authorization.** Any authenticated user can manage any user. The challenge defines no roles.
- **`POST /auth/register` and `POST /users`** both create a user through the same service. The first is public sign-up; the second is the protected operation the challenge lists.
- **Update** is `PATCH` with optional `name`/`email`, done as one atomic `findOneAndUpdate`.
- **IDs** are MongoDB ObjectIDs exposed as hex strings. A malformed ID gives `400`; an unknown one gives `404`.
- **List** returns all users sorted by `created_at` with no pagination, as the challenge asks. A real service would add cursor pagination.
- **Timeouts.** Every database call has a 5 s timeout, and the HTTP server sets `ReadHeaderTimeout`.
- **Background job.** A `time.Ticker` goroutine logs the user count every 10 s and exits when the root context is cancelled.
- **Graceful shutdown.** `signal.NotifyContext` catches SIGINT/SIGTERM. The HTTP and gRPC servers drain within 10 s; gRPC is force-stopped if it overruns. Then the ticker goroutine is awaited and MongoDB is disconnected.
- **Logging** uses `log/slog` (JSON). Requests are logged with method, path, status and duration. Query strings, bodies and tokens are never logged.
- **`.env.example`** holds working development values so `docker compose up` works from a fresh clone. The JWT secret in it is for development only.
