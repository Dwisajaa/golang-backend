# Package Domain

Phase 8C deliverable. Full vertical slice for the `packages` catalog domain
(public list/detail + admin CRUD), including transactional package-item
management.

## Laravel Audit

Endpoints verified from `C:\V1\api-dwidev`:

| Method | Path | Auth | Role | Notes |
|---|---|---|---|---|
| GET | `/api/packages` | none | — | active packages with active-service items; `?search=`; order `is_popular DESC, name`; per_page honored (1..50) |
| GET | `/api/packages/{package}` | none | — | 404 if inactive; items filtered to active services |
| POST | `/api/admin/packages` | Bearer | admin,super_admin | 201; `DB::transaction { create + createMany items }` |
| PUT | `/api/admin/packages/{package}` | Bearer | admin,super_admin | `DB::transaction { update + delete items + createMany items }` (full item replacement) |
| DELETE | `/api/admin/packages/{package}` | Bearer | admin,super_admin | hard delete (**no guard**); FK CASCADE removes items |

### PackageResource

`id, name, slug, description, price (decimal:2 string), duration, is_active,
is_popular, items (PackageItemResource[])`.

**PackageItemResource:** `id, quantity, service (ServiceResource)`.
ServiceResource nested: category = null (not loaded in package context).

### Validation (Store/UpdatePackageRequest)

`name` required/max255/unique · `slug` auto from name · `description`
nullable/max1000 · `price` required/numeric/min:0 · `duration`
nullable/int/min:1 · `is_active`/`is_popular` sometimes/boolean · `items`
required/array/min:1 · `items.*.service_id` required/exists:services ·
`items.*.quantity` required/int/min:1. Update: unique ignore-self, slug
re-derived.

### Transaction

Store: `DB::transaction { Package::create + items()->createMany }`.
Update: `DB::transaction { update + items()->delete + items()->createMany }`.

## Architecture

- **Model:** `model.Package` (PriceCents int64) + `model.PackageItem`
  (with Service pointer for DTO). `model.PackageItemInput` for writes.
- **Repository:** `PackageStore` — CountActive/ListActive (batch item+service
  load, no N+1), FindActiveByID/FindByID (with items), NameTaken/SlugTaken,
  Create/InsertItems/DeleteItems/Update/Delete. 1062 →
  ErrDuplicateName/ErrDuplicateSlug. `ServiceIDsExist` on
  `MySQLServiceStore` validates items' service_ids.
- **Service:** `PackageService` — list (pagination+search), detail (active-404),
  Create (tx: unique+service-exists checks → create pkg → insert items),
  Update (tx: find → checks → update → delete items → insert items → re-read),
  Delete (find → hard delete, no guard). All via `TxManager`.
- **Handler + DTO:** price parsed exactly via `big.Rat` → cents; response
  price as two-decimal string; nested items with ServiceResource (category
  null); `boolish` for is_active/is_popular boolean validation.

## Money

Same boundary as FASE 8B: JSON → `big.Rat` → cents int64 → `DECIMAL(12,2)` →
cents → `"300.00"` string.

## Package Item Boundary

Items are managed **only as part of Package operations** (create/update):
- Store: items inserted in the same transaction as the package.
- Update: items fully replaced (delete all + re-create) in one transaction.
- Delete: FK CASCADE removes items.

No standalone Package Item CRUD exists in Laravel. Item domain is self-contained
within Package — **no separate FASE 8D needed.**

## Laravel → Go Parity

| Item | Status |
|---|---|
| 5 endpoints/methods/roles | MATCH |
| Active-only public list + items active-service filter | MATCH |
| Search + is_popular DESC + name order + per_page | MATCH |
| Slug auto + update re-derive | MATCH |
| PackageResource + PackageItemResource + nested ServiceResource | MATCH |
| Items fully replaced on update (tx) | MATCH |
| Hard delete no guard (FK CASCADE removes items) | MATCH |
| Price string 2dp | MATCH |
| Validation rules + messages | MATCH |
| Service-exists check on items | APPROXIMATION (bulk check; Laravel validates per-index) |

## Testing

- Service 8 tests: list+detail, create (slug+items+service-check), dup name,
  bad service, update (replace items), delete (hard, 404), repo error.
- Handler 7 tests: public list (price string, nested items/service), detail
  404, admin 201/422/delete/403, 500 generic.
- Repository gated: CRUD, items insert/delete/replace, money round-trip, unique
  slug, active list + item attachment, double-delete 404.
- Full regression: all prior phases pass.

## Verification

| Check | Result |
|---|---|
| `go fmt` | clean |
| `go vet` | 0 |
| `go test -p 1 ./...` | PASS (all packages) |
| `go test -race -p 1 ./...` | PASS (race=0) |
| `go build ./...` | 0 |
| Repo integration | SKIP without `TEST_DATABASE_URL` |

## Known Limitations

- Service-exists validation is bulk (count vs expected); Laravel validates
  per-index → individual field-path errors approximated as generic items error.
- `scanner` interface redeclared (renamed to avoid collision with user_store's
  `rowScanner`; same-package types coexist).

## Fundamental Go Concepts

Transactional multi-write (create package + items atomically via `TxManager`);
batch N+1 avoidance (JOIN items+services per package set); `big.Rat` exact
money parse; items full-replace pattern (delete+recreate); FK CASCADE as a
delete strategy; `boolish` for strict JSON boolean; DTO nesting (3 levels:
package → item → service).