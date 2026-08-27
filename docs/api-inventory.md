# API Inventory — api-dwidev Laravel

Source: `C:\V1\api-dwidev`  
Inventory date: 2026-08-27  
Method: Cross-verification of routes/api.php, controllers, FormRequests, policies, services, and OpenAPI spec

---

## Domain: Authentication (P0 — Foundation)

### POST /api/register

| Attribute | Value |
|---|---|
| **Controller** | `Api\AuthController@register` |
| **Middleware** | `throttle:auth-register` (5 requests/minute) |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | None |
| **Request body** | `{ name, email, password, password_confirmation }` |
| **FormRequest** | `RegisterRequest` |
| **Validation** | `name`: required, string, max:255<br>`email`: required, email, max:255, unique:users<br>`password`: required, string, min:8, confirmed |
| **Query params** | None |
| **Response** | `{ user: UserResource, message: "..." }` |
| **Success status** | 201 |
| **Error status** | 422 (validation), 429 (rate limit) |
| **Business logic** | Creates customer user account with `role=customer`, sends email verification OTP |
| **Services** | None |
| **Models** | `User::create()` |
| **Relationships** | None |
| **File upload** | No |
| **Side effects** | User creation, OTP email queued via `$user->sendOtp(TYPE_EMAIL_VERIFICATION)` |
| **Transaction** | No |
| **Queue** | Yes — OTP email sent via queued mailable |
| **Notifications** | Email verification OTP sent |

---

### POST /api/login

| Attribute | Value |
|---|---|
| **Controller** | `Api\AuthController@login` |
| **Middleware** | `throttle:auth-login` (10 requests/minute) |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | None |
| **Request body** | `{ email, password }` |
| **FormRequest** | `LoginRequest` |
| **Validation** | `email`: required, string, email<br>`password`: required, string |
| **Query params** | None |
| **Response** | `{ user: UserResource, token: "...", token_type: "Bearer" }` |
| **Success status** | 200 |
| **Error status** | 401 (invalid credentials or unverified email), 422 (validation), 429 (rate limit) |
| **Business logic** | Authenticates user, checks email verification, creates Sanctum token |
| **Services** | None |
| **Models** | `User::where('email')->first()`, `Hash::check()`, `$user->hasVerifiedEmail()`, `$user->createToken()` |
| **Relationships** | None |
| **File upload** | No |
| **Side effects** | Token creation |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### POST /api/logout

| Attribute | Value |
|---|---|
| **Controller** | `Api\AuthController@logout` |
| **Middleware** | `auth:sanctum` |
| **Authentication** | Required (Sanctum token) |
| **Authorization** | Authenticated user |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ message: "Logged out successfully." }` |
| **Success status** | 200 |
| **Error status** | 401 (unauthenticated) |
| **Business logic** | Deletes current access token |
| **Services** | None |
| **Models** | `$request->user()->currentAccessToken()->delete()` |
| **Relationships** | None |
| **File upload** | No |
| **Side effects** | Current token deletion |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### POST /api/email/verification/resend

| Attribute | Value |
|---|---|
| **Controller** | `Api\EmailVerificationController@resend` |
| **Middleware** | `throttle:otp-resend` (3 requests/10 minutes) |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | None |
| **Request body** | `{ email }` |
| **FormRequest** | `ResendVerificationRequest` |
| **Validation** | `email`: required, email |
| **Query params** | None |
| **Response** | `{ message: "Kode verifikasi telah dikirim ke email Anda." }` |
| **Success status** | 200 |
| **Error status** | 422 (validation), 429 (rate limit) |
| **Business logic** | Resends email verification OTP if user exists and is not yet verified |
| **Services** | None |
| **Models** | `User::where('email')->first()`, `$user->hasVerifiedEmail()`, `$user->sendOtp(TYPE_EMAIL_VERIFICATION)` |
| **Relationships** | None |
| **File upload** | No |
| **Side effects** | OTP generation and email queuing |
| **Transaction** | No |
| **Queue** | Yes — OTP email queued |
| **Notifications** | Email verification OTP sent |

---

### POST /api/email/verification/verify

| Attribute | Value |
|---|---|
| **Controller** | `Api\EmailVerificationController@verifyOtp` |
| **Middleware** | `throttle:otp-verify` (10 requests/minute) |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | None |
| **Request body** | `{ email, otp }` |
| **FormRequest** | `VerifyEmailRequest` |
| **Validation** | `email`: required, email<br>`otp`: required, digits:6 |
| **Query params** | None |
| **Response** | `{ user: UserResource, token: "...", token_type: "Bearer", message: "..." }` |
| **Success status** | 200 |
| **Error status** | 400 (already verified), 422 (invalid/expired OTP), 429 (max attempts reached or rate limit) |
| **Business logic** | Verifies OTP, marks email as verified, creates token |
| **Services** | None |
| **Models** | `User::where('email')->first()`, `$user->emailVerificationOtps()->where()->latest()->first()`, `Hash::check()`, `$otpRecord->increment('attempts')`, `$otpRecord->update(['used_at'])`, `$user->forceFill(['email_verified_at'])->save()`, `$user->createToken()` |
| **Relationships** | `emailVerificationOtps` |
| **File upload** | No |
| **Side effects** | Email verification, OTP marked as used, token creation |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | None |

---

### POST /api/password/forgot

| Attribute | Value |
|---|---|
| **Controller** | `Api\PasswordResetController@forgotPassword` |
| **Middleware** | `throttle:password-reset` (5 requests/10 minutes) |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | None |
| **Request body** | `{ email }` |
| **FormRequest** | `ForgotPasswordRequest` |
| **Validation** | `email`: required, email |
| **Query params** | None |
| **Response** | `{ message: "..." }` (generic response for security) |
| **Success status** | 200 |
| **Error status** | 422 (validation), 429 (rate limit) |
| **Business logic** | Sends password reset OTP if user exists; returns generic success message regardless |
| **Services** | None |
| **Models** | `User::where('email')->first()`, `$user->sendOtp(TYPE_PASSWORD_RESET)` |
| **Relationships** | None |
| **File upload** | No |
| **Side effects** | OTP generation and email queuing (only if user exists) |
| **Transaction** | No |
| **Queue** | Yes — OTP email queued |
| **Notifications** | Password reset OTP sent |

---

### POST /api/password/reset

| Attribute | Value |
|---|---|
| **Controller** | `Api\PasswordResetController@resetPassword` |
| **Middleware** | `throttle:password-reset` (5 requests/10 minutes) |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | None |
| **Request body** | `{ email, otp, password, password_confirmation }` |
| **FormRequest** | `ResetPasswordRequest` |
| **Validation** | `email`: required, email<br>`otp`: required, digits:6<br>`password`: required, string, min:8, confirmed |
| **Query params** | None |
| **Response** | `{ message: "Password berhasil diatur ulang." }` |
| **Success status** | 200 |
| **Error status** | 422 (invalid/expired OTP), 429 (max attempts or rate limit) |
| **Business logic** | Verifies OTP, resets password, revokes all tokens |
| **Services** | None |
| **Models** | `User::where('email')->first()`, `$user->emailVerificationOtps()->where()->latest()->first()`, `Hash::check()`, `$otpRecord->increment('attempts')`, `$otpRecord->update(['used_at'])`, `$user->forceFill(['password'])->save()`, `$user->tokens()->delete()` |
| **Relationships** | `emailVerificationOtps`, `tokens` |
| **File upload** | No |
| **Side effects** | Password reset, OTP marked as used, all tokens deleted |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | None |

---

## Domain: User/Profile (P1 — Core)

### GET /api/profile

| Attribute | Value |
|---|---|
| **Controller** | `Api\ProfileController@show` |
| **Middleware** | `auth:sanctum` |
| **Authentication** | Required |
| **Authorization** | Authenticated user |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ user: UserResource }` |
| **Success status** | 200 |
| **Error status** | 401 (unauthenticated) |
| **Business logic** | Returns authenticated user profile |
| **Services** | None |
| **Models** | `$request->user()` |
| **Relationships** | None |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### PUT /api/profile

| Attribute | Value |
|---|---|
| **Controller** | `Api\ProfileController@update` |
| **Middleware** | `auth:sanctum` |
| **Authentication** | Required |
| **Authorization** | Authenticated user |
| **Path params** | None |
| **Request body** | `{ name, email }` |
| **FormRequest** | `UpdateProfileRequest` |
| **Validation** | `name`: required, string, max:255<br>`email`: required, email, max:255, unique:users (ignore self) |
| **Query params** | None |
| **Response** | `{ user: UserResource, message: "..." }` |
| **Success status** | 200 |
| **Error status** | 401 (unauthenticated), 422 (validation) |
| **Business logic** | Updates user profile; if email changes, resets email_verified_at and sends new OTP |
| **Services** | None |
| **Models** | `$user->forceFill(['name', 'email', 'email_verified_at'])->save()`, `$user->sendOtp(TYPE_EMAIL_VERIFICATION)` if email changed |
| **Relationships** | None |
| **File upload** | No |
| **Side effects** | Profile update, email verification reset if email changed, OTP queued if email changed |
| **Transaction** | No |
| **Queue** | Conditional — OTP email queued if email changed |
| **Notifications** | Email verification OTP sent if email changed |

---

### PUT /api/profile/password

| Attribute | Value |
|---|---|
| **Controller** | `Api\ProfileController@updatePassword` |
| **Middleware** | `auth:sanctum` |
| **Authentication** | Required |
| **Authorization** | Authenticated user |
| **Path params** | None |
| **Request body** | `{ current_password, password, password_confirmation }` |
| **FormRequest** | `UpdatePasswordRequest` |
| **Validation** | `current_password`: required, current_password<br>`password`: required, string, min:8, confirmed |
| **Query params** | None |
| **Response** | `{ message: "Password berhasil diperbarui." }` |
| **Success status** | 200 |
| **Error status** | 401 (unauthenticated), 422 (validation — wrong current password) |
| **Business logic** | Updates password and revokes all tokens |
| **Services** | None |
| **Models** | `$request->user()->update(['password'])`, `$request->user()->tokens()->delete()` |
| **Relationships** | `tokens` |
| **File upload** | No |
| **Side effects** | Password update, all tokens deleted |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

## Domain: Customer Profile (P1 — Core)

### GET /api/customer-profile

| Attribute | Value |
|---|---|
| **Controller** | `Api\CustomerProfileController@show` |
| **Middleware** | `auth:sanctum`, `role:customer` |
| **Authentication** | Required |
| **Authorization** | Customer role |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ data: CustomerProfileResource }` |
| **Success status** | 200 |
| **Error status** | 401 (unauthenticated), 403 (wrong role) |
| **Business logic** | Returns customer profile |
| **Services** | None |
| **Models** | `$request->user()->customerProfile` |
| **Relationships** | `customerProfile` |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### PUT /api/customer-profile

| Attribute | Value |
|---|---|
| **Controller** | `Api\CustomerProfileController@update` |
| **Middleware** | `auth:sanctum`, `role:customer` |
| **Authentication** | Required |
| **Authorization** | Customer role |
| **Path params** | None |
| **Request body** | `{ full_name, phone, address, city, postal_code }` |
| **FormRequest** | `UpdateCustomerProfileRequest` |
| **Validation** | `full_name`: required, string, max:255<br>`phone`: required, string, max:20<br>`address`: required, string, max:255<br>`city`: required, string, max:100<br>`postal_code`: nullable, string, max:10 |
| **Query params** | None |
| **Response** | `{ data: CustomerProfileResource, message: "..." }` |
| **Success status** | 200 |
| **Error status** | 401 (unauthenticated), 403 (wrong role), 422 (validation) |
| **Business logic** | Creates or updates customer profile |
| **Services** | None |
| **Models** | `$request->user()->customerProfile()->updateOrCreate(['user_id' => ...], $validated)` |
| **Relationships** | `customerProfile` |
| **File upload** | No |
| **Side effects** | Customer profile update/create |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

## Domain: Catalog (P2 — Public Read, Admin CRUD)

### GET /api/categories

| Attribute | Value |
|---|---|
| **Controller** | `Api\CatalogController@categories` |
| **Middleware** | None (public) |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | `page` (pagination) |
| **Response** | `{ data: [CategoryResource], meta: { ... }, links: { ... } }` |
| **Success status** | 200 |
| **Error status** | None |
| **Business logic** | Returns active categories that have active services, with eager-loaded services |
| **Services** | None |
| **Models** | `ServiceCategory::query()->where('is_active', true)->whereHas('services', fn => $query->where('is_active', true))->with(['services' => fn => $query->where('is_active', true)])->orderBy('name')->paginate()` |
| **Relationships** | `services` |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### GET /api/services

| Attribute | Value |
|---|---|
| **Controller** | `Api\CatalogController@services` |
| **Middleware** | None (public) |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | `category` (filter by category ID), `limit` (capped at 100), `page` |
| **Response** | `{ data: [ServiceResource], meta: { ... }, links: { ... } }` |
| **Success status** | 200 |
| **Error status** | None |
| **Business logic** | Returns active services with active categories; filterable by category; pagination limit capped at 100 |
| **Services** | None |
| **Models** | `Service::query()->where('is_active', true)->whereHas('category', fn => $query->where('is_active', true))->with('category')->when($request->filled('category'), fn => $query->where('service_category_id', $request->input('category')))->orderBy('name')->paginate(min($request->integer('limit', 15), 100))` |
| **Relationships** | `category` |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### GET /api/services/{service}

| Attribute | Value |
|---|---|
| **Controller** | `Api\CatalogController@service` |
| **Middleware** | None (public) |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | `service` (Service model binding) |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ data: ServiceResource }` |
| **Success status** | 200 |
| **Error status** | 404 (service not found or inactive, or category inactive) |
| **Business logic** | Returns single service if active and its category is active |
| **Services** | None |
| **Models** | Route model binding `Service`, `$service->category()->where('is_active', true)->exists()`, `$service->load('category')` |
| **Relationships** | `category` |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### GET /api/packages

| Attribute | Value |
|---|---|
| **Controller** | `Api\CatalogController@packages` |
| **Middleware** | None (public) |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | `popular` (filter boolean), `limit` (capped at 100), `page` |
| **Response** | `{ data: [PackageResource], meta: { ... }, links: { ... } }` |
| **Success status** | 200 |
| **Error status** | None |
| **Business logic** | Returns active packages with items; filterable by popularity; ordered by popularity then name |
| **Services** | None |
| **Models** | `Package::query()->where('is_active', true)->whereHas('items.service', fn => $query->where('is_active', true))->with(['items.service'])->when($request->filled('popular'), fn => $query->where('is_popular', $request->boolean('popular')))->orderByDesc('is_popular')->orderBy('name')->paginate(min($request->integer('limit', 15), 100))` |
| **Relationships** | `items.service` |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### GET /api/packages/{package}

| Attribute | Value |
|---|---|
| **Controller** | `Api\CatalogController@package` |
| **Middleware** | None (public) |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | `package` (Package model binding) |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ data: PackageResource }` |
| **Success status** | 200 |
| **Error status** | 404 (package not found or inactive) |
| **Business logic** | Returns single package if active |
| **Services** | None |
| **Models** | Route model binding `Package`, `abort_unless($package->is_active, 404)`, `$package->load(['items.service'])` |
| **Relationships** | `items.service` |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### POST /api/admin/categories

| Attribute | Value |
|---|---|
| **Controller** | `Api\Admin\CategoryController@store` |
| **Middleware** | `auth:sanctum`, `role:admin,super_admin` |
| **Authentication** | Required |
| **Authorization** | Admin or super_admin role |
| **Path params** | None |
| **Request body** | `{ name, slug, description, icon, is_active }` |
| **FormRequest** | `StoreCategoryRequest` |
| **Validation** | `name`: required, string, max:255, unique:service_categories<br>`slug`: required, string, max:255, unique:service_categories (auto-generated from name if missing)<br>`description`: nullable, string, max:1000<br>`icon`: nullable, string, max:100<br>`is_active`: sometimes, boolean |
| **Query params** | None |
| **Response** | `{ data: CategoryResource, message: "..." }` |
| **Success status** | 201 |
| **Error status** | 401 (unauthenticated), 403 (wrong role), 422 (validation) |
| **Business logic** | Creates new service category |
| **Services** | None |
| **Models** | `ServiceCategory::create($validated)` |
| **Relationships** | None |
| **File upload** | No |
| **Side effects** | Category creation |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### PUT /api/admin/categories/{category}

| Attribute | Value |
|---|---|
| **Controller** | `Api\Admin\CategoryController@update` |
| **Middleware** | `auth:sanctum`, `role:admin,super_admin` |
| **Authentication** | Required |
| **Authorization** | Admin or super_admin role |
| **Path params** | `category` (ServiceCategory model binding) |
| **Request body** | `{ name, slug, description, icon, is_active }` |
| **FormRequest** | `UpdateCategoryRequest` |
| **Validation** | Same as store, but unique rules ignore current category |
| **Query params** | None |
| **Response** | `{ data: CategoryResource, message: "..." }` |
| **Success status** | 200 |
| **Error status** | 401, 403, 404, 422 |
| **Business logic** | Updates category |
| **Services** | None |
| **Models** | `$category->update($validated)` |
| **Relationships** | None |
| **File upload** | No |
| **Side effects** | Category update |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### DELETE /api/admin/categories/{category}

| Attribute | Value |
|---|---|
| **Controller** | `Api\Admin\CategoryController@destroy` |
| **Middleware** | `auth:sanctum`, `role:admin,super_admin` |
| **Authentication** | Required |
| **Authorization** | Admin or super_admin role |
| **Path params** | `category` (ServiceCategory model binding) |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ message: "..." }` |
| **Success status** | 200 |
| **Error status** | 401, 403, 404, 409 (conflict — has services) |
| **Business logic** | Deletes category if it has no services |
| **Services** | None |
| **Models** | `$category->services()->exists()`, `$category->delete()` |
| **Relationships** | `services` |
| **File upload** | No |
| **Side effects** | Category deletion |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### POST /api/admin/services
### PUT /api/admin/services/{service}
### DELETE /api/admin/services/{service}
### POST /api/admin/packages
### PUT /api/admin/packages/{package}
### DELETE /api/admin/packages/{package}

Similar pattern to categories CRUD. Details available in controller analysis output.

---

## Domain: Booking (P3 — Customer Workflow)

### GET /api/bookings

| Attribute | Value |
|---|---|
| **Controller** | `Api\BookingController@index` |
| **Middleware** | `auth:sanctum`, `role:customer` |
| **Authentication** | Required |
| **Authorization** | Customer role |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | `page` |
| **Response** | `{ data: [BookingResource], meta: {...}, links: {...} }` |
| **Success status** | 200 |
| **Error status** | 401, 403 |
| **Business logic** | Returns authenticated customer's bookings with items and invoice |
| **Services** | None |
| **Models** | `$request->user()->bookings()->with(['items', 'invoice'])->latest()->paginate()` |
| **Relationships** | `items`, `invoice` |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### POST /api/bookings

| Attribute | Value |
|---|---|
| **Controller** | `Api\BookingController@store` |
| **Middleware** | `auth:sanctum`, `role:customer`, `throttle:booking-create` (10/min) |
| **Authentication** | Required |
| **Authorization** | Customer role + complete profile required |
| **Path params** | None |
| **Request body** | `{ item_type, service_id, package_id, quantity, booking_date, booking_time, address, address_detail, latitude, longitude, customer_note, additional_jobdesk }` |
| **FormRequest** | `StoreBookingRequest` |
| **Validation** | `item_type`: required, in:service,package<br>`service_id`: required_if:item_type,service, exists:services (active)<br>`package_id`: required_if:item_type,package, exists:packages (active)<br>`quantity`: required, integer, min:1, max:99<br>`booking_date`: required, date, after_or_equal:today<br>`booking_time`: required, in:[TIME_SLOTS]<br>`address`: required, string, max:255<br>`address_detail`: nullable, string, max:255<br>`latitude`: nullable, numeric, between:-90,90<br>`longitude`: nullable, numeric, between:-180,180<br>`customer_note`: nullable, string, max:2000<br>`additional_jobdesk`: nullable, string, max:2000 |
| **Query params** | None |
| **Response** | `{ data: BookingResource, message: "..." }` |
| **Success status** | 201 |
| **Error status** | 401, 403, 422 (validation or incomplete profile), 429 (rate limit) |
| **Business logic** | Creates booking with price snapshot from active catalog, creates booking item, generates invoice automatically, sends notification to admins |
| **Services** | `BookingPricingService::multiply()` |
| **Models** | `Service/Package::query()->whereKey()->where('is_active', true)->lockForUpdate()->firstOrFail()`, `Booking::create()`, `$booking->items()->create()`, `Invoice::create()` |
| **Relationships** | `items`, `invoice` |
| **File upload** | No |
| **Side effects** | Booking creation, BookingItem creation, Invoice creation, notification to admins |
| **Transaction** | Yes (DB::transaction) |
| **Queue** | No |
| **Notifications** | `booking_created` to administrators |

---

### GET /api/bookings/{booking}

| Attribute | Value |
|---|---|
| **Controller** | `Api\BookingController@show` |
| **Middleware** | `auth:sanctum`, `role:customer` |
| **Authentication** | Required |
| **Authorization** | Customer role + ownership (BookingPolicy@view) |
| **Path params** | `booking` (Booking model binding) |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ data: BookingResource }` |
| **Success status** | 200 |
| **Error status** | 401, 403 (not owner), 404 |
| **Business logic** | Returns single booking with items and invoice |
| **Services** | None |
| **Models** | `Gate::allows('view', $booking)`, `$booking->load(['items', 'invoice'])` |
| **Relationships** | `items`, `invoice` |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### POST /api/bookings/{booking}/cancel

| Attribute | Value |
|---|---|
| **Controller** | `Api\BookingController@cancel` |
| **Middleware** | `auth:sanctum`, `role:customer` |
| **Authentication** | Required |
| **Authorization** | Customer role + ownership (BookingPolicy@cancel) |
| **Path params** | `booking` (Booking model binding) |
| **Request body** | `{ reason }` (optional) |
| **FormRequest** | `CancelBookingRequest` |
| **Validation** | `reason`: sometimes, nullable, string, max:1000 |
| **Query params** | None |
| **Response** | `{ data: BookingResource, message: "..." }` |
| **Success status** | 200 |
| **Error status** | 401, 403 (not owner), 409 (invalid state transition), 422 |
| **Business logic** | Transitions booking to cancelled, cascades invoice cancellation |
| **Services** | None |
| **Models** | `Gate::allows('cancel', $booking)`, `$booking->transitionTo(STATUS_CANCELLED)`, `$booking->invoice()->update(['status' => Invoice::STATUS_CANCELLED])` |
| **Relationships** | `invoice` |
| **File upload** | No |
| **Side effects** | Booking status change, invoice cancellation |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | None |

---

## Domain: Payment (P4 — Customer Upload, Admin Verify)

### POST /api/invoices/{invoice}/payment-proof

| Attribute | Value |
|---|---|
| **Controller** | `Api\PaymentController@storeProof` |
| **Middleware** | `auth:sanctum`, `role:customer`, `throttle:payment-upload` (5/10min) |
| **Authentication** | Required |
| **Authorization** | Customer role + invoice ownership (InvoicePolicy@view) |
| **Path params** | `invoice` (Invoice model binding) |
| **Request body** | `{ payment_method, amount, customer_note, proof_image }` (multipart/form-data) |
| **FormRequest** | `UploadPaymentProofRequest` |
| **Validation** | `payment_method`: required, in:bank_transfer<br>`amount`: required, numeric, min:0.01<br>`customer_note`: nullable, string, max:1000<br>`proof_image`: required, file, mimes:jpg,jpeg,png, max:2048 (KB), mimetypes:image/jpeg,image/png |
| **Query params** | None |
| **Response** | `{ data: PaymentResource, message: "..." }` |
| **Success status** | 201 |
| **Error status** | 401, 403 (not owner), 409 (duplicate pending payment or invalid invoice status or amount mismatch), 422 (validation), 429 (rate limit) |
| **Business logic** | Uploads payment proof to private storage, creates Payment record, updates invoice status to pending_payment, transitions booking to waiting_verification, sends notification to admins. Rollback file on error. |
| **Services** | `NotificationService::send()` |
| **Models** | `Gate::allows('view', $invoice)`, `Invoice::query()->whereKey()->with('booking')->lockForUpdate()->firstOrFail()`, `$invoice->payments()->whereIn('status', Payment::pendingVerificationStatuses())->exists()`, `$invoice->payments()->create()`, `$invoice->update(['status' => Invoice::STATUS_PENDING_PAYMENT])`, `$booking->transitionTo(Booking::STATUS_WAITING_VERIFICATION)`, `Storage::disk('payment_proofs')->storeAs()` |
| **Relationships** | `booking`, `payments` |
| **File upload** | Yes — proof_image stored to `payment_proofs` disk (private) |
| **Side effects** | Payment record creation, file upload, invoice status update, booking status change, notification to admins |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | `payment_proof_submitted` to administrators |

---

### GET /api/invoices/{invoice}/payment-proof

| Attribute | Value |
|---|---|
| **Controller** | `Api\PaymentController@showProof` |
| **Middleware** | `auth:sanctum`, `role:customer` |
| **Authentication** | Required |
| **Authorization** | Customer role + invoice ownership (InvoicePolicy@view) |
| **Path params** | `invoice` (Invoice model binding) |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | File (image) |
| **Success status** | 200 |
| **Error status** | 401, 403 (not owner), 404 (no proof uploaded) |
| **Business logic** | Returns payment proof file from private storage |
| **Services** | None |
| **Models** | `Gate::allows('view', $invoice)`, `$invoice->load('booking')`, `$invoice->payments()->whereNotNull('proof_image')->latest('id')->first()` |
| **Relationships** | `booking`, `payments` |
| **File upload** | No (download) |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### GET /api/admin/payments

| Attribute | Value |
|---|---|
| **Controller** | `Api\Admin\PaymentController@index` |
| **Middleware** | `auth:sanctum`, `role:admin,super_admin` |
| **Authentication** | Required |
| **Authorization** | Admin or super_admin role |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | `status` (filter), `page` |
| **Response** | `{ data: [PaymentResource], meta: {...}, links: {...} }` |
| **Success status** | 200 |
| **Error status** | 401, 403 |
| **Business logic** | Returns all payments with invoice/booking/customer; filterable by status |
| **Services** | None |
| **Models** | `Payment::query()->with('invoice.booking.customer')->when($request->filled('status'), fn => $query->where('status', $request->input('status')))->latest('id')->paginate()` |
| **Relationships** | `invoice.booking.customer` |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### GET /api/admin/payments/{payment}/proof

| Attribute | Value |
|---|---|
| **Controller** | `Api\Admin\PaymentController@showProof` |
| **Middleware** | `auth:sanctum`, `role:admin,super_admin` |
| **Authentication** | Required |
| **Authorization** | Admin or super_admin role (PaymentPolicy@view) |
| **Path params** | `payment` (Payment model binding) |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | File (image) |
| **Success status** | 200 |
| **Error status** | 401, 403, 404 (no proof) |
| **Business logic** | Returns payment proof file from private storage |
| **Services** | None |
| **Models** | `Gate::allows('view', $payment)`, `$payment->load('invoice.booking')`, `Storage::disk('payment_proofs')->path($payment->proof_image)` |
| **Relationships** | `invoice.booking` |
| **File upload** | No (download) |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### POST /api/admin/payments/{payment}/verify

| Attribute | Value |
|---|---|
| **Controller** | `Api\Admin\PaymentController@verify` |
| **Middleware** | `auth:sanctum`, `role:admin,super_admin` |
| **Authentication** | Required |
| **Authorization** | Admin or super_admin role (PaymentPolicy@verify) |
| **Path params** | `payment` (Payment model binding) |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ data: PaymentResource, message: "..." }` |
| **Success status** | 200 |
| **Error status** | 401, 403, 422 (invalid status or no proof image) |
| **Business logic** | Verifies payment proof exists in storage, updates payment to paid, updates invoice to paid, transitions booking: pending_payment → paid → confirmed. Sends notification to customer. Rollback on error. |
| **Services** | `NotificationService::send()` |
| **Models** | `Gate::allows('verify', $payment)`, `Payment::query()->whereKey()->with('invoice.booking')->lockForUpdate()->firstOrFail()`, `Storage::disk('payment_proofs')->exists($locked->proof_image)`, `$locked->update(['status' => Payment::STATUS_PAID, 'verified_by' => auth()->id(), 'verified_at' => now()])`, `$locked->invoice->update(['status' => Invoice::STATUS_PAID])`, `$booking->transitionTo(STATUS_PAID)`, `$booking->transitionTo(STATUS_CONFIRMED)` |
| **Relationships** | `invoice.booking` |
| **File upload** | No |
| **Side effects** | Payment verified, invoice paid, booking status changes (paid → confirmed), notification to customer |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | `payment_verified` to customer |

---

### POST /api/admin/payments/{payment}/reject

| Attribute | Value |
|---|---|
| **Controller** | `Api\Admin\PaymentController@reject` |
| **Middleware** | `auth:sanctum`, `role:admin,super_admin` |
| **Authentication** | Required |
| **Authorization** | Admin or super_admin role (PaymentPolicy@verify) |
| **Path params** | `payment` (Payment model binding) |
| **Request body** | `{ admin_note }` |
| **FormRequest** | `RejectPaymentRequest` |
| **Validation** | `admin_note`: required, string, max:1000 |
| **Query params** | None |
| **Response** | `{ data: PaymentResource, message: "..." }` |
| **Success status** | 200 |
| **Error status** | 401, 403, 422 (invalid status or validation) |
| **Business logic** | Rejects payment, updates payment to rejected with admin note, updates invoice to unpaid, transitions booking back to pending_payment. Sends notification to customer. Rollback on error. |
| **Services** | `NotificationService::send()` |
| **Models** | `Gate::allows('verify', $payment)`, `Payment::query()->whereKey()->with('invoice.booking')->lockForUpdate()->firstOrFail()`, `$locked->update(['status' => Payment::STATUS_REJECTED, 'admin_note' => ...])`, `$locked->invoice->update(['status' => Invoice::STATUS_UNPAID])`, `$locked->invoice->booking->transitionTo(Booking::STATUS_PENDING_PAYMENT)` |
| **Relationships** | `invoice.booking` |
| **File upload** | No |
| **Side effects** | Payment rejected, invoice unpaid, booking status reverted, notification to customer |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | `payment_rejected` to customer |

---

## Domain: Technician (P5 — Profile + Job Workflow)

### GET /api/technician/profile
### PUT /api/technician/profile

Similar pattern to customer profile — technician manages own TechnicianProfile.

---

### GET /api/technician/jobs

| Attribute | Value |
|---|---|
| **Controller** | `Api\TechnicianJobController@index` |
| **Middleware** | `auth:sanctum`, `role:technician` |
| **Authentication** | Required |
| **Authorization** | Technician role |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | `page` |
| **Response** | `{ data: [BookingAssignmentResource], meta: {...}, links: {...} }` |
| **Success status** | 200 |
| **Error status** | 401, 403 |
| **Business logic** | Returns authenticated technician's assignments with booking/items/customer/invoice |
| **Services** | None |
| **Models** | `BookingAssignment::query()->where('technician_id', $request->user()->id)->with(['booking.items', 'booking.customer', 'booking.invoice'])->latest('id')->paginate()` |
| **Relationships** | `booking.items`, `booking.customer`, `booking.invoice` |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### POST /api/technician/jobs/{assignment}/accept

| Attribute | Value |
|---|---|
| **Controller** | `Api\TechnicianJobController@accept` |
| **Middleware** | `auth:sanctum`, `role:technician` |
| **Authentication** | Required |
| **Authorization** | Technician role + ownership (BookingAssignmentPolicy@act) |
| **Path params** | `assignment` (BookingAssignment model binding) |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ data: BookingAssignmentResource, message: "..." }` |
| **Success status** | 200 |
| **Error status** | 401, 403 (not owner), 422 (invalid status) |
| **Business logic** | Accepts pending assignment, sets accepted_at timestamp, sends notification to admins |
| **Services** | `NotificationService::send()` |
| **Models** | `Gate::allows('act', $assignment)`, `BookingAssignment::query()->whereKey()->with('booking.invoice')->lockForUpdate()->firstOrFail()`, `$locked->update(['status' => BookingAssignment::STATUS_ACCEPTED, 'accepted_at' => now()])` |
| **Relationships** | `booking.invoice` |
| **File upload** | No |
| **Side effects** | Assignment accepted, notification to admins |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | `assignment_accepted` to administrators |

---

### POST /api/technician/jobs/{assignment}/reject

| Attribute | Value |
|---|---|
| **Controller** | `Api\TechnicianJobController@reject` |
| **Middleware** | `auth:sanctum`, `role:technician` |
| **Authentication** | Required |
| **Authorization** | Technician role + ownership (BookingAssignmentPolicy@act) |
| **Path params** | `assignment` (BookingAssignment model binding) |
| **Request body** | `{ rejection_reason, rejection_reason_detail }` |
| **FormRequest** | `RejectAssignmentRequest` |
| **Validation** | `rejection_reason`: required, string, in:[...predefined reasons]<br>`rejection_reason_detail`: nullable, string, max:500 |
| **Query params** | None |
| **Response** | `{ data: BookingAssignmentResource, message: "..." }` |
| **Success status** | 200 |
| **Error status** | 401, 403 (not owner), 422 (invalid status or validation) |
| **Business logic** | Rejects pending assignment, reverts booking to confirmed status, sends notification to admins |
| **Services** | `NotificationService::send()` |
| **Models** | `Gate::allows('act', $assignment)`, `BookingAssignment::query()->whereKey()->with('booking.invoice')->lockForUpdate()->firstOrFail()`, `$locked->update(['status' => BookingAssignment::STATUS_REJECTED, 'rejected_at' => now(), 'rejection_reason' => ..., 'rejection_reason_detail' => ...])`, `$locked->booking->transitionTo(Booking::STATUS_CONFIRMED)` |
| **Relationships** | `booking.invoice` |
| **File upload** | No |
| **Side effects** | Assignment rejected, booking status reverted, notification to admins |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | `assignment_rejected` to administrators |

---

### POST /api/technician/jobs/{assignment}/start

| Attribute | Value |
|---|---|
| **Controller** | `Api\TechnicianJobController@start` |
| **Middleware** | `auth:sanctum`, `role:technician` |
| **Authentication** | Required |
| **Authorization** | Technician role + ownership (BookingAssignmentPolicy@act) |
| **Path params** | `assignment` (BookingAssignment model binding) |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ data: BookingAssignmentResource, message: "..." }` |
| **Success status** | 200 |
| **Error status** | 401, 403, 422 (invalid status) |
| **Business logic** | Starts accepted assignment, sets started_at timestamp, transitions booking to in_progress, sends notification to customer |
| **Services** | `NotificationService::send()` |
| **Models** | `Gate::allows('act', $assignment)`, `BookingAssignment::query()->whereKey()->with('booking.invoice')->lockForUpdate()->firstOrFail()`, `$locked->update(['started_at' => now(), 'technician_note' => 'Job started.'])`, `$locked->booking->transitionTo(Booking::STATUS_IN_PROGRESS)` |
| **Relationships** | `booking.invoice` |
| **File upload** | No |
| **Side effects** | Job started, booking status change, notification to customer |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | `job_started` to customer |

---

### POST /api/technician/jobs/{assignment}/complete

| Attribute | Value |
|---|---|
| **Controller** | `Api\TechnicianJobController@complete` |
| **Middleware** | `auth:sanctum`, `role:technician` |
| **Authentication** | Required |
| **Authorization** | Technician role + ownership (BookingAssignmentPolicy@act) |
| **Path params** | `assignment` (BookingAssignment model binding) |
| **Request body** | `{ technician_note }` |
| **FormRequest** | `CompleteJobRequest` |
| **Validation** | `technician_note`: required, string, max:2000 |
| **Query params** | None |
| **Response** | `{ data: BookingAssignmentResource, message: "..." }` |
| **Success status** | 200 |
| **Error status** | 401, 403, 422 (invalid status or validation) |
| **Business logic** | Marks assignment as completed, sets completed_at timestamp, transitions booking to awaiting_verification, sends notification to admins and customer |
| **Services** | `NotificationService::send()` (2 calls) |
| **Models** | `Gate::allows('act', $assignment)`, `BookingAssignment::query()->whereKey()->with('booking.invoice')->lockForUpdate()->firstOrFail()`, `$locked->update(['status' => BookingAssignment::STATUS_COMPLETED, 'completed_at' => now(), 'technician_note' => ...])`, `$locked->booking->transitionTo(Booking::STATUS_AWAITING_VERIFICATION)` |
| **Relationships** | `booking.invoice` |
| **File upload** | No |
| **Side effects** | Job completed, booking awaiting verification, notifications to admins and customer |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | `job_completed` to administrators and customer |

---

## Domain: Admin — Technician Management (P5)

### GET /api/admin/technicians
### POST /api/admin/technicians
### PUT /api/admin/technicians/{technician}
### PATCH /api/admin/technicians/{technician}/toggle

Admin CRUD for technician users. Details available in controller analysis.

---

### POST /api/admin/bookings/{booking}/assign

| Attribute | Value |
|---|---|
| **Controller** | `Api\Admin\AssignmentController@assign` |
| **Middleware** | `auth:sanctum`, `role:admin,super_admin` |
| **Authentication** | Required |
| **Authorization** | Admin or super_admin role |
| **Path params** | `booking` (Booking model binding) |
| **Request body** | `{ technician_id }` |
| **FormRequest** | `AssignTechnicianRequest` |
| **Validation** | `technician_id`: required, integer, exists:users |
| **Query params** | None |
| **Response** | `{ data: BookingAssignmentResource, message: "..." }` |
| **Success status** | 201 |
| **Error status** | 401, 403, 422 (validation or invalid booking status or inactive technician) |
| **Business logic** | Assigns active technician to paid/confirmed booking; auto-rejects previous active assignment if exists; creates new assignment; transitions booking to technician_assigned; sends notification to technician |
| **Services** | `NotificationService::send()` |
| **Models** | `Booking::query()->whereKey()->with('invoice')->lockForUpdate()->firstOrFail()`, `User::query()->whereKey($technician_id)->with('technicianProfile')->lockForUpdate()->first()`, `$lockedBooking->assignments()->whereIn('status', [pending, accepted])->latest('id')->first()`, `$activeAssignment->update(['status' => rejected, 'rejected_at' => now(), 'rejection_reason' => ...])`, `$lockedBooking->assignments()->create([...])`, `$lockedBooking->transitionTo(Booking::STATUS_TECHNICIAN_ASSIGNED)` |
| **Relationships** | `invoice`, `assignments`, `technicianProfile` |
| **File upload** | No |
| **Side effects** | Technician assignment, previous assignment auto-rejected, booking status change, notification to technician |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | `assignment_created` to technician |

---

## Domain: Admin — Booking Management (P3)

### GET /api/admin/bookings

| Attribute | Value |
|---|---|
| **Controller** | `Api\Admin\BookingController@index` |
| **Middleware** | `auth:sanctum`, `role:admin,super_admin` |
| **Authentication** | Required |
| **Authorization** | Admin or super_admin role |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | `status`, `date_from`, `date_to`, `customer_id`, `technician_id`, `page` |
| **Response** | `{ data: [BookingResource], meta: {...}, links: {...} }` |
| **Success status** | 200 |
| **Error status** | 401, 403 |
| **Business logic** | Returns all bookings with customer/items/invoice/assignments; filterable by status, date range, customer, technician |
| **Services** | None |
| **Models** | `Booking::query()->with(['customer', 'items', 'invoice', 'assignments.technician'])->when($request->filled('status'), ...)...->latest('id')->paginate()` |
| **Relationships** | `customer`, `items`, `invoice`, `assignments.technician` |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### POST /api/admin/bookings/{booking}/verify

| Attribute | Value |
|---|---|
| **Controller** | `Api\Admin\BookingController@verify` |
| **Middleware** | `auth:sanctum`, `role:admin,super_admin` |
| **Authentication** | Required |
| **Authorization** | Admin or super_admin role |
| **Path params** | `booking` (Booking model binding) |
| **Request body** | `{ action, admin_verification_note }` |
| **FormRequest** | `VerifyCompletionRequest` |
| **Validation** | `action`: required, string, in:approve,reject<br>`admin_verification_note`: nullable, string, max:1000 (required if action=reject) |
| **Query params** | None |
| **Response** | `{ data: BookingResource, message: "..." }` |
| **Success status** | 200 |
| **Error status** | 401, 403, 422 (validation or invalid booking status or no completed assignment) |
| **Business logic** | Admin verifies/rejects technician's job completion. If approve: booking → completed, assignment note updated, sends notification to customer, sends review reminder if no review exists. If reject: booking → in_progress, assignment note updated, sends notification to customer. Rollback on error. |
| **Services** | `NotificationService::send()` (1-2 calls) |
| **Models** | `Booking::query()->whereKey()->lockForUpdate()->firstOrFail()`, `$lockedBooking->assignments()->where('status', BookingAssignment::STATUS_COMPLETED)->latest('id')->first()`, `BookingAssignment::query()->whereKey($assignment->id)->lockForUpdate()->firstOrFail()`, `$lockedBooking->transitionTo(Booking::STATUS_COMPLETED or STATUS_IN_PROGRESS)`, `$lockedAssignment->update(['admin_verification_note' => ...])`, `$booking->review()->exists()`, `$booking->assignedTechnician()` |
| **Relationships** | `assignments`, `review` |
| **File upload** | No |
| **Side effects** | Booking completion verification or rejection, assignment note updated, notification to customer, review reminder if approved and no review exists |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | `job_completed_verified` or `job_completion_rejected` to customer; `review_reminder` to customer if approved and no review |

---

## Domain: Notification (P6)

### GET /api/notifications
### POST /api/notifications/{notification}/read
### POST /api/notifications/read-all

Standard notification CRUD — user sees own notifications only. Details in controller analysis.

---

## Domain: Review (P7)

### GET /api/bookings/{booking}/review

| Attribute | Value |
|---|---|
| **Controller** | `Api\ReviewController@show` |
| **Middleware** | `auth:sanctum`, `role:customer` |
| **Authentication** | Required |
| **Authorization** | Customer role + booking ownership |
| **Path params** | `booking` (Booking model binding) |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ data: ReviewResource }` or 404 |
| **Success status** | 200 |
| **Error status** | 401, 403 (not owner), 404 (no review) |
| **Business logic** | Returns review for completed booking |
| **Services** | None |
| **Models** | Manual ownership check `$booking->customer_id !== request()->user()->id`, `$booking->review()->with(['customer', 'technician'])->first()` |
| **Relationships** | `review`, `customer`, `technician` |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### POST /api/bookings/{booking}/review

| Attribute | Value |
|---|---|
| **Controller** | `Api\ReviewController@store` |
| **Middleware** | `auth:sanctum`, `role:customer` |
| **Authentication** | Required |
| **Authorization** | Customer role + booking ownership + ReviewPolicy@create (booking completed, has technician, no existing review) |
| **Path params** | `booking` (Booking model binding) |
| **Request body** | `{ rating, comment }` |
| **FormRequest** | `StoreReviewRequest` |
| **Validation** | `rating`: required, integer, in:1,2,3,4,5<br>`comment`: nullable, string, max:1000 |
| **Query params** | None |
| **Response** | `{ data: ReviewResource, message: "..." }` |
| **Success status** | 201 |
| **Error status** | 401, 403 (not owner or policy), 409 (duplicate review), 422 (validation) |
| **Business logic** | Creates review for completed booking with assigned technician; sends notification to technician |
| **Services** | `NotificationService::send()` |
| **Models** | Manual ownership check, `$booking->status !== Booking::STATUS_COMPLETED`, `$booking->assignedTechnician()`, `Booking::query()->whereKey()->with('review')->lockForUpdate()->firstOrFail()`, `$lockedBooking->review()->exists()`, `$lockedBooking->review()->create([...])` |
| **Relationships** | `review` |
| **File upload** | No |
| **Side effects** | Review creation, notification to technician |
| **Transaction** | Yes |
| **Queue** | No |
| **Notifications** | `review_submitted` to technician |

---

### GET /api/admin/reviews
### POST /api/admin/reviews/{review}/moderate

Admin can list all reviews and moderate (publish/hidden/rejected). Details in controller analysis.

---

## Domain: Dashboard/Reports (P8)

### GET /api/dashboard

| Attribute | Value |
|---|---|
| **Controller** | `Api\DashboardController@customer` |
| **Middleware** | `auth:sanctum`, `role:customer` |
| **Authentication** | Required |
| **Authorization** | Customer role |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ data: CustomerDashboardResource }` |
| **Success status** | 200 |
| **Error status** | 401, 403 |
| **Business logic** | Aggregates customer metrics (booking counts by status, invoice/payment stats, latest bookings) |
| **Services** | `DashboardService::customer()` |
| **Models** | (inside service) queries on Booking, Invoice, Payment filtered by customer_id |
| **Relationships** | Various (eager loaded in service) |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### GET /api/admin/dashboard

| Attribute | Value |
|---|---|
| **Controller** | `Api\Admin\DashboardController@__invoke` |
| **Middleware** | `auth:sanctum`, `role:admin,super_admin` |
| **Authentication** | Required |
| **Authorization** | Admin or super_admin role |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ data: AdminDashboardResource }` |
| **Success status** | 200 |
| **Error status** | 401, 403 |
| **Business logic** | Aggregates global metrics (bookings/invoices/payments by status, revenue from latest paid payments per invoice, technician counts, review averages) |
| **Services** | `DashboardService::admin()` |
| **Models** | (inside service) global queries on Booking, Invoice, Payment, User (technician), Review |
| **Relationships** | Various (eager loaded in service) |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### GET /api/admin/reports/overview

| Attribute | Value |
|---|---|
| **Controller** | `Api\Admin\ReportController@overview` |
| **Middleware** | `auth:sanctum`, `role:admin,super_admin` |
| **Authentication** | Required |
| **Authorization** | Admin or super_admin role |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ data: ReportResource }` |
| **Success status** | 200 |
| **Error status** | 401, 403 |
| **Business logic** | Aggregates booking states, payment states, revenue (from latest paid payment per invoice, no double counting) |
| **Services** | `DashboardService::reportOverview()` |
| **Models** | (inside service) Booking, Payment queries |
| **Relationships** | Various (eager loaded in service) |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

## Public Utility Endpoints

### GET /api/health

| Attribute | Value |
|---|---|
| **Controller** | Closure (routes/api.php line 60) |
| **Middleware** | None |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | `{ status: "ok" }` |
| **Success status** | 200 |
| **Error status** | None |
| **Business logic** | Health check endpoint |
| **Services** | None |
| **Models** | None |
| **Relationships** | None |
| **File upload** | No |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

### GET /api/openapi.yaml

| Attribute | Value |
|---|---|
| **Controller** | Closure (routes/api.php line 63) |
| **Middleware** | None |
| **Authentication** | Public |
| **Authorization** | None |
| **Path params** | None |
| **Request body** | None |
| **FormRequest** | None |
| **Validation** | None |
| **Query params** | None |
| **Response** | OpenAPI YAML file (Content-Type: application/yaml) |
| **Success status** | 200 |
| **Error status** | 404 (file not found) |
| **Business logic** | Serves OpenAPI specification from `docs/openapi.yaml` |
| **Services** | None |
| **Models** | None |
| **Relationships** | None |
| **File upload** | No (download) |
| **Side effects** | None |
| **Transaction** | No |
| **Queue** | No |
| **Notifications** | None |

---

## Invoice Endpoints

### GET /api/invoices
### GET /api/invoices/{invoice}

Customer can list and view own invoices. Details in controller analysis.

---

# API Coverage

| Metric | Count |
|---|---|
| **Total endpoints** | **63** |
| **GET** | 22 |
| **POST** | 31 |
| **PUT** | 7 |
| **PATCH** | 1 |
| **DELETE** | 2 |
| **Public endpoints** | 11 (register, login, catalog, health, openapi, email verification, password reset) |
| **Authenticated endpoints** | 52 (requires `auth:sanctum`) |
| **Admin endpoints** | 24 (require `role:admin,super_admin`) |
| **Customer endpoints** | 14 (require `role:customer`) |
| **Technician endpoints** | 8 (require `role:technician`) |
| **File upload endpoints** | 1 (POST /api/invoices/{invoice}/payment-proof) |
| **File download endpoints** | 3 (payment proof views + openapi.yaml) |
| **Endpoints with FormRequest validation** | 28 |
| **Endpoints with transaction** | 15 |
| **Endpoints with side effects** | 30+ (state changes, cascades) |
| **Endpoints with queue** | 4 (OTP emails) |
| **Endpoints with notifications** | 13 (booking, payment, technician, review workflows) |

---

# API Discrepancies (OpenAPI vs Laravel Implementation)

| Issue | OpenAPI | Laravel Implementation | Source of Truth |
|---|---|---|---|
| **Path count** | 55 paths defined | 63 routes defined | **Laravel routes/api.php** — OpenAPI incomplete or outdated |
| **(Not yet cross-verified in detail)** | Requires manual path-by-path comparison | — | Defer to FASE 1 completion task |

**Recommendation:** OpenAPI spec should be regenerated or manually updated to match current Laravel routes. Several endpoints may be missing from OpenAPI or have outdated schemas.

---

# Migration Priority

Based on **dependency analysis** from Laravel implementation:

| Priority | Domain | Rationale |
|---|---|---|
| **P0** | Authentication | Foundation — register, login, logout, email verification, password reset. All other domains require auth. |
| **P1** | User/Profile + Customer Profile | Core user data management. Required before booking (booking requires complete profile). |
| **P2** | Catalog (public + admin CRUD) | Service/package data required for booking creation (price snapshot). |
| **P3** | Booking + Invoice | Core business workflow. Auto-creates invoice. Dependencies: P0 (auth), P1 (customer profile complete), P2 (catalog active). |
| **P4** | Payment | Depends on invoice (P3). Booking state transitions depend on payment verification. |
| **P5** | Technician (profile + job workflow) + Admin Technician Mgmt + Admin Assignment | Job assignment depends on booking being paid/confirmed (P3→P4). Technician workflow (accept/reject/start/complete) drives booking state to completion. |
| **P6** | Notification | Cross-cutting — triggered by P3, P4, P5 workflows. Can be implemented in parallel after P3. |
| **P7** | Review | Depends on completed booking (P3→P4→P5). Can only review after technician completes job and admin verifies. |
| **P8** | Dashboard/Reports | Aggregation layer — depends on all domain data. Implement last. |

**Dependency chain:**
```
P0 (Auth) → P1 (Profile) → P2 (Catalog) → P3 (Booking) → P4 (Payment) → P5 (Technician) → P7 (Review)
                                                ↓
                                           P6 (Notification) — parallel after P3
                                                ↓
                                           P8 (Dashboard) — last
```

---

# Notes

1. **Route closures**: `/api/health` and `/api/openapi.yaml` use closures (cannot use `route:cache`). This is documented in FASE 0 as a minor issue but does not block Go migration.

2. **File upload**: Only one endpoint uploads files (`POST /api/invoices/{invoice}/payment-proof`). Storage disk is `payment_proofs` (private). Go implementation must replicate private storage pattern.

3. **Transactions**: 15 endpoints use `DB::transaction`. Go implementation must replicate transactional behavior (rollback on error).

4. **State machines**: `Booking` status has strict transition rules enforced by `Booking::transitionTo()`. Go implementation must replicate this validation.

5. **Notifications**: 13 endpoints trigger database notifications via `NotificationService`. Go implementation must replicate notification dispatch.

6. **OTP system**: Email verification and password reset use OTP with attempts tracking, expiration, and single-use enforcement. Go implementation must replicate this security model.

7. **Policies**: 5 policy classes enforce ownership/authorization beyond role checks. Go implementation must replicate policy logic.

8. **Rate limiting**: 8 custom rate limiters defined in `AppServiceProvider`. Go implementation must replicate rate limits.

9. **Pagination**: Most list endpoints use Laravel pagination (meta, links). Go implementation should replicate pagination response format for API compatibility.

10. **Resource classes**: 21 API Resource classes serialize responses. Go implementation must replicate response JSON structure exactly for API compatibility.

---

**END OF API INVENTORY**
