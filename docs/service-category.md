# Service Category

Phase 8A deliverable. Catalog category list (public) + admin category CRUD,
preserving Laravel behavior.

## Laravel Audit

Endpoints verified from `C:\V1\api-dwidev`:

| Method | Path | Auth | Role | Notes |
|---|---|---|---|---|
| GET | `/api/categories` | none | — | active categories having ≥1 active service; nested active services (ordered by name); ordered by name; paginated (fixed `per_page` default 15 — `categories()` calls `perPage()` without the request, so client `per_page` is ignored) |
| POST | `/api/admin/categories` | Bearer | admin,super_admin | 201 |
| PUT | `/api/admin/categories/{category}` | Bearer | admin,super_admin | 200 |
| DELETE | `/api/admin/categories/{category}` | Bearer | admin,super_admin | 200 / 409 when it has services |

No public single-category endpoint exists in Laravel — none was added.

### Key behaviors

- **List** (`CatalogController@categories`): `is_active=1`, `whereHas(active
  services)`, `with(active services, orderBy name)`, `orderBy name`,
  `paginate(15)`.
- **Slug**: auto-generated `Str::slug(name)` on store when missing; on update
  always re-derived from name (client slug ignored).
- **Resource** (`CategoryResource`): id, name, slug, description, icon,
  is_active, services (ServiceResource when loaded). ServiceResource fields:
  id, name, slug, description, price (decimal:2 → string), unit,
  estimated_duration, is_active, category (not loaded in list → null).
- **Destroy**: 409 `"Category cannot be deleted while it has services.
  Deactivate it instead."`; else hard delete → `"Category deleted
  successfully."`.

## Endpoints (Go)

Same four routes, verbatim paths. List stays public; admin group uses
`middleware.Auth` + `RequireRole(admin, super_admin)`.

## Request / Response

- **GET /api/categories** → Laravel paginator envelope (data / links / meta);
  per-page fixed at 15, `?page=` honored.
- **POST** → 201 `{"message":"Category created successfully.","data":{...}}`.
- **PUT** → 200 `{"message":"Category updated successfully.","data":{...}}`.
- **DELETE** → 200 `{"message":"Category deleted successfully."}` or 409.

DTO mirrors the Resources field-for-field (price as string `"150.00"`;
embedded `services[].category: null`).

## Validation

Store/Update mirror the FormRequests: name required/max255/unique,
slug max255/unique (auto), description nullable/max1000, icon nullable/max100,
is_active sometimes-boolean (non-boolean → 422
`The is active field must be true or false.`). Unique checks are DB-backed and
produce `The name has already been taken.` / `The slug has already been taken.`

## Model / Repository / Service

- `internal/model/service_category.go` + `ServiceLite` (read-only service
  projection needed by the list until the Service domain lands — FASE 8B).
- `repository.ServiceCategoryStore`: CountActive, ListActive (with nested
  active services via `IN (...)` + ordering), FindByID, HasServices,
  NameTaken/SlugTaken (ignore-self), Create, Update, Delete. Price read as
  integer cents via `CAST(price*100 AS SIGNED)`.
- `service.ServiceCategoryService`: pagination math, slugify (Str::slug
  equivalent), unique pre-checks + DB backstop, is_active default/keep
  semantics, 404/409/422 mapping. All guards + mutation run inside one
  TxManager transaction.

## Error Mapping

404 `Resource not found.` · 409 destroy-guard message · 422 validation
(name/slug taken, shape) · 500 generic (driver text never reaches the client).

## Concurrency

Unique name/slug on `service_categories` are the backstop; pre-checks +
constraint restart produce the Laravel validation messages. Single-statement
writes in a tx — no extra locking needed.

## Laravel → Go Parity

| Item | Status |
|---|---|
| Four endpoints / methods / roles | MATCH |
| Active-only + has-active-service list, name order | MATCH |
| Nested active services ordered by name | MATCH |
| Fixed per_page 15, page param | MATCH |
| Slug auto-gen + update re-derive | MATCH |
| Resource fields + price string + services[].category null | MATCH (verified field-by-field) |
| Create/update/delete messages + status (incl. 409) | MATCH |
| Unique messages | MATCH (via pre-check + classified constraint) |
| `services[]` projection implementation | INTENTIONAL IMPROVEMENT (temporary projection; full Service domain in FASE 8B) |
| is_active non-boolean → 422 message | MATCH |

## Testing

- Service (fake store, no DB): list+meta, auto-slug+default active, duplicate
  name, slug deriving on update, 404, delete guard 409, delete, repo error 500.
- Handler (real auth + role middleware): public list shape (price string,
  category null), admin 201/update/delete, 401, 403, 422 (empty name,
  non-boolean is_active), 409, 500 generic.
- Repository (gated `TEST_DATABASE_URL`): create/find/update/delete, unique
  classification, taken helpers (ignore-self), has-services, active list with
  cents projection, double-delete 404.

Commands: `go test -p 1 ./...` and `go test -race -p 1 ./...` (system linker
OOM under parallel builds — documented), optional
`TEST_DATABASE_URL=... go test ./internal/repository -v`.

## Known Limitations / Migration Risks

| Item | Severity | Notes |
|---|---|---|
| `services` projection until FASE 8B | LOW | read-only, hidden behind ServiceLite type |
| `per_page` ignored on categories (Laravel parity) | — | page-only pagination |
| price reading via `CAST(price*100 AS SIGNED)` | LOW | money stays integer cents |
| Linker/page OOM | ENV | serial builds on this machine |

## Fundamental Go Concepts

Interface-based persistence (catStore) + DI; service-owned transactions
(`txRunner`); slugify (deterministic normalization); typed sentinel errors with
constraint-key classification (`ErrDuplicateName`/`ErrDuplicateSlug`); DTO vs
model (price as string, no secrets); paginator envelope replication; role
middleware composition; context propagation end-to-end.