# Technician Assignment Domain

Phase 12A deliverable. Admin technician-assignment for bookings, with
eligibility checks, active-assignment replacement, booking cascade, and FOR
UPDATE serialization.

## Laravel Audit

**Endpoint (inventory — only one exists):**

| Method | Path | Auth | Role | Policy | Purpose |
|---|---|---|---|---|---|
| POST | `/api/admin/bookings/{booking}/assign` | Bearer | admin,super_admin | — | Assign technician to a confirmed & paid booking |

Technician `jobs` endpoints (accept/reject/start/complete) belong to the
technician **workflow** (FASE 12B) — deliberately out of scope here.

### Assign flow (Admin AssignmentController@assign)
tx {
  lock booking FOR UPDATE (with invoice)
  → cancelled → 422 `{"booking":["Cancelled bookings cannot be assigned."]}`
  → status != confirmed OR invoice != paid
     → 422 `{"booking":["Booking must be confirmed and paid before assignment."]}`
  → lock technician user (with technicianProfile)
     → !technician || role != technician || !profile.is_active
        → 422 `{"technician_id":["Technician is invalid or inactive."]}`
  → find active assignment (pending|accepted, latest)
     → replace: status=rejected, rejected_at=now,
       rejection_reason="Assignment replaced by admin."
  → create assignment (technician_id, assigned_by=admin id, assigned_at=now,
     status=pending)
  → booking.transitionTo(technician_assigned)   // confirmed → technician_assigned
}
→ response 201 `{"message":"Technician assigned successfully.","data":BookingAssignmentResource}`
→ notification to technician → **DEFERRED**.

### Technician eligibility (Laravel-verified)
- user exists, `role === technician`, `technician_profile.is_active = true`.
- No availability/specialization/occupancy system exists in Laravel — none added.

### AssignTechnicianRequest
`technician_id` required/integer/exists:users.

### Reassignment
Only while the booking is still `confirmed` (status check). Any prior active
assignment is replaced (rejected + reason). Once the booking moves past
confirmed, reassign is blocked by the same status check — Laravel parity.

### BookingAssignmentResource
scalars (id, booking_id, technician_id, assigned_by, status, assigned_at,
accepted_at, rejected_at, started_at, completed_at, rejection_reason,
technician_note) + nested `booking {id, booking_code, booking_date,
booking_time, address, address_detail, customer_note, status, items[],
customer{id,name,email}}`.

## Model
`model.BookingAssignment` (mirrors migration) + status constants
(pending/accepted/rejected/completed), `IsActiveAssignment`, `ReplacementReason`.

## Repository
`AssignmentStore`: FindBookingForAssign (lock booking+invoice),
FindTechnicianForAssign (lock user row + load profile), FindActiveAssignment,
ReplaceAssignment, Create, UpdateBookingStatus, LoadBookingForResponse
(booking + items + customer batch). Interface + MySQL impl, all parameterized.

## Service
`AssignmentService.Assign(ctx, adminID, bookingID, technicianID)` — the whole
flow in one `TxManager` transaction. Admin identity from context (assigned_by).

## Concurrency
The booking `FOR UPDATE` serializes concurrent assigns to the same booking:
second assign sees status technician_assigned → 422 (must be confirmed). Unit
race test emulates the lock (serializer store); real FOR UPDATE proven by the
integration test.

## Booking Integration
Booking is loaded with its invoice (eligibility) and, for the response, items
+ customer. `BookingResource.latest_assignment` remains DEFERRED (FASE 9
decision) — assigning does not change the booking response contract.

## Laravel → Go Parity

| Item | Status |
|---|---|
| Endpoint / method / role | MATCH |
| Status + invoice guards (messages verbatim) | MATCH |
| Technician eligibility checks | MATCH |
| Active-assignment replacement (reason verbatim) | MATCH |
| booking → technician_assigned cascade | MATCH |
| 201 + resource shape | MATCH |
| audit route model | DEFERRED (workflow FASE 12B) |
| notification to technician | DEFERRED |
| BookingResource.latest_assignment | DEFERRED |

## Testing
- Service 10: success (assigned_by + booking cascade), not-confirmed 422,
  cancelled msg, unpaid invoice 422, wrong-role-tech 422, inactive-tech 422,
  reassignment replaces active (reason), booking 404, concurrent single-winner.
- Handler 7: 201 + message, validation 422, business 422, 401, 403 role, 404,
  500.
- Repository gated: lock booking+invoice, technician eligibility load, create,
  find active, replace, load-for-response, 404.
- Full regression: **all early phases PASS**.

## Verification
| Check | Result |
|---|---|
| `go fmt` | clean |
| `go vet` | 0 |
| `go test -p 1 ./...` | PASS (full regression) |
| `go test -race -p 1 ./...` | PASS (GOMAXPROCS=1; TSAN shadow OOM at higher concurrency — environmental) |
| `go build` | 0 |
| Repo integration | SKIP without `TEST_DATABASE_URL` |

## Known Limitations / Migration Risks
Notification DEFERRED; technician workflow (accept/reject/start/complete)
DEFERRED to FASE 12B; admin booking verify endpoint (post-completion) still
DEFERRED; BookingResource.latest_assignment DEFERRED; environmental memory
pressure during race builds.

## Fundamental Go Concepts
FOR UPDATE + consistent lock order (booking → technician → assignment);
business rules centralized in service; eligibility from joined profile;
typed domain errors; interface-based repository + DI; batch response loading
(no N+1); state constants instead of scattered strings; side-effect after
commit (notification DEFERRED); composition-root wiring.

---

# Technician Workflow (FASE 12B)

## Laravel Audit
**Endpoints (technician role, all under `role:technician`):**

| Method | Path | Role | Purpose |
|---|---|---|---|
| GET | `/api/technician/jobs` | technician | own assignments, booking.items/customer/invoice, per_page |
| GET | `/api/technician/jobs/{assignment}` | technician | policy `act` (owner) → detail |
| POST | `/api/technician/jobs/{assignment}/accept` | technician | pending → accepted |
| POST | `/api/technician/jobs/{assignment}/reject` | technician | pending → rejected (+ booking revert) |
| POST | `/api/technician/jobs/{assignment}/start` | technician | accepted + start → booking in_progress |
| POST | `/api/technician/jobs/{assignment}/complete` | technician | accepted + started → completed |

### Accept
tx{ lock assignment FOR UPDATE (with booking+invoice) → owner → status==pending
else 422 `"Assignment is not in the required state."` → invoice==paid &&
booking==technician_assigned else 422 `"Booking is not ready for acceptance."`
→ status=accepted, accepted_at=now } → notification (DEFERRED) → 200
`"Assignment accepted successfully."`

### Reject
RejectAssignmentRequest: `rejection_reason` required `in[Tidak tersedia,
Jadwal bentrok, Lokasi terlalu jauh, Masalah teknis, Lainnya]`; detail nullable
max500 → tx{ lock → owner → pending → reason = reason [+ " - " + detail] →
rejected + rejected_at → if booking==technician_assigned transitionTo(confirmed)
} → 200 `"Assignment rejected successfully."`

### Start
tx{ lock → owner → accepted → if started_at || booking!=technician_assigned →
422 `"Assignment cannot be started in its current state."` → started_at=now +
technician_note="Job started." → booking transitionTo(in_progress) } → 200
`"Job started successfully."`

### Complete
CompleteJobRequest: `technician_note` required max2000 → tx{ lock → owner →
accepted → if !started_at || booking!=in_progress → 422 `"Job must be started
before completion."` → completed + completed_at + note → booking
transitionTo(awaiting_verification) } → 200 `"Job completed and awaiting
verification."`

## State / Idempotency
Assignment states: pending → accepted/rejected → (start) → complete. Status
checks reuse the same `"Assignment is not in the required state."` on repeat
actions (idempotency = Laravel parity: second accept fails 422). Booking
transitions validated via `model.CanTransition` (existing state machine).

## Locking & Concurrency
Technician workflow locks **only the assignment row** (Laravel parity — the
booking/invoice are read without FOR UPDATE). Lock order is canonical:
assignment-first in workflow; booking-first in admin-assign; no cycle since
each path locks only one side. Concurrent accepts serialize on the assignment
row → exactly one succeeds (unit race test emulates FOR UPDATE; MySQL
integration covers real transitions).

## Repository
Extended `AssignmentStore`: FindWorkForUpdate (assignment FOR UPDATE + booking +
invoice), FetchByID, CountByTechnician/ListByTechnician (with booking+items+
customer+invoice batch), MarkAccepted/Rejected/Started/Completed.

## Service
`AssignmentService`: ListJobs, ShowJob (owner check), Accept, Reject, Start,
Complete — each with ownership → state → business guard → transition → booking
cascade inside one `TxManager` transaction. Reject merges reason+detail.

## Parity

| Item | Status |
|---|---|
| 6 endpoints / method / role technician | MATCH |
| State guards (messages verbatim) | MATCH |
| Accept readiness (invoice paid + booking assigned) | MATCH |
| Reject reason enum + detail merge | MATCH |
| Start started_at + "Job started." | MATCH |
| Complete note + booking awaiting_verification | MATCH |
| Repeat-action 422 (idempotency) | MATCH |
| Notifications | DEFERRED |

## Known Limitations
Notification dispatch DEFERRED (accept/reject/start/complete); review
availability after completion DEFERRED (Review domain); environmental OOM
during race builds (GOMAXPROCS=1 used).