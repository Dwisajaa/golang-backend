package db

import (
	"context"
	"database/sql"
	"fmt"
)

// TxManager owns transaction boundaries for business operations. Services use
// Within to run an atomic unit of work; the manager guarantees rollback on
// error and commit only on success.
type TxManager struct {
	db *sql.DB
}

func NewTxManager(db *sql.DB) *TxManager { return &TxManager{db: db} }

// Within begins a transaction, runs fn (giving it the *sql.Tx), then commits.
// If fn returns an error (or panics), Rollback runs instead.
func (m *TxManager) Within(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// Rollback after Commit is a no-op; this guard ensures we never leave a
	// transaction dangling when fn fails or panics.
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
