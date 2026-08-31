# Benchmark Matrix

All cases: same MySQL instance, same dataset, same request payload, same load
tool (`hey`), same machine. Auth tokens fetched once and reused (per
FASE 19 §7). Catalog + authenticated-read cases; write workload (POST
booking / payment-proof) only against isolated test data.

| # | Group | Endpoint | Auth | Concurrency | Duration | Notes |
|---|---|---|---|---|---|---|
| 1 | Health | GET /health (Go), GET /api/health (Laravel) | no | 1,5,25 | 30s | supplementary |
| 2 | Catalog | GET /api/categories?per_page=15 | no | 1,5,25 | 30s | N+1 watch |
| 3 | Catalog | GET /api/services?per_page=15 | no | 1,5,25 | 30s | |
| 4 | Catalog | GET /api/packages?per_page=15 | no | 1,5,25 | 30s | |
| 5 | Booking | GET /api/bookings?per_page=15 | bearer | 1,5,25 | 30s | |
| 6 | Booking | GET /api/bookings/{id} | bearer | 1,5,25 | 30s | id from seeded data |
| 7 | Invoice | GET /api/invoices?per_page=15 | bearer | 1,5,25 | 30s | |
| 8 | Notification | GET /api/notifications?per_page=15 | bearer | 1,5,25 | 30s | |
| 9 | Review | GET /api/admin/reviews?per_page=15 | admin bearer | 1,5,25 | 30s | |
| W1 | Write | POST /api/bookings (isolated customer) | bearer | 1,5 | 30s | **only on test data** |
| W2 | Write | POST /api/invoices/{id}/payment-proof | bearer | 1 | 30s | *optional*, fixture-controlled storage |

Concurrency levels may be raised (10,50,100) up to saturation on the deploy
host; record system saturation (CPU/RAM/DB) per case.

## Aggregation rule

3 runs minimum per (case, concurrency) → use **median** of RPS and latency
percentiles; report min/max too. Never mix "best Laravel" with "worst Go"
(FASE 19 §18).