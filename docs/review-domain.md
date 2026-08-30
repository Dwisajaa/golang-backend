# Review Domain

Phase 15 deliverable. Customer review (show/create) + admin review
list/moderate, preserving Laravel behavior with DB unique-constraint backstop.

## Laravel Audit

**Customer endpoints (role:customer):**
- `GET /api/bookings/{booking}/review` — booking owner (403), review with
  customer+technician; missing → 404 `"Review not found."`
- `POST /api/bookings/{booking}/review` — owner (403), booking completed AND
  assigned-technician exists (409 `"Booking is not eligible for review."`),
  no existing review (409 `"This booking already has a review."`); tx{ lock
  booking FOR UPDATE → re-check review + eligibility → create review
  (customer_id=auth, technician_id=assignedTechnician().id, rating, comment,
  status=published) } → notify technician (`review_submitted`, if
  wasRecentlyCreated) → 201 `"Review submitted successfully."`

**Admin endpoints (admin,super_admin):**
- `GET /api/admin/reviews` — all with customer+technician+booking, status
  filter, latest, paginate.
- `POST /api/admin/reviews/{review}/moderate` — status required in
  [published, hidden, rejected], policy moderate (admin) → 403; update →
  200 `"Review moderated successfully."`

### Eligibility (ReviewPolicy.create → same checks in controller)
customer role && `booking.customer_id == user.id` && booking completed &&
`assignedTechnician() != null` (technician of the **latest** assignment by id,
any status) && no existing review.

### StoreReviewRequest
`rating` required `integer in[1,2,3,4,5]`; `comment` nullable string max1000.
### ModerateReviewRequest — `status` required in Review::statuses().

### ReviewResource
`id, booking_id, rating, comment, status, created_at, customer{id,name},
technician{id,name}` (user objects only when loaded).

### Migration
booking_id **unique** (one review per booking), customer_id/technician_id FKs
cascade, rating unsignedTinyInt, comment text nullable, status(20)
default published, index(status, technician_id).

## Model
`model.Review` + status constants. Rating = int (1–5).

## Repository
`ReviewStore`: Create (1062 → ErrDuplicate — unique booking backstop),
FindByBooking/FindByID, ReviewExists, LatestAssignmentTechnicianID,
AdminCount/AdminList (with customer/technician batch), UpdateStatus.

## Service
`ReviewService`: Show (owner), Create (tx{ lock booking FOR UPDATE →
owner → eligibility → duplicate → create } → post-commit notify technician),
AdminList, Moderate. All in TxManager.

## Booking Integration
- `assignedTechnician()` mirrored via `LatestAssignmentTechnicianID` (latest
  assignment by id) — reviewed inside the locked flow.
- `BookingResource` does NOT list a review field in Laravel — none added.

## Notification Integration
`ReviewController@store` notifies the technician `review_submitted`.
`review_reminder` (payment/booking verify flow) condition remains
`!$booking->review()->exists()` — at verify time no review can exist yet, so
the reminder fires (documented parity; no code change needed).

## Concurrency / Duplicate
Booking `FOR UPDATE` serializes concurrent creates; the DB unique
`booking_id` is the authoritative backstop (second create → ErrDuplicate →
409 already-reviewed). Unit race test: 10 goroutines → exactly 1 success.

## Parity

| Item | Status |
|---|---|
| 4 endpoints / roles / messages | MATCH |
| Eligibility checks (completed + assigned technician) | MATCH |
| Duplicate guard + DB unique backstop | MATCH |
| rating 1..5 validation | MATCH |
| comment max1000 | MATCH |
| Moderate status enum + 403 policy | MATCH |
| ReviewResource fields | MATCH |
| technician notify review_submitted | MATCH |
| FOR UPDATE on create | MATCH (Laravel locks booking too) |
| BookingResource review field | none in Laravel → not added |

## Testing
Service 9: create success, not eligible (pending), no technician 409, duplicate
409, not owner 403, show 404, moderate, repo error, concurrent single-win.
Handler 9: 201 + message, invalid rating 422, 404, 409, moderate 200, invalid
status 422, 403 role, 401, 500. Repository gated: latest-tech, exists, create,
find with users, duplicate ErrDuplicate, moderate status, admin list. Full
regression all phases.

## Verification
| Check | Result |
|---|---|
| `go fmt` | clean |
| `go vet` | 0 |
| `go test -p 1 ./...` | PASS (full regression) |
| `go test -race -p 1 ./...` | PASS (GOMAXPROCS=1) |
| `go build` | 0 |
| Repo integration | SKIP without `TEST_DATABASE_URL` |

## Known Limitations
Review moderation only sets status (no content sanitization — Laravel has
none); no review aggregate/analytics; `review_reminder` exists-check always
true at verify time (parity). Race-build OOM → GOMAXPROCS=1.

## Fundamental Go Concepts (mapped)
TxManager + booking FOR UPDATE serialization; DB unique constraint as
duplicate backstop (INTENTIONAL improvement: consistent with Laravel's
read-then-write under the same booking lock); `LatestAssignmentTechnicianID`
mirrors a domain relation without N+1; batch user attach; typed errors
(Conflict/Forbidden/NotFound/Validation); thin handler + explicit DTO
(ReviewResource); post-commit notifier injection; parameterized SQL; identity
from context only.