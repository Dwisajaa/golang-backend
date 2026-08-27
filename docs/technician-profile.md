# Technician Profile

Phase 7C-3 deliverable. Authenticated technician profile read + create-or-update,
preserving Laravel behavior (role-gated to technicians).

## Responsibility

Own the technician's own profile row (1:1 with `users`). Identity is always the
authenticated user id from the auth middleware context — never from the client.

## Contract (audited from Laravel)

| Method | Path | Auth | Role | Request | Response |
|---|---|---|---|---|---|
| GET | `/api/technician/profile` | Bearer | technician | — | 200 `{"data": Resource}` or **404** `{"message":"Technician profile not found."}` |
| PUT | `/api/technician/profile` | Bearer | technician | `{phone,specialization,address,bio}` | 200 `{"message":"Technician profile updated successfully.","data":Resource}` |

### Behavior parity (TechnicianController)

- **GET**: `$user->technicianProfile`; missing → 404 (unlike customer profile's
  `data:null`).
- **PUT**: `firstOrCreate(['user_id' => id], ['technician_code' =>
  generateTechnicianCode()])` then `update($validated)`.
- Code format: `TECH-` + `random_int(1,9999)` left-padded to 4 — regenerated
  while a collision exists.
- Update touches only the validated fields; `is_active` stays as created (default
  true) and is never set by this endpoint.
- Resource fields: `id, technician_code, phone, specialization, address, bio,
  is_active`.

### Validation parity (UpdateTechnicianProfileRequest)

`phone` nullable/max20 · `specialization` nullable/max255 · `address`
nullable/max255 · `bio` nullable/max2000.

## Model

`internal/model/technician_profile.go` — mirrors the migration: id, user_id
(unique), technician_code (unique), nullable contact fields, is_active bool,
timestamps. `TechnicianCodePrefix` = "TECH-".

## Request Flow

```
GET/PUT /api/technician/profile
  ↓ Router (technician group)
  ↓ middleware.Auth: Bearer → token → user (context)
  ↓ middleware.RequireRole(model.RoleTechnician): else 403 "Forbidden."
  ↓ TechnicianProfileHandler: CurrentUser(c) → decode → validate
  ↓ TechnicianProfileService: identity = context user id
  ↓ TechnicianProfileRepository: SELECT / INSERT / UPDATE (tx)
  ↓ MySQL
  ↑ Service → DTO → JSON
```

## Architecture

### Repository (`internal/repository/technician_profile*.go`)
- `FindByUserID` → profile or `ErrNotFound`.
- `InsertProfile` — plain INSERT; unique violations translated: technician_code
  collision → `ErrDuplicateTechnicianCode`; other 1062 (concurrent user_id) →
  `ErrDuplicate` (classified via the driver's key name, never leaked outward).
- `UpdateProfile` — refreshes only nullable profile fields + updated_at; never
  `technician_code`/`is_active`.
- All parameterized; `Queryer` for tx membership.

### Service (`internal/service/technician_profile.go`)
- `GetByUserID`: `ErrNotFound` → typed 404 with Laravel's message.
- `UpdateProfile` (tx):
  - exists → `UpdateProfile`;
  - missing → generate `TECH-XXXX` (`crypto/rand`, exactly like Laravel's
    numeric range) and `InsertProfile`; on code collision regenerate (bounded,
    mirroring Laravel's `do…while (exists)`); on concurrent `user_id` duplicate
    re-read and fall back to update. Empty optional fields stored as NULL.

### Handler + DTO
Decode → mirror validation → service → DTO matching the Laravel Resource
field-for-field. No secrets/internal fields.

## Transaction / Concurrency

- All reads/writes run via `TxManager` (`txRunner`); no external side effects
  inside a transaction.
- Concurrency: two simultaneous first-time writes cannot duplicate the profile —
  the `user_id` unique constraint plus the insert→duplicate→update fallback make
  the create-or-update race-safe (Laravel's `firstOrCreate` is read-then-write
  and can race). Code collisions are retried against the `technician_code`
  unique constraint (Laravel's existence-loop, made atomic). No `FOR UPDATE`
  needed: single-statement inserts + unique constraints give the guarantee.

## Security

Auth + role gate; identity from context only; parameterized SQL; context
propagated; DTO has no password/token/secret; driver key names never sent to
the client; no global state.

## Error Mapping

| Case | Status | Body |
|---|---|---|
| Unauthenticated | 401 | `{"message":"Unauthenticated."}` |
| Wrong role | 403 | `{"message":"Forbidden."}` |
| Missing profile (GET) | 404 | `{"message":"Technician profile not found."}` |
| Validation | 422 | `{"message":"The given data was invalid.","errors":{...}}` |
| Too many code collisions | 500 | `{"message":"Server error."}` |
| DB/internal | 500 | `{"message":"Server error."}` + log with request_id |

## Laravel → Go Parity

| Laravel | Go | Status |
|---|---|---|
| GET/PUT paths + method | same | MATCH |
| role:technician | RequireRole | MATCH |
| 404 message on missing | identical | MATCH |
| firstOrCreate + update | insert-or-update in tx | MATCH (Go race-safe, same observable behavior) |
| generateTechnicianCode | crypto/rand TECH-XXXX + retry | MATCH |
| update only validated fields | UpdateProfile (4 fields) | MATCH |
| resource fields | DTO identical | MATCH |
| message/status | identical | MATCH |

## Testing

- Service (`internal/service/technician_profile_test.go`): found, missing→404,
  create generates code, empty fields→NULL, update preserves code, code
  collision retries (inserts count), concurrent user_id fallback to update, repo
  error→500.
- Handler (`internal/httphandler/technician_profile_test.go`) via real
  middleware + role gate: GET found, 404, 401, 403 wrong role, PUT 200, 422,
  400, 500 generic.
- Repository (`internal/repository/technician_profile_mysql_test.go`, gated by
  `TEST_DATABASE_URL`): find-missing, insert (id + is_active defaults), code
  collision sentinel, update preserves code + nullable semantics.

Commands: `go test -p 1 ./...` (system linker OOMs on parallel builds),
`go test -race -p 1 ./...` (MinGW linker + paging constraints on this machine),
optional `TEST_DATABASE_URL=... go test ./internal/repository -v`.

## Known Limitations

- Code generation retries are bounded (5) — beyond that the write fails 500
  (Laravel loops without a bound; a practical guard, documented in parity).
- `postal`-style null-normalization applies to empty strings (nullable rule
  parity), same convention as customer profile.

## Migration Risks

| Risk | Severity | Notes |
|---|---|---|
| Code collision + bounded retry | LOW | practically unreachable (9999 space) |
| Concurrent first-create | LOW | unique(user_id) + fallback update |
| System linker OOM | ENV | serial `-p 1` used for test/race builds |

## Fundamental Go Concepts

Authenticated identity via middleware context; role authorization middleware;
DI constructor pattern; repository boundary (no HTTP/status); service business
rules (no gin); DTO vs model; transaction via TxManager; `crypto/rand` for
unpredictable codes vs `math/rand`; empty→NULL normalization; typed sentinel
errors for DB constraint classification; request context propagation.