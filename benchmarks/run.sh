#!/usr/bin/env bash
# Benchmark harness: Laravel (port 8000) vs Go (port 8080) against the SAME
# MySQL database & SAME dataset (seeded by Laravel `php artisan e2e:reset`).
#
# Prereqs (see docs/benchmark-runbook.md):
#   - MySQL reachable at $DB_HOST:$DB_PORT (Docker or VPS)
#   - Laravel running: php artisan serve --host=127.0.0.1 --port=8000
#   - Go built:        go build -o /tmp/gobmk ./cmd/api
#   - Go env:          APP_ENV=production DATABASE_HOST=... etc (same DB)
#   - hey installed:   go install github.com/rakyll/hey@latest
#
# Usage:
#   BENCH_DURATION=30s BENCH_CONC=5,25 BENCH_DURATION=30s ./benchmarks/run.sh

set -euo pipefail

GO_BASE="${GO_BASE:-http://127.0.0.1:8080}"
LA_BASE="${LA_BASE:-http://127.0.0.1:8000}"
DUR="${BENCH_DURATION:-30s}"
CONCS="${BENCH_CONC:-1 5 25}"
HEY="${HEY:-$(go env GOPATH)/bin/hey}"
OUT="benchmarks/raw"
mkdir -p "$OUT"

# Authenticated endpoints need a token. Two tokens are fetched once and reused
# for all concurrency levels (per docs/benchmark-matrix.md).
login_token() {
  local base="$1"
  curl -s -X POST "$base/api/login" -H "Content-Type: application/json" \
    -d '{"email":"customer@example.test","password":"password123"}' | sed -E 's/.*"token":"([^"]+)".*/\1/'
}
GO_TOKEN="$(login_token "$GO_BASE")"
LA_TOKEN="$(login_token "$LA_BASE")"

run_case() {
  local name="$1" url="$2" token="$3"
  shift 3
  local conc="$1"
  local h=""; [ -n "$token" ] && h="-H Authorization:Bearer $token"
  # shellcheck disable=SC2086
  "$HEY" -n 0 -z "$DUR" -c "$conc" $h -m GET "$url" \
    > "$OUT/${name}-c${conc}-$(date +%s).txt"
}

for c in $CONCS; do
  # health (baseline, supplementary)
  run_case go-health   "$GO_BASE/health"   "" $c
  run_case la-health   "$LA_BASE/api/health" "" $c
  # catalog (public)
  run_case go-cat      "$GO_BASE/api/categories?per_page=15" "" $c
  run_case la-cat      "$LA_BASE/api/categories?per_page=15" "" $c
  run_case go-svc      "$GO_BASE/api/services?per_page=15" "" $c
  run_case la-svc      "$LA_BASE/api/services?per_page=15" "" $c
  run_case go-pkg      "$GO_BASE/api/packages?per_page=15" "" $c
  run_case la-pkg      "$LA_BASE/api/packages?per_page=15" "" $c
  # authenticated reads (reused token)
  run_case go-bk        "$GO_BASE/api/bookings?per_page=15" "$GO_TOKEN" $c
  run_case la-bk        "$LA_BASE/api/bookings?per_page=15" "$LA_TOKEN" $c
  run_case go-inv       "$GO_BASE/api/invoices?per_page=15" "$GO_TOKEN" $c
  run_case la-inv       "$LA_BASE/api/invoices?per_page=15" "$LA_TOKEN" $c
  run_case go-notif     "$GO_BASE/api/notifications?per_page=15" "$GO_TOKEN" $c
  run_case la-notif     "$LA_BASE/api/notifications?per_page=15" "$LA_TOKEN" $c
done

echo "raw results written to $OUT"
echo "aggregate per docs/benchmark.md (parse .txt: Total/RPS/Latency p50/p95/p99/Error distribution)"