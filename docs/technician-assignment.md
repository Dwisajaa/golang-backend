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