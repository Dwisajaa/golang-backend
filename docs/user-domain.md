# User Domain

Phase 7A deliverable — first vertical slice in the Go backend.

## Responsibility

Read a single user by ID. Endpoint: `GET /api/users/:id`.

Full slice (no auth yet): handler → service → repository → MySQL.

## Model

`internal/model/user.go` — `User` maps the `users` table 1:1
(id, name, email, role, email_verified_at, password, remember_token,
created_at, updated_at). `password` is loaded (the login phase will need it)
but never serialized. Role constants mirror Laravel.

Model/DTO separation (FASE 3 §7): the same struct serves as DB + domain model
for now — the read rule set is trivial, so a separate domain type would be
decorative. The response always goes through a dedicated DTO, never the model.

## Repository

`internal/repository/user.go` (interface) + `user_mysql.go` (MySQL).

```go
type UserRepository interface {
    FindByID(ctx context.Context, id uint64) (*model.User, error)
}
```

- Owns SQL, scanning, and the `sql.ErrNoRows` → `repository.ErrNotFound`
  translation.
- **Never** builds HTTP output, imports gin, decides status codes, authorizes,
  or enforces business rules. Reasons: the repository is the persistence
  boundary — mixing HTTP concerns pins storage code to the web layer, kills
  reuse, and makes tests need an HTTP harness.
- Query is parameterized (`WHERE id = ?`), never string-built: injection-safe,
  and the driver handles quoting/escaping.

## Service

`internal/service/user.go`:

```go
func (s *UserService) GetUserByID(ctx, id) (*model.User, error)
```

- Maps `repository.ErrNotFound` → `httperr.NotFound("Resource not found.")`,
  everything else → `httperr.Internal(err)` (details preserved for logs, never
  shown to clients).
- Stands between HTTP and storage so rules live in one place and are tested
  without a server. No gin, no JSON, no Content-Type.

## Handler

`internal/httphandler/user.go` — parses `:id` (`strconv.ParseUint`), calls the
service, serializes the DTO. Invalid/non-numeric id → 404 (matches Laravel
route-model binding). No SQL, no business rules. Errors go through
`respond.go`:

| Err kind | HTTP | body |
|---|---|---|
| NotFound | 404 | `{"message":"Resource not found."}` |
| Internal | 500 | `{"message":"Server error."}` (+ server log with request_id) |

## DTO

`internal/httphandler/user_dto.go` mirrors the Laravel `UserResource`:

```json
{
  "data": {
    "id": 1,
    "name": "Dev Customer",
    "email": "customer@example.test",
    "email_verified_at": null,
    "role": "customer"
  }
}
```

Timestamps use Laravel's microsecond ISO-8601 format (`2026-08-27T12:34:56.123456Z`).
`password`, `remember_token`, and all hash/secret fields are structurally
absent.

## Error Handling

Typed errors in `internal/httperr` (kind + client-safe message + optional
underlying error). `respondError` maps kind → status and logs 5xx with request
id. Driver/SQL text never reaches the response body.

## Request Flow

```
GET /api/users/1
  ↓ Router (gin): match GET /api/users/:id, middleware (recovery, requestid,
                  logging, headers)
  ↓ Handler.Get:   parse "id" → 1
  ↓ Service.GetUserByID: rule classification (found/notfound/internal)
  ↓ Repository.FindByID: SELECT id,name,...,updated_at FROM users WHERE id=?
  ↓ MySQL
  ↑ rows: Scan → model.User
  ↑ Service: nil error
  ↑ Handler: toUserResponse(u) → JSON
  ↓ HTTP 200 {"data":{...}}
```

## Testing

| Level | File | Coverage |
|---|---|---|
| Service unit | `internal/service/user_test.go` | found / not-found / repo error → typed errors |
| Handler API | `internal/httphandler/user_test.go` | 200 + no password leak, invalid id → 404, not-found → 404, internal → 500 generic |
| Repository integration | `internal/repository/user_mysql_test.go` | gated by `TEST_DATABASE_URL` (skips, clearly, without a disposable MySQL) |

Run:

```
go test ./...            # unit/api pass without any database
TEST_DATABASE_URL=... go test ./internal/repository -run TestMySQLUserRepository_FindByID -v
```

## Security

- Password/hash fields structurally excluded from the DTO.
- Parameterized queries (no SQL injection point).
- Internal errors return a generic message; details only in server logs with
  request id.
- No auth middleware yet — this endpoint is intentionally public during the
  foundation phase; auth lands in the auth phase.

## Fundamental Concepts

1. **struct** — composite data type holding named fields with types.
2. **method** — function with a receiver (`func (s *UserService) Get...`),
   binding behavior to a type.
3. **interface** — set of method names a value must implement to satisfy it.
   `repository.UserRepository` names the two endpoints storage must provide.
4. **Why interface on repository** — the service depends on a contract, not a
   concrete DB; tests inject a fake; alternative data sources can drop in
   without touching the service.
5. **Dependency injection** — the service receives its repository in the
   constructor (`NewUserService(repo)`); nothing is global or hard-wired.
6. **context.Context** — carries deadlines/cancellation across the whole call
   stack; request-scoped.
7. **Why queries take context** — a cancelled client or a timeout stops the DB
   work instead of leaking goroutines/connections.
8. **QueryRowContext** — runs a statement expecting one row, returns a
   `*sql.Row`.
9. **Scan** — copies the single row's columns into the provided pointers,
   aligning types and nullability.
10. **sql.ErrNoRows** — sentinel returned by Scan when zero rows matched;
     repository translates it to its own domain sentinel.
11. **Repository must not know HTTP** — storage is transport-agnostic;
     coupling it to gin means every future caller (worker, CLI, test) inherits
     an HTTP dependency.
12. **Service must not know gin** — keeps business logic testable in pure Go
     and portable across delivery mechanisms.
13. **Password never in response** — it is a credential; the DTO simply does
     not contain the field (structural, not just "convention").
14. **DTO** — explicit output shape decouples API contract from storage schema
     and prevents accidental leaks (e.g. a future added column).
15. **Full journey** — described in Request Flow above; parse → service rule →
     repo query → scan → classify → serialize.