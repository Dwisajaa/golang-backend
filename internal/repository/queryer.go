package repository

import (
	"context"
	"database/sql"
)

// Queryer is the DB-agnostic surface that runs within a transaction: both
// *sql.DB and *sql.Tx satisfy it, so repositories can be called from a service
// with either the pool or the active transaction.
type Queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

var _ Queryer = (*sql.DB)(nil)
var _ Queryer = (*sql.Tx)(nil)
