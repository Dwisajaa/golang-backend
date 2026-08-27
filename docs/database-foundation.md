# Database Foundation

## database/sql

`database/sql` is the standard library interface for SQL databases. It manages
a **connection pool** (see below) and exposes `*sql.DB` plus
`*sql.Tx`/`*sql.Rows`. Queries are parameterized via `?` placeholders, which
completely avoids SQL-injection string concatenation. We chose it over GORM
and sqlc (see `go-architecture.md` §23) for exact control of `FOR UPDATE`
locks and row mapping.

## MySQL Driver

`github.com/go-sql-driver/mysql` — the de-facto MySQL driver, `database/sql`
compatible. Added explicitly because `database/sql` only defines the interface;
a concrete driver is required.

## DSN

Built with the driver's own `mysql.NewConfig()` (not string concatenation),
so escaping is handled correctly:

```go
User = cfg.User
Passwd = cfg.Password
Net = "tcp"
Addr = host:port
DBName = cfg.Name
ParseTime = true      // scan DATETIME/TIMESTAMP into time.Time
Params = {"charset": "utf8mb4"}
// Loc defaults to time.UTC in the driver; the DSN omits `loc=` for the default.
```

| Parameter | Why |
|---|---|
| `parseTime=true` | Drive `DATETIME`/`TIMESTAMP` into `time.Time` instead of raw bytes |
| `charset=utf8mb4` | Full Unicode (emojis, special characters) — matches Laravel schema |
| `Loc` (default UTC) | Deterministic timezone handling; timestamps are UTC |

The DSN renders like:
`user:password@tcp(host:3306)/dbname?parseTime=true&charset=utf8mb4`

## Connection Pool

`*sql.DB` is a pool, not one connection. Configured in `internal/db`:

| Setting | Value | Meaning |
|---|---|---|
| `SetMaxOpenConns` | 25 | Max simultaneously-open connections. Above this, callers wait. |
| `SetMaxIdleConns` | 25 | Max idle connections kept warm in the pool. |
| `SetConnMaxLifetime` | 5m | Hard cap on a connection's age (rotation, avoids long-lived stale TCP). |
| `SetConnMaxIdleTime` | 5m | Drop idle connections older than this (returns them to MySQL). |

Open connection = a live TCP connection to MySQL. Idle connection = open but
not currently executing a query, parked for reuse. Too many open connections
exhaust MySQL's `max_connections` (each Go worker pool connection + each idle
connection counts server-side); `MaxOpenConns` must stay well below the
server's limit. These are **initial values, not tuned** — the benchmark phase
produces real numbers (documented, no assumptions).

## Context

Every database operation uses `context.Context`. `PingContext` at startup uses
`context.WithTimeout(ctx, 5s)`. Later, business operations will flow the HTTP
request context down to queries, so:
- client disconnects cancel the query,
- timeouts are enforced per operation,
- `context.Background()` is reserved for startup/shutdown with an explicit
  deadline, never for per-request work.

## Health Check

Two probes, separated by meaning:

| Probe | Meaning | Resp |
|---|---|---|
| `GET /health` | Liveness — process is up | 200 `{"status":"ok"}` (no DB involved) |
| `GET /ready` | Readiness — primary dependency reachable | 200 `{"status":"ready"}` / 503 `{"status":"unavailable"}` (DB ping ≤ 2s) |

`ReadyHandler` depends on a `Pinger` interface, so tests stub the DB.

## Lifecycle

```
startup → load config → open pool → ping (fail = exit) → start HTTP server
shutdown → SIGINT/SIGTERM → http.Shutdown(10s) → pool.Close()
```

Bringing the pool down after the HTTP server stops guarantees no in-flight
request is left holding a connection at process exit.

## Testing

- Unit: config load/validation (no DB), DSN rendering, ready-handler stub.
- The repo deliberately has **no live-DB tests yet**: none of the foundation
  tests contact a database. Opening a real pool requires a MySQL instance.
- Integration coverage (repository tests with a disposable MySQL database)
  lands in FASE 10. Until then `TestOpenRequiresRunningMySQL` documents the
  requirement and is skipped.

```
go test ./...            # PASS (unit-only)
go test -race ./...      # PASS
```

## Production Tuning

- Benchmark will set: `MaxOpenConns`/`MaxIdleConns` vs observed concurrency;
  `ConnMaxLifetime` if `wait_timeout`/`mysql has gone away` appears.
- Keep total pooled connections × replicas < MySQL `max_connections`.
- Locate DB on the same network as the API in benchmarks to avoid network
  variable in comparisons.