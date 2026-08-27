# Auth Middleware

Phase 7B-2 deliverable. Sanctum-equivalent Bearer authentication middleware.

## 1. Purpose

Authenticates a request ("who is this user?") before it reaches a protected
handler. Authentication only — role authorization and resource policies stay in
separate layers (FASE 7B+).

## 2. Request Flow

```
HTTP Request
  ↓ Gin middleware chain (recovery, request id, logging, headers)
  ↓ Auth middleware:
  │   read Authorization header
  │   validate "Bearer <token>" format
  │   SHA-256(raw) → look up token row
  │   check expires_at (UTC)
  │   load owning user by tokenable_id
  │   store user (and raw token for logout) in request context
  ↓ c.Next() → handler
```

Any failure calls `c.AbortWithStatusJSON(401, ...)`; the handler is never
reached.

## 3. Bearer Authentication

Only `Authorization: Bearer <token>` is accepted (Laravel/Sanctum parity —
scheme must be exactly `Bearer `). Missing header, wrong scheme, empty token →
401.

## 4. Token Hashing

The raw token is hashed once via the shared `TokenHasher` (SHA-256) and only
the hash is used for the DB lookup. Raw token lives only in the request context
during that request and is never logged or persisted.

## 5. Database Lookup

`repository.TokenStore.FindByTokenHash(ctx, hash)` — parameterized
`WHERE token = ?`; `sql.ErrNoRows` → `repository.ErrNotFound` → 401. The repo
scans the full row (id, tokenable*, name, token, abilities, last_used_at,
expires_at, timestamps).

## 6. Expiration

`expires_at <= now(UTC)` → 401. NULL `expires_at` (never-expiring) stays valid,
matching Sanctum. All comparison in UTC; no local timezone.

## 7. Authenticated User Context

`Auth` stores `*model.User` (id, name, email, role, email_verified_at, …) and
the raw token into request-scoped gin context keys. Helpers:

```go
middleware.CurrentUser(c)      (*model.User, bool)
middleware.CurrentRawToken(c)  (string, bool)
```

No global/singleton state. Each request gets its own context — concurrent
requests never share the authenticated user (covered by a dedicated test under
`-race`).

## 8. Error Handling

| Failure | Response |
|---|---|
| header missing / not Bearer / empty | 401 `{"message":"Unauthenticated."}` |
| token not found | 401 `{"message":"Unauthenticated."}` |
| token expired | 401 `{"message":"Unauthenticated."}` |
| token owner (user) not found | 401 `{"message":"Unauthenticated."}` |
| DB/internal failure | 500 `{"message":"Server error."}` + server log |

The body mirrors the audited Laravel `bootstrap/app.php` mapping
(AuthenticationException → 401 "Unauthenticated."). Authentication failures are
category-generic — the client cannot distinguish token-present-but-invalid
cases, matching Laravel. Auth failures are 401, never 403 (403 is reserved for
authorization).

Security-event logging uses broad categories only ("missing or malformed
Authorization header", "token not found", "token expired", "token owner not
found") plus request_id/method/path/client_ip — never the token, hash, or
password.

## 9. Security

- raw token only lives during one request; DB stores SHA-256 only.
- expired and revoked tokens are rejected.
- token/hash never in logs or responses.
- parameterized queries only.
- no global authentication state; request-scoped context.
- passwords are never used as tokens.
- internal error detail never reaches the client.

## 10. Dependency Injection

```go
middleware.Auth(tokenStore TokenStore, users UserFinder, hasher TokenHasher) gin.HandlerFunc
```

All dependencies are injected by `cmd/api/main.go` (composition root):
`repository.NewMySQLTokenStore(pool)`, `repository.NewMySQLUserRepository(pool)`,
and the shared `auth.RandomTokenGenerator`. No package-level store.

## 11. Testing Strategy

`internal/middleware/auth_test.go` (a fake token store + fake user finder +
`httptest`; no MySQL):

- valid token → handler runs, `CurrentUser` correct
- missing header → 401, handler not called
- `Basic`, `Bearer` empty, lowercase scheme → 401
- unknown token → 401
- expired token → 401
- valid token but owner missing → 401
- generic body, no token material in response
- concurrent requests keep users isolated (runs under `-race`)

Repository integration for `FindByTokenHash` is gated by `TEST_DATABASE_URL`
(skip with a clear message otherwise); it verifies found, not-found, expires_at
scanning, and revocation.

## Wiring

`POST /api/logout` is now protected by the middleware and revokes the current
token (contract: `200 {"message":"Logout successful"}` via
`AuthService.RevokeToken`). Other authenticated endpoints arrive in later
phases.

## 12. Fundamental Go Concepts

1. **Middleware** — a function wrapping the next handler; it runs before and
   can decide whether the handler runs at all.
2. **Middleware runs first** — request passes through the chain in order, so a
   middleware can authenticate before business code touches the request.
3. **`c.Next()`** — continue the chain (call the next middleware/handler).
4. **Without `c.Next()`** — chain stops; if we already wrote a response
   (`AbortWithStatusJSON`), nothing else runs.
5. **`context.Context`** — outlives a request for deadlines/cancellation; here
   it flows into DB calls.
6. **`context.Context` vs `gin.Context`** — gin.Context is HTTP-specific
   (request/response/params); context.Context is the portable cancellation
   value passed to DB/services. We use gin context for the user + raw token
   (HTTP-scoped) and request context for DB.
7. **Request-scoped user** — each request owns its data; no sharing across
   requests, so concurrency is safe and a slow request cannot expose another
   user.
8. **Global variables are dangerous** — HTTP servers handle many concurrent
   requests; a global current-user would be overwritten and leak between
   requests (race). Everything is per-request.
9. **Why hash the raw token** — a DB leak must not leak usable tokens.
10. **SHA-256 for lookup** — deterministic (find by hash) yet one-way; suitable
    because tokens are high-entropy secrets (bcrypt's salt+work would prevent
    indexed lookup and add nothing for random 238-bit tokens).
11. **bcrypt vs SHA-256** — passwords are low-entropy human guesses, so
    bcrypt's salt + expensive work-rate limits brute force; random tokens are
    already unguessable, so a cheap one-way hash is enough.
12. **Reject expired tokens** — an expired token is a stolen/lapsed credential;
    accepting it would break revocation intent.
13. **HTTP 401** — "unauthenticated": the request lacks or has bad credentials.
14. **401 vs 403** — 403 = authenticated but not allowed. (Role/policy phases
    will use 403.)
15. **DI in middleware** — the middleware receives its collaborators in the
    constructor; tests inject fakes, prod injects MySQL-backed stores.
16. **Middleware must not contain domain logic** — it stays a transport-level
    gate; business rules live in services.
17. **Repository must not know HTTP** — it returns domain values/errors; the
    middleware maps those to statuses.
18. **Request context into DB** — cancel/expire work together; a disconnected
    client aborts the DB query.
19. **One request end-to-end** — router → middleware (auth gate) → handler →
    service → repository → DB, then unwinding back out.
20. **Concurrent safety** — no package-level mutable state; per-request
    context values; verified by the concurrency test under `-race`.