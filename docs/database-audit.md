# Database Audit — api-dwidev Laravel

Source: `C:\V1\api-dwidev`
Audit date: 2026-08-27

---

## Database Environment

| Environment | Engine | Version | Database Name |
|---|---|---|---|
| Development (Docker) | MySQL | 8.0 | configurable via env |
| Testing (PHPUnit) | SQLite | :memory: | in-memory |
| Staging (compose.staging.yaml) | MySQL | 8.0 | configurable via env |
| .env.example default | SQLite | — | laravel |

Production target: MySQL 8.0.

---

## Final Schema

23 tables total (16 domain + 7 framework).

### Table Summary

| # | Table | Purpose | PK Type | Rows Model |
|---|---|---|---|---|
| 1 | users | User accounts | BIGINT auto | User |
| 2 | password_reset_tokens | Laravel password resets | VARCHAR(email) | — |
| 3 | sessions | Web sessions | VARCHAR(id) | — |
| 4 | cache | Cache store | VARCHAR(key) | — |
| 5 | cache_locks | Cache locks | VARCHAR(key) | — |
| 6 | jobs | Queue jobs | BIGINT auto | — |
| 7 | job_batches | Batch jobs | VARCHAR(id) | — |
| 8 | failed_jobs | Failed queue jobs | BIGINT auto | — |
| 9 | personal_access_tokens | Sanctum tokens | BIGINT auto | — |
| 10 | email_verification_otps | OTP records | BIGINT auto | EmailVerificationOtp |
| 11 | service_categories | Service categories | BIGINT auto | ServiceCategory |
| 12 | services | Individual services | BIGINT auto | Service |
| 13 | packages | Service packages | BIGINT auto | Package |
| 14 | package_items | Package ↔ Service pivot | BIGINT auto | PackageItem |
| 15 | customer_profiles | Customer details | BIGINT auto | CustomerProfile |
| 16 | bookings | Service bookings | BIGINT auto | Booking |
| 17 | booking_items | Booking line items | BIGINT auto | BookingItem |
| 18 | invoices | Billing invoices | BIGINT auto | Invoice |
| 19 | payments | Payment records | BIGINT auto | Payment |
| 20 | technician_profiles | Technician details | BIGINT auto | TechnicianProfile |
| 21 | booking_assignments | Tech job assignments | BIGINT auto | BookingAssignment |
| 22 | notifications | Database notifications | UUID | — (polymorphic) |
| 23 | reviews | Customer reviews | BIGINT auto | Review |

---

## Detailed Column Schema (Domain Tables Only)

### users

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| name | VARCHAR(255) | NO | — | — | — |
| email | VARCHAR(255) | NO | — | UNIQUE | — |
| role | VARCHAR(30) | NO | 'customer' | INDEX | — |
| email_verified_at | TIMESTAMP | YES | NULL | — | — |
| password | VARCHAR(255) | NO | — | — | — |
| remember_token | VARCHAR(100) | YES | NULL | — | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

### email_verification_otps

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| user_id | BIGINT UNSIGNED | NO | — | COMPOSITE | users(id) CASCADE |
| type | VARCHAR(30) | NO | 'email_verification' | COMPOSITE | — |
| otp_hash | VARCHAR(255) | NO | — | — | — |
| expires_at | TIMESTAMP | NO | — | COMPOSITE | — |
| used_at | TIMESTAMP | YES | NULL | COMPOSITE | — |
| attempts | TINYINT UNSIGNED | NO | 0 | — | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

Composite index: `(user_id, type, used_at, expires_at)` — OTP lookup.

### service_categories

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| name | VARCHAR(255) | NO | — | — | — |
| slug | VARCHAR(255) | NO | — | UNIQUE | — |
| description | TEXT | YES | NULL | — | — |
| icon | VARCHAR(255) | YES | NULL | — | — |
| is_active | BOOLEAN | NO | true | — | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

### services

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| service_category_id | BIGINT UNSIGNED | NO | — | — | service_categories(id) CASCADE |
| name | VARCHAR(255) | NO | — | — | — |
| slug | VARCHAR(255) | NO | — | UNIQUE | — |
| description | TEXT | YES | NULL | — | — |
| price | DECIMAL(12,2) | NO | 0 | — | — |
| unit | VARCHAR(255) | NO | 'per_service' | — | — |
| estimated_duration | INTEGER | YES | NULL | — | — |
| is_active | BOOLEAN | NO | true | — | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

### packages

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| name | VARCHAR(255) | NO | — | — | — |
| slug | VARCHAR(255) | NO | — | UNIQUE | — |
| description | TEXT | YES | NULL | — | — |
| price | DECIMAL(12,2) | NO | 0 | — | — |
| duration | INTEGER | YES | NULL | — | — |
| is_active | BOOLEAN | NO | true | — | — |
| is_popular | BOOLEAN | NO | false | — | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

### package_items (pivot)

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| package_id | BIGINT UNSIGNED | NO | — | COMPOSITE | packages(id) CASCADE |
| service_id | BIGINT UNSIGNED | NO | — | COMPOSITE | services(id) CASCADE |
| quantity | INTEGER | NO | 1 | — | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

### customer_profiles

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| user_id | BIGINT UNSIGNED | NO | — | UNIQUE | users(id) CASCADE |
| full_name | VARCHAR(255) | NO | — | — | — |
| phone | VARCHAR(20) | NO | — | — | — |
| address | VARCHAR(255) | NO | — | — | — |
| city | VARCHAR(100) | NO | — | — | — |
| postal_code | VARCHAR(10) | YES | NULL | — | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

### bookings

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| booking_code | VARCHAR(255) | NO | — | UNIQUE | — |
| customer_id | BIGINT UNSIGNED | NO | — | COMPOSITE | users(id) CASCADE |
| booking_date | DATE | NO | — | — | — |
| booking_time | VARCHAR(255) | NO | — | — | — |
| address | VARCHAR(255) | NO | — | — | — |
| address_detail | VARCHAR(255) | YES | NULL | — | — |
| latitude | DECIMAL(10,7) | YES | NULL | — | — |
| longitude | DECIMAL(10,7) | YES | NULL | — | — |
| customer_note | TEXT | YES | NULL | — | — |
| additional_jobdesk | TEXT | YES | NULL | — | — |
| subtotal | DECIMAL(12,2) | NO | 0 | — | — |
| additional_cost | DECIMAL(12,2) | NO | 0 | — | — |
| total_price | DECIMAL(12,2) | NO | 0 | — | — |
| status | VARCHAR(255) | NO | 'pending_payment' | INDEX | — |
| created_at | TIMESTAMP | YES | NULL | COMPOSITE(customer_id, created_at) | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

### booking_items

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| booking_id | BIGINT UNSIGNED | NO | — | COMPOSITE | bookings(id) CASCADE |
| service_id | BIGINT UNSIGNED | YES | NULL | — | services(id) SET NULL |
| package_id | BIGINT UNSIGNED | YES | NULL | — | packages(id) SET NULL |
| item_type | VARCHAR(20) | NO | — | COMPOSITE | — |
| item_name | VARCHAR(255) | NO | — | — | — |
| quantity | INTEGER | NO | 1 | — | — |
| unit_price | DECIMAL(12,2) | NO | 0 | — | — |
| subtotal | DECIMAL(12,2) | NO | 0 | — | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

FK behavior: `service_id` and `package_id` use SET NULL (snapshot: item_name/unit_price preserved even if catalog deleted).

### invoices

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| booking_id | BIGINT UNSIGNED | NO | — | UNIQUE | bookings(id) CASCADE |
| invoice_number | VARCHAR(255) | NO | — | UNIQUE | — |
| issued_at | TIMESTAMP | NO | — | — | — |
| due_at | TIMESTAMP | YES | NULL | — | — |
| subtotal | DECIMAL(12,2) | NO | 0 | — | — |
| additional_cost | DECIMAL(12,2) | NO | 0 | — | — |
| total_amount | DECIMAL(12,2) | NO | 0 | — | — |
| status | VARCHAR(255) | NO | 'unpaid' | INDEX | — |
| notes | TEXT | YES | NULL | — | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

### payments

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| invoice_id | BIGINT UNSIGNED | NO | — | COMPOSITE | invoices(id) CASCADE |
| payment_code | VARCHAR(255) | NO | — | UNIQUE | — |
| payment_method | VARCHAR(30) | NO | — | — | — |
| amount | DECIMAL(12,2) | NO | — | — | — |
| paid_at | TIMESTAMP | YES | NULL | — | — |
| status | VARCHAR(20) | NO | 'waiting_verification' | COMPOSITE | — |
| proof_image | VARCHAR(255) | YES | NULL | — | — |
| customer_note | TEXT | YES | NULL | — | — |
| admin_note | TEXT | YES | NULL | — | — |
| verified_by | BIGINT UNSIGNED | YES | NULL | — | users(id) SET NULL |
| verified_at | TIMESTAMP | YES | NULL | — | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

### technician_profiles

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| user_id | BIGINT UNSIGNED | NO | — | UNIQUE | users(id) CASCADE |
| technician_code | VARCHAR(255) | NO | — | UNIQUE | — |
| phone | VARCHAR(20) | YES | NULL | — | — |
| specialization | VARCHAR(255) | YES | NULL | — | — |
| address | VARCHAR(255) | YES | NULL | — | — |
| bio | TEXT | YES | NULL | — | — |
| is_active | BOOLEAN | NO | true | — | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

### booking_assignments

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| booking_id | BIGINT UNSIGNED | NO | — | INDEX | bookings(id) CASCADE |
| technician_id | BIGINT UNSIGNED | NO | — | COMPOSITE | users(id) CASCADE |
| assigned_by | BIGINT UNSIGNED | YES | NULL | — | users(id) SET NULL |
| assigned_at | TIMESTAMP | YES | NULL | — | — |
| accepted_at | TIMESTAMP | YES | NULL | — | — |
| rejected_at | TIMESTAMP | YES | NULL | — | — |
| started_at | TIMESTAMP | YES | NULL | — | — |
| completed_at | TIMESTAMP | YES | NULL | — | — |
| status | VARCHAR(20) | NO | 'pending' | COMPOSITE | — |
| rejection_reason | TEXT | YES | NULL | — | — |
| technician_note | TEXT | YES | NULL | — | — |
| admin_verification_note | TEXT | YES | NULL | — | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

### notifications (UUID PK)

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | UUID | NO | — | PK | — |
| type | VARCHAR(255) | NO | — | — | — |
| notifiable_type | VARCHAR(255) | NO | — | COMPOSITE | — |
| notifiable_id | BIGINT UNSIGNED | NO | — | COMPOSITE | — |
| data | TEXT | NO | — | — | — |
| read_at | TIMESTAMP | YES | NULL | INDEX | — |
| created_at | TIMESTAMP | YES | NULL | COMPOSITE | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

Polymorphic morph: `notifiable_type` + `notifiable_id` → `User`.

### reviews

| Column | Type | Nullable | Default | Index | FK |
|---|---|---|---|---|---|
| id | BIGINT UNSIGNED | NO | auto | PK | — |
| booking_id | BIGINT UNSIGNED | NO | — | UNIQUE | bookings(id) CASCADE |
| customer_id | BIGINT UNSIGNED | NO | — | — | users(id) CASCADE |
| technician_id | BIGINT UNSIGNED | NO | — | COMPOSITE | users(id) CASCADE |
| rating | TINYINT UNSIGNED | NO | — | — | — |
| comment | TEXT | YES | NULL | — | — |
| status | VARCHAR(20) | NO | 'published' | COMPOSITE | — |
| created_at | TIMESTAMP | YES | NULL | — | — |
| updated_at | TIMESTAMP | YES | NULL | — | — |

---

## Foreign Keys Summary

| Table | Column | References | On Delete |
|---|---|---|---|
| email_verification_otps | user_id | users(id) | CASCADE |
| services | service_category_id | service_categories(id) | CASCADE |
| package_items | package_id | packages(id) | CASCADE |
| package_items | service_id | services(id) | CASCADE |
| customer_profiles | user_id | users(id) | CASCADE |
| bookings | customer_id | users(id) | CASCADE |
| booking_items | booking_id | bookings(id) | CASCADE |
| booking_items | service_id | services(id) | SET NULL |
| booking_items | package_id | packages(id) | SET NULL |
| invoices | booking_id | bookings(id) | CASCADE |
| payments | invoice_id | invoices(id) | CASCADE |
| payments | verified_by | users(id) | SET NULL |
| technician_profiles | user_id | users(id) | CASCADE |
| booking_assignments | booking_id | bookings(id) | CASCADE |
| booking_assignments | technician_id | users(id) | CASCADE |
| booking_assignments | assigned_by | users(id) | SET NULL |
| reviews | booking_id | bookings(id) | CASCADE |
| reviews | customer_id | users(id) | CASCADE |
| reviews | technician_id | users(id) | CASCADE |

Total: **19 foreign keys**.

CASCADE vs SET NULL reasoning:
- CASCADE: child record meaningless without parent (booking_items without booking, invoice without booking)
- SET NULL: reference optional or snapshot preserved (booking_items.service_id SET NULL because item_name/unit_price are snapshot; payments.verified_by SET NULL because admin user might be deleted but payment record stays)

---

## Relationships (Model → Model)

```
User
 ├── hasOne  CustomerProfile       (user_id)
 ├── hasOne  TechnicianProfile     (user_id)
 ├── hasMany Booking               (customer_id)
 ├── hasMany BookingAssignment     (technician_id)
 ├── hasMany Review                (technician_id)  [as technicianReviews]
 ├── hasMany EmailVerificationOtp  (user_id)
 └── morphMany Notification        (notifiable)

Booking
 ├── belongsTo User                (customer_id)
 ├── hasMany   BookingItem         (booking_id)
 ├── hasOne    Invoice             (booking_id)
 ├── hasMany   BookingAssignment   (booking_id)
 └── hasOne    Review              (booking_id)

BookingItem
 ├── belongsTo Booking             (booking_id)
 ├── belongsTo Service             (service_id)  [nullable]
 └── belongsTo Package             (package_id)  [nullable]

Invoice
 ├── belongsTo Booking             (booking_id)
 └── hasMany   Payment             (invoice_id)

Payment
 ├── belongsTo Invoice             (invoice_id)
 └── belongsTo User                (verified_by) [nullable]

BookingAssignment
 ├── belongsTo Booking             (booking_id)
 ├── belongsTo User                (technician_id)
 ├── belongsTo User                (assigned_by)  [nullable]
 └── belongsTo TechnicianProfile   (technician_id→user_id)

Review
 ├── belongsTo Booking             (booking_id)
 ├── belongsTo User                (customer_id)
 └── belongsTo User                (technician_id)

ServiceCategory
 └── hasMany Service               (service_category_id)

Service
 ├── belongsTo    ServiceCategory  (service_category_id)
 └── belongsToMany Package         (via package_items, pivot: quantity)

Package
 ├── hasMany      PackageItem      (package_id)
 └── belongsToMany Service         (via package_items, pivot: quantity)

CustomerProfile
 └── belongsTo User                (user_id)

TechnicianProfile
 └── belongsTo User                (user_id)

EmailVerificationOtp
 └── belongsTo User                (user_id)
```

Total: **30 relationships** across 14 models.

---

## Enum / Status Fields

### Booking.status (9 states)

| State | Default | Terminal |
|---|---|---|
| pending_payment | YES | — |
| waiting_verification | — | — |
| paid | — | — |
| confirmed | — | — |
| technician_assigned | — | — |
| in_progress | — | — |
| awaiting_verification | — | — |
| completed | — | YES |
| cancelled | — | YES |

### Booking State Transitions (from `Booking::transitions()`)

```
pending_payment       → [waiting_verification, cancelled]
waiting_verification  → [paid, pending_payment, cancelled]
paid                  → [confirmed, cancelled]
confirmed             → [technician_assigned, cancelled]
technician_assigned   → [in_progress, confirmed]
in_progress           → [awaiting_verification]
awaiting_verification → [completed, in_progress]
completed             → (terminal)
cancelled             → (terminal)
```

Enforced by `Booking::transitionTo()` — throws `ValidationException` on invalid transition.

### Payment.status (9 states)

| State | Default | Terminal |
|---|---|---|
| unpaid | — | — |
| waiting_verification | YES (on proof upload) | — |
| pending | — | — |
| paid | — | YES |
| rejected | — | — |
| failed | — | YES |
| expired | — | YES |
| refunded | — | YES |
| cancelled | — | YES |

No formal state machine. Transitions managed by controller logic.

### Invoice.status (5 states)

| State | Default | Terminal |
|---|---|---|
| unpaid | YES | — |
| pending_payment | — | — |
| paid | — | YES |
| cancelled | — | YES |
| expired | — | YES |

### BookingAssignment.status (4 states)

| State | Default | Terminal |
|---|---|---|
| pending | YES | — |
| accepted | — | — |
| rejected | — | YES |
| completed | — | — (can revert to accepted on admin reject) |

### Review.status (3 states)

| State | Default |
|---|---|
| published | YES |
| hidden | — |
| rejected | — |

### User.role (4 values)

| Role | Default |
|---|---|
| customer | YES |
| technician | — |
| admin | — |
| super_admin | — |

### EmailVerificationOtp.type (2 values)

| Type | Default |
|---|---|
| email_verification | YES |
| password_reset | — |

---

## Transaction Analysis

15 endpoints use `DB::transaction`:

| # | Endpoint | Operations Inside Transaction |
|---|---|---|
| 1 | POST /api/email/verification/verify | verify OTP → mark used → verify email → create token |
| 2 | POST /api/password/reset | verify OTP → mark used → update password → delete all tokens |
| 3 | POST /api/bookings | lock catalog → create booking → create items → create invoice |
| 4 | POST /api/bookings/{booking}/cancel | transition booking → cancel invoice |
| 5 | POST /api/invoices/{invoice}/payment-proof | lock invoice → check pending → upload file → create payment → update invoice → transition booking |
| 6 | POST /api/bookings/{booking}/review | lock booking → check duplicate → create review |
| 7 | POST /api/admin/bookings/{booking}/assign | lock booking → lock technician → reject old assignment → create new → transition booking |
| 8 | POST /api/admin/bookings/{booking}/verify | lock booking → lock assignment → transition booking → update assignment → notify |
| 9 | POST /api/admin/payments/{payment}/verify | lock payment → check proof file → update payment → update invoice → transition booking ×2 |
| 10 | POST /api/admin/payments/{payment}/reject | lock payment → update payment → update invoice → transition booking |
| 11 | POST /api/admin/packages | create package → create items |
| 12 | PUT /api/admin/packages/{package} | update package → delete items → create items |
| 13 | POST /api/admin/technicians | create user → create profile |
| 14 | POST /api/technician/jobs/{assignment}/accept | lock assignment → update status |
| 15 | POST /api/technician/jobs/{assignment}/reject | lock assignment → update status → transition booking |
| 16 | POST /api/technician/jobs/{assignment}/start | lock assignment → update status → transition booking |
| 17 | POST /api/technician/jobs/{assignment}/complete | lock assignment → update status → transition booking |

Key patterns:
- **Row locking**: `lockForUpdate()` used on every transactional write to prevent race conditions
- **Cascade state updates**: booking/invoice/payment states often change together atomically
- **File + DB mixed**: payment proof upload stores file INSIDE transaction, with cleanup on rollback

---

## Business Rules (Not Visible from Schema Alone)

| Rule | Source | Go Implication |
|---|---|---|
| Booking requires complete customer profile (full_name, phone, address, city) | BookingController::store + CustomerProfile::isComplete() | Must check profile completeness before booking creation |
| Booking price is snapshot at creation (not live from catalog) | BookingController::store (lockForUpdate + copy price) | Store price at creation time, not join to catalog |
| Only one pending/accepted assignment per booking | AssignmentController::assign (auto-rejects old) | Enforce in service layer |
| Review only for completed booking with assigned technician | ReviewController::store + ReviewPolicy::create | Multi-condition check before creation |
| One review per booking | reviews.booking_id UNIQUE + lockForUpdate check | DB unique + app-level double check |
| One invoice per booking | invoices.booking_id UNIQUE | DB constraint |
| No pending payment allowed when submitting new proof | PaymentController::storeProof (pendingVerificationStatuses check) | Check existing payments in same transaction |
| Payment amount must match invoice total | PaymentController::storeProof (amount validation) | Controller-level validation |
| OTP single-use, max attempts, expiry | EmailVerificationController / PasswordResetController | Must replicate attempts counter, expiry, used_at check |
| Password change revokes all tokens | ProfileController::updatePassword | Delete all personal_access_tokens for user |
| Email change resets verification | ProfileController::update | Set email_verified_at to NULL, send new OTP |
| Booking code format: BJA-YYYYMMDD-XXXX | Booking::generateBookingCode() | Must generate unique code |
| Invoice number format: INV-{code}-XXXX | Invoice::generateInvoiceNumber() | Must generate from booking code |
| Payment code format: PAY-{code}-XXXX | Payment::generatePaymentCode() | Must generate from booking code |
| Technician code format: TECH-XXXX | TechnicianProfile::generateTechnicianCode() | Must generate unique code |
| Time slots: 08,09,10,11,13,14,15,16 | Booking::TIME_SLOTS | Validate in request |
| Login requires verified email | AuthController::login | 403 if email not verified |
| Password forgot returns generic message | PasswordResetController::forgotPassword | No user enumeration |

---

## File Storage

| Column | Table | Disk | Path Pattern | Private | Upload Validation |
|---|---|---|---|---|---|
| proof_image | payments | payment_proofs | `storage/app/private/payment-proofs/{hash}.{ext}` | YES | jpg,jpeg,png; max 2048KB; mimetypes image/jpeg,image/png |

- Upload: `POST /api/invoices/{invoice}/payment-proof`
- Customer download: `GET /api/invoices/{invoice}/payment-proof`
- Admin download: `GET /api/admin/payments/{payment}/proof`
- Authorization: InvoicePolicy (customer) / PaymentPolicy (admin)
- File cleanup: on transaction rollback, `Storage::disk('payment_proofs')->delete($path)`

Go implication: Must replicate private file storage with authorization-gated access. File path stored in DB, file served via API (not public URL).

---

## Seeders / Factories

### Seeders

| Seeder | Purpose |
|---|---|
| DatabaseSeeder | Default — creates one test user via UserFactory |
| DevE2eSeeder | E2E testing — creates 6 accounts at `@example.test` with password `password123` |

DevE2eSeeder accounts:

| Email | Role |
|---|---|
| customer@example.test | customer |
| customer2@example.test | customer |
| technician@example.test | technician |
| technician2@example.test | technician |
| admin@example.test | admin |
| superadmin@example.test | super_admin |

Artisan command: `php artisan e2e:reset` — truncates domain tables and re-seeds.

### Factories (6)

| Factory | Model | Key Fields |
|---|---|---|
| UserFactory | User | Random name/email, hashed "password", verified email |
| CustomerProfileFactory | CustomerProfile | Random profile data |
| ServiceCategoryFactory | ServiceCategory | Random name/slug |
| ServiceFactory | Service | Random name/slug/price |
| PackageFactory | Package | Random name/slug/price |
| TechnicianProfileFactory | TechnicianProfile | Random code/phone/specialization |

---

## Database Tests

11 feature test files, all use `RefreshDatabase` trait (SQLite in-memory).

| Test File | DB-Related Coverage |
|---|---|
| AuthApiTest (14 tests) | User creation, OTP records, token creation/deletion, email verification state |
| BookingApiTest (9 tests) | Booking/item/invoice creation, snapshot pricing, cancel cascade, rollback |
| PaymentApiTest (8 tests) | Payment creation, proof upload, verify/reject cascade, rollback |
| AdminBookingApiTest (3 tests) | Admin booking list, filters |
| AdminCompletionVerificationTest (9 tests) | Booking completion verify/reject, assignment state, rollback |
| CatalogApiTest (6 tests) | Category/service/package CRUD, delete conflict (409) |
| DashboardApiTest (6 tests) | Dashboard aggregation, empty data, revenue no double-counting |
| HardeningApiTest (4 tests) | Health, CORS, security headers |
| NotificationApiTest (5 tests) | Notification creation, ownership, read |
| ReviewApiTest (6 tests) | Review creation, duplicate prevention, moderate |
| TechnicianApiTest (8 tests) | Technician CRUD, assignment, job workflow |

Total: 79 tests, 402 assertions — all PASS.

Behaviors guaranteed by tests:
- Transaction rollback on failure (BookingApiTest, PaymentApiTest)
- Foreign key cascade (booking cancel → invoice cancel)
- Unique constraint enforcement (duplicate review → 409)
- Ownership authorization (customer cannot access other's booking)
- State transition validation (invalid status → 422)

---

## Laravel → Go Migration Considerations

### Decimal Precision

All monetary columns use `DECIMAL(12,2)`. Go must use a decimal library (e.g., `shopspring/decimal`) or integer arithmetic (cents). `float64` is NOT acceptable for money.

### Coordinates

`DECIMAL(10,7)` for lat/lng. Go `float64` is acceptable for coordinates (precision sufficient).

### UUID

Only `notifications` table uses UUID primary key. All other tables use BIGINT auto-increment. Go should use `uuid` package for notifications, `int64` for everything else.

### Timestamps

Laravel uses nullable `TIMESTAMP` for `created_at`/`updated_at`. Go should use `*time.Time` (pointer for nullable) or `sql.NullTime`.

### Polymorphic Relation

`notifications` table uses `notifiable_type` + `notifiable_id` (polymorphic). In Go, this is just two columns — `notifiable_type` will always be `App\Models\User` (hardcode the check).

### Row Locking

Laravel uses `lockForUpdate()` (SELECT FOR UPDATE). Go with `database/sql` must use raw SQL with `FOR UPDATE` clause inside a transaction.

---

## Migration Risks

| Risk | Severity | Description | Mitigation |
|---|---|---|---|
| Decimal precision loss | **HIGH** | Using float64 for money loses precision | Use `shopspring/decimal` or integer cents |
| Transaction isolation | **HIGH** | MySQL default is REPEATABLE READ; Go must match | Set isolation level explicitly in DB connection |
| State machine enforcement | **HIGH** | Booking::transitionTo() is application-level; no DB constraint | Must replicate transition map + validation in Go |
| Row locking (FOR UPDATE) | **HIGH** | Race conditions on concurrent booking/payment | Must use SELECT FOR UPDATE in Go transactions |
| Cascade delete behavior | **MEDIUM** | 15 CASCADE + 4 SET NULL foreign keys | Go must not delete parents without understanding cascade |
| OTP security model | **MEDIUM** | Bcrypt hash comparison, attempts tracking, expiry | Must replicate exact bcrypt + timing behavior |
| File in transaction | **MEDIUM** | Payment proof stored inside DB transaction with cleanup | Go must handle file cleanup on transaction rollback |
| Unique code generation | **MEDIUM** | Booking/invoice/payment codes generated with retry loop | Must handle unique constraint violations + retry |
| Polymorphic notification | **LOW** | `notifiable_type` is PHP class name string | Go can hardcode to `App\Models\User` or use own namespace |
| SQLite vs MySQL | **LOW** | Tests use SQLite but production uses MySQL | Go tests should also use MySQL or handle dialect differences |
| Soft delete | **NONE** | No models use soft delete | No concern |
| JSON columns | **NONE** | Only `notifications.data` is JSON-like (TEXT) | Parse as JSON string in Go |

---

# Database ERD

See separate file: `C:\V1\golang-backend\docs\database-erd.md`
