package schema

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

// unitCheck runs structural consistency checks over the expected schema — no
// database required. Failures here mean the schema definition itself is wrong.
func TestExpectedSchemaIsInternallyConsistent(t *testing.T) {
	seenTables := map[string]bool{}
	for _, tb := range Expected {
		if seenTables[tb.Name] {
			t.Errorf("duplicate table %q", tb.Name)
		}
		seenTables[tb.Name] = true

		colSet := map[string]bool{}
		for _, col := range tb.Columns {
			if colSet[col.Name] {
				t.Errorf("%s: duplicate column %q", tb.Name, col.Name)
			}
			colSet[col.Name] = true
		}

		ixSet := map[string]bool{}
		for _, ix := range tb.Indexes {
			if ixSet[ix.Name] {
				t.Errorf("%s: duplicate index %q", tb.Name, ix.Name)
			}
			ixSet[ix.Name] = true
			if len(ix.Columns) == 0 {
				t.Errorf("%s: index %q has no columns", tb.Name, ix.Name)
			}
			for _, col := range ix.Columns {
				if !colSet[col] {
					t.Errorf("%s: index %q references unknown column %q", tb.Name, ix.Name, col)
				}
			}
		}

		fkSet := map[string]bool{}
		for _, fk := range tb.ForeignKeys {
			if fkSet[fk.Name] {
				t.Errorf("%s: duplicate FK %q", tb.Name, fk.Name)
			}
			fkSet[fk.Name] = true
			if !colSet[fk.Column] {
				t.Errorf("%s: FK %q references unknown column %q", tb.Name, fk.Name, fk.Column)
			}
			parent := tableByName(fk.RefTable)
			if parent == nil {
				t.Errorf("%s: FK %q references missing table %q", tb.Name, fk.Name, fk.RefTable)
				continue
			}
			if pc := parent.findColumn(fk.RefColumn); pc == nil {
				t.Errorf("%s: FK %q references missing column %s.%s", tb.Name, fk.Name, fk.RefTable, fk.RefColumn)
			} else if !pc.NotNull {
				t.Errorf("%s: FK %q references nullable key %s.%s", tb.Name, fk.Name, fk.RefTable, fk.RefColumn)
			}
			switch fk.OnDelete {
			case "CASCADE", "SET NULL":
			default:
				t.Errorf("%s: FK %q has unsupported ON DELETE %q", tb.Name, fk.Name, fk.OnDelete)
			}
		}

		hasPK := false
		for _, ix := range tb.Indexes {
			if ix.Name == "PRIMARY" {
				hasPK = true
			}
		}
		if !hasPK {
			t.Errorf("%s: missing PRIMARY index", tb.Name)
		}
	}
}

func TestCountsMatchAudit(t *testing.T) {
	// 16 migrated tables = 15 domain + 1 framework (personal_access_tokens).
	if got := len(Expected); got != 16 {
		t.Fatalf("expected 16 tables, got %d", got)
	}

	fkTotal, uniqueTotal, composite, money := 0, 0, 0, 0
	for _, tb := range Expected {
		fkTotal += len(tb.ForeignKeys)
		uniqueTotal += tb.UniqueCount()
		for _, ix := range tb.Indexes {
			if len(ix.Columns) > 1 && ix.Name != "PRIMARY" {
				composite++
			}
		}
		for _, col := range tb.Columns {
			if col.Type == "decimal(12,2)" {
				money++
			}
		}
	}
	if fkTotal != 19 {
		t.Errorf("expected 19 foreign keys, got %d", fkTotal)
	}
	if uniqueTotal != 13 { // 12 domain + tokens.token
		t.Errorf("expected 13 unique constraints, got %d", uniqueTotal)
	}
	if money != 11 {
		t.Errorf("expected 11 DECIMAL(12,2) columns, got %d", money)
	}
	if composite != 11 {
		t.Errorf("expected 11 composite indexes, got %d", composite)
	}
}

func TestStatusColumnTypes(t *testing.T) {
	statuses := map[string]struct{ typ, defaultVal string }{
		"bookings.status":              {"varchar(255)", "pending_payment"},
		"invoices.status":              {"varchar(255)", "unpaid"},
		"payments.status":              {"varchar(20)", "waiting_verification"},
		"booking_assignments.status":   {"varchar(20)", "pending"},
		"reviews.status":               {"varchar(20)", "published"},
		"users.role":                   {"varchar(30)", "customer"},
		"email_verification_otps.type": {"varchar(30)", "email_verification"},
		"notifications.id":             {"char(36)", ""},
	}
	for key, want := range statuses {
		parts := splitKey(key)
		tb := tableByName(parts[0])
		if tb == nil {
			t.Fatalf("missing table for %s", key)
		}
		col := tb.findColumn(parts[1])
		if col == nil {
			t.Fatalf("missing column for %s", key)
		}
		if col.Type != want.typ {
			t.Errorf("%s: type %q != %q", key, col.Type, want.typ)
		}
	}
}

func splitKey(k string) [2]string {
	var out [2]string
	dot := -1
	for i := 0; i < len(k); i++ {
		if k[i] == '.' {
			dot = i
			break
		}
	}
	out[0] = k[:dot]
	out[1] = k[dot+1:]
	return out
}

// TestLiveSchemaValidation runs only when a test MySQL DSN is provided via
// TEST_DATABASE_URL. It never touches production data: it runs the same
// migrations the repo ships into the given database and verifies the schema.
func TestLiveSchemaValidation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; live schema validation requires a disposable MySQL test database")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	rep, err := Validate(ctx, db)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !rep.Healthy() {
		t.Errorf("schema validation errors: %v", rep.Errors)
	}
}
