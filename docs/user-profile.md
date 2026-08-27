# User Profile

Phase 7C-1 deliverable. Authenticated user profile read/update and password
change, preserving Laravel behavior.

## Contract (audited from Laravel)

Laravel exposes these under `auth:sanctum` (no role gate) — note the actual
path is `/api/profile`, **not** `/api/user` (that route does not exist in
`routes/api.php`; `AuthController@user` is not bound). FASE 7C-1 follows the
Laravel contract.

| Method | Path | Auth | Request | Response |
|---|---|---|---|---|
| GET | `/api/profile` | Bearer | — | 200 `{"user": {/*UserResource*/}}` |
| PUT | `/api/profile` | Bearer | `{name, email}` | 200 `{"message":"Profile updated successfully","user":{...}}` |
| PUT | `/api/profile/password` | Bearer | `{current_password, password, password_confirmation}` | 200 `{"message":"Password updated successfully"}` |

### Validation parity (Laravel FormRequests)

- UpdateProfileRequest: `name` required/string/max255; `email`
  required/string/email/max255/`unique:users,email ignore:self`.
- UpdatePasswordRequest: `current_password` required/`current_password`;
  `password` required/string/min8/`confirmed`.

### Behavior parity

- **Email change** (`ProfileController@update`): `email_verified_at` reset to
  NULL and a new verification OTP dispatched (async). Verified email is
  therefore invalidated until re-verified (login → 403 again).
- **Password change** (`ProfileController@updatePassword`): all personal access
  tokens are deleted (revoke-all). No email/verification side effects.

## Request Flow

```
GET/PUT /api/profile
  ↓ Router (protected group → Auth middleware)
  ↓ middleware.Auth: Bearer → SHA-256 → token row → expiry → user → context
  ↓ Handler: CurrentUser(c) (never client-supplied id) → decode → validate
  ↓ Service: rules (unique email, current-password check), tx
  ↓ Repository: parameterized UPDATE
  ↓ MySQL
  ↑ DTO → JSON
```

## Architecture

### Repositories
- `user:UpdateNameEmail(ctx, q, id, name, email, emailVerifiedAt)` — 1062 →
  `ErrDuplicateEmail`.
- `user:UpdatePassword(ctx, q, id, hash)`.
- `token:RevokeAllForUser(ctx, q, userID)` — deletes every token (morph pair).

### Service (`internal/service/profile.go`)
- `UpdateProfile(userID, {Name, Email})`:
  - load user (404 if gone — shouldn't happen behind auth),
  - duplicate-email check (ignore-self) → 422 `The email has already been
    taken.`,
  - tx: `UpdateNameEmail` (sets `email_verified_at` param; nil when email
    changed),
  - after commit, when email changed: `otpDispatcher.ResendVerificationOtp`
    (async, after tx — reuses FASE 7B-3 flow).
- `UpdatePassword(userID, current, new)`:
  - bcrypt-compare current → mismatch → 422
    `The current password field does not match your password.`,
  - hash new → tx { `UpdatePassword`, `RevokeAllForUser` } (atomic, Laravel
    parity + atomicity),
  - reads/hashes never leave the service; no tokens surviving the change.

### Handler (`internal/httphandler/profile.go`)
Decodes, validates structure, calls service, serializes DTO. Identity comes
from `middleware.CurrentUser(c)`. 401 if middleware absent.

### Transaction
Both updates use the existing `TxManager` (`txRunner`). Multi-write
(update+revoke) is atomic; OTP is dispatched strictly after commit.

## Security

- All three endpoints behind the FASE 7B-2 auth middleware (401 generic).
- Profile writes target `currentUser.ID` from context — never request body.
- Password: bcrypt-12, never in DTO/log/error; current password verified
  before any change.
- Email change invalidates verification (parity) — login re-locked until the
  new OTP is verified.
- Parameterized SQL; request context propagated; no global state.

## Error Mapping

| Case | Status | Body |
|---|---|---|
| Unauthenticated | 401 | `{"message":"Unauthenticated."}` |
| Validation (profile/password shape) | 422 | `{"message":"The given data was invalid.","errors":{...}}` |
| Duplicate email | 422 | errors.email → "The email has already been taken." |
| Wrong current password | 422 | errors.current_password → "The current password field does not match your password." |
| Not found | 404 | `{"message":"Resource not found."}` |
| DB/internal | 500 | `{"message":"Server error."}` + server log with request_id |

## Laravel → Go Parity

| Laravel | Go | Status |
|---|---|---|
| GET/PUT /profile, PUT /profile/password | same (audited) | MATCH |
| name/email validation incl. unique ignore-self | structural + service unique check | MATCH |
| current_password rule | service bcrypt compare → 422 shape | MATCH |
| password min8 confirmed | structural | MATCH |
| email change → reset verified + OTP | verifiedAt nil + ResendVerificationOtp (async) | MATCH |
| password change → delete all tokens | RevokeAllForUser in tx | MATCH (Go adds atomicity) |
| response messages/status | identical strings | MATCH |

## Testing

- Service (`internal/service/profile_test.go`): email unchanged keeps verified;
  email change resets + dispatches OTP; duplicate → 422; repo error → 500;
  wrong current → 422; success hashes (bcrypt check) + revokes all; 404.
- Handler (`internal/httphandler/profile_test.go`) through the **real auth
  middleware**: GET 200 (no password/remember_token/token); unauth 401;
  update 200; validation 422; malformed 400; password 200; wrong current 422
  (Laravel message); internal 500 generic.
- Repository (`internal/repository/user_mysql_test.go`, gated by
  `TEST_DATABASE_URL`): UpdateNameEmail + verifiedAt; duplicate → ErrDuplicate;
  UpdatePassword; RevokeAllForUser.

Run: `go test ./...`, `go test -race ./...` (use `-p 1` on this machine to
avoid MinGW linker OOM), optional
`TEST_DATABASE_URL=... go test ./internal/repository -v`.

## Known Limitations

- Email body/template parity is plain-text (subject matches) — same as
  FASE 7B-3.
- Unique-email re-check is service-side (pre-check) plus the DB unique
  constraint as the authoritative backstop (ErrDuplicateEmail) — Laravel only
  relies on the rule; Go adds the DB backstop.
- `GET /api/user` (the task's label) is documented as the audited
  `GET /api/profile` — no extra alias created (no Laravel route to back it).