# Laravel Audit — api-dwidev

Source: `C:\V1\api-dwidev`
Audit date: 2026-08-27
Auditor: FASE 0 automated inspection (no Laravel code modified)

---

## 1. Versions

| Component | Version |
|---|---|
| Laravel Framework | 12.67.0 |
| PHP (host) | 8.2.12 (cli, ZTS) |
| PHP (Docker/prod) | 8.4-cli |
| Composer | 2.7.7 |
| PHPUnit | 11.5.56 |

`composer.json` requires `php ^8.2`.

---

## 2. Dependencies

### Production

| Package | Version |
|---|---|
| laravel/framework | ^12.0 (locked v12.67.0) |
| laravel/sanctum | ^4.0 (locked v4.3.3) |
| laravel/tinker | ^2.10.1 (locked v2.11.1) |

### Development

| Package | Purpose |
|---|---|
| fakerphp/faker | Test data generation |
| laravel/pail | Real-time log viewer |
| laravel/pint | Code style fixer |
| laravel/sail | Docker dev environment |
| mockery/mockery | Test mocking |
| nunomaduro/collision | CLI error rendering |
| phpunit/phpunit | Test framework |

No external HTTP clients, Redis, WebSocket, or payment-gateway packages.

---

## 3. Database

### Engine

MySQL 8.0 (Docker). Tests use SQLite `:memory:`.

### Migrations (21 files)

| # | Migration | Tables / Changes |
|---|---|---|
| 1 | `0001_01_01_000000` | `users`, `password_reset_tokens`, `sessions` |
| 2 | `0001_01_01_000001` | `cache`, `cache_locks` |
| 3 | `0001_01_01_000002` | `jobs`, `job_batches`, `failed_jobs` |
| 4 | `2026_08_15_004849` | `personal_access_tokens` (Sanctum) |
| 5 | `2026_08_16_123450` | `email_verification_otps` |
| 6 | `2026_08_21_024852` | adds `type` column to `email_verification_otps` |
| 7 | `2026_08_23_000001` | adds `role` column + index to `users` |
| 8 | `2026_08_23_000002` | composite index on `email_verification_otps` |
| 9 | `2026_08_23_000010` | `service_categories` |
| 10 | `2026_08_23_000011` | `services` |
| 11 | `2026_08_23_000012` | `packages` |
| 12 | `2026_08_23_000013` | `package_items` (pivot Service ↔ Package) |
| 13 | `2026_08_23_000020` | `customer_profiles` |
| 14 | `2026_08_23_000021` | `bookings` |
| 15 | `2026_08_23_000022` | `booking_items` |
| 16 | `2026_08_23_000023` | `invoices` |
| 17 | `2026_08_23_000030` | `payments` |
| 18 | `2026_08_23_000040` | `technician_profiles` |
| 19 | `2026_08_23_000041` | `booking_assignments` |
| 20 | `2026_08_23_000050` | `notifications` |
| 21 | `2026_08_23_000060` | `reviews` |

Total application tables: 16 domain + 5 framework (cache, jobs, sessions, password_reset_tokens, personal_access_tokens).

---

## 4. Models (14)

| Model | Table | Key Relationships |
|---|---|---|
| User | users | hasMany(Booking via customer_id), hasOne(CustomerProfile), hasOne(TechnicianProfile), hasMany(BookingAssignment via technician_id), hasMany(Review via technician_id), hasMany(EmailVerificationOtp) |
| Booking | bookings | belongsTo(User/customer_id), hasMany(BookingItem), hasOne(Invoice), hasMany(BookingAssignment), hasOne(Review) |
| BookingAssignment | booking_assignments | belongsTo(Booking), belongsTo(User/technician_id), belongsTo(User/assigned_by), belongsTo(TechnicianProfile) |
| BookingItem | booking_items | belongsTo(Booking), belongsTo(Service), belongsTo(Package) |
| CustomerProfile | customer_profiles | belongsTo(User) |
| EmailVerificationOtp | email_verification_otps | belongsTo(User) |
| Invoice | invoices | belongsTo(Booking), hasMany(Payment) |
| Package | packages | hasMany(PackageItem), belongsToMany(Service via package_items) |
| PackageItem | package_items | belongsTo(Package), belongsTo(Service) |
| Payment | payments | belongsTo(Invoice), belongsTo(User/verified_by) |
| Review | reviews | belongsTo(Booking), belongsTo(User/customer_id), belongsTo(User/technician_id) |
| Service | services | belongsTo(ServiceCategory), belongsToMany(Package via package_items) |
| ServiceCategory | service_categories | hasMany(Service) |
| TechnicianProfile | technician_profiles | belongsTo(User) |

### Role Constants (User model)

- `customer`, `technician`, `admin`, `super_admin`

### Status State Machines

**Booking:** `pending_payment` → `waiting_verification` → `paid` → `confirmed` → `technician_assigned` → `in_progress` → `awaiting_verification` → `completed` / `cancelled` (transitions enforced by `Booking::transitionTo()`)

**Invoice:** `unpaid`, `pending_payment`, `paid`, `cancelled`, `expired`

**Payment:** `unpaid`, `waiting_verification`, `pending`, `paid`, `rejected`, `failed`, `expired`, `refunded`, `cancelled`

**BookingAssignment:** `pending`, `accepted`, `rejected`, `completed`

**Review:** `published`, `hidden`, `rejected`

### Code Generators

- `Booking::generateBookingCode()` → `BJA-YYYYMMDD-XXXX`
- `Invoice::generateInvoiceNumber()` → `INV-{booking_code}-XXXX`
- `Payment::generatePaymentCode()` → `PAY-{booking_code}-XXXX`
- `TechnicianProfile::generateTechnicianCode()` → `TECH-XXXX`

---

## 5. Controllers (25)

### Public API (15 controllers)

| Controller | Actions |
|---|---|
| AuthController | register, login, logout |
| EmailVerificationController | verifyOtp, resend |
| PasswordResetController | forgotPassword, resetPassword |
| ProfileController | show, update, updatePassword |
| CatalogController | categories, services, service, packages, package |
| BookingController | index, store, show, cancel |
| CustomerProfileController | show, update |
| DashboardController | customer |
| InvoiceController | index, show |
| PaymentController | storeProof, showProof |
| ReviewController | show, store |
| NotificationController | index, read, readAll |
| TechnicianController | profile, updateProfile |
| TechnicianJobController | index, show, accept, reject, start, complete |

### Admin API (10 controllers under `Api\Admin`)

| Controller | Actions |
|---|---|
| BookingController | index, verify |
| AssignmentController | assign |
| CategoryController | store, update, destroy |
| ServiceController | store, update, destroy |
| PackageController | store, update, destroy |
| PaymentController | index, showProof, verify, reject |
| DashboardController | __invoke (single action) |
| ReportController | overview |
| ReviewController | index, moderate |
| TechnicianController | index, store, update, toggle |

---

## 6. Routes

### Summary

| Group | Count | Auth | Role |
|---|---|---|---|
| Public (no auth) | 11 | — | — |
| Authenticated (any role) | 6 | sanctum | — |
| Customer | 14 | sanctum | customer |
| Technician | 8 | sanctum | technician |
| Admin | 24 | sanctum | admin,super_admin |
| Framework (web/sanctum/storage) | 5 | varies | — |
| **Total** | **68** | | |

Application API routes: 63 (excluding framework routes).

### Rate-Limited Routes

| Route | Limiter | Limit |
|---|---|---|
| POST /api/login | auth-login | 10/min |
| POST /api/register | auth-register | 5/min |
| POST /api/email/verification/verify | otp-verify | 10/min |
| POST /api/email/verification/resend | otp-resend | 3/10min |
| POST /api/password/forgot | password-reset | 5/10min |
| POST /api/password/reset | password-reset | 5/10min |
| POST /api/bookings | booking-create | 10/min |
| POST /api/invoices/{invoice}/payment-proof | payment-upload | 5/10min |

---

## 7. Authentication & Authorization

### Authentication

- **Mechanism:** Laravel Sanctum (token-based API authentication)
- **Token storage:** `personal_access_tokens` table
- **Token expiration:** configurable via `SANCTUM_TOKEN_EXPIRATION` (default 10080 min = 7 days)
- **Login flow:** email + password → bcrypt verify → create Sanctum token → return `{ user, token, token_type }`
- **Logout:** deletes current access token
- **Password change:** revokes all other tokens

### Email Verification (OTP)

- 6-digit OTP, hashed with bcrypt
- Stored in `email_verification_otps` table
- Two types: `email_verification`, `password_reset`
- Expiration: `OTP_EXPIRATION_MINUTES` (default 10)
- Max attempts: `OTP_MAX_ATTEMPTS` (default 5)
- OTP email sent via queued mail (database queue)

### Authorization

- **Role middleware** (`RoleMiddleware`): checks `$request->user()->role` against allowed roles; 403 on mismatch
- **Policies:** ownership-based authorization for Booking, BookingAssignment, Invoice, Payment, Review
- Policy checks include: ownership verification, role checks, state guards (e.g., review requires completed booking with assigned technician)

---

## 8. Form Requests / Validation (28 files)

| Request | Key Rules |
|---|---|
| RegisterRequest | name required, email unique, password min:8 confirmed |
| LoginRequest | email required, password required |
| VerifyEmailRequest | email required exists, otp required string |
| ResendVerificationRequest | email required exists |
| ForgotPasswordRequest | email required email |
| ResetPasswordRequest | email required, otp required, password min:8 confirmed |
| UpdateProfileRequest | name, email unique (ignore self) |
| UpdatePasswordRequest | current_password required, password min:8 confirmed |
| StoreBookingRequest | booking_date, booking_time (in TIME_SLOTS), address, items array with service_id/package_id/quantity |
| CancelBookingRequest | (empty — authorization only) |
| UploadPaymentProofRequest | proof_image file max:5MB (jpg/png/pdf), amount, payment_method, customer_note |
| StoreReviewRequest | rating 1-5, comment max:1000 |
| StoreCategoryRequest | name, slug unique, description, icon, is_active |
| UpdateCategoryRequest | same, slug unique ignore self |
| StoreServiceRequest | category_id exists, name, slug unique, price, unit, duration, is_active |
| UpdateServiceRequest | same, slug unique ignore self |
| StorePackageRequest | name, slug unique, price, duration, items array |
| UpdatePackageRequest | same, slug unique ignore self |
| StoreTechnicianRequest | name, email unique, password min:8 confirmed, phone, specialization |
| UpdateTechnicianRequest | name, email unique ignore, password nullable, phone, specialization, is_active |
| UpdateTechnicianProfileRequest | phone, address, bio |
| UpdateCustomerProfileRequest | full_name, phone, address, city, postal_code |
| AssignTechnicianRequest | technician_id exists |
| RejectAssignmentRequest | rejection_reason required |
| CompleteJobRequest | technician_note nullable |
| RejectPaymentRequest | admin_note required |
| ModerateReviewRequest | status in published/hidden/rejected |
| VerifyCompletionRequest | action in approve/reject, admin_verification_note |

---

## 9. Services (3)

| Service | Responsibility |
|---|---|
| BookingPricingService | Integer-arithmetic monetary calculation (avoids float imprecision) |
| DashboardService | Aggregate metrics for customer dashboard, admin dashboard, admin reports |
| NotificationService | Dispatches `SystemNotification` to users; fetches admin users |

---

## 10. API Resources (21)

Used for JSON response serialization:
AdminDashboardResource, BookingAssignmentResource, BookingItemResource, BookingResource, CategoryResource, CustomerDashboardResource, CustomerProfileResource, InvoiceResource, NotificationResource, PackageItemResource, PackageResource, PaymentResource, ReportResource, ReviewResource, ServiceResource, TechnicianProfileResource, TechnicianResource, UserResource.

---

## 11. Middleware

| Middleware | Purpose |
|---|---|
| RoleMiddleware | Role-based access control (403 on mismatch) |
| RequestIdMiddleware | Generates/propagates `X-Request-ID` header + log context |
| SecurityHeadersMiddleware | `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`, conditional HSTS |
| CorsAllowlistMiddleware | Strips CORS headers if origin not in allowlist |

Global API middleware stack (registered in `bootstrap/app.php`):
`RequestIdMiddleware` → `SecurityHeadersMiddleware` → `CorsAllowlistMiddleware`

---

## 12. Exception Handling

Centralized in `bootstrap/app.php` for `api/*` routes:

| Exception | HTTP Status | Response |
|---|---|---|
| NotFoundHttpException | 404 | `{ "message": "Resource not found." }` |
| AuthenticationException / RouteNotFoundException | 401 | `{ "message": "Unauthenticated." }` |
| AuthorizationException | 403 | `{ "message": "Forbidden." }` |
| ThrottleRequestsException | 429 | `{ "message": "Too many requests. Please try again later." }` |
| ValidationException | 422 | `{ "message": "The given data was invalid.", "errors": { ... } }` |
| HttpExceptionInterface | varies | Generic message |
| Unhandled | 500 | `{ "message": "Server error." }` + structured log |

Unhandled exceptions log: `request_id`, `route`, `user_id`, `exception class`. No sensitive data (no auth header, password, OTP, or paths).

---

## 13. File Upload

- **Payment proof**: uploaded via `POST /api/invoices/{invoice}/payment-proof`
- **Storage disk**: `payment_proofs` → `storage/app/private/payment-proofs`
- **Validation**: max 5 MB, types jpg/png/pdf
- **Access**: admin via `GET /api/admin/payments/{payment}/proof`, customer via `GET /api/invoices/{invoice}/payment-proof`
- **Security**: private disk, no public URL, ownership checks via policies

---

## 14. Email & Notifications

### Mail (2 Mailables)

| Mailable | Queue | View |
|---|---|---|
| EmailVerificationOtpMail | no (sync) | emails.email-verification-otp |
| PasswordResetOtpMail | yes (ShouldQueue) | emails.password_reset_otp |

### Notifications (1 class)

- `SystemNotification` — database channel only
- Carries: event, title, message, optional bookingId/invoiceId/paymentId/assignmentId/actionUrl
- Dispatched by `NotificationService` on booking/payment/assignment state changes

---

## 15. Queue & Cache

- **Queue driver**: `database` (production), `sync` (testing)
- **Cache store**: `database` (production), `array` (testing)
- **Session driver**: `database` (production), `array` (testing)
- Queue tables: `jobs`, `job_batches`, `failed_jobs` (from migration 3)
- One queued mailable: `PasswordResetOtpMail`

---

## 16. Configuration / Environment Variables

### Custom env vars (beyond Laravel defaults)

| Variable | Default | Purpose |
|---|---|---|
| CORS_ALLOWED_ORIGINS | http://localhost:3000 | Comma-separated allowed origins |
| SANCTUM_TOKEN_EXPIRATION | 10080 | Token lifetime in minutes |
| OTP_EXPIRATION_MINUTES | 10 | OTP validity window |
| OTP_MAX_ATTEMPTS | 5 | OTP verification attempts before lockout |
| SECURITY_HSTS | false | Enable HSTS header |
| BCRYPT_ROUNDS | 12 | Password hashing cost |
| TRUSTED_PROXIES | 127.0.0.1 | Reverse proxy IPs for rate limiter |

---

## 17. Tests

### Test Suite: 79 tests, 402 assertions — all PASS

| File | Tests | Domain |
|---|---|---|
| AuthApiTest | 14 | Register, login, logout, profile, password, OTP, rate limit |
| AdminBookingApiTest | 3 | Admin booking list + filters |
| AdminCompletionVerificationTest | 9 | Completion approve/reject, rollback |
| BookingApiTest | 9 | Booking CRUD, pricing, profile, cancellation, rollback |
| CatalogApiTest | 6 | Public catalog, admin CRUD, validation, delete conflicts |
| DashboardApiTest | 6 | Customer/admin dashboard, report, empty data |
| HardeningApiTest | 4 | Health, OpenAPI, CORS, security headers |
| NotificationApiTest | 5 | Notification CRUD, ownership, workflow triggers |
| PaymentApiTest | 8 | Upload proof, verify, reject, rollback |
| ReviewApiTest | 6 | Create, moderate, ownership, validation |
| TechnicianApiTest | 8 | Admin manage, technician profile, job workflow |

Test config: PHPUnit 11, SQLite in-memory, mail=array, queue=sync.

---

## 18. OpenAPI Documentation

`docs/openapi.yaml` exists (513 lines). Covers main endpoints. Should be cross-verified against route list in FASE 1.

---

## 19. Major API Domains

```
Authentication (register, login, logout, OTP verify, password reset)
    ↓
User/Profile (profile CRUD, password update)
    ↓
Customer Profile (extended profile with address/phone)
    ↓
Catalog (categories, services, packages — public read, admin CRUD)
    ↓
Booking (create, list, show, cancel — with pricing snapshot + invoice auto-creation)
    ↓
Payment (upload proof, admin verify/reject — state sync with invoice + booking)
    ↓
Technician (admin manage, self profile, job accept/reject/start/complete)
    ↓
Notification (database notifications triggered by workflow events)
    ↓
Review (customer create, admin moderate)
    ↓
Dashboard/Reports (customer metrics, admin metrics, admin report overview)
```

---

## 20. Deployment Infrastructure

- **Dockerfile**: PHP 8.4-cli, Composer 2, `deploy/entrypoint.sh` (config:cache, view:cache, artisan serve)
- **compose.yaml**: dev (MySQL 8.0 + app)
- **compose.staging.yaml**: production-like (APP_ENV=production, APP_DEBUG=false, all secrets required via interpolation, persistent volume for payment proofs, healthcheck)
- **CI**: GitHub Actions (`ci.yml`) — PHP 8.4, SQLite, `composer test`

---

## 21. Observed Issues

| # | Issue | Impact | Recommendation |
|---|---|---|---|
| 1 | `/api/health` and `/api/openapi.yaml` use route closures | `route:cache` cannot be used | Move to a controller |
| 2 | `EmailVerificationOtpMail` is not queued (sync) | Blocks HTTP response during email send | Add `ShouldQueue` |
| 3 | No pagination on admin reviews/payments list | May become slow with large datasets | Add cursor/offset pagination |

None of these block Go migration. Documented for awareness.

---

## 22. Summary Statistics

| Metric | Count |
|---|---|
| API routes | 63 |
| Models | 14 |
| Controllers | 25 |
| Form Requests | 28 |
| Services | 3 |
| API Resources | 21 |
| Policies | 5 |
| Middleware (custom) | 4 |
| Migrations | 21 |
| Database tables | 21 |
| Test files | 11 |
| Test cases | 79 |
| Assertions | 402 |
| Composer packages (prod) | 3 |
| Composer packages (dev) | 7 |
