# Service Domain

Phase 8B deliverable. Full vertical slice for the `services` catalog domain,
preserving Laravel behavior and integrating cleanly with the FASE 8A Category
list.

## Laravel Audit

Endpoints verified from `C:\V1\api-dwidev`:

| Method | Path | Auth | Role | Notes |
|---|---|---|---|---|
| GET | `/api/services` | none | — | active services whose category is active; `?category_id=` filter; `?search=` on name/description; order by name; paginate with **honored** `per_page` (default 15, clamped 1..50 — unlike categories which use a fixed default) |
| GET | `/api/services/{service}` | none | — | 404 unless active AND category active; `{"data": ServiceResource(category loaded)}` |
| POST | `/api/admin/services` | Bearer | admin,super_admin | 201, category loaded |
| PUT | `/api/admin/services/{service}` | Bearer | admin,super_admin | 200, fresh + category loaded |
| DELETE | `/api/admin/services/{service}` | Bearer | admin,super_admin | 409 if used by a package, else 200 |

No other service endpoints exist in Laravel (no public single create/etc.).

### ServiceResource

`id, name, slug, description, price, unit, estimated_duration, is_active,
category` — `category` via `whenLoaded` (always loaded on these endpoints).
`price` is `decimal:2` cast → **string** `"150.00"`. Nested category renders
`services: []` (category loaded without its services).

### Validation (Store/UpdateServiceRequest)

`service_category_id` required/exists · `name` required/max255/unique ·
`slug` required/max255/unique (auto from name) · `description` nullable/max1000 ·
`price` required/numeric/min:0 · `unit` required `in:per_service,per_room,
per_unit,per_hour,per_meter,custom` · `estimated_duration` nullable/integer/min:1 ·
`is_active` sometimes/boolean. Update: unique ignore-self, slug always
re-derived from name.

## Request / Response (Go)

- **GET /api/services** → Laravel paginator envelope; keeps query params in
  `links`; `price` strings; nested `category` object (services `[]`).
- **GET /api/services/:id** → `{"data":{...}}`.
- **POST/PUT/DELETE admin** → Laravel messages/status (`201/200/200`), delete
  guard → 409 message verbatim.

## Category Integration / ServiceLite

`GET /api/categories` (FASE 8A) embeds services via the **ServiceResource**
contract: `{id,name,slug,description,price,unit,estimated_duration,is_active,
category:null}`.

- The FASE 8A `ServiceLite` projection already produces exactly this shape
  (categories load services without the category relation → `category:null`).
  **Decision:** keep `ServiceLite` as the *query-level projection* used only by
  the category list — it is not a public domain model and carries no business
  rules. The Service domain now owns `model.Service`, its repository, service,
  and DTO (`serviceData`); `ServiceLite` is documented as a read-only
  projection to avoid an N+1-per-category load and to keep the verified 8A
  contract untouched. All 8A tests still pass unchanged.

## Money

`price` enters as a JSON number (or numeric string) via `json.RawMessage`,
parsed to **integer cents exactly** with `big.Rat` (no floats). Repository
reads DECIMAL as a string and converts with `parsePriceString`; writes serialize
cents via `fmtCents`. DTO emits `"150.00"` (two-decimal string) — Laravel
`decimal:2` cast parity.

## Architecture

- `repository.ServiceStore`: Count/List (active + category-active, category_id +
  search filters, batch category load — no N+1), FindActiveByID (404 semantics
  at SQL level), FindByID (admin), CategoryExists, NameTaken/SlugTaken, Create,
  Update, HasPackages (delete guard over package_items), Delete. 1062 →
  ErrDuplicateName/ErrDuplicateSlug (services keys).
- `service.ServiceService`: pagination math + per_page clamp, slug derive,
  category-exists + unique pre-checks (DB backstop), is_active default/keep,
  404/409/422 mapping, all within `TxManager`.
- `httphandler.ServiceHandler` + DTO: thin; price parse errors → 422 numeric
  message; non-boolean `is_active` → 422 boolean message (reused `boolish`).

## Error Mapping

404 `Resource not found.` · 409 `"Service cannot be deleted while it is used by
a package. Deactivate it instead."` · 422 (`The selected service category id is
invalid.`, `The name has already been taken.`, `The price field must be a
number.` / `must be at least 0.`, `The selected unit is invalid.`, etc.) · 500
generic.

## Laravel → Go Parity

| Item | Status |
|---|---|
| Four endpoints/methods/roles | MATCH |
| Active-only + category-active public list/detail | MATCH |
| category_id/search filters, name order | MATCH |
| per_page honored (1..50, default 15) | MATCH |
| slug auto + update re-derive | MATCH |
| ServiceResource fields + price string + nested category | MATCH |
| Category exists message; unique messages | MATCH |
| Delete package guard 409 message | MATCH |
| categories() ServiceLite projection | INTENTIONAL IMPROVEMENT (query projection; contract identical; Service domain owns the real model) |

## Testing

- Service (fake store): list meta, active-only detail, category-invalid 422,
  duplicate name, slug derive + is_active keep, delete guard 409, delete 404,
  repo error 500.
- Handler (real auth + role): list shape (price string, nested category with
  `services: []`), detail 404, admin 201/200, 403 wrong role, 422 shape
  (incl. non-numeric price, unit), category-invalid via service, 409, 500, 400.
- Repository (gated `TEST_DATABASE_URL`): CRUD, money round-trip (cents ↔
  DECIMAL), active-only list/detail, search count, unique classification,
  package guard, double-delete 404.
- **Regression:** full suite includes all FASE 0–8A tests (categories list with
  ServiceLite unchanged).

Commands: `go test -p 1 ./...`, `go test -race -p 1 ./...` (environment linker
OOM on parallel builds), optional `TEST_DATABASE_URL=... go test
./internal/repository -v`.

## Known Limitations / Migration Risks

| Item | Severity | Notes |
|---|---|---|
| `services` search uses `LIKE %term%` (SQL semantics) | LOW | matches Laravel `like` |
| `parsePriceString` truncates >2 fractional digits (DECIMAL guarantees 2) | LOW | schema-enforced |
| ServiceLite remains a projection until a future consolidation | LOW | documented; not a domain model |
| Linker/page OOM | ENV | serial builds |

## Fundamental Go Concepts

Interface-based repository + DI; service-owned transactions (txRunner);
`big.Rat` exact decimal→cents (no float money); raw `json.RawMessage` price
validation at the boundary; batched `IN (...)` category loading (no N+1); typed
error classification for unique constraints; DTO vs model (string price, nested
category, no internal fields); query-param passthrough in paginator links;
context propagation end-to-end.