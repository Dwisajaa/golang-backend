# Production Readiness Matrix

| Area | Status | Evidence | Risk |
|---|---|---|---|
| Application | READY | FASE 7A–16 domains; 79+ tests, race clean | LOW |
| Security | READY | FASE 17 (rate-limit, limits, CORS, recovery, headers) | LOW |
| Docker | PARTIAL | Dockerfile + .dockerignore written; local build not validated (daemon offline) | LOW (verify on deploy host) |
| Database | READY | schema parity (FASE 6); pool 25/25 + 5m lifetime | LOW |
| Redis | NOT APPLICABLE | not used | — |
| Storage | READY | private local disk + path-traversal safe | LOW (must be persistent mount) |
| Nginx | PARTIAL | template `deploy/nginx.conf.example` provided | LOW (adapt domain) |
| TLS | DEFERRED | certbot/nginx at deploy | MEDIUM (must be HTTPS before launch) |
| Firewall | DEFERRED | UFW rules documented only | MEDIUM (enforce before launch) |
| SSH Hardening | DEFERRED | key auth recommended only | MEDIUM |
| Cloudflare/WAF | OPTIONAL | not required | LOW |
| Logging | READY | slog JSON + request_id | LOW |
| Log Rotation | DEFERRED | journald/docker log config at deploy | LOW |
| Monitoring | PARTIAL | /health + /ready + access log; no external monitor | MEDIUM |
| Resource Limits | PARTIAL | systemd ProtectSystem + docker restart; no CPU/mem cap yet | LOW |
| DB Resilience | READY | generic 500 on DB failure; pool bounded | LOW |
| Redis Failure | NOT APPLICABLE | n/a | — |
| Deployment | READY | documented steps (compose + systemd) | LOW |
| Rollback | PARTIAL | app binary rollback; DB forward-only + backup restore | MEDIUM |
| Zero Downtime | NOT CLAIMED | short restart window; true ZDT deferred | MEDIUM |
| Smoke Test | READY | health/ready/login/profile/catalog | LOW |
| Benchmark Prep | READY | stable contract + health endpoint; same-dataset required | — |

## Risk Register

| Severity | Risk | Mitigation |
|---|---|---|
| CRITICAL | DB publicly exposed | UFW: block 3306/6379/8080 public |
| CRITICAL | Missing TLS in production | HTTPS via Nginx/certbot before launch |
| HIGH | No backups / no restore test | cron backup + quarterly restore drill |
| HIGH | Root container in production | distroless nonroot (Dockerfile) / www-data (systemd) |
| MEDIUM | Rate limiter trust spoof | `TRUSTED_PROXIES` (default 127.0.0.1) |
| MEDIUM | Payment proofs lost on container recreate | persistent volume (compose) |
| MEDIUM | DB migration without backup | deploy step 2 (backup before migrate) |

## Deferred Production Items

Kubernetes, service mesh, microservices, Kafka, distributed tracing,
multi-region, sharding, read replicas, autoscaling, true zero-downtime,
CI MySQL service, distributed (Redis) rate limiting.
