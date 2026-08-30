# Security Matrix

Phase 17. Authorization matrix for all 48 Go routes (Laravel parity).

Legend: P=public, C=customer, T=technician, A=admin/super_admin, O=ownership.

| Method | Path | Public | Customer | Technician | Admin | Ownership (after role) |
|---|---|---|---|---|---|---|
| POST | /api/register | ✅ | — | — | — | — |
| POST | /api/login | ✅ | — | — | — | — |
| POST | /api/email/verification/resend | ✅ | — | — | — | — |
| POST | /api/email/verification/verify | ✅ | — | — | — | — |
| GET | /api/categories | ✅ | — | — | — | — |
| GET | /api/services | ✅ | — | — | — | — |
| GET | /api/services/:id | ✅ | — | — | — | — |
| GET | /api/packages | ✅ | — | — | — | — |
| GET | /api/packages/:id | ✅ | — | — | — | — |
| GET | /api/health · /api/ready | ✅ | — | — | — | — |
| GET | /api/users/:id | ✅ | — | — | — | — (FASE 7A educational, extra) |
| POST | /api/logout | — | ✅ | ✅ | ✅ | current token |
| GET | /api/profile | — | ✅ | ✅ | ✅ | — |
| PUT | /api/profile | — | ✅ | ✅ | ✅ | — |
| PUT | /api/profile/password | — | ✅ | ✅ | ✅ | — |
| GET | /api/notifications | — | ✅ | ✅ | ✅ | owner notifications |
| POST | /api/notifications/read-all | — | ✅ | ✅ | ✅ | own rows |
| POST | /api/notifications/:id/read | — | ✅ | ✅ | ✅ | row.belongs(user) |
| GET | /api/customer-profile | — | ✅ | — | — | own profile |
| PUT | /api/customer-profile | — | ✅ | — | — | own profile |
| GET | /api/bookings | — | ✅ | — | — | own bookings |
| POST | /api/bookings | — | ✅ | — | — | own profile completeness |
| GET | /api/bookings/:id | — | ✅ | — | — | booking.customer_id |
| POST | /api/bookings/:id/cancel | — | ✅ | — | — | booking.customer_id + state |
| GET | /api/invoices | — | ✅ | — | — | invoice→booking.customer_id |
| GET | /api/invoices/:id | — | ✅ | — | — | invoice→booking.customer_id |
| POST | /api/invoices/:id/payment-proof | — | ✅ | — | — | invoice owner + state + amount |
| GET | /api/invoices/:id/payment-proof | — | ✅ | — | — | invoice owner |
| GET | /api/bookings/:id/review | — | ✅ | — | — | booking.customer_id |
| POST | /api/bookings/:id/review | — | ✅ | — | — | owner + completed + tech + dup |
| GET | /api/technician/profile | — | — | ✅ | — | own profile |
| PUT | /api/technician/profile | — | — | ✅ | — | own profile |
| GET | /api/technician/jobs | — | — | ✅ | — | own assignments |
| GET | /api/technician/jobs/:id | — | — | ✅ | — | assignment.technician_id |
| POST | /api/technician/jobs/:id/accept | — | — | ✅ | — | assignment.technician_id + state |
| POST | /api/technician/jobs/:id/reject | — | — | ✅ | — | assignment.technician_id + state |
| POST | /api/technician/jobs/:id/start | — | — | ✅ | — | assignment.technician_id + state |
| POST | /api/technician/jobs/:id/complete | — | — | ✅ | — | assignment.technician_id + state |
| POST | /api/admin/categories | — | — | — | ✅ | role |
| PUT | /api/admin/categories/:id | — | — | — | ✅ | role |
| DELETE | /api/admin/categories/:id | — | — | — | ✅ | role + no-services 409 |
| POST | /api/admin/services | — | — | — | ✅ | role |
| PUT | /api/admin/services/:id | — | — | — | ✅ | role |
| DELETE | /api/admin/services/:id | — | — | — | ✅ | role + package-guard 409 |
| POST | /api/admin/packages | — | — | — | ✅ | role |
| PUT | /api/admin/packages/:id | — | — | — | ✅ | role |
| DELETE | /api/admin/packages/:id | — | — | — | ✅ | role |
| GET | /api/admin/bookings | — | — | — | ✅ | role |
| POST | /api/admin/bookings/:id/verify | — | — | — | ✅ | role + state/assignment guards |
| POST | /api/admin/bookings/:id/assign | — | — | — | ✅ | role + booking confirmed/paid |
| GET | /api/admin/payments | — | — | — | ✅ | role |
| GET | /api/admin/payments/:id/proof | — | — | — | ✅ | role (owner customer also via policy) |
| POST | /api/admin/payments/:id/verify | — | — | — | ✅ | role + pending state |
| POST | /api/admin/payments/:id/reject | — | — | — | ✅ | role + pending state + note |
| GET | /api/admin/reviews | — | — | — | ✅ | role |
| POST | /api/admin/reviews/:id/moderate | — | — | — | ✅ | role + status enum |

## Notes
- No missing auth; no wrong-role drift; no privilege escalation found.
- Routers attach rate limits on sensitive endpoints (register/login/otp/
  booking/payment) — see `docs/security.md`.
- Every customer-scoped id endpoint verifies ownership before returning data
  (IDOR closed); technician scoping via assignment.technician_id.