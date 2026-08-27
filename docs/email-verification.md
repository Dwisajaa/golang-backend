# Email Verification (OTP)

Phase 7B-3 deliverable. Email verification via one-time passcode, preserving
Laravel behavior and reusing FASE 7B-1/7B-2 primitives (bcrypt hasher,
Sanctum-equivalent token, auth middleware).

## OTP Lifecycle

```
register(user unverified)
  → resend/register → generate 6-digit OTP → store bcrypt hash, expires +10m
  → user submits OTP → validate + single-use (tx, FOR UPDATE)
  → email_verified_at = now(UTC) → token minted → login now allowed
wrong code → attempts++ ; attempts >= 5 → 429
expired / already used → 422
resend → old active OTPs invalidated (used_at), new OTP issued
```

## Laravel Behavior Source

Audited from `C:\V1\api-dwidev`:

- `app/Models/User.php::sendOtp()` — deletes expired unused OTPs; marks
  previous active OTPs of the type as used; OTP = `random_int(100000,999999)`;
  expiry = now + `config('auth.otp_expiration', 10)` minutes; stored as
  `Hash::make(otp)` (bcrypt, NOT plaintext); `Mail::to(...)->queue()` (async).
- `app/Http/Controllers/Api/EmailVerificationController.php` — messages/status
  below.
- `VerifyEmailRequest` / `ResendVerificationRequest` validation rules.
- `EmailVerificationOtp::MAX_ATTEMPTS = 5`, `config('auth.otp_max_attempts')`.
- Rate limiters: `otp-verify` 10/min, `otp-resend` 3/10min (ThrottleRequests).

### Endpoints

`POST /api/email/verification/verify`, `POST /api/email/verification/resend`
— both public (identify by request email, like Laravel; no auth middleware).

Responses (verified against controller):

| Case | Status | Body |
|---|---|---|
| Verify success | 200 | `{"message":"Email berhasil diverifikasi.","user":{...},"token":"<raw>","token_type":"Bearer"}` |
| Verify unknown user | 422 | `{"message":"Kode verifikasi tidak valid atau tidak ditemukan."}` |
| Verify already verified | 400 | `{"message":"Email sudah diverifikasi."}` |
| Verify no active/expired OTP | 422 | `{"message":"Kode verifikasi tidak valid atau sudah kedaluwarsa."}` |
| Verify wrong OTP | 422 | `{"message":"Kode verifikasi tidak valid."}` |
| Verify attempts exceeded | 429 | `{"message":"Terlalu banyak percobaan. Silakan kirim ulang OTP."}` |
| Resend (any: unknown/already-verified/sent) | 200 | `{"message":"Jika email terdaftar dan belum diverifikasi, kode telah dikirim ulang."}` |

### Validation (verify)

email required + format; otp required + exactly 6 digits → 422
`{"message":"The given data was invalid.","errors":{...}}`. Resend: email only.

## Generation

`internal/auth/otp.go` — `OtpGenerator`: `crypto/rand.Int` in `100000..999999`
(matches Laravel `random_int`; leading zeros impossible by construction).

## Storage

Column `otp_hash` holds **bcrypt**(cost 12) of the code — never the plaintext
(critical parity; chosen security model identical to Laravel).

## Expiration / Attempts

- `expires_at <= now(UTC)` → invalid; comparison in UTC only.
- `attempts`: incremented **inside the same transaction** as the failed check,
  committed atomically — wrong try #N is never lost.
- `attempts >= OTP_MAX_ATTEMPTS(5)` → 429 before comparing.

## Single-Use (atomic)

`VerifyEmail` runs in a `db.TxManager.Within` transaction:

```
BEGIN
SELECT ... WHERE user_id=? AND type='email_verification'
         AND used_at IS NULL AND expires_at > ? ORDER BY id DESC LIMIT 1 FOR UPDATE
  → attempts>=5 → 429 | hash mismatch → attempts++ (commit) → 422
  → correct → used_at=now ; users.email_verified_at=now
COMMIT
```

`FOR UPDATE` + the `used_at IS NULL` predicate make concurrent verify requests
serialize: only the first can consume the OTP (race-tested; repository
integration test proves the same against real MySQL).

## Resend

`sendOtp` parity: in one transaction DELETE expired-unused OTPs, mark other
active OTPs of the type as used, INSERT the new hash. Unknown/verified users
still return the generic 200 (anti user-enumeration).

## Email Sender

`internal/mailer`: `Mailer.Send(ctx, Message)` interface. `SMTPMailer`
(net/smtp, STARTTLS, credentials from config) and `LogMailer` (Laravel
`MAIL_MAILER=log` parity when SMTP is unconfigured). Handlers never touch
email; services only see the interface.

Body builder `VerificationEmail(name, otp, expiresAt)` → subject
"Kode Verifikasi Email - Dwidev", expiry formatted Asia/Jakarta (WIB,
`d F Y, H:i`) like Laravel `->format('d F Y, H:i')`.

## Queue / Worker (in-process)

`internal/mailer/worker.go` — bounded channel (64) + one consuming goroutine;
`Send` enqueues and returns immediately (async, matches `Mail::queue()`),
retry ×3 with backoff, then drop+log. **Never** log body (may contain OTP).
Graceful shutdown: close channel, drain backlog, then return; wired in main.

## Transaction Boundaries

- Email is enqueued **after** commit — a DB transaction never holds during
  network I/O.
- Verify: OTP state + user verification are one atomic tx; token mint happens
  after commit (matches Laravel's `DB::transaction` then `createToken`).

## Row Locking

`FindActiveForUpdate` appends `FOR UPDATE`. Lock order is consistent (single
row). Explained in docs/auth-foundation context notes: without the lock, two
verify requests could both pass the `used_at IS NULL` read.

## Error Handling

- Structured validation → 422 shape.
- Custom-message 422 (wrong/expired/unknown) → `Unprocessable`.
- 429 attempts, 400 already-verified, 500 generic for DB/SMTP internals.
- Server logs carry request_id + category; never OTP, hash, or token.

## Security

- OTP 6-digit from crypto/rand; stored bcrypt; single-use; expiry; attempt cap.
- No plaintext/password/token in logs or responses.
- Login stays 403 for unverified; 200+token only after verification.
- Anti user-enumeration on resend and on unknown-email verify (422).

## Laravel → Go Parity Table

| Laravel behavior | Go behavior | Status |
|---|---|---|
| OTP length/format | 6-digit `random_int(100000,999999)`, crypto/rand | MATCH |
| OTP storage | bcrypt hash (cost 12) | MATCH |
| Expiry | now+10m UTC | MATCH |
| Attempts limit | 5, incremented on wrong | MATCH |
| Single-use | used_at + tx + FOR UPDATE | MATCH (Go adds atomicity; Laravel is read-then-write) |
| Resend invalidates old OTP | prune + mark-used + new | MATCH |
| email_verified_at | set to UTC now | MATCH |
| Endpoints/method | POST verify + resend, public | MATCH |
| Validation | required/email/digits:6 → 422 | MATCH (regex approx.) |
| Response status/body | 200/400/422/429 with exact messages | MATCH |
| Queue | in-process worker (channel) | MATCH (behavior); infra differs |
| Mailer behavior | SMTP real + log fallback | APPROXIMATION (template body text, not byte-identical) |
| Rate limit (otp-verify 10/m, otp-resend 3/10m) | not implemented yet | DEFERRED (security hardening phase) |
| Transaction | service-owned tx for verify+resend | MATCH |

## Testing

| Level | File | Notes |
|---|---|---|
| Generator | `internal/auth/otp_test.go` | numeric, 6-digit, range 100000–999999 |
| Service | `internal/service/otp_test.go` | create+queued, silent unknown/verified resend, success→verified+token, wrong→attempts++ & verified untouched, expired, used-once, attempts-exceeded 429, already-verified 400, unknown-user 422, concurrent single-use |
| Handler | `internal/httphandler/otp_test.go` | 200 verify w/ token, 422 validation shape, 400 malformed, custom 422, resend generic 200, resend errors |
| Repository | `internal/repository/otp_mysql_test.go` | gated by `TEST_DATABASE_URL`: create → find FOR UPDATE in tx → mark used + verify user → gone after use; attempts increment; prune |
| Worker | `internal/mailer/worker_test.go` | delivery, retry-then-drop |
| Login integration | service/handler suites | unverified → 403 (7B-1 preserved); verified → 200 (via VerifyEmail then login path) |

Run: `go test ./...`, `go test -race ./...`, optional
`TEST_DATABASE_URL=... go test ./internal/repository -v`.

## Known Limitations

- Verification body/template is a plain-text approximation of Laravel's Blade
  email (subject matches exactly).
- Rate limits (10/min verify, 3/10min resend) deferred to the security hardening
  phase (documented; Laravel enforces them).
- `password_reset` OTP type supported by schema/constants but the password-reset
  domain is out of scope (deferred).

## Fundamental Go Concepts (why each choice)

1. **Interface `Mailer`** — the seam around email; services call `Send`, they
   never know SMTP.
2. **Service takes Mailer interface** — DI: prod passes a Worker, tests pass a
   recording fake.
3. **Dependency Injection** — constructors receive collaborators; nothing global.
4. **crypto/rand** — OS-backed entropy for secrets.
5. **math/rand not for OTP** — predictable/seedable; a guessable 6-digit code
   is trivially brute-forced within the attempt budget.
6. **Context propagation** — request ctx flows into tx/queries; cancellation
   aborts DB work.
7. **Transaction** — unit of atomic multi-write ($opt state + user verified).
8. **BeginTx** — starts a real MySQL transaction on the same connection.
9. **defer tx.Rollback()** — safe cleanup if fn fails or panics; commit later
   makes the rollback a no-op.
10. **Commit** — durable state point.
11. **SELECT ... FOR UPDATE** — acquires a row lock so concurrent readers of the
    same row serialize; second verifier sees `used_at IS NULL` false.
12. **Race condition** — two goroutines reading the same row before either
    writes; prevented here by row locking.
13. **Atomicity** — all-or-nothing: OTP used and email verified together.
14. **Idempotency** — a successful verify that the client retries yields
    "already verified" (400), not a double token.
15. **Worker goroutine** — one thread draining the email channel concurrently
    with the request handlers.
16. **channel** — safe hand-off of Messages between request goroutines and the
    worker.
17. **Graceful shutdown** — close channel → finish backlog → return, so emails
    aren't silently lost on SIGTERM.
18. **pointer vs value** — `*time.Time` / `*User` used where nullability or
    mutation across layers (EmailVerifiedAt) is needed.
19. **interface vs concrete** — the service depends on what it needs
    (`otpStore`, `mailer.Mailer`, `txRunner`), never on MySQL/gin.
20. **Why no SMTP in handler** — transport is infra; handlers own HTTP only.
21. **Why no SMTP inside a DB tx** — a transaction holding connections during
    network I/O blocks the pool and risks long locks.
22. **OTP single-use** — a used code is worthless; replay must fail.
23. **UTC comparisons** — one timezone source; `expires_at` from MySQL is
    compared against `time.Now().UTC()`, immune to Windows-local drift.
24. **authentication vs email verification** — auth proves "who" (token);
    verification proves "email ownership" (OTP); both gates are required before
    login (403 until verified).
25. **Request flow** — POST → router → handler (decode/validate) → OtpService
    (rules + tx + FOR UPDATE) → repository (SQL) → MySQL → commit → token mint
    → response; email goes service → worker channel → mailer concurrently.