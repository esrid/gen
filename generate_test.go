package main

import (
	"strings"
	"testing"
)

// extractFn pulls the source of a single generated function out of a larger
// generated string so assertions can be scoped to one method at a time.
func extractFn(src, name string) string {
	marker := "func (s *Store) " + name
	i := strings.Index(src, marker)
	if i == -1 {
		return ""
	}
	rest := src[i:]
	j := strings.Index(rest[1:], "\nfunc ")
	if j == -1 {
		return rest
	}
	return rest[:j+1]
}

// ── helpers ───────────────────────────────────────────────────────────────────

func TestToSnakeCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"FullName", "full_name"},
		{"fullName", "full_name"},
		{"full_name", "full_name"},
		{"ID", "id"},
		{"UserID", "user_id"},
		{"A", "a"},
	}
	for _, c := range cases {
		if got := toSnakeCase(c.in); got != c.want {
			t.Errorf("toSnakeCase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	t.Run("consecutive uppercase not split", func(t *testing.T) {
		t.Skip("known limitation: HTTPSUrl → httpsurl, not https_url")
	})
}

func TestToPascalCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"full_name", "FullName"},
		{"user_id", "UserID"},
		{"language_id", "LanguageID"},
		{"api_key", "APIKey"},
		{"url", "URL"},
		{"Product", "Product"},
	}
	for _, c := range cases {
		if got := toPascalCase(c.in); got != c.want {
			t.Errorf("toPascalCase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestToPlural(t *testing.T) {
	cases := []struct{ in, want string }{
		{"product", "products"},
		{"category", "categories"},
		{"box", "boxes"},
		{"status", "status"},
		{"dish", "dishes"},
		{"buzz", "buzzes"},
		{"key", "keys"},
		{"products", "products"},
	}
	for _, c := range cases {
		if got := toPlural(c.in); got != c.want {
			t.Errorf("toPlural(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTableOf(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Product", "products"},
		{"WordAssociation", "word_associations"},
		{"Category", "categories"},
		{"OrderItem", "order_items"},
	}
	for _, c := range cases {
		if got := tableOf(c.in); got != c.want {
			t.Errorf("tableOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── parseField ────────────────────────────────────────────────────────────────

func TestParseField(t *testing.T) {
	cases := []struct {
		raw     string
		want    Field
		wantErr bool
	}{
		{
			raw:  "name:string",
			want: Field{GoName: "Name", DBName: "name", GoType: "string", SQLType: "TEXT NOT NULL DEFAULT ''"},
		},
		{
			raw:  "price:float",
			want: Field{GoName: "Price", DBName: "price", GoType: "float64", SQLType: "DOUBLE PRECISION NOT NULL DEFAULT 0"},
		},
		{
			raw:  "count:*int",
			want: Field{GoName: "Count", DBName: "count", GoType: "*int64", SQLType: "BIGINT"},
		},
		{
			raw:  "active:bool",
			want: Field{GoName: "Active", DBName: "active", GoType: "bool", SQLType: "BOOLEAN NOT NULL DEFAULT FALSE"},
		},
		{
			raw:  "slug:string{100}",
			want: Field{GoName: "Slug", DBName: "slug", GoType: "string", SQLType: "VARCHAR(100) NOT NULL DEFAULT ''"},
		},
		{
			raw:  "user_id:ref:users",
			want: Field{GoName: "UserID", DBName: "user_id", GoType: "string", SQLType: "TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE", RefTable: "users"},
		},
		{
			raw:  "owner:*ref:accounts",
			want: Field{GoName: "Owner", DBName: "owner", GoType: "*string", SQLType: "TEXT REFERENCES accounts(id) ON DELETE SET NULL", RefTable: "accounts"},
		},
		{raw: "nocodon", wantErr: true},
		{raw: ":string", wantErr: true},
		{raw: "name:unknown", wantErr: true},
		{raw: "name:ref:", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			f, err := parseField(c.raw)
			if c.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if f != c.want {
				t.Errorf("\n  got  %+v\n  want %+v", f, c.want)
			}
		})
	}
}

// ── SQL builder helpers ───────────────────────────────────────────────────────

var sampleFields = []Field{
	{GoName: "Name", DBName: "name", GoType: "string", SQLType: "TEXT NOT NULL DEFAULT ''"},
	{GoName: "Price", DBName: "price", GoType: "float64", SQLType: "DOUBLE PRECISION NOT NULL DEFAULT 0"},
}

func TestBuildSelectCols(t *testing.T) {
	got := buildSelectCols(sampleFields)
	want := "id, name, price, created_at, updated_at"
	if got != want {
		t.Errorf("buildSelectCols = %q, want %q", got, want)
	}
}

func TestBuildSelectCols_NoFields(t *testing.T) {
	got := buildSelectCols(nil)
	want := "id, created_at, updated_at"
	if got != want {
		t.Errorf("buildSelectCols(nil) = %q, want %q", got, want)
	}
}

func TestColList(t *testing.T) {
	got := colList(sampleFields)
	if got != "name, price" {
		t.Errorf("colList = %q", got)
	}
}

func TestPlaceholderList(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "$1"},
		{3, "$1, $2, $3"},
		{0, ""},
	}
	for _, c := range cases {
		if got := placeholderList(c.n); got != c.want {
			t.Errorf("placeholderList(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestUpdateSetList(t *testing.T) {
	got := updateSetList(sampleFields)
	if got != "name = $1, price = $2" {
		t.Errorf("updateSetList = %q", got)
	}
}

func TestPgToSQLiteType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"TEXT NOT NULL DEFAULT ''", "TEXT NOT NULL DEFAULT ''"},
		{"BIGINT NOT NULL DEFAULT 0", "INTEGER NOT NULL DEFAULT 0"},
		{"DOUBLE PRECISION NOT NULL DEFAULT 0", "REAL NOT NULL DEFAULT 0"},
		{"BOOLEAN NOT NULL DEFAULT FALSE", "INTEGER NOT NULL DEFAULT 0"},
		{"TIMESTAMPTZ NOT NULL DEFAULT NOW()", "DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP"},
		{"JSONB NOT NULL DEFAULT '{}'", "TEXT NOT NULL DEFAULT '{}'"},
		{"BIGINT", "INTEGER"},
		{"TIMESTAMPTZ", "DATETIME"},
	}
	for _, c := range cases {
		if got := pgToSQLiteType(c.in); got != c.want {
			t.Errorf("pgToSQLiteType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── genCreateMigration ────────────────────────────────────────────────────────

func TestGenCreateMigration_Pgx(t *testing.T) {
	got := genCreateMigration("Product", sampleFields, "pgx")

	must := []string{
		"-- +goose Up",
		"-- +goose Down",
		"CREATE TABLE IF NOT EXISTS products",
		"UUID        PRIMARY KEY DEFAULT uuidv7()",
		// full column lines — avoids bare "name" matching unrelated context
		"name           TEXT NOT NULL DEFAULT ''",
		"price          DOUBLE PRECISION NOT NULL DEFAULT 0",
		"TIMESTAMPTZ NOT NULL DEFAULT NOW()",
		"CREATE INDEX ON products (created_at DESC)",
		"DROP TABLE IF EXISTS products",
	}
	for _, s := range must {
		if !strings.Contains(got, s) {
			t.Errorf("pgx migration missing %q\n---\n%s", s, got)
		}
	}
}

func TestGenCreateMigration_SQLite(t *testing.T) {
	got := genCreateMigration("Product", sampleFields, "sqlite")

	must := []string{
		"CREATE TABLE IF NOT EXISTS products",
		"TEXT        PRIMARY KEY",
		"DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP",
		"CREATE INDEX products_created_at_idx ON products (created_at DESC)",
		"DROP TABLE IF EXISTS products",
	}
	for _, s := range must {
		if !strings.Contains(got, s) {
			t.Errorf("sqlite migration missing %q\n---\n%s", s, got)
		}
	}
	for _, bad := range []string{"UUID", "uuidv7()", "TIMESTAMPTZ", "NOW()"} {
		if strings.Contains(got, bad) {
			t.Errorf("sqlite migration contains Postgres syntax %q", bad)
		}
	}
}

func TestGenCreateMigration_NoFields(t *testing.T) {
	got := genCreateMigration("Event", nil, "pgx")
	if !strings.Contains(got, "UUID        PRIMARY KEY DEFAULT uuidv7()") {
		t.Errorf("no-field migration missing UUID PK\n%s", got)
	}
	if strings.Contains(got, "RETURNING") {
		t.Errorf("CREATE TABLE should not contain RETURNING")
	}
}

// ── genDomainSection ──────────────────────────────────────────────────────────

func TestGenDomainSection(t *testing.T) {
	got := genDomainSection("Product", sampleFields)

	must := []string{
		"ErrProductNotFound",
		"ErrProductConflict",
		"errors.New(",
		"type Product struct",
		`db:"id"`,
		`json:"id"`,
		"Name",
		`db:"name"`,
		`json:"name"`,
		"Price",
		`db:"price"`,
		`json:"price"`,
		"CreatedAt",
		`json:"created_at"`,
		"UpdatedAt",
		`json:"updated_at"`,
	}
	for _, s := range must {
		if !strings.Contains(got, s) {
			t.Errorf("domain section missing %q\n---\n%s", s, got)
		}
	}
}

func TestGenDomainSection_NoFields(t *testing.T) {
	got := genDomainSection("Event", nil)
	if !strings.Contains(got, "ErrEventNotFound") {
		t.Errorf("missing ErrEventNotFound\n%s", got)
	}
	if !strings.Contains(got, "ErrEventConflict") {
		t.Errorf("missing ErrEventConflict\n%s", got)
	}
}

// ── genPortSection ────────────────────────────────────────────────────────────

func TestGenPortSection(t *testing.T) {
	got := genPortSection("Product", nil)

	must := []string{
		"type ProductStore interface",
		"type ProductService interface",
		"CreateProduct(ctx context.Context, p domain.Product) (*domain.Product, error)",
		"ProductByID(ctx context.Context, id string) (*domain.Product, error)",
		"UpdateProduct(ctx context.Context, p domain.Product) (*domain.Product, error)",
		"DeleteProduct(ctx context.Context, id string) error",
		"ListProducts(ctx context.Context, limit, offset int) ([]domain.Product, error)",
		"GetProduct(ctx context.Context, id string) (*domain.Product, error)",
	}
	for _, s := range must {
		if !strings.Contains(got, s) {
			t.Errorf("port section missing %q\n---\n%s", s, got)
		}
	}
}

// ── genServiceSection ─────────────────────────────────────────────────────────

func TestGenServiceSection(t *testing.T) {
	got := genServiceSection("mymodule", "Product", nil)

	must := []string{
		"func (s *ProductService) UpdateProduct(ctx context.Context, p domain.Product) (*domain.Product, error)",
		"func (s *ProductService) ListProducts(ctx context.Context, limit, offset int) ([]domain.Product, error)",
		"return s.store.UpdateProduct(ctx, p)",
		"return s.store.ListProducts(ctx, limit, offset)",
	}
	for _, s := range must {
		if !strings.Contains(got, s) {
			t.Errorf("service section missing %q\n---\n%s", s, got)
		}
	}
}

// ── genStoreSectionPgx ────────────────────────────────────────────────────────

func TestGenStoreSectionPgx(t *testing.T) {
	got := genStoreSectionPgx("Product", sampleFields)

	must := []string{
		// Create: RETURNING and conflict mapping
		"INSERT INTO products",
		"RETURNING id, name, price, created_at, updated_at",
		`pgErr.Code == "23505"`,
		"ErrProductConflict",
		// ByID: ErrNoRows → ErrNotFound
		"ProductByID",
		"WHERE id = $1",
		"pgx.ErrNoRows",
		"ErrProductNotFound",
		// Update: RETURNING (single round trip)
		"UPDATE products SET",
		"RETURNING id",
		// Delete: RowsAffected → ErrNotFound
		"DELETE FROM products WHERE id = $1",
		"tag.RowsAffected() == 0",
		// List: LIMIT/OFFSET
		"ORDER BY created_at DESC LIMIT $1 OFFSET $2",
	}
	for _, s := range must {
		if !strings.Contains(got, s) {
			t.Errorf("pgx store missing %q\n---\n%s", s, got)
		}
	}
}

func TestGenStoreSectionPgx_Update(t *testing.T) {
	got := extractFn(genStoreSectionPgx("Product", sampleFields), "UpdateProduct")
	if got == "" {
		t.Fatal("UpdateProduct function not found in generated output")
	}
	for _, s := range []string{
		"RETURNING id, name, price, created_at, updated_at",
		"pgx.ErrNoRows",
		"ErrProductNotFound",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("UpdateProduct missing %q\n---\n%s", s, got)
		}
	}
}

func TestGenStoreSectionPgx_Delete(t *testing.T) {
	got := extractFn(genStoreSectionPgx("Product", sampleFields), "DeleteProduct")
	if got == "" {
		t.Fatal("DeleteProduct function not found in generated output")
	}
	for _, s := range []string{
		"tag.RowsAffected() == 0",
		"ErrProductNotFound",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("DeleteProduct missing %q\n---\n%s", s, got)
		}
	}
}

func TestGenStoreSectionPgx_NoFields(t *testing.T) {
	got := genStoreSectionPgx("Event", nil)
	if !strings.Contains(got, "DEFAULT VALUES RETURNING") {
		t.Errorf("no-field pgx Create should use DEFAULT VALUES RETURNING\n%s", got)
	}
	if !strings.Contains(got, "UPDATE events SET updated_at = NOW() WHERE id = $1 RETURNING") {
		t.Errorf("no-field pgx Update missing RETURNING\n%s", got)
	}
}

// ── genStoreSectionSQLite ─────────────────────────────────────────────────────

func TestGenStoreSectionSQLite(t *testing.T) {
	got := genStoreSectionSQLite("Product", sampleFields)

	must := []string{
		// Create: UNIQUE constraint mapping
		"INSERT INTO products",
		`"UNIQUE constraint failed"`,
		"ErrProductConflict",
		// ByID: sql.ErrNoRows → ErrNotFound
		"ProductByID",
		"WHERE id = ?",
		"sql.ErrNoRows",
		"ErrProductNotFound",
		// List: LIMIT/OFFSET
		"ORDER BY created_at DESC LIMIT ? OFFSET ?",
	}
	for _, s := range must {
		if !strings.Contains(got, s) {
			t.Errorf("sqlite store missing %q\n---\n%s", s, got)
		}
	}
}

func TestGenStoreSectionSQLite_Update(t *testing.T) {
	got := extractFn(genStoreSectionSQLite("Product", sampleFields), "UpdateProduct")
	if got == "" {
		t.Fatal("UpdateProduct function not found in generated output")
	}
	for _, s := range []string{
		"err2 := res.RowsAffected()",
		"ErrProductNotFound",
		"ProductByID(ctx, p.ID)",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("SQLite UpdateProduct missing %q\n---\n%s", s, got)
		}
	}
}

func TestGenStoreSectionSQLite_Delete(t *testing.T) {
	got := extractFn(genStoreSectionSQLite("Product", sampleFields), "DeleteProduct")
	if got == "" {
		t.Fatal("DeleteProduct function not found in generated output")
	}
	for _, s := range []string{
		"err2 := res.RowsAffected()",
		"ErrProductNotFound",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("SQLite DeleteProduct missing %q\n---\n%s", s, got)
		}
	}
	// Delete must NOT call ByID — that's Update's job
	if strings.Contains(got, "ByID") {
		t.Errorf("SQLite DeleteProduct should not call ByID")
	}
}

// ── genStoreFile imports ──────────────────────────────────────────────────────

func TestGenStoreFile_PgxImports(t *testing.T) {
	got := genStoreFile("mymodule", "Product", sampleFields, "pgx")
	for _, imp := range []string{
		`"github.com/jackc/pgx/v5"`,
		`"github.com/jackc/pgx/v5/pgconn"`,
		`"errors"`,
	} {
		if !strings.Contains(got, imp) {
			t.Errorf("pgx store file missing import %s\n%s", imp, got)
		}
	}
}

func TestGenStoreFile_SQLiteImports(t *testing.T) {
	got := genStoreFile("mymodule", "Product", sampleFields, "sqlite")
	for _, imp := range []string{
		`"database/sql"`,
		`"errors"`,
		`"strings"`,
	} {
		if !strings.Contains(got, imp) {
			t.Errorf("sqlite store file missing import %s\n%s", imp, got)
		}
	}
}

// ── genHandlerSection ─────────────────────────────────────────────────────────

func TestGenHandlerSection(t *testing.T) {
	got := genHandlerSection("mymodule", "Product")

	must := []string{
		// routes
		`r.Get("/", h.handleList)`,
		`r.Post("/", h.handleCreate)`,
		`r.Get("/{id}", h.handleGet)`,
		`r.Put("/{id}", h.handleUpdate)`,
		`r.Delete("/{id}", h.handleDelete)`,
		// handleList: pagination + 400 on bad params
		"limit, offset := 100, 0",
		`"invalid limit"`,
		`"invalid offset"`,
		"StatusBadRequest",
		"ListProducts(r.Context(), limit, offset)",
		// handleCreate: conflict → 409, mask others
		"ErrProductConflict",
		"StatusConflict",
		// handleGet: ErrNotFound → 404, others → 500
		"ErrProductNotFound",
		"StatusNotFound",
		"StatusInternalServerError",
		// handleUpdate: uses returned value, ErrNotFound → 404
		"UpdateProduct(r.Context(),",
		"ErrProductNotFound",
		// handleDelete: ErrNotFound → 404
		"DeleteProduct(r.Context(), id)",
		"StatusNoContent",
	}
	for _, s := range must {
		if !strings.Contains(got, s) {
			t.Errorf("handler section missing %q\n---\n%s", s, got)
		}
	}

	// must NOT leak raw errors to clients
	for _, bad := range []string{
		"err.Error(), http.StatusInternalServerError",
		"err.Error(), http.StatusBadRequest",
	} {
		if strings.Contains(got, bad) {
			t.Errorf("handler leaks raw error: found %q", bad)
		}
	}
}

// ── genWireSection ────────────────────────────────────────────────────────────

func TestGenWireSection(t *testing.T) {
	got := genWireSection("Product")

	must := []string{
		"var ProductModule = fx.Options(",
		"fx.Provide(",
		"ports.ProductStore",
		"ports.ProductService",
		"services.NewProductService",
		"httpAdapter.NewProductHandler",
	}
	for _, s := range must {
		if !strings.Contains(got, s) {
			t.Errorf("wire section missing %q\n---\n%s", s, got)
		}
	}
}

func TestGenWireFile(t *testing.T) {
	got := genWireFile("mymodule", "Product")

	must := []string{
		"package app",
		`httpAdapter "mymodule/internal/adapters/http"`,
		`"mymodule/internal/adapters/store"`,
		`"mymodule/internal/core/ports"`,
		`"mymodule/internal/core/services"`,
		`"go.uber.org/fx"`,
		"var ProductModule",
	}
	for _, s := range must {
		if !strings.Contains(got, s) {
			t.Errorf("wire file missing %q\n---\n%s", s, got)
		}
	}
}

// ── genAlterMigration ─────────────────────────────────────────────────────────

func TestGenAlterMigration_Pgx(t *testing.T) {
	fields := []Field{
		{DBName: "slug", SQLType: "TEXT NOT NULL DEFAULT ''"},
	}
	got := genAlterMigration("Product", fields, "pgx")
	must := []string{
		"-- +goose Up",
		"ALTER TABLE products ADD COLUMN IF NOT EXISTS",
		"slug",
		"-- +goose Down",
		"DROP COLUMN IF EXISTS slug",
	}
	for _, s := range must {
		if !strings.Contains(got, s) {
			t.Errorf("pgx alter migration missing %q\n---\n%s", s, got)
		}
	}
}

func TestGenAlterMigration_SQLite(t *testing.T) {
	fields := []Field{
		{DBName: "slug", SQLType: "TEXT NOT NULL DEFAULT ''"},
	}
	got := genAlterMigration("Product", fields, "sqlite")
	if strings.Contains(got, "IF NOT EXISTS") {
		t.Errorf("SQLite ALTER TABLE should not use IF NOT EXISTS")
	}
	if !strings.Contains(got, "ALTER TABLE products ADD COLUMN") {
		t.Errorf("missing ALTER TABLE ADD COLUMN\n%s", got)
	}
}
