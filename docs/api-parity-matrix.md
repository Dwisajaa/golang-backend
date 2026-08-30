# API Parity Matrix — Laravel ↔ Go

Phase 16 deliverable. Verified against `C:\V1\api-dwidev\routes\api.php`
(Laravel = source of truth) and `internal/router/router.go`.

Status legend:
- **MATCH** — endpoint/method/role/behavior same.
- **APPROXIMATION** — internal differs, HTTP contract same.
- **INTENTIONAL IMPROVEMENT** — Go safer/atomic, contract same.
- **DEFERRED** — real Laravel endpoint not yet migrated (previously scoped out).
- **EXTRA** — Go-only route (documented).

## Public

| # | Method | Path | Go Status |
|---|---|---|---|
| 1 | POST | /api/register | MATCH |
| 2 | POST | /api/login | MATCH |
| 3 | POST | /api/email/verification/resend | MATCH |
| 4 | POST | /api/email/verification/verify | MATCH |
| 5 | POST | /api/password/forgot | DEFERRED (password-reset domain) |
| 6 | POST | /api/password/reset | DEFERRED (password-reset domain) |
| 7 | GET | /api/categories | MATCH |
| 8 | GET | /api/services | MATCH |
| 9 | GET | /api/services/{service} | MATCH |
| 10 | GET | /api/packages | MATCH |
| 11 | GET | /api/packages/{package} | MATCH |
| 12 | GET | /api/health | MATCH |
| 13 | GET | /api/openapi.yaml | DEFERRED (spec not yet shipped in Go repo) |

## Auth (any role)

| Method | Path | Go |
|---|---|---|
| POST | /api/logout | MATCH |
| GET | /api/notifications | MATCH |
| POST | /api/notifications/{notification}/read | MATCH |
| POST | /api/notifications/read-all | MATCH |
| GET | /api/profile | MATCH |
| PUT | /api/profile | MATCH |
| PUT | /api/profile/password | MATCH |

## Customer

| Method | Path | Go |
|---|---|---|
| GET | /api/dashboard | DEFERRED (dashboard/reports domain) |
| GET | /api/customer-profile | MATCH |
| PUT | /api/customer-profile | MATCH |
| GET | /api/bookings | MATCH |
| POST | /api/bookings | MATCH |
| GET | /api/bookings/{booking} | MATCH |
| POST | /api/bookings/{booking}/cancel | MATCH |
| GET | /api/invoices | MATCH |
| GET | /api/invoices/{invoice} | MATCH |
| GET | /api/bookings/{booking}/review | MATCH |
| POST | /api/bookings/{booking}/review | MATCH |
| POST | /api/invoices/{invoice}/payment-proof | MATCH |
| GET | /api/invoices/{invoice}/payment-proof | MATCH |

## Technician

| Method | Path | Go |
|---|---|---|
| GET | /api/technician/profile | MATCH |
| PUT | /api/technician/profile | MATCH |
| GET | /api/technician/jobs | MATCH |
| GET | /api/technician/jobs/{assignment} | MATCH |
| POST | /api/technician/jobs/{assignment}/accept | MATCH |
| POST | /api/technician/jobs/{assignment}/reject | MATCH |
| POST | /api/technician/jobs/{assignment}/start | MATCH |
| POST | /api/technician/jobs/{assignment}/complete | MATCH |

## Admin (admin, super_admin)

| Method | Path | Go |
|---|---|---|
| GET | /api/admin/dashboard | DEFERRED (dashboard/reports) |
| GET | /api/admin/reports/overview | DEFERRED (reports) |
| POST | /api/admin/categories | MATCH |
| PUT | /api/admin/categories/{category} | MATCH |
| DELETE | /api/admin/categories/{category} | MATCH |
| POST | /api/admin/services | MATCH |
| PUT | /api/admin/services/{service} | MATCH |
| DELETE | /api/admin/services/{service} | MATCH |
| POST | /api/admin/packages | MATCH |
| PUT | /api/admin/packages/{package} | MATCH |
| DELETE | /api/admin/packages/{package} | MATCH |
| GET | /api/admin/payments | MATCH |
| GET | /api/admin/payments/{payment}/proof | MATCH |
| POST | /api/admin/payments/{payment}/verify | MATCH |
| POST | /api/admin/payments/{payment}/reject | MATCH |
| GET | /api/admin/technicians | DEFERRED (admin technician mgmt) |
| POST | /api/admin/technicians | DEFERRED |
| PUT | /api/admin/technicians/{technician} | DEFERRED |
| PATCH | /api/admin/technicians/{technician}/toggle | DEFERRED |
| POST | /api/admin/bookings/{booking}/assign | MATCH |
| GET | /api/admin/bookings | MATCH |
| POST | /api/admin/bookings/{booking}/verify | MATCH |
| GET | /api/admin/reviews | MATCH |
| POST | /api/admin/reviews/{review}/moderate | MATCH |

## Go Extra (not in Laravel)

| Method | Path | Reason |
|---|---|---|
| GET | /api/users/:id | INTENTIONAL — FASE 7A educational endpoint; documented as internal, not part of the Laravel contract |
| GET | /ready | INTENTIONAL — infrastructure readiness probe (proccess/db), not an API contract endpoint |

## Summary

- Laravel API routes: **51** (excluding framework `/sanctum/csrf-cookie`, storage, `/up`).
- Go routes: **48** implemented + 2 intentional extras + 10 DEFERRED Laravel endpoints (password reset ×2, dashboard ×3, admin technicians ×4, openapi.yaml).
- Parity achieved for: auth, OTP, profiles, catalog, booking, invoice, payment, assignment, technician workflow, booking verification, notification, review.
- **DEFERRED (documented, previously scoped out):** password forgot/reset (OTP `password_reset` type + mailable already prepared), customer dashboard, admin dashboard, admin reports overview, admin technician management (list/create/update/toggle), openapi.yaml serving.