# Deployment Strategy

This project deploys as a single Go binary (+ in-process worker), backed by
MySQL 8.0, fronted by Nginx, and mounting a private storage volume.

## 1. Prerequisites (VPS setup)

- **Firewall (UFW)**: ALLOW `22/tcp` (SSH), `80/tcp`, `443/tcp`. Block
  everything else (3306, 8080 must be private).
- **MySQL**: running locally or on a private network; create database and user;
  note credentials.
- **Secrets file**: Create `/etc/api-dwidev.env` (chmod 0600, root/app user
  only). Fill it using `.env.example` as a template.
- **Storage**: Create `/opt/api-dwidev/storage` or a Docker volume; ownership
  must match the app user (`www-data` or distroless `65532`).

## 2. CI/CD Pipeline

The `.github/workflows/go.yml` runs formatting, vetting, and race-detector
tests on every push. Deployment happens by pulling the artifact (or Docker
image) compiled from `main`.

*Note: Database integration tests (`internal/repository/*`) are gated by
`TEST_DATABASE_URL` and skipped when missing. Provisioning a MySQL test
service in CI is DEFERRED to future CI hardening.*

## 3. Deployment Steps (Docker Compose example)

1. **Pull image / Code**: `docker compose -f deploy/compose.prod.yaml pull api`
   (or build locally).
2. **Backup DB**: `mysqldump` (see `backup-and-recovery.md`).
3. **Migration**: Run the `migrate` binary once.
   ```bash
   docker compose -f deploy/compose.prod.yaml --env-file /etc/api-dwidev.env run --rm migrate
   ```
4. **Restart App**:
   ```bash
   docker compose -f deploy/compose.prod.yaml --env-file /etc/api-dwidev.env up -d api
   ```
5. **Wait for Health**:
   The compose file exposes no external ports directly.
   ```bash
   curl -s http://127.0.0.1:8080/health  # should be 200 {"status":"ok"}
   curl -s http://127.0.0.1:8080/ready   # should be 200 {"status":"ready"} (MySQL connected)
   ```

## 4. Rollback Strategy

The application binary rollback is fast (revert to previous Docker tag/binary).

**Database rollback limitation**: Migrations are strictly forward-only
(idempotent `CREATE TABLE` and additive changes). We do NOT ship `DOWN`
migrations (destructive) as a deployment routine.
If a bad migration corrupts data, restore the DB from the pre-deploy backup. If
the migration was additive (e.g. new column), the old app binary usually safely
ignores it (forward-compatible), but you must verify.

## 5. Zero-Downtime

Current deployment via standard `docker compose up -d` involves a short
(sub-second) downtime while the container cycles. The Go app shuts down
gracefully (10s SIGTERM window), so in-flight requests finish.
True zero-downtime requires Nginx upstream load-balancing across two
containers/ports, which is DEFERRED (over-engineering for the current phase).