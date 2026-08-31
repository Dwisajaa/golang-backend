# Benchmark: Laravel vs Go

Status: **NOT EXECUTED in the development environment** — see Limitations.

## 1. Objective
Measure equivalent API workloads on Laravel (behavioral reference) and the Go
migration, on the same machine, same MySQL, same dataset, same load tool, same
request payload. No performance claims without measured data (FASE 19 rule).

## 2. Environment (required / intended)

| Item | Value (target) |
|---|---|
| Machine | Single VPS/host, both APIs + MySQL + load tool on same box |
| OS | Linux (docker or native) |
| CPU | as deployed (document) |
| RAM | as deployed (document) |
| Network | loopback only |

## 3. Software versions (intended)

| Component | Version |
|---|---|
| Laravel | 12.67.0 / PHP 8.4 (Docker) |
| Go | 1.27 (module go directive) |
| MySQL | 8.0 |
| Load tool | hey (ulul.cloud / rakyll) |

## 4. Database & Dataset
Same MySQL 8 database; schema = Laravel migrations (Go forward migrations
replicate it — parity validated FASE 6/16). Dataset seeded by Laravel
`php artisan e2e:reset` so both apps read identical rows (record counts per
table in the results section).

## 5. Endpoints & Matrix
See `docs/benchmark-matrix.md`. Auth tokens reused (not per-request login).
Catalog + authenticated reads; writes only on isolated test data.

## 6. Methodology (fixed)
- Load tool: **hey** for both.
- Warm-up: e.g., 20s at each concurrency before a 30s measurement.
- Concurrency: 1, 5, 25 (expand to saturation on host).
- Runs: ≥3 per (endpoint, concurrency); report **median** (and min/max).
- Metrics: RPS, avg, p50, p95, p99, error %, plus system CPU/RAM and DB
  connections during the run.
- Response validation: status 200 + required JSON fields verified before
  measuring (contract parity already audited — FASE 16).

## 7. Results

> **Results are BLOCKED in this environment** (see Limitations). The harness
> and runbook are ready; the numbers below are intentionally left empty rather
> than fabricated.

### Aggregated (median of ≥3 runs)

| Endpoint | Backend | Conc | RPS | p50 | p95 | p99 | Error% |
|---|---|---|---:|---:|---:|---:|---:|
| /api/services | Go | 1 | — | — | — | — | — |
| /api/services | Laravel | 1 | — | — | — | — | — |
| /api/bookings | Go | 25 | — | — | — | — | — |
| /api/bookings | Laravel | 25 | — | — | — | — | — |
| … (all matrix cases) | | | | | | | |

### Resource (median)

| Backend | CPU% | RAM | DB Connections |
|---|---:|---:|---:|

### Saturation point (to be determined on host)

blanks.

## 8. Findings (no code changes made)
No optimizations were applied before baseline (FASE 19 §2). Any
opportunities found during measurement will be recorded here without changing
business code.

## 9. Limitations
- **Not executed**: development machine has **no MySQL server** (Laravel env
  `DB_HOST=mysql` and Go require MySQL), Docker daemon offline, and no load
  tool installed — a fair paired benchmark is impossible locally. Running it
  against mismatched engines (e.g., Laravel/SQLite vs Go/MySQL) would violate
  parity and produce meaningless numbers.
- The `benchmarks/run.sh` + `docs/benchmark-runbook.md` + matrix are
  deploy-host ready; execute them there and fill §7.
- No claim of "Go faster" or any ratio appears anywhere in this repo until §7
  contains measured data.

## 10. Conclusion (provisional)
The comparison is reproducible and queued; it must run on the production-class
host with MySQL present. Until then, performance parity is **unverified**.