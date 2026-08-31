# Production Architecture

Target deployment: single VPS, Docker or systemd, Nginx reverse proxy.

```
Internet
   ↓ (optional Cloudflare)
Nginx  (443 TLS, WAF boundary optional)
   ↓ X-Forwarded-* — only trusted proxy
127.0.0.1:8080  (Go API, non-root container/user)
   ├── MySQL 8.0 (localhost/private; never public)
   ├── private payment-proof storage (persistent volume/bind)
   └── (in-process mail worker for OTP)
```

- **Go API** binds 127.0.0.1:8080 (never public internet).
- **Rate limiter / ClientIP** trusts `TRUSTED_PROXIES` only (default
  `127.0.0.1`); anything else must be harcoded explicit.
- **Storage** must be a persistent mount; ephemeral container fs is not
  acceptable for payment proofs.
- **Worker/OTP** runs in-process (bounded channel) — single API process is the
  only runtime; do not start a second copy of the worker.

## Deployable artifacts

- `Dockerfile` (multi-stage, distroless nonroot runtime) + `migrations/` baked.
- `cmd/migrate` binary included for one-shot forward migrations.
- `deploy/nginx.conf.example`, `deploy/api-dwidev.service.example`
  (systemd, non-root + ProtectSystem), `deploy/compose.prod.yaml.example`
  (API + migrate; MySQL external; secrets via `--env-file`).

## Environment (production)

Referenced from `.env.example`; secrets externalized (`/etc/api-dwidev.env`
chmod 0600). Required: `DATABASE_*`, `APP_ENV=production`, `SMTP_*`
(else LogMailer!), `CORS_ALLOWED_ORIGINS` (no wildcard), `TRUSTED_PROXIES`,
`STORAGE_PAYMENT_PROOFS_PATH`.