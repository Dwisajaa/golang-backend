# Authentication Core

Phase 7B-1 deliverable. Pure Go auth primitives preserving Laravel/Sanctum
behavior; no middleware, no OTP yet.

## Scope

- Password hashing + verification (bcrypt, cost 12)
- User lookup by email, user create
- Cryptographically secure bearer token generation + SHA-256 hashing
- Personal access token persistence (create / revoke)
- `POST /api/register`, `POST /api/login`
- Revoke-by-token core (logout route deferred: needs auth middleware, FASE 7B-2)

Out of scope this phase: auth middleware, authenticated endpoints, OTP/email
verification, password reset/forgot, profiles, catalog, bookings.

## Laravel Behavior Source

Verified directly against `C:\V1\api-dwidev`:

- `app/Http/Controllers/Api/AuthController.php` (register/login/logout)
- `app/Http/Requests/RegisterRequest.php`, `LoginRequest.php`
- `app/Models/User.php` (role default, hasVerifiedEmail)
- Sanctum token semantics: 40-char random token, DB stores SHA-256 (64 hex).

### register (`POST /api/register`)
- Request: name, email, password, password_confirmation
- Rules: required | email unique | password min:8 confirmed
- Response `201`:
  ```json
  {"message":"Registration successful. Verification code sent to your email.","requires_verification":true,"user":{...}}
  ```
- role defaults to `customer`. Side effect `sendOtp()` is **deferred to FASE
  7B-3** — the response still carries the same message/flag (contract parity).

### login (`POST /api/login`)
- wrong email/password → `401 {"message":"The provided credentials are incorrect."}` (generic, no user enumeration)
- unverified email (email_verified_at NULL) → `403 {"message":"Email belum diverifikasi.","requires_verification":true,"user":{id,name,email,email_verified_at}}`
- success → `200 {"message":"Login successful","user":{...},"token":"<raw>","token_type":"Bearer"}`
- token name `mobile-app`, abilities default, expiry = `SANCTUM_TOKEN_EXPIRATION` (default 7 days).

### logout
- deletes the current token; if none, still returns `{"message":"Logout successful"}` 200.
- Go `RevokeToken(ctx, rawToken)` implements the core (delete by SHA-256 hash,
  missing row = no-op). Route wiring deferred to FASE 7B-2.

## Password Hashing

`internal/auth/password.go`:

```go
type PasswordHasher interface {
    Hash(password string) (string, error)
    Compare(hash, password string) error
}
```

`BcryptHasher` → bcrypt cost 12 (FASE 3 security decision; Laravel
`BCRYPT_ROUNDS=12`). Hashing lives in the **service** layer, not the
repository: it is application policy that every write path must honor, and it
keeps storage dumb (never sees plaintext policy). Go's bcrypt accepts the
`$2y$` prefix Laravel/PHP produces (maps to `$2a$`), so hashes stored by
Laravel verify correctly in Go — important for a phased cutover.

## User Lookup

`repository.UserStore.FindByEmail(ctx, email)` — parameterized
`WHERE email = ?`, `sql.ErrNoRows` → `repository.ErrNotFound`. `Create` inserts
and backfills the auto-increment ID; MySQL 1062 → `repository.ErrDuplicateEmail`.

## Token Generation

`internal/auth/token.go`: 40 random characters from a 62-char alphabet drawn
via `crypto/rand.Int` (not `math/rand`, not timestamp) → ~238 bits of entropy.
`Generate()` returns `(raw, hash, err)`.

## Token Hashing

Only `SHA-256(raw)` (hex, 64 chars) is stored. If the DB leaks, an attacker
cannot replay tokens — they only have a one-way digest, and the raw token is
never logged anywhere.

## Personal Access Tokens

`internal/repository` TokenStore:

```go
type TokenStore interface {
    Create(ctx context.Context, t *model.PersonalAccessToken) error
    RevokeByTokenHash(ctx context.Context, tokenHash string) error
}
```

Stored row: tokenable_type=`App\Models\User`, tokenable_id=user id,
name=`mobile-app`, token=SHA-256 hash, expires_at=+7d UTC, timestamps.

## Register Flow

```
POST /api/register
  Handler  : decode + structural validate (mirror FormRequest) → 422 on error
  Service  : hasher.Hash(plaintext) → user.Create(hashed) → duplicate → 422(email taken)
  Response : 201 {message, requires_verification:true, user}
```
No transaction: Laravel `User::create` is a single insert (no DB::transaction
wrapper) — documented, nothing to wrap.

## Login Flow

```
POST /api/login
  decode/validate
  FindByEmail → ErrNotFound → generic 401
  hasher.Compare → mismatch → generic 401
  EmailVerifiedAt == nil → EmailUnverifiedError (403 + user payload)
  generator.Generate() → raw, hash
  token.Create(hash, +7d) → 200 {message, user, token:raw, token_type:Bearer}
```

Errors `ErrNotFound` and wrong-password map to the **same** generic response —
no email-enumeration oracle, matching Laravel's `if (!$user || !Hash::check(...))`.

## Logout Core

`AuthService.RevokeToken(ctx, raw)` → `generator.Hash(raw)` →
`tokenStore.RevokeByTokenHash(ctx, hash)`. Missing row → success (Laravel
parity). Endpoint wiring waits for the auth middleware (FASE 7B-2) because
logout identifies the token from the request.

## Error Handling

- `httperr.Validation(errors)` → 422 `{"message":"The given data was invalid.","errors":{field:[...]}}` (Laravel shape)
- `InvalidCredentialsError` → 401 generic
- `EmailUnverifiedError` → 403 with user payload (handler-specific body)
- `KindInternal` → 500 `{"message":"Server error."}`; driver/SQL/bcrypt text
  only in server logs (with request_id via `respondError`).

## Security

- bcrypt cost 12; salts random → identical passwords hash differently.
- raw token: 40 chars, crypto/rand; **never persisted, never logged**.
- DB stores SHA-256 only; token expiry 7 days; revoke on logout.
- password/token fields structurally excluded from every DTO.
- parameterized SQL everywhere; no DSN/stack/bcrypt internals to clients.

## Testing

| Level | File | Notes |
|---|---|---|
| Password | `internal/auth/password_test.go` | hash/compare/salt-differs/cost |
| Token | `internal/auth/token_test.go` | uniqueness, determinism, raw≠hash, length |
| Service | `internal/service/auth_test.go` | register (hash stored, dup → 422, err → 500), login (raw returned, hash stored, generic wrong/unknown, unverified, token-persist fail), revoke |
| Handler | `internal/httphandler/auth_test.go` | 201 body, validation 422 shape, malformed JSON 400, dup email, login 200 token, 401 generic, 403 unverified, no password/token_hash leakage |
| Repository | `internal/repository/*_test.go` | gated by `TEST_DATABASE_URL`, skip otherwise |

```
go test ./...
go test -race ./...
TEST_DATABASE_URL=... go test ./internal/repository -v
```

## Request Flow (register)

```
HTTP POST /api/register
  Router (gin, middleware: recovery/requestid/logging/headers)
  AuthHandler.Register: decode → validate → service
  AuthService.Register: hasher.Hash → userStore.Create
  MySQL: INSERT users(...,password=bcrypt)
  ← row id backfilled
  ← registerResponse{message, requires_verification, user}
  JSON 201
```

## Deferred Work (FASE 7B-2/7B-3)

- FASE 7B-2: auth middleware (Bearer → resolve token → user in context),
  `POST /api/logout`, authenticated user endpoint.
- FASE 7B-3: OTP + email verification (register currently behaves as Laravel's
  unverified flow: login → 403 until verified), OTP email sender.

## Known Limitations

- Malformed JSON → 400 `{"message":"Invalid JSON payload."}` (approximation of
  Laravel's JSON-decode 400; not byte-identical).
- Email validation uses a pragmatic regex mirroring Laravel's `email` rule; not
  identical to PHP's filter — documented for FASE 11 contract tests.
- Login enforces the unverified-email 403 now (parity), so a user registered on
  Go cannot log in until FASE 7B-3 verification exists — faithful to Laravel.