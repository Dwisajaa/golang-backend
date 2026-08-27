# Database Schema

## Source of Truth

Laravel `C:\V1\api-dwidev` migrations are the authoritative schema. Cross-checked
against `docs/database-audit.md`, `docs/database-erd.md`, and the Laravel
models/controllers. This repo's SQL replicates that FINAL schema exactly.

Synced 2026-08-27. Discrepancies found during this phase (see bottom) were
documented and the audit docs corrected.

## Domain Tables (15 migrated)

| # | Table | Purpose |
|---|---|---|
| 1 | users | Accounts, roles, auth |
| 2 | email_verification_otps | OTP records (email verification + password reset) |
| 3 | service_categories | Catalog categories |
| 4 | services | Catalog services (DECIMAL price) |
| 5 | packages | Catalog packages |
| 6 | package_items | Package ↔ Service pivot |
| 7 | customer_profiles | Customer details |
| 8 | bookings | Orders (state machine) |
| 9 | booking_items | Booking line items (price snapshot) |
| 10 | invoices | Billing (1:1 booking) |
| 11 | payments | Payment attempts + proof |
| 12 | technician_profiles | Technician details |
| 13 | booking_assignments | Technician job assignments |
| 14 | notifications | Database notifications (UUID PK) |
| 15 | reviews | Customer reviews |

## Framework Tables (decision)

| Table | Migrate? | Why |
|---|---|---|
| personal_access_tokens | **YES** | Go auth replicates Sanctum (token row + expiry) |
| password_reset_tokens | No | Go uses OTP flow, not Laravel token flow |
| sessions | No | Web-session driver not used by API token auth |
| cache / cache_locks | No | Laravel cache internals; Go has no matching need |
| jobs / job_batches / failed_jobs | No | Laravel database queue; Go uses an in-process worker |

7 tables intentionally not migrated. `schema.Validate` reports any of them as
unexpected if they ever appear in a Go database (guards against drift).

## Migration Order

Filename order (FK-safe):

```
001_users
002_email_verification_otps
003_service_categories
004_services
005_packages
006_package_items
007_customer_profiles
008_bookings
009_booking_items
010_invoices
011_payments
012_technician_profiles
013_booking_assignments
014_notifications
015_reviews
016_personal_access_tokens
```

Parents always precede children so every `CONSTRAINT ... REFERENCES` resolves
on first run.

## Primary Keys

- 15 tables: `id BIGINT UNSIGNED AUTO_INCREMENT`
- `notifications`: `id CHAR(36)` (UUID, generated app-side — Laravel/Eloquent
  generates the UUID, MySQL has no default)

## Foreign Keys (19)

| Child | Column | Parent | ON DELETE |
|---|---|---|---|
| email_verification_otps | user_id | users.id | CASCADE |
| services | service_category_id | service_categories.id | CASCADE |
| package_items | package_id | packages.id | CASCADE |
| package_items | service_id | services.id | CASCADE |
| customer_profiles | user_id | users.id | CASCADE |
| bookings | customer_id | users.id | CASCADE |
| booking_items | booking_id | bookings.id | CASCADE |
| booking_items | service_id | services.id | **SET NULL** |
| booking_items | package_id | packages.id | **SET NULL** |
| invoices | booking_id | bookings.id | CASCADE |
| payments | invoice_id | invoices.id | CASCADE |
| payments | verified_by | users.id | **SET NULL** |
| technician_profiles | user_id | users.id | CASCADE |
| booking_assignments | booking_id | bookings.id | CASCADE |
| booking_assignments | technician_id | users.id | CASCADE |
| booking_assignments | assigned_by | users.id | **SET NULL** |
| reviews | booking_id | bookings.id | CASCADE |
| reviews | customer_id | users.id | CASCADE |
| reviews | technician_id | users.id | CASCADE |

SET NULL preserves audit/snapshot data (booking_items keep name+price after
catalog deletion; payments keep verified_by-absence after admin deletion).

## Unique Constraints (13)

| Table | Column |
|---|---|
| users | email |
| service_categories | slug |
| services | slug |
| packages | slug |
| customer_profiles | user_id |
| bookings | booking_code |
| invoices | booking_id |
| invoices | invoice_number |
| payments | payment_code |
| technician_profiles | user_id |
| technician_profiles | technician_code |
| reviews | booking_id |
| personal_access_tokens | token |

## Indexes

Non-unique / composite indexes (13 non-unique total; 11 composite):

| Table | Index | Columns |
|---|---|---|
| users | users_role_index | role |
| email_verification_otps | user_expires | (user_id, expires_at) |
| email_verification_otps | lookup | (user_id, type, used_at, expires_at) |
| package_items | package_service | (package_id, service_id) |
| bookings | status | status |
| bookings | customer_created | (customer_id, created_at) |
| booking_items | booking_type | (booking_id, item_type) |
| invoices | status | status |
| payments | status_invoice | (status, invoice_id) |
| booking_assignments | status_technician | (status, technician_id) |
| booking_assignments | booking_id | booking_id |
| personal_access_tokens | tokenable | (tokenable_type, tokenable_id) |
| personal_access_tokens | expires_at | expires_at |
| notifications | notifiable | (notifiable_type, notifiable_id) |
| notifications | read_at | read_at |
| notifications | notifiable_created | (notifiable_type, notifiable_id, created_at) |
| reviews | status_technician | (status, technician_id) |

No additional indexes were added beyond Laravel's. Potential future
optimizations are listed under Future Optimization below — none are
implemented without benchmark backing.

## Enum / Status (DB representation = VARCHAR, NOT MySQL ENUM)

| Column | Allowed values | Default | Length |
|---|---|---|---|
| users.role | customer, technician, admin, super_admin | customer | VARCHAR(30) |
| email_verification_otps.type | email_verification, password_reset | email_verification | VARCHAR(30) |
| bookings.status | pending_payment, waiting_verification, paid, confirmed, technician_assigned, in_progress, awaiting_verification, completed, cancelled | pending_payment | VARCHAR(255) |
| invoices.status | unpaid, pending_payment, paid, cancelled, expired | unpaid | VARCHAR(255) |
| payments.status | unpaid, waiting_verification, pending, paid, rejected, failed, expired, refunded, cancelled | waiting_verification | VARCHAR(20) |
| booking_assignments.status | pending, accepted, rejected, completed | pending | VARCHAR(20) |
| reviews.status | published, hidden, rejected | published | VARCHAR(20) |

Transition logic is NOT in the schema (Laravel enforces it in
`Booking::transitionTo()` + controllers). It will be reimplemented in the Go
service layer (see `docs/go-architecture.md` §11).

## Decimal / Money

- 11 monetary columns, all `DECIMAL(12,2)` (int64-cents in Go, see FASE 3):
  services.price, packages.price, bookings.subtotal/additional_cost/total_price,
  booking_items.unit_price/subtotal, invoices.subtotal/additional_cost/total_amount,
  payments.amount
- 2 coordinate columns `DECIMAL(10,7)`: bookings.latitude, bookings.longitude
- No FLOAT/DOUBLE anywhere monetary.

## UUID

- Only `notifications.id` is UUID: `CHAR(36)` PK, generated v4 app-side
  (Laravel generates; Go will use `github.com/google/uuid`). NOT auto-increment.

## Migration Execution

`internal/db.Migrator` + `cmd/migrate`:

```
go run ./cmd/migrate   # applies migrations/*.sql, records schema_migrations
```

- Tracks applied versions in a `schema_migrations` table (version PK,
  applied_at) — each `.sql` file runs exactly once.
- Deterministic order: filenames sort lexically (001..016).
- **Not transactional**: MySQL DDL implicitly commits, so a failing file may be
  partially applied. The runner stops, reports `file statement N`, and a human
  resolves before retrying.
- Failure handling: creates `schema_migrations` first, re-runs only missing
  files; a failed run never marks the file applied.

## Schema Validation

`internal/schema` holds the expected schema (`schema.Expected`) and:

- Unit consistency checks (no DB needed): FK targets exist + are NOT NULL keys,
  index columns exist, per-table uniqueness of index/FK names, counts vs audit
  (16 tables / 19 FK / 13 unique / 11 money / 11 composite).
- Live validation `schema.Validate(ctx, db)` against `information_schema`
  (table, column type + nullability, PK, unique/index name + column order,
  FK column/reference/delete rule). Extra unexpected tables are reported.

Live check requires a disposable test DB:

```
TEST_DATABASE_URL="user:pass@tcp(host:3306)/dbname?..." go test ./internal/schema -run TestLiveSchemaValidation -v
```

LIMITATION: column DEFAULT comparison is not performed (MySQL formats numeric
defaults version-dependently); documented as a bounded limitation.

## Production Safety

### Migration Safety Rules

- Migrations are **forward-only** and **never destructive**: no DROP,
  TRUNCATE, DELETE, or schema changes that lose data.
- Never run against a database containing production data without a verified
  backup (mysqldump, see Laravel runbook §9).
- `cmd/migrate` targets whatever `DATABASE_*` env points at — point it at a
  fresh/disposable MySQL for tests and a staged DB for rollouts.
- Laravel's own database remains untouched — Go migrations are for the Go
  database only; the two never share a schema audit table.

## Future Optimization (not implemented)

- Composite index on `bookings(status, booking_date)` if admin filtering by
  date+status dominates query plans.
- An index on `notifications(notifiable_id, read_at)` if read-state filtering
  pattern emerges.
- Migrations land only after the benchmark phase justifies them.

## Discrepancies Corrected This Phase

| Source | Audit doc said | Actual | Action |
|---|---|---|---|
| money cols | 12 | 11 DECIMAL(12,2) | `database-audit.md` corrected; schema test asserts 11 |
| composite indexes | 7 | 11 | corrected; schema test asserts 11 |
| tokens.name | TEXT | TEXT (verified against Laravel migration) | no change |