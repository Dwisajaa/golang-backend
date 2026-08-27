# HTTP Foundation

Phase 4 deliverable. Minimal, testable HTTP server per `docs/go-architecture.md`.

## Server

- Binary entry: `cmd/api/main.go` (composition root).
- Wires dependencies (logger → handler → router → `http.Server`), starts listener on `:8080`, waits for SIGINT/SIGTERM, drains gracefully (10s shutdown timeout).
- `ReadHeaderTimeout: 5s` prevents slowloris-style header stalls.

## Router

- `internal/router/router.go` builds a Gin engine and applies global middleware in a fixed order:
  `Recovery → RequestID → Logging → SecurityHeaders`.
- Routes are registered here (`GET /health`), not in `main.go`.

## Handler

- `internal/httphandler/health.go` — `HealthHandler` returns `{"status":"ok"}`.
- Handlers only translate HTTP; no business logic (there is none yet).

## Middleware

| Middleware | Purpose |
|---|---|
| `gin.Recovery()` | Panic → 500 JSON; server never dies |
| `RequestID()` | Accept/generate `X-Request-ID`, echo, store on context |
| `Logging()` | One structured line: method, path, status, latency_ms, client_ip, request_id |
| `SecurityHeaders()` | `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, `Permissions-Policy` |

Deferred to later phases (explicitly documented in architecture, not built yet):
CORS (`CorsAllowlist`), rate limiting, auth, role — they need authentication/config which is not part of the foundation.

## Error Handling

- `panic` vs `error`: Go treats unexpected programmer mistakes (nil deref, index out of range) as panics; expected outcomes (bad input, missing row) are returned as errors. Handlers should fail with errors; `gin.Recovery()` is the safety net for real panics so one bad request cannot take the server down.
- 404: Gin's default plain-text 404 is kept. A custom JSON 404 mapper is NOT built yet because the foundation has a single endpoint; the decision is documented here rather than adding an abstraction before a need exists.

## Health Endpoint

```
GET /health
```
Response: `200` `application/json`

```json
{"status":"ok"}
```

No metadata was added — the probe contract is deliberately minimal.

## Testing

`internal/httphandler/health_test.go` (three tests):

1. `TestHealthEndpoint` — method, route, status code, content type, JSON body.
2. `TestNotFoundIs404` — unknown route returns 404.
3. `TestHealthMethodNotAllowed` — a method with no registered route (POST /health) falls back to 404.

Tests use `httptest.NewRequest` + `httptest.NewRecorder` + `router.ServeHTTP`, so no real port/network is involved: the Gin engine is an `http.Handler`, and `ServeHTTP` is invoked in-process.

## Commands

```
go build ./...
go vet ./...
go test ./...
go test -race ./...
go run ./cmd/api          # serves :8080
curl http://localhost:8080/health
```

## Results (2026-08-27)

| Check | Result |
|---|---|
| `go fmt ./...` | 0 |
| `go vet ./...` | 0 |
| `go test ./...` | PASS |
| `go test -race ./...` | PASS |
| `go build ./...` | PASS |
| Runtime `GET /health` | 200 `{"status":"ok"}` |
| Runtime `GET /not-found` | 404 |