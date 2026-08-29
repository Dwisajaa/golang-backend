# Invoice Domain

Phase 10 deliverable. Promotes the booking-creation invoice side-effect into a
proper read domain (list + detail), preserving Laravel behavior.

## Laravel Audit

**Endpoints (role:customer):**
- `GET /api/invoices` — invoices `whereHas booking.customer_id = user.id`,
  `with('booking')`, `latest()`, paginate (per_page 1..50, default 15).
  Response: InvoiceResource::collection (paginator envelope).
- `GET /api/invoices/:id` — `load('booking')` then `InvoicePolicy.view`
  (`user.id === invoice.booking.customer_id`) → 403, else `{"data":...}`.

`POST/GET /api/invoices/{invoice}/payment-proof` → **Payment domain
(DEFERRED to FASE 11)**.

No create/update/destroy endpoints exist — none added.

### InvoiceResource
`id, booking_id, invoice_number, issued_at, due_at, subtotal,
additional_cost, total_amount, status, notes` (no nested booking in the
resource body despite `with('booking')` used for the policy).

### Invoice model
5 statuses (`unpaid, pending_payment, paid, cancelled, expired`); **no
transitions()/transitionTo** — states are driven by Booking/Payment services,
so no state-machine map lives here. `generateInvoiceNumber`:
`INV-{booking_code}-XXXX` (random_int 1..9999 padded, retry while exists).

## Booking Integration
Invoice creation remains the booking side-effect inside the booking creation
transaction (`BookingStore.CreateInvoice`), so `POST /api/bookings` is
unchanged (atomic booking+item+invoice). This phase only adds the read domain
around it — no change to the FASE 9 contract (verified by regression).

## Architecture
- **Model** (`internal/model/invoice.go`): full Invoice struct + status
  constants (moved out of booking.go; booking.go keeps its `Invoice` reference).
- **Repository** (`InvoiceStore`): CountByCustomer, ListByCustomer (join
  bookings for ownership), FindByID — all attach the booking row (batch, no
  N+1) which the policy check needs.
- **Service** (`InvoiceService`): ListByCustomer (ownership via SQL join),
  Show (load booking → `booking.customer_id == customerID` else 403;
  ErrNotFound → 404). All via `TxManager`.
- **Handler + DTO**: reuses `invoiceData` (already mirrors InvoiceResource
  exactly) + `buildInvoicePage` paginator. Thin.

## Ownership / Authorization
Auth + RequireRole(customer). Ownership = invoice's booking
`customer_id == user.id` (InvoicePolicy), enforced in the service against
context identity. 403 Forbidden on mismatch, 404 on missing.

## Money
Invoice money read as integer cents (DECIMAL boundary via `parsePriceString`);
DTO emits `"300.00"` strings. No recomputation — values snapshot the booking.

## Invoice Number
`INV-{booking_code}-XXXX` crypto/rand + retry on unique collision (booking
creation path, FASE 9). DB unique constraint = backstop.

## Laravel → Go Parity

| Item | Status |
|---|---|
| GET /invoices list (ownership, booking loaded, latest, per_page) | MATCH |
| GET /invoices/:id (policy view 403, 404) | MATCH |
| InvoiceResource fields + price strings | MATCH |
| Creation = booking side-effect (atomic) | MATCH |
| payment-proof endpoints | DEFERRED (FASE 11 Payment) |
| State transitions | Not applicable (Laravel has no Invoice transitions(); states driven by Booking/Payment) |

## Testing
- Service 5 (+fake store): list meta, show owned/forbidden/nil-booking 403,
  not found 404, repo error.
- Handler 7: list shape (money strings), show ok, 403, 404, 401, wrong-role
  403, 500.
- Repository: previously covered invoice create/attach in booking gated test;
  this phase's read paths are unit-tested (integration for read path mirrors
  booking test's attach, gated)
- Full regression across all prior phases incl. `POST /api/bookings`.

## Verification
| Check | Result |
|---|---|
| `go fmt` | clean |
| `go vet` | 0 |
| `go test -p 1 ./...` | PASS (all packages, full regression) |
| `go test -race -p 1 ./...` | PASS (GOMAXPROCS=2 to avoid MinGW linker OOM) |
| `go build` | 0 |
| Repo integration | SKIP without `TEST_DATABASE_URL` |

## Known Limitations
- Payment proof endpoints not implemented (FASE 11).
- Invoice status transitions not exposed (Laravel has none; Payment domain
  drives paid/pending states later).
- `with('booking')` list load is used only for the ownership join; the
  resource body never nests booking (Laravel parity).

## Fundamental Go Concepts (as used here)
Interface-based persistence + DI; ownership authorization split between SQL
(whereHas via join) and service policy check; eager-load via batch `IN (...)` to
avoid N+1; sentinel `ErrNotFound` → typed 404; integer-cents money boundary;
Thin handler + explicit DTO; composition-root wiring; single tx read path via
`txRunner`.