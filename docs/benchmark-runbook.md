# Benchmark Runbook

Reproducible steps to benchmark Laravel vs Go on the same MySQL & dataset.

## Prerequisites (one-time, on the benchmark host)

1. MySQL 8.0 reachable (Docker container or VPS service); note `DB_HOST/PORT`.
2. This repo + the Laravel repo (`api-dwidev`) on the same machine.
3. Tools: `php`, `go`, `curl`, `hey`
   (`go install github.com/rakyll/hey@latest` → `$(go env GOPATH)/bin/hey`).

## 1. Prepare the shared database

```bash
# Laravel side (source of the dataset — same schema both read):
cd api-dwidev
cp .env.example .env        # fill DB_* to the shared MySQL
php artisan migrate:fresh   # on the isolated benchmark DB only
php artisan e2e:reset       # seeds users/profiles/catalog/bookings/... E2E rows
cd .. && cd golang-backend
go run ./cmd/migrate        # applies the Go forward migrations to the SAME DB
```

Record row counts for the report:

```bash
mysql -h$DB_HOST -P$DB_PORT -u$DB_USER -p"$DB_PASSWORD" $DB_DATABASE \
 -e "SELECT 'users',COUNT(*) FROM users UNION ALL SELECT 'services',COUNT(*) FROM services ...;"
```

## 2. Run both APIs on the same host

```bash
# Laravel (port 8000)
cd api-dwidev && php artisan serve --host=127.0.0.1 --port=8000 &

# Go (port 8080) — production-ish env, same DB
cd golang-backend && APP_ENV=production \
  DATABASE_HOST=$DB_HOST DATABASE_PORT=$DB_PORT DATABASE_NAME=$DB_DATABASE \
  DATABASE_USER=$DB_USER DATABASE_PASSWORD=$DB_PASSWORD \
  CORS_ALLOWED_ORIGINS= TRUSTED_PROXIES=127.0.0.1 \
  go run ./cmd/api &
```

IP: both bind loopback; the load tool runs on the same box (same network
topology for both).

## 3. Run the benchmark

```bash
b=0
candidate() { [ -x "$1" ] && b=1; }
[ -x "$(go env GOPATH)/bin/hey" ] && b=1
if [ "$b" != 1 ]; then echo "install hey"; exit 1; fi

BENCH_DURATION=30s BENCH_CONC="1 5 25" bash benchmarks/run.sh
```

## 4. Collect system metrics (same method for both)

- CPU/RAM: `docker stats` (Docker) or `pidstat`/`ps` (native) at intervals.
- DB: `SHOW GLOBAL STATUS LIKE 'Threads_connected';` queried periodically;
  capture `Threads_running`/`Connections`.
- Query count (optional): enable MySQL general_log during a single
  concurrency-1 run for each backend.

## 5. Aggregate results

Parse `benchmarks/raw/*.txt` (hey output: `Requests/sec`, `Latency`
distribution with p50/p95/p99, `Status code distribution` for error rate).
Compute median across ≥3 runs. Fill the tables in `docs/benchmark.md`.

## 6. Guardrails

- Never run against production data; use an isolated benchmark DB + `e2e:reset`.
- Do not disable auth or rate limits on one side only.
- Record every environment variable; a different dataset/DB makes the numbers
  invalid (FASE 19 §1).