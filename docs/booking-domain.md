# Booking Domain

Phase 9 deliverable. Customer booking CRUD + admin booking list, with
price-snapshot items + invoice side-effect + centralized state machine.

## Responsibility
Customer creates bookings (with price snapshot + auto-invoice), views own
bookings, cancels cancellable bookings. Admin lists all bookings with filters.

## Laravel Audit
**Customer endpoints (role:customer):**
- `GET /api/bookings` — own bookings, with items+invoice, latest first, per_page
- `POST /api/bookings` — tx{lock catalog → create booking+item+invoice}, notification DEFERRED
- `GET /api/bookings/{booking}` — policy view (owner), with items+invoice
- `POST /api/bookings/{booking}/cancel` — policy cancel (owner), state check → tx{transition+invoice cancelled}

**Admin endpoints (role:admin,super_admin):**
- `GET /api/admin/bookings` — all bookings, search(booking_code)+status filter, with items+invoice
- `POST /api/admin/bookings/{booking}/verify` — DEFERRED (needs assignment domain)

## State Machine (centralized in `model.BookingTransitions`)
```
pending_payment       → [waiting_verification, cancelled]
waiting_verification  → [pending_payment, paid, cancelled]
paid                  → [confirmed, cancelled]
confirmed             → [technician_assigned, cancelled]
technician_assigned   → [in_progress, confirmed, cancelled]
in_progress           → [awaiting_verification, cancelled]
awaiting_verification → [completed, in_progress]
completed/cancelled   → (terminal)
```
Invalid transition → 409 "Booking cannot be cancelled in its current status."

## Ownership
`booking.customer_id = user.id` (BookingPolicy). Identity from
`middleware.CurrentUser(c)`, never from body. Forbidden → 403.

## Pricing / Snapshot
`unitPriceCents * quantity = subtotalCents` (integer arithmetic, no float).
BookingItem stores: `item_name` (snapshot), `unit_price` (snapshot at creation),
`subtotal`. Booking: `subtotal = sum(items)`, `additional_cost = 0`,
`total_price = subtotal + additional_cost`. Service/package price locked with
`SELECT ... FOR UPDATE` inside the creation transaction.

## Invoice Side-Effect
Invoice INSERT is part of the booking creation transaction (response contract
requires `invoice` in BookingResource). Created with: `INV-{booking_code}-XXXX`
(crypto/rand, retry), status=unpaid, amounts mirroring booking. Cancel sets
invoice status=cancelled. **Full Invoice domain (endpoints, payment
integration) is DEFERRED.**

## Booking Code
`BJA-YYYYMMDD-XXXX` (crypto/rand 1..9999, retry on unique collision).

## Resources (DTO)
**BookingResource:** id, booking_code, booking_date, booking_time, address,
address_detail, latitude, longitude, customer_note, additional_jobdesk,
subtotal, additional_cost, total_price, status, items[], invoice.
**BookingItemResource:** id, service_id, package_id, item_type, item_name,
quantity, unit_price, subtotal.
**InvoiceResource:** id, booking_id, invoice_number, issued_at, due_at,
subtotal, additional_cost, total_amount, status, notes.

## Validation (StoreBookingRequest)
item_type required in[service,package]; service_id/package_id required_if +
exists(active); quantity 1..99; booking_date required/date/after_or_equal:today;
booking_time required in TIME_SLOTS; address required/max255; nullable fields
max-checked. Profile completeness validated before creation.

## Architecture
- **Model:** `Booking` (9 status constants, `BookingTransitions` map,
  `CanTransition`, `BookingCodePrefix`, money as cents), `BookingItem`
  (snapshot fields), `Invoice` (minimal for side-effect).
- **Repository:** `BookingStore` — CRUD + items + invoice insert + attach (batch
  N+1-free) + admin list with filters + FOR UPDATE for cancel + code existence.
  `CatalogLookup` — active service/package FOR UPDATE with price parse.
  `ProfileLookup` — customer profile completeness.
- **Service:** `BookingService` — profile check, tx{lock catalog → snapshot
  price → create booking+item+invoice, code gen retry}, cancel tx{FOR UPDATE →
  state check → transition + invoice cancel}, admin list. All via `TxManager`.
- **Handler:** thin; structural validation; ownership from context; DTO.

## Transaction
- Create: `tx{lock service/package FOR UPDATE → create booking → create item →
  create invoice → commit}`. Notification DEFERRED (after commit).
- Cancel: `tx{FindByIDForUpdate → ownership check → CanTransition → update
  status → update invoice status → commit}`.

## Locking
Cancel uses `SELECT ... FOR UPDATE` on bookings (prevents concurrent
cancel+transition race). Catalog lock on service/package during create
(snapshot price consistency).

## Laravel → Go Parity

| Item | Status |
|---|---|
| 5 customer+admin endpoints | MATCH (admin verify DEFERRED) |
| State machine (9 states, transitions map) | MATCH |
| Ownership policy (customer_id) | MATCH |
| Price snapshot (item_name, unit_price, subtotal) | MATCH |
| Integer pricing (no float) | MATCH |
| Booking code BJA-YYYYMMDD-XXXX | MATCH |
| Invoice auto-created in tx | MATCH |
| Invoice code INV-{code}-XXXX | MATCH |
| Cancel state check → 409 | MATCH |
| Cancel cascades invoice cancelled | MATCH |
| Validation rules + messages | MATCH |
| FOR UPDATE on cancel + catalog | MATCH |
| Admin list filters (search, status) | APPROXIMATION (assigned/technician_id filters DEFERRED — needs assignment domain) |
| Notification after create | DEFERRED |
| Admin verify completion | DEFERRED (needs assignment domain) |
| BookingResource customer/latest_assignment | DEFERRED (needs full eager load) |

## Testing
- Service 8: create success (pricing+snapshot+invoice), incomplete profile 422,
  inactive service 404, show ownership, cancel transition, cancel invalid state
  409, cancel forbidden 403, repo error 500.
- Handler 9: list 200 (price string), create 201, show 403, cancel 409,
  validation 422, 401, 403 role, admin list 200, 500.
- Repository gated: create+item+invoice, find, attach items/invoices, update
  status.

## Verification
| Check | Result |
|---|---|
| `go fmt` | clean |
| `go vet` | 0 |
| `go test -p 1 ./...` | PASS (all packages, full regression) |
| `go test -race -p 1 ./...` | PASS (race=0) |
| `go build` | 0 |
| Repo integration | SKIP without `TEST_DATABASE_URL` |

## Known Limitations
- Admin list assigned/technician_id filters DEFERRED (needs assignment domain).
- BookingResource customer/latest_assignment fields DEFERRED.
- Notification side-effect DEFERRED.
- CancelBookingRequest `reason` field accepted but not persisted (Laravel
  controller ignores it beyond validation).
- Invoice full domain (endpoints, payment) DEFERRED.

## Fundamental Go Concepts
State machine as a data structure (map transitions); ownership authorization
in service layer; price snapshot (write-time data preserved); integer money
arithmetic (`cents * quantity`); `SELECT ... FOR UPDATE` for atomic state
transitions; multi-write transaction (booking+item+invoice); code generation
with retry (crypto/rand + unique constraint); side-effect boundary (invoice
created inside tx; notification after commit); batch attach for N+1
prevention; typed domain errors for conflict/forbidden.