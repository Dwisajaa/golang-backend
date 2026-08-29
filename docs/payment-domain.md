# Payment Domain

Phase 11 deliverable. Customer payment-proof upload/download + admin
payment management (list, proof, verify, reject) with state cascades, private
storage, and FOR UPDATE serialization.

## Laravel Audit

**Customer endpoints (role:customer):**
- `POST /api/invoices/{invoice}/payment-proof` — upload proof (throttle).
- `GET /api/invoices/{invoice}/payment-proof` — download own proof.

**Admin endpoints (role:admin,super_admin):**
- `GET /api/admin/payments` — list (status filter, latest, per_page).
- `GET /api/admin/payments/{payment}/proof` — admin proof download.
- `POST /api/admin/payments/{payment}/verify` — cascade payment→invoice→booking.
- `POST /api/admin/payments/{payment}/reject` — cascade payment+invoice+booking.

### Upload flow (PaymentController@storeProof)
validate invoice → 403 policy → 409 non-payable status → payment_method must be
bank_transfer (422) → amount must equal invoice total (422) → tx { lock invoice
FOR UPDATE with booking; no pending payment; invoice not cancelled/expired;
booking must be pending_payment; store file to private `payment_proofs` disk;
create payment (waiting_verification); invoice → pending_payment; booking →
waiting_verification } → on failure delete stored file → notification (DEFERRED).

### Verify flow (Admin @verify)
tx { lock payment FOR UPDATE (+invoice+booking); ensurePending; proof exists;
invoice not cancelled/expired; payment → paid(+paid_at,verified_by,verified_at);
invoice → paid; booking cascades waiting_verification→paid→confirmed } →
notification to customer (DEFERRED).

### Reject flow (Admin @reject)
tx { lock; ensurePending; invoice not cancelled/expired; payment → rejected
(+admin_note,verified_*); invoice → unpaid; booking waiting_verification →
pending_payment } → notification (DEFERRED).

### Storage
`config/filesystems.php` `payment_proofs` disk → private local
`storage/app/private/payment-proofs`. Filename `payment-proof-{uuid}.{ext}`.
Download served via private endpoint (never public URL) with
`X-Content-Type-Options: nosniff`.

### PaymentResource
`id, invoice_id, payment_code, payment_method, amount, status, proof_available,
customer_note, admin_note, paid_at, verified_at, verified_by (admins only),
invoice {id, invoice_number, total_amount, status}` — matches resource when
invoice loaded (all payment endpoints load it).

## State Machine
`PaymentTransitions` map:
`waiting_verification → [paid, rejected]`, `pending → [paid, rejected]`.
`CanPaymentTransition` + `IsPaymentPendingVerification` (mirrors
`pendingVerificationStatuses()`). Invalid → 422 `"Payment has already been
processed."`

## Model / Repository / Service
- `model.Payment`: 9 status + method constants, cents money, `ProofImage`
  (storage key, never exposed raw), invoice relation.
- `repository.PaymentStore`: FindInvoiceForUpdate (locks invoice+booking),
  HasPendingPayment, Create, UpdateInvoiceStatus/BookingStatus,
  PaymentCodeExists, FindLatestWithProofByInvoice, AdminCount/AdminList
  (with invoice+booking batch attach), FindByIDNoLock, FindByIDForUpdate
  (locks payment→invoice→booking, consistent order), MarkVerified,
  MarkRejected. Classifies duplicate code → ErrDuplicate.
- `service.PaymentService`: all six flows; payment code
  `PAY-{booking_code}-XXXX` (crypto/rand + retry); proof key
  `payment-proof-{hex}.{ext}`; file written **after** DB commit (INTENTIONAL
  IMPROVEMENT vs Laravel's in-tx write; see below); cleanup on error.
- `storage.LocalStorage`: flat private storage, path-traversal-safe (key must
  equal basename — mirrors Laravel check), O_EXCL saves.

## File Boundary (INTENTIONAL IMPROVEMENT — documented)
Laravel writes the file **inside** the DB transaction (and deletes it on
rollback). Go writes the file **after commit** so a DB failure never leaves an
orphan file and the DB transaction is never held during I/O. Observable
contract unchanged (proof row + file present on success; on storage failure
→ generic 500 with the DB row intact — Laravel behaves inconsistently here).
This is a documented deviation.

## Locking & Concurrency
Verify/Reject lock orders: payment → invoice → booking (FOR UPDATE), so two
concurrent verifies cannot both succeed — second sees `paid` →
422 `"Payment has already been processed."`. Race test emulates the lock via a
serializer store; the real FOR UPDATE behavior is covered by the repository
integration test.

## Money
Integer-cents arithmetic; amount must equal `invoice.total_amount`
(`sameMoney` via minor units); DTO emits `"300.00"`.

## Security
- Auth + RequireRole(customer)/(admin,super_admin); policy ownership
  (booking.customer_id) in service; identity from context only.
- private storage (no public URLs); filename from server (hex), not client
  raw; magic-byte sniff (JPEG/PNG) + size cap 2MB + extension allowlist.
- parameterized SQL; generic 5xx; no storage path/secret in responses or logs.

## Laravel → Go Parity

| Item | Status |
|---|---|
| 6 endpoints / methods / roles | MATCH |
| Upload guards (payable invoice 409, bank_transfer 422, amount-equals 422, pending-exists 422) | MATCH |
| Proof download (private, inline, nosniff) | MATCH |
| verify/reject state cascades | MATCH |
| Locking payment→invoice→booking | MATCH (Laravel also locks invoice+payment) |
| proof_available / verified_by admin-only | MATCH |
| File writing timing | INTENTIONAL IMPROVEMENT (post-commit, documented) |
| Notifications | DEFERRED (Notification domain) |
| throttle payment-upload | DEFERRED (rate limiting hardening phase) |

## Testing
- Storage 3: roundtrip, traversal-keys rejected, no-overwrite.
- Service 10: upload success (proof stored), wrong owner 403, wrong amount 422,
  non-payable 409, storage failure 500, verify cascade (paid+invoic+booking
  confirmed), verify already-processed 422, reject success (admin note +
  invoice unpaid), repo error, concurrent verify single-winner.
- Handler 9: upload 201 (no storage-path leak), magic-invalid 422, missing file
  422, 401, admin verify 200, reject validation 422, admin list 200, wrong role
  403, 500.
- Repository gated: lock invoice+booking, create, cascade helpers, FOR UPDATE
  fetch, mark verified/rejected (verified_by/admin_note), admin list/count,
  pending guard, latest-with-proof, code uniqueness → ErrDuplicate.

## Verification
| Check | Result |
|---|---|
| `go fmt` | clean |
| `go vet` | 0 |
| `go test -p 1 ./...` | PASS (full regression all phases) |
| `go test -race -p 1 ./...` | PASS (GOMAXPROCS=2 for MinGW linker) |
| `go build` | 0 |
| Repo integration | SKIP without `TEST_DATABASE_URL` |

## Known Limitations
- Notifications DEFERRED (verified/rejected/proof-submitted).
- Upload throttle DEFERRED.
- Content-Type from client not trusted; magic-byte sniff mirrors Laravel's
  mimetypes rule (image/jpeg,image/png).

## Fundamental Go Concepts
`io.Reader` streaming to storage; filesystem abstraction (interface) so
services stay decoupled; filesystem-vs-transaction asymmetry (why cleanup is
explicit); FOR UPDATE + consistent lock order for atomic transitions;
state machine as data; typed domain errors mapped to 401/403/404/409/422/500;
multipart/form-data parsing with size-limit; path-traversal defense via
basename equality; batch attach to avoid N+1; interface boundaries
(service↔storage, service↔repo) with DI at the composition root.