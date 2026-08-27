# Configuration

## Environment Variables

All configuration is read from the process environment at startup. No config
library is used — `os.LookupEnv`/`os.Getenv` are enough and keep production
dependency-free.

| Variable | Required | Default | Notes |
|---|---|---|---|
| `APP_ENV` | no | `development` | `development` \| `production` \| `testing` |
| `APP_PORT` | no | `8080` | validated 1–65535 |
| `DATABASE_HOST` | **yes** | — | startup fails if missing |
| `DATABASE_PORT` | no | `3306` | validated 1–65535 |
| `DATABASE_NAME` | **yes** | — | startup fails if missing |
| `DATABASE_USER` | **yes** | — | startup fails if missing |
| `DATABASE_PASSWORD` | no | — | optional (MySQL may allow empty) |

`.env.example` documents these. A real `.env` is git-ignored and never created
by the code.

## Configuration Structure

`internal/config/config.go`:

```go
type Config struct {
    App      AppConfig      // Env string, Port int
    Database DatabaseConfig // Host, Port, Name, User, Password
}
```

- A struct groups related settings and gives them types.
- `Load(fn func(string) string)` is the constructor. It takes the getter as a
  parameter so tests can inject a fake environment map; `LoadFromOS()` binds it
  to the real `os.Getenv`.
- The struct is a value produced once in `cmd/api/main.go` and passed down
  (dependency injection). Nothing global; tests build their own config.

## Validation

At startup `Load`:
- requires `DATABASE_HOST`, `DATABASE_NAME`, `DATABASE_USER` — names them in
  the error, exits with code 1.
- validates `APP_PORT` and `DATABASE_PORT` are integers 1–65535.
- returns a typed `*Config`; `main` refuses to start the server when loading
  fails (`database dependency is mandatory` policy).

A live database is also mandatory: `db.Open` pings on startup and `main`
exits if the ping fails. The server never runs in a half-configured state.

## Secrets

- The database password is part of the DSN (the MySQL driver needs it for
  connecting) but is **never logged**, never included in config errors, and
  never printed at startup.
- Logs at startup include host/port/name only.

## Local Development

Two supported ways:
- Export variables in the shell, or
- Use Docker compose to pass the environment (`compose.yaml` for reference).

There is intentionally no auto-loading of a local `.env` file; if you need
one, export it (`set -a; source .env; set +a` on Linux/macOS, or a small
script) before `go run ./cmd/api`.

## Production

- Values come from the runtime environment / secret manager / compose
  `--env-file`; nothing is baked into the binary.
- `DATABASE_PASSWORD` must never be committed; `.gitignore` covers `.env.*`.