# Notification Domain

Phase 14 deliverable. Database-channel system notifications (Laravel
`SystemNotification`, `via() = ['database']`) delivered post-commit, plus the
in-app notification API (list / read / read-all).

## Laravel Audit

### Notification inventory (all triggers found via NotificationService::send)

| Event | Trigger | Recipient | Channel | Data IDs |
|---|---|---|---|---|
| booking_created | Booking created | all admins | database | booking, invoice |
| payment_proof_submitted | Customer uploads proof | all admins | database | booking, invoice, payment |
| payment_verified | Admin verifies payment | customer | database | booking, invoice, payment |
| payment_rejected | Admin rejects payment | customer | database | booking, invoice, payment |
| assignment_created | Admin assigns technician | technician | database | booking, assignment |
| assignment_accepted | Technician accepts | all admins | database | booking, assignment |
| assignment_rejected | Technician rejects | all admins | database | booking, assignment |
| job_started | Technician starts | customer | database | booking, assignment |
| job_completed | Technician completes | admins + customer | database | booking, assignment |
| job_completed_verified | Admin approves completion | customer | database | booking |
| review_reminder | After approve (no review yet) | customer | database | booking |
| job_completion_rejected | Admin rejects completion | customer | database | booking |

No email/broadcast/SMS/WhatsApp channels exist in Laravel for system
notifications — database only. Recipients come from domain data (SQL admin
lookup, `booking.customer_id`, `assignment.technician_id`), never from client
input.

### Notification API (authenticated, any role)
- `GET /api/notifications` — own notifications, latest, paginate (per_page).
- `POST /api/notifications/{notification}/read` — mark one read.
- `POST /api/notifications/read-all` — mark all read.

### NotificationResource
`id, type(data.event), title, message(data.body), data{booking_id, invoice_id,
payment_id, assignment_id, action_url}, read_at, created_at`.

## Architecture

- **model.SystemNotification** payload + **model.Notification** row.
- **repository.NotificationStore**: InsertNotification (DB row), AdminIDs
  (batch admin lookup), read API queries/mutations. UUID v4 id generated with
  crypto/rand. Payload JSON matches Laravel `toDatabase` keys.
- **notify.Notifier** interface: `NotifyUser`, `NotifyAdmins`. **DBNotifier**
  writes rows on the pool; failures are logged (warn) and never fail the
  committed business result. **Noop** for tests.
- **service.NotificationService**: list/read/read-all (TxManager).
- **Integration**: booking create, payment upload/verify/reject, assignment
  assign/accept/reject, technician start/complete, booking verify
  approve/reject(+reminder) all call `notifier` **after commit**.

## Trigger Boundary

```
business tx → COMMIT → notifier.NotifyX → DB row (async in Laravel's queue;
Go: synchronous insert immediately after commit — APPROXIMATION, observable
row is the same, and a notification failure never rolls back business).
```

Laravel dispatches through the database queue (ShouldQueue); Go writes the
row synchronously post-commit (no queue hop). Documented APPROXIMATION.

## Mailer / Worker Impact
System notifications use the database channel only — the OTP **Mailer/Worker**
from FASE 7B-3 is untouched (regression: OTP suite passes).

## Testing
- notify: user+admins notification, failure-logged-not-fatal.
- service notification: list/read/read-all, not-found 404.
- flow-level: booking create → admins booking_created; payment upload →
  admins payment_proof_submitted + verify → customer payment_verified;
  assignment accept → admins; technician start → customer; booking verify
  approve → customer verified + review_reminder; business success even when
  notifier fails.
- handler: list shape, read, read-all, 401, 404.
- repository (gated): insert, count/list/payload decode, find-by-user,
  mark-read/all, admin ids.

## Parity

| Item | Status |
|---|---|
| 11 trigger events + recipients | MATCH |
| Database channel only | MATCH |
| Payload keys + NotificationResource | MATCH |
| Post-commit dispatch | INTENTIONAL IMPROVEMENT (Laravel queues; Go writes post-commit synchronously — same observable row, never inside business tx) |
| Notification API (list/read/read-all) | MATCH |
| Mailer/worker reuse | n/a (database channel) |
| Notification.NOTIFICATION_FAILURE → business error | never (Laravel parity via queue) |

## Known Limitations
`review_reminder` fires unconditionally on approve (no Review domain yet, so
`review()->exists()` is always false — parity condition collapses to always
true; Review domain lands separately). Channel broadcast/email not implemented
(Laravel has none). Environmental race-build OOM → GOMAXPROCS=1.

## Fundamental Go Concepts (mapped to code)
Interface (`notify.Notifier`) + DI at composition root (main wires
`DBNotifier` into booking/payment/assignment services); post-commit side-effect
boundary (notification never inside `TxManager`); failure isolation (logged,
business unaffected); concurrency-safe notifier (stateless + store writes;
race-tested); batched admin recipient lookup (single query, no N+1);
UUID v4 via crypto/rand; thin handler + explicit DTO; parameterized SQL.