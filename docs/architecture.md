# Architecture (Final)

Actual Go backend architecture after phases 7A–15. Laravel behavior is the
reference; Go mirrors it with typed errors, `TxManager`, `FOR UPDATE`, exact
integer-cents money, and a post-commit notification boundary.

## Request Flow

```
HTTP Request
  ↓ Gin Router (/api, role groups)
  ↓ Middleware: Recovery → RequestID → Logging → SecurityHeaders
  ↓ Auth (bearer → personal_access_tokens SHA-256) → CurrentUser context
  ↓ Role middleware (customer | technician | admin,super_admin) → 403
  ↓ Handler (decode → structural validate → service → DTO → JSON)
  ↓ Service (business rules, state machines, policy, TxManager)
  ↓ Repository (parameterized SQL on Queryer)
  ↓ MySQL 8.0
```

## Dependency Direction

```
cmd/api (composition root)
  → router · middleware
  → httphandler (thin; depends on service interfaces)
  → service (business rules; depends on repository interfaces, notify.Notifier)
  → repository (SQL persistence; Queryer)
  → db (TxManager, pool)
  → MySQL/min storage · config

model  (pure structs)  ← used by repository/service/handler
notify · storage · auth · mailer   ← injected, not imported by business paths
```

Rules enforced throughout: repository never imports gin; service never imports
gin/net/http; handler never runs SQL or owns transactions; identity always from
`middleware.CurrentUser(c)`.

## Packages

| Package | Responsibility |
|---|---|
| `internal/config` | env → typed Config (DB, SMTP, OTP, storage) |
| `internal/db` | `database/sql` pool + `TxManager.Within` |
| `internal/model` | domain structs + status constants + `CanTransition` maps |
| `internal/repository` | stores per aggregate; `Queryer` (DB/Tx), sentinels (ErrNotFound/ErrDuplicate…) |
| `internal/service` | business rules, policies, state machines, TxManager orchestration, post-commit notify |
| `internal/httphandler` | decode → validate → service → DTO; `respond.go` error mapping |
| `internal/middleware` | Auth (sanctum-equivalent), RequireRole, RequestID, Logging, SecurityHeaders |
| `internal/auth` | bcrypt hasher, token generator (crypto/rand + SHA-256), OTP generator |
| `internal/mailer` | Mailer interface, SMTP, LogMailer, async worker (OTP) |
| `internal/notify` | Notifier interface, DBNotifier (post-commit database notifications) |
| `internal/storage` | private disk storage (path-traversal-safe) |
| `internal/httperr` | typed errors → HTTP status/JSON |

## Money

JSON → `big.Rat` → int64 cents → `DECIMAL(12,2)` → cents → `"12345.67"`
string DTO. No float64 in any monetary arithmetic (verified by audit).

## State Machines

- `model.BookingTransitions` (9 states) + `CanTransition` — used by cancel,
  payment cascade, workflow, verify.
- `model.PaymentTransitions` + `IsPaymentPendingVerification` — verify/reject.
- Assignment statuses via constants + `IsActiveAssignment`.

## Transactions & Locking

`TxManager.Within`: Begin → fn(tx) → Commit; `defer Rollback`. Row locks and
canonical orders (audited, no deadlock cycle):

| Flow | Locks | Order |
|---|---|---|
| Booking create | catalog service/package | — |
| Payment upload | invoice → booking | invoice then booking |
| Payment verify/reject | payment → invoice → booking | payment-first |
| Booking cancel | booking | alone |
| Assignment assign | booking (+reads invoice) | booking-first |
| Technician workflow | assignment | assignment-only |
| Booking verify | booking → assignment | booking-first |
| Review create | booking | alone |

No flow locks `assignment → booking`, so the booking-first ordering used by
assign/verify cannot cycle with the assignment-only workflow.

## Post-Commit Side Effects

- Notification dispatch (database channel) happens strictly after COMMIT;
  failure is logged, never rolls back business.
- Payment-proof file is written after the DB transaction commits (documented
  INTENTIONAL IMPROVEMENT).

## Observability

`log/slog` JSON; RequestID correlation; middleware request log; 5xx generic
`{"message":"Server error."}` + detail logged with request_id.

## Configuration

`.env.example` documents: APP_ENV/APP_PORT, DATABASE_*, OTP_*, SMTP_*,
STORAGE_PAYMENT_PROOFS_PATH. No secrets committed; `.env*` git-ignored.