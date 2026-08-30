# Security

Phase 17 deliverable. Application-level security hardening. Status legend:
IMPLEMENTED · DEFERRED (production/deployment phase) · ENVIRONMENTAL.

## 1. Authentication — IMPLEMENTED
Bearer via `middleware.Auth`: `Authorization: Bearer <token>` → SHA-256 →
`personal_access_tokens` row → expiry (expires_at, NULL never-expires) →
user (deleted/inactive treated as unauthenticated). All failures → generic
401 `{"message":"Unauthenticated."}` — no distinction between missing /
invalid / expired / revoked tokens (Laravel parity).

## 2. Authorization — IMPLEMENTED
`RequireRole` (customer | technician | admin,super_admin) AFTER auth; resource
ownership/policy in services (403). Identity strictly from context; no
client-controlled identity fields anywhere.

## 3. Ownership / IDOR — IMPLEMENTED
Booking/Invoice/Payment by `customer_id` relation; Assignment/Job by
`technician_id`; review by owner; notification rows scoped to user; admin by
role. 403 on mismatch; 404 for missing. (Audited across all 48 routes.)

## 4. Token Security — IMPLEMENTED
40-char crypto/rand token; DB stores SHA-256 only; 7-day expiry; revoke on
logout / password change / email verify. No token/hash in logs or responses.

## 5. Password — IMPLEMENTED
bcrypt cost 12; never in responses/logs/errors; current-password verified on
change; all tokens revoked after change.

## 6. OTP — IMPLEMENTED
6-digit crypto/rand; bcrypt-hashed at rest; single-use (FOR UPDATE);
expiry; attempt cap (5); never in responses/logs/emails leak.

## 7. Rate Limiting — IMPLEMENTED (NEW, FASE 17)
In-memory fixed-window per IP (mutex-safe), mirrors Laravel ThrottleRequests:

| Limiter | Limit | Window |
|---|---|---|
| auth-register | 5 | 1 min |
| auth-login | 10 | 1 min |
| otp-verify | 10 | 1 min |
| otp-resend | 3 | 10 min |
| booking-create | 10 | 1 min |
| payment-upload | 5 | 10 min |

429 body `{"message":"Too many requests. Please try again later."}` +
Retry-After. Single-instance; distributed limiting DEFERRED to production.

## 8. Request Limits — IMPLEMENTED (NEW)
`LimitBody`: JSON groups capped 1 MiB; customer group (multipart) 10 MiB
(built on http.MaxBytesReader before parsing). Payment proof additionally
bounded at 2 MB by the handler. Prevents memory exhaustion.

## 9. File Upload Security — IMPLEMENTED
Server-generated filename (`payment-proof-{hex}.{ext}`); client filename only
feeds the extension allowlist (jpg/jpeg/png); magic-byte sniffing (JPEG/PNG);
size cap 2 MB; `LocalStorage` keys must equal basename (traversal-proof: `..`,
`a/b`, absolute, backslashes rejected); O_EXCL no-overwrite; private directory;
proof served only via authenticated endpoints (inline + nosniff).

## 10. SQL Injection — IMPLEMENTED
All queries parameterized; dynamic `IN (...)` built from server-side id lists
with placeholders only; no client string interpolated into SQL identifiers.
(Audited; no `ORDER BY`/`LIMIT` from client.)

## 11. Mass Assignment — IMPLEMENTED
Explicit request DTOs; identity/status/counter fields never bindable from
clients (audited all JSON decodes).

## 12. CORS — IMPLEMENTED (NEW)
`CORS_ALLOWED_ORIGINS` allow-list; only allow-listed origins receive CORS
headers; preflight OPTIONS → 204; credentials enabled (bearer in
Authorization). Never `*`. Empty list = no CORS (same-origin/API clients).

## 13. Security Headers — IMPLEMENTED
Global: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
`Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy: ""`.
Proof responses add `Content-Disposition: inline` + `nosniff`.

## 14. HTTP Server Hardening — IMPLEMENTED (NEW)
ReadHeaderTimeout 5s, ReadTimeout 15s, WriteTimeout 30s, IdleTimeout 60s,
MaxHeaderBytes 1 MiB; graceful shutdown (10s) after SIGTERM/SIGINT.

## 15. Panic Recovery — IMPLEMENTED (NEW)
`JSONRecovery` (replaces gin.Recovery): panic → generic JSON 500, stack logged
server-side with request_id; never sent to the client.

## 16. Error Handling — IMPLEMENTED
Typed errors (httperr) → status + Laravel-shaped JSON; DB/driver/storage details
never reach clients; 500 generic + request_id log.

## 17. Request ID — IMPLEMENTED
`X-Request-ID` accepted/generated, echoed, logged; never used as identity.

## 18. Logging — IMPLEMENTED
slog JSON; no password/token/OTP/hash/paths/credentials/body logged. Payment
logs only ids/status.

## 19-23. Payment / Booking / Review / Notification — IMPLEMENTED
Payment: amount must equal invoice total (exact cents, no float); verify/reject
admin-only; concurrent verify single-winner (FOR UPDATE) — no client-controlled
status. Booking: state transitions validated; roles/ownership enforced per
endpoint. Review: owner + completed + technician + unique(booking_id) + rating
1–5 + admin-only moderation. Notification: user reads only own rows; recipient
ids server-derived; foreign id → 404.

## 24. Storage — IMPLEMENTED
Write only inside configured private directory; no directory listing exposure;
proofs not public.

## 25. Configuration Security — IMPLEMENTED
`.env.example` documents variables; `.env*` git-ignored (verified); no secrets
in code/logs.

## 26. Dependency Security — IMPLEMENTED (baseline)
Minimal deps: gin, mysql driver, x/crypto (bcrypt), uuid-free (crypto/rand).
No known outstanding critical packages used at runtime.

## 27. Security Tests — IMPLEMENTED
middleware: rate-limit threshold+window+concurrency, body cap, recovery (no
leak), CORS allow/deny/preflight; plus full domain suites cover
authorization/ownership/IDOR/duplicate/validation/concurrency.

## DEFERRED (production/deployment)
UFW/iptables/fail2ban/Cloudflare/WAF/firewall, TLS/reverse-proxy/SSH hardening,
distributed (redis) rate limiting, vertical prod dependency scanning cadence.

## KNOWN LIMITATIONS
- In-memory rate limit resets on process restart; single-instance only.
- `LimitBody` reports parser errors as 400 `"Invalid JSON payload."` (it bounds
  memory but Fast path chooses 400 over 413; documented).
- CORS disabled when env empty (no headers) — browsers require explicit
  allow-list for cross-origin.