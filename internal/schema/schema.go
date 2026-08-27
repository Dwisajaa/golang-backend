// Package schema is the single expected-schema definition for the Go backend.
// It mirrors the Laravel FINAL schema (docs/database-audit.md, migrations/*.sql)
// and is used both for unit consistency checks and for live validation against
// information_schema.
package schema

type Column struct {
	Name    string
	Type    string // matches information_schema.COLUMNS.COLUMN_TYPE, e.g. "varchar(255)"
	NotNull bool
}

type Index struct {
	Name    string
	Columns []string // ordered, left to right
	Unique  bool
}

type ForeignKey struct {
	Name      string
	Column    string
	RefTable  string
	RefColumn string
	OnDelete  string // CASCADE | SET NULL
}

type Table struct {
	Name        string
	Columns     []Column
	Indexes     []Index // includes PRIMARY and unique indexes
	ForeignKeys []ForeignKey
}

// Expected is the authoritative schema table list, in migration order.
var Expected = []*Table{
	T("users",
		cols(
			c("id", "bigint unsigned", true),
			c("name", "varchar(255)", true),
			c("email", "varchar(255)", true),
			c("role", "varchar(30)", true),
			c("email_verified_at", "timestamp", false),
			c("password", "varchar(255)", true),
			c("remember_token", "varchar(100)", false),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("users_email_unique", "email").unique(),
			idx("users_role_index", "role"),
		),
		fks(),
	),
	T("email_verification_otps",
		cols(
			c("id", "bigint unsigned", true),
			c("user_id", "bigint unsigned", true),
			c("type", "varchar(30)", true),
			c("otp_hash", "varchar(255)", true),
			c("expires_at", "timestamp", true),
			c("used_at", "timestamp", false),
			c("attempts", "tinyint unsigned", true),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("email_verification_otps_user_expires_index", "user_id", "expires_at"),
			idx("email_verification_otps_lookup_index", "user_id", "type", "used_at", "expires_at"),
		),
		fks(fk("fk_email_verification_otps_user_id", "user_id", "users", "id", "CASCADE")),
	),
	T("personal_access_tokens",
		cols(
			c("id", "bigint unsigned", true),
			c("tokenable_type", "varchar(255)", true),
			c("tokenable_id", "bigint unsigned", true),
			c("name", "text", true),
			c("token", "varchar(64)", true),
			c("abilities", "text", false),
			c("last_used_at", "timestamp", false),
			c("expires_at", "timestamp", false),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("personal_access_tokens_token_unique", "token").unique(),
			idx("personal_access_tokens_tokenable_index", "tokenable_type", "tokenable_id"),
			idx("personal_access_tokens_expires_at_index", "expires_at"),
		),
		fks(),
	),
	T("service_categories",
		cols(
			c("id", "bigint unsigned", true),
			c("name", "varchar(255)", true),
			c("slug", "varchar(255)", true),
			c("description", "text", false),
			c("icon", "varchar(255)", false),
			c("is_active", "tinyint(1)", true),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("service_categories_slug_unique", "slug").unique(),
		),
		fks(),
	),
	T("services",
		cols(
			c("id", "bigint unsigned", true),
			c("service_category_id", "bigint unsigned", true),
			c("name", "varchar(255)", true),
			c("slug", "varchar(255)", true),
			c("description", "text", false),
			c("price", "decimal(12,2)", true),
			c("unit", "varchar(255)", true),
			c("estimated_duration", "int", false),
			c("is_active", "tinyint(1)", true),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("services_slug_unique", "slug").unique(),
		),
		fks(fk("fk_services_service_category_id", "service_category_id", "service_categories", "id", "CASCADE")),
	),
	T("packages",
		cols(
			c("id", "bigint unsigned", true),
			c("name", "varchar(255)", true),
			c("slug", "varchar(255)", true),
			c("description", "text", false),
			c("price", "decimal(12,2)", true),
			c("duration", "int", false),
			c("is_active", "tinyint(1)", true),
			c("is_popular", "tinyint(1)", true),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("packages_slug_unique", "slug").unique(),
		),
		fks(),
	),
	T("package_items",
		cols(
			c("id", "bigint unsigned", true),
			c("package_id", "bigint unsigned", true),
			c("service_id", "bigint unsigned", true),
			c("quantity", "int", true),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("package_items_package_service_index", "package_id", "service_id"),
		),
		fks(
			fk("fk_package_items_package_id", "package_id", "packages", "id", "CASCADE"),
			fk("fk_package_items_service_id", "service_id", "services", "id", "CASCADE"),
		),
	),
	T("customer_profiles",
		cols(
			c("id", "bigint unsigned", true),
			c("user_id", "bigint unsigned", true),
			c("full_name", "varchar(255)", true),
			c("phone", "varchar(20)", true),
			c("address", "varchar(255)", true),
			c("city", "varchar(100)", true),
			c("postal_code", "varchar(10)", false),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("customer_profiles_user_id_unique", "user_id").unique(),
		),
		fks(fk("fk_customer_profiles_user_id", "user_id", "users", "id", "CASCADE")),
	),
	T("bookings",
		cols(
			c("id", "bigint unsigned", true),
			c("booking_code", "varchar(255)", true),
			c("customer_id", "bigint unsigned", true),
			c("booking_date", "date", true),
			c("booking_time", "varchar(255)", true),
			c("address", "varchar(255)", true),
			c("address_detail", "varchar(255)", false),
			c("latitude", "decimal(10,7)", false),
			c("longitude", "decimal(10,7)", false),
			c("customer_note", "text", false),
			c("additional_jobdesk", "text", false),
			c("subtotal", "decimal(12,2)", true),
			c("additional_cost", "decimal(12,2)", true),
			c("total_price", "decimal(12,2)", true),
			c("status", "varchar(255)", true),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("bookings_booking_code_unique", "booking_code").unique(),
			idx("bookings_status_index", "status"),
			idx("bookings_customer_created_index", "customer_id", "created_at"),
		),
		fks(fk("fk_bookings_customer_id", "customer_id", "users", "id", "CASCADE")),
	),
	T("booking_items",
		cols(
			c("id", "bigint unsigned", true),
			c("booking_id", "bigint unsigned", true),
			c("service_id", "bigint unsigned", false),
			c("package_id", "bigint unsigned", false),
			c("item_type", "varchar(20)", true),
			c("item_name", "varchar(255)", true),
			c("quantity", "int", true),
			c("unit_price", "decimal(12,2)", true),
			c("subtotal", "decimal(12,2)", true),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("booking_items_booking_type_index", "booking_id", "item_type"),
		),
		fks(
			fk("fk_booking_items_booking_id", "booking_id", "bookings", "id", "CASCADE"),
			fk("fk_booking_items_service_id", "service_id", "services", "id", "SET NULL"),
			fk("fk_booking_items_package_id", "package_id", "packages", "id", "SET NULL"),
		),
	),
	T("invoices",
		cols(
			c("id", "bigint unsigned", true),
			c("booking_id", "bigint unsigned", true),
			c("invoice_number", "varchar(255)", true),
			c("issued_at", "timestamp", true),
			c("due_at", "timestamp", false),
			c("subtotal", "decimal(12,2)", true),
			c("additional_cost", "decimal(12,2)", true),
			c("total_amount", "decimal(12,2)", true),
			c("status", "varchar(255)", true),
			c("notes", "text", false),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("invoices_booking_id_unique", "booking_id").unique(),
			idx("invoices_invoice_number_unique", "invoice_number").unique(),
			idx("invoices_status_index", "status"),
		),
		fks(fk("fk_invoices_booking_id", "booking_id", "bookings", "id", "CASCADE")),
	),
	T("payments",
		cols(
			c("id", "bigint unsigned", true),
			c("invoice_id", "bigint unsigned", true),
			c("payment_code", "varchar(255)", true),
			c("payment_method", "varchar(30)", true),
			c("amount", "decimal(12,2)", true),
			c("paid_at", "timestamp", false),
			c("status", "varchar(20)", true),
			c("proof_image", "varchar(255)", false),
			c("customer_note", "text", false),
			c("admin_note", "text", false),
			c("verified_by", "bigint unsigned", false),
			c("verified_at", "timestamp", false),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("payments_payment_code_unique", "payment_code").unique(),
			idx("payments_status_invoice_index", "status", "invoice_id"),
		),
		fks(
			fk("fk_payments_invoice_id", "invoice_id", "invoices", "id", "CASCADE"),
			fk("fk_payments_verified_by", "verified_by", "users", "id", "SET NULL"),
		),
	),
	T("technician_profiles",
		cols(
			c("id", "bigint unsigned", true),
			c("user_id", "bigint unsigned", true),
			c("technician_code", "varchar(255)", true),
			c("phone", "varchar(20)", false),
			c("specialization", "varchar(255)", false),
			c("address", "varchar(255)", false),
			c("bio", "text", false),
			c("is_active", "tinyint(1)", true),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("technician_profiles_user_id_unique", "user_id").unique(),
			idx("technician_profiles_technician_code_unique", "technician_code").unique(),
		),
		fks(fk("fk_technician_profiles_user_id", "user_id", "users", "id", "CASCADE")),
	),
	T("booking_assignments",
		cols(
			c("id", "bigint unsigned", true),
			c("booking_id", "bigint unsigned", true),
			c("technician_id", "bigint unsigned", true),
			c("assigned_by", "bigint unsigned", false),
			c("assigned_at", "timestamp", false),
			c("accepted_at", "timestamp", false),
			c("rejected_at", "timestamp", false),
			c("started_at", "timestamp", false),
			c("completed_at", "timestamp", false),
			c("status", "varchar(20)", true),
			c("rejection_reason", "text", false),
			c("technician_note", "text", false),
			c("admin_verification_note", "text", false),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("booking_assignments_status_technician_index", "status", "technician_id"),
			idx("booking_assignments_booking_id_index", "booking_id"),
		),
		fks(
			fk("fk_booking_assignments_booking_id", "booking_id", "bookings", "id", "CASCADE"),
			fk("fk_booking_assignments_technician_id", "technician_id", "users", "id", "CASCADE"),
			fk("fk_booking_assignments_assigned_by", "assigned_by", "users", "id", "SET NULL"),
		),
	),
	T("notifications",
		cols(
			c("id", "char(36)", true),
			c("type", "varchar(255)", true),
			c("notifiable_type", "varchar(255)", true),
			c("notifiable_id", "bigint unsigned", true),
			c("data", "text", true),
			c("read_at", "timestamp", false),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("notifications_notifiable_index", "notifiable_type", "notifiable_id"),
			idx("notifications_read_at_index", "read_at"),
			idx("notifications_notifiable_created_index", "notifiable_type", "notifiable_id", "created_at"),
		),
		fks(),
	),
	T("reviews",
		cols(
			c("id", "bigint unsigned", true),
			c("booking_id", "bigint unsigned", true),
			c("customer_id", "bigint unsigned", true),
			c("technician_id", "bigint unsigned", true),
			c("rating", "tinyint unsigned", true),
			c("comment", "text", false),
			c("status", "varchar(20)", true),
			c("created_at", "timestamp", false),
			c("updated_at", "timestamp", false),
		),
		indexes(
			idx("PRIMARY", "id"),
			idx("reviews_booking_id_unique", "booking_id").unique(),
			idx("reviews_status_technician_index", "status", "technician_id"),
		),
		fks(
			fk("fk_reviews_booking_id", "booking_id", "bookings", "id", "CASCADE"),
			fk("fk_reviews_customer_id", "customer_id", "users", "id", "CASCADE"),
			fk("fk_reviews_technician_id", "technician_id", "users", "id", "CASCADE"),
		),
	),
}

func c(name, typ string, notNull bool) Column  { return Column{Name: name, Type: typ, NotNull: notNull} }
func cols(list ...Column) []Column             { return list }
func idx(name string, columns ...string) Index { return Index{Name: name, Columns: columns} }
func (i Index) unique() Index                  { i.Unique = true; return i }
func indexes(list ...Index) []Index            { return list }
func fk(name, col, refTable, refColumn, onDelete string) ForeignKey {
	return ForeignKey{Name: name, Column: col, RefTable: refTable, RefColumn: refColumn, OnDelete: onDelete}
}
func fks(list ...ForeignKey) []ForeignKey { return list }

func T(name string, columns []Column, indexes []Index, foreignKeys []ForeignKey) *Table {
	return &Table{Name: name, Columns: columns, Indexes: indexes, ForeignKeys: foreignKeys}
}

// tableByName indexes Expected for lookups.
func tableByName(name string) *Table {
	for _, tb := range Expected {
		if tb.Name == name {
			return tb
		}
	}
	return nil
}

// findColumn returns an expected column by name.
func (tb *Table) findColumn(name string) *Column {
	for i := range tb.Columns {
		if tb.Columns[i].Name == name {
			return &tb.Columns[i]
		}
	}
	return nil
}

// UniqueCount returns the number of unique (non-PRIMARY) constraints.
func (tb *Table) UniqueCount() int {
	n := 0
	for _, ix := range tb.Indexes {
		if ix.Unique && ix.Name != "PRIMARY" {
			n++
		}
	}
	return n
}
