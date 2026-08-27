# Customer Profile

Phase 7C-2 deliverable. Authenticated customer profile read + create-or-update,
preserving Laravel behavior (role-gated to customers).

## Contract (audited from Laravel)

| Method | Path | Auth | Role | Request | Response |
|---|---|---|---|---|---|
| GET | `/api/customer-profile` | Bearer | customer | — | 200 `{"data": Resource | null}` |
| PUT | `/api/customer-profile` | Bearer | customer | `{full_name,phone,address,city,postal_code}` | 200 `{"message":"Customer profile updated successfully.","data":Resource}` |

Routes live in Laravel's `role:customer` group. Go replicates with
`middleware.Auth` + `middleware.RequireRole(model.RoleCustomer)` (new — the
role middleware is required for this endpoint's authorization parity).

### Behavior parity

- **GET**: `$user->customerProfile` → `{"data": Resource}` or `{"data": null}`
  when the profile does not exist (NOT 404).
- **PUT**: `updateOrCreate(['user_id' => $user->id], validated)` — first write
  creates, later writes update.
- Resource fields: `id, user_id, full_name, phone, address, city, postal_code,
  is_complete` (`is_complete` = all four required fields non-empty).

### Validation parity (UpdateCustomerProfileRequest)

`full_name` required/max255 · `phone` required/max20 · `address`
required/max255 · `city` required/max100 · `postal_code` nullable/max10.

## Model

`internal/model/customer_profile.go` — mirrors the migration: id, user_id
(unique), full_name, phone, address, city, postal_code (nullable), timestamps;
`IsComplete()` mirrors the model helper.

## Request Flow

```
GET/PUT /api/customer-profile
  ↓ Router (customer group)
  ↓ middleware.Auth: Bearer → token → user (context)
  ↓ middleware.RequireRole: role must be customer → else 403 "Forbidden."
  ↓ CustomerProfileHandler: CurrentUser(c) → decode → validate
  ↓ CustomerProfileService: identity = context user id (never client input)
  ↓ CustomerProfileRepository: SELECT / INSERT..ON DUPLICATE (tx)
  ↓ MySQL
  ↑ Service → DTO → JSON
```

## Architecture

### Repository (`internal/repository/customer_profile*.go`)
- `FindByUserID(ctx, q, userID)` → profile or `ErrNotFound`.
- `Upsert(ctx, q, p)` — single `INSERT ... ON DUPLICATE KEY UPDATE`:
  the unique `user_id` constraint is the atomic switch (Laravel `updateOrCreate`
  is read-then-write and can race; the single statement cannot duplicate).
- All parameterized; no HTTP knowledge; takes `Queryer` for tx membership.

### Service (`internal/service/customer_profile.go`)
- `GetByUserID` — `ErrNotFound` → `(nil, nil)` (produces `data:null`, parity).
- `Upsert` — builds the model (empty postal_code → NULL), inserts-or-updates in
  a transaction, then reads back the id for the DTO. Ownership is implied by
  using the authenticated `userID`; there is nothing client-controlled.
- Errors → `httperr.Internal` for DB failures.

### Handler + DTO
Decode → structural validation (mirror FormRequest) → service → DTO.
`customerProfileData` matches the Laravel Resource field-for-field (incl.
`is_complete`); no secret/internal fields.

## Transaction / Concurrency

- Writes run inside `TxManager` (`txRunner`) with Begin/Rollback-defer/Commit.
- No external side effects inside the transaction.
- Create-vs-update races are handled by the DB-level `UNIQUE(user_id)` —
  documented; no additional locking needed for a single upsert statement.

## Error Mapping

| Case | Status | Body |
|---|---|---|
| Unauthenticated | 401 | `{"message":"Unauthenticated."}` |
| Wrong role | 403 | `{"message":"Forbidden."}` |
| Validation | 422 | `{"message":"The given data was invalid.","errors":{...}}` |
| Missing profile (GET) | 200 | `{"data":null}` (parity) |
| DB/internal | 500 | `{"message":"Server error."}` + log with request_id |

## Security

- Auth middleware + role gate; identity from context only.
- Parameterized SQL; request context propagated; no global state.
- No password/token/secret fields in the DTO; no SQL/driver detail to client.

## Laravel → Go Parity

| Laravel | Go | Status |
|---|---|---|
| GET/PUT paths + method | same | MATCH |
| role:customer restriction | RequireRole | MATCH |
| GET data:null when missing | same | MATCH |
| updateOrCreate | INSERT..ON DUPLICATE (single stmt) | MATCH (Go race-safe) |
| validation rules | structural mirror | MATCH |
| resource fields + is_complete | DTO | MATCH |
| message/status | identical | MATCH |

## Testing

- Service (`internal/service/customer_profile_test.go`): missing → nil not
  error; found; upsert creates; upsert updates (postal NULL↔value); repo error → 500.
- Handler (`internal/httphandler/customer_profile_test.go`) via real
  middleware + role gate: GET found / data:null / 401 / **403 wrong role**;
  PUT success / 422 / 400 malformed; 500 generic.
- Role middleware (`internal/middleware/role_test.go`): allow, wrong-role 403,
  multi-role allow.
- Repository (`internal/repository/customer_profile_mysql_test.go`, gated by
  `TEST_DATABASE_URL`): ErrNotFound on missing; upsert create; upsert update;
  exactly one row (unique respected).

Commands: `go test ./...`, `go test -race ./...` (use `-p 1` on this machine —
MinGW linker OOMs during parallel race builds), optional
`TEST_DATABASE_URL=... go test ./internal/repository -v`.

## Known Limitations

- `postal_code` empty string is stored as NULL (Laravel `nullable` rule would
  keep an empty string if provided; here request coercion treats empty as
  absent) — documented; contract tests (FASE 11) will pin whichever Laravel
  truly persists via `validated()`.
- First-ever role middleware introduced here (required by this endpoint's
  authorization parity).