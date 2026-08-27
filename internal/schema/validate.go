package schema

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Report is the outcome of a live schema validation.
type Report struct {
	TablesChecked int
	Errors        []string
}

func (r *Report) Healthy() bool { return len(r.Errors) == 0 }

// Validate compares the expected schema against the actual MySQL
// information_schema for the connected database (schema = DATABASE()).
//
// Coverage: table existence, column name/type/nullability, primary key,
// unique + non-unique indexes (name and column order), and foreign keys
// (child column, referenced table/column, ON DELETE rule).
// LIMITATION: column DEFAULT values are not compared — information_schema
// renders numeric defaults in MySQL-version-dependent formatting, so default
// comparison is intentionally out of scope (documented in
// docs/database-schema.md).
func Validate(ctx context.Context, db *sql.DB) (*Report, error) {
	rep := &Report{}
	for _, tb := range Expected {
		rep.TablesChecked++
		if err := validateTable(ctx, db, tb, rep); err != nil {
			return nil, err
		}
	}
	extra, err := findExtraTables(ctx, db)
	if err != nil {
		return nil, err
	}
	for _, name := range extra {
		rep.Errors = append(rep.Errors, fmt.Sprintf("unexpected table %q (framework tables are deliberately not migrated)", name))
	}
	return rep, nil
}

func validateTable(ctx context.Context, db *sql.DB, tb *Table, rep *Report) error {
	exists, err := tableExists(ctx, db, tb.Name)
	if err != nil {
		return err
	}
	if !exists {
		rep.Errors = append(rep.Errors, fmt.Sprintf("missing table %q", tb.Name))
		return nil
	}

	actual, err := actualColumns(ctx, db, tb.Name)
	if err != nil {
		return err
	}
	for _, col := range tb.Columns {
		got, ok := actual[col.Name]
		if !ok {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: missing column %q", tb.Name, col.Name))
			continue
		}
		if !strings.EqualFold(got.columnType, col.Type) {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s.%s: type %q != expected %q", tb.Name, col.Name, got.columnType, col.Type))
		}
		if got.isNullable() == col.NotNull {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s.%s: nullability mismatch (expected not-null=%v)", tb.Name, col.Name, col.NotNull))
		}
	}

	indexes, err := actualIndexes(ctx, db, tb.Name)
	if err != nil {
		return err
	}
	for _, ix := range tb.Indexes {
		got, ok := indexes[ix.Name]
		if !ok {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: missing index %q", tb.Name, ix.Name))
			continue
		}
		if strings.Join(got.columns, ",") != strings.Join(ix.Columns, ",") {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: index %q columns %v != expected %v", tb.Name, ix.Name, got.columns, ix.Columns))
		}
		if got.unique != ix.Unique {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: index %q unique flag mismatch", tb.Name, ix.Name))
		}
	}

	fks, err := actualForeignKeys(ctx, db, tb.Name)
	if err != nil {
		return err
	}
	for _, fk := range tb.ForeignKeys {
		got, ok := fks[fk.Name]
		if !ok {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: missing FK %q", tb.Name, fk.Name))
			continue
		}
		if got.column != fk.Column || got.refTable != fk.RefTable || got.refColumn != fk.RefColumn || !strings.EqualFold(got.onDelete, fk.OnDelete) {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: FK %q mismatch (got %+v, expected %+v)", tb.Name, fk.Name, got, fk))
		}
	}
	return nil
}

type columnInfo struct {
	columnType string
	nullable   string
}

func (c columnInfo) isNullable() bool { return c.nullable == "YES" }

type indexInfo struct {
	columns []string
	unique  bool
}

type fkInfo struct {
	column    string
	refTable  string
	refColumn string
	onDelete  string
}

func tableExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, name).
		Scan(&n)
	return n > 0, err
}

func findExtraTables(ctx context.Context, db *sql.DB) ([]string, error) {
	want := make(map[string]bool, len(Expected))
	for _, tb := range Expected {
		want[tb.Name] = true
	}
	rows, err := db.QueryContext(ctx,
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var extra []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if !want[name] {
			extra = append(extra, name)
		}
	}
	return extra, rows.Err()
}

func actualColumns(ctx context.Context, db *sql.DB, table string) (map[string]columnInfo, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT column_name, column_type, is_nullable
		 FROM information_schema.columns
		 WHERE table_schema = DATABASE() AND table_name = ?
		 ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]columnInfo{}
	for rows.Next() {
		var name, typ, nullable string
		if err := rows.Scan(&name, &typ, &nullable); err != nil {
			return nil, err
		}
		out[name] = columnInfo{columnType: typ, nullable: nullable}
	}
	return out, rows.Err()
}

func actualIndexes(ctx context.Context, db *sql.DB, table string) (map[string]indexInfo, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT index_name, GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ','), non_unique
		 FROM information_schema.statistics
		 WHERE table_schema = DATABASE() AND table_name = ?
		 GROUP BY index_name, non_unique`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]indexInfo{}
	for rows.Next() {
		var name, cols string
		var nu int
		if err := rows.Scan(&name, &cols, &nu); err != nil {
			return nil, err
		}
		out[name] = indexInfo{columns: strings.Split(cols, ","), unique: nu == 0}
	}
	return out, rows.Err()
}

func actualForeignKeys(ctx context.Context, db *sql.DB, table string) (map[string]fkInfo, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT constraint_name, column_name, referenced_table_name, referenced_column_name, delete_rule
		 FROM information_schema.key_column_usage
		 WHERE table_schema = DATABASE() AND table_name = ? AND referenced_table_name IS NOT NULL`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]fkInfo{}
	for rows.Next() {
		var fi fkInfo
		var name string
		if err := rows.Scan(&name, &fi.column, &fi.refTable, &fi.refColumn, &fi.onDelete); err != nil {
			return nil, err
		}
		out[name] = fi
	}
	return out, rows.Err()
}
