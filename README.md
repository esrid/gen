# gen

Code generator for hexagonal Go projects — supports **pgx** (PostgreSQL) and **database/sql** (SQLite).

Reads your existing code before touching anything. Never overwrites work outside generated sections.

## Usage

**Bundled binary** (preferred — each boilerplate ships with `./gen` at project root):
```bash
./gen scaffold Product name:string price:float
```

**Global install:**
```bash
cd /path/to/gen && go install .
gen scaffold Product name:string price:float
```

Run from your project root (where `go.mod` lives). Driver auto-detected from `go.mod` — `modernc.org/sqlite` or `mattn/go-sqlite3` → SQLite, otherwise pgx.

---

## Commands

### `gen scaffold <Model> [field:type ...]`

Creates all layers for a new model in one shot:

| File | What |
|------|------|
| `internal/core/domain/<model>.go` | struct (db + json tags) + sentinel errors |
| `internal/core/ports/<model>.go` | `<Model>Store` + `<Model>Service` interfaces |
| `internal/adapters/store/<model>_store.go` | CRUD store implementation |
| `internal/core/services/<model>_service.go` | service layer (delegates to store) |
| `internal/adapters/http/<model>_handler.go` | JSON REST handler (list/create/get/update/delete) |
| `internal/app/wire_<model>.go` | `<Model>Module` — fx dependency injection wiring |
| `internal/adapters/store/migrations/<N>_create_<model>.sql` | goose CREATE TABLE migration |

Auto-adds `id`, `created_at`, `updated_at` to every model. Skips any file that already exists — never overwrites.

```bash
gen scaffold Product name:string price:float active:bool
gen scaffold Post    title:string body:text user_id:ref:users
gen scaffold Country code:string{2} name:string
gen scaffold WordAssociation word_id:ref:words meaning:string weight:float
```

---

### `gen add <Model> [field:type ...]`

Adds one or more fields to an existing model.

- Parses the existing struct via AST — reads current fields
- Skips fields that already exist (safe to re-run)
- Inserts new fields into the struct
- Regenerates all store CRUD SQL inside `// gen:begin` / `// gen:end` markers
- Creates an `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` migration

```bash
gen add Product description:*string category:string
gen add Post    published:bool published_at:*time
```

---

### `gen remove <Model> <field> [field2 ...]`

Removes one or more fields from an existing model.

- Validates each field exists and is not auto-managed (`id`, `created_at`, `updated_at`)
- Removes the field(s) from the domain struct
- Regenerates store SQL without the dropped columns
- Creates a `DROP COLUMN` migration — `-- +goose Down` includes `ADD COLUMN` for rollback

```bash
gen remove Product category active
gen remove Post    draft_body
```

---

### `gen destroy <Model> [--force]`

Removes all generated Go files for a model.

- Shows a plan and asks `y/N` before deleting (skip with `--force` / `-f`)
- Skips any file that contains user code **outside** `// gen:begin/end` markers
- Never deletes migrations — prints a reminder to run `goose down` manually

```bash
gen destroy Product           # shows plan, asks y/N
gen destroy Product --force   # no confirmation
```

---

## Field types

| Syntax | Go type | SQL |
|--------|---------|-----|
| `name:string` | `string` | `TEXT NOT NULL DEFAULT ''` |
| `name:*string` | `*string` | `TEXT` |
| `name:int` | `int64` | `BIGINT NOT NULL DEFAULT 0` |
| `name:*int` | `*int64` | `BIGINT` |
| `name:float` | `float64` | `DOUBLE PRECISION NOT NULL DEFAULT 0` |
| `name:*float` | `*float64` | `DOUBLE PRECISION` |
| `name:bool` | `bool` | `BOOLEAN NOT NULL DEFAULT FALSE` |
| `name:*bool` | `*bool` | `BOOLEAN` |
| `name:time` | `time.Time` | `TIMESTAMPTZ NOT NULL DEFAULT NOW()` |
| `name:*time` | `*time.Time` | `TIMESTAMPTZ` |
| `name:json` | `[]byte` | `JSONB NOT NULL DEFAULT '{}'` |
| `name:string{n}` | `string` | `VARCHAR(n) NOT NULL DEFAULT ''` |
| `name:ref:<table>` | `string` | `TEXT NOT NULL REFERENCES <table>(id) ON DELETE CASCADE` |
| `name:*ref:<table>` | `*string` | `TEXT REFERENCES <table>(id) ON DELETE SET NULL` |

#### `ref:` fields also generate query methods

Every `ref:` field on a model produces a `List<Plural>By<GoName>(ctx, id string, limit, offset int)` method in three places, all inside `// gen:begin/end` markers:

| Layer | What is generated |
|-------|-------------------|
| `internal/core/ports/<model>.go` | added to `<Model>Store` and `<Model>Service` interfaces |
| `internal/core/services/<model>_service.go` | delegates to the store |
| `internal/adapters/store/<model>_store.go` | executes `SELECT … WHERE <field> = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3` |

```bash
gen scaffold Order restaurant_id:ref:restaurants item_id:ref:items amount:float
# generates:
#   ListOrdersByRestaurantID(ctx, restaurantID string, limit, offset int) ([]domain.Order, error)
#   ListOrdersByItemID(ctx, itemID string, limit, offset int)        ([]domain.Order, error)
```

**Important — `gen add` vs `gen scaffold`:**
`gen add` only regenerates the store section (inside markers). It does **not** touch the port or service files. If you add a `ref:` field via `gen add`, the store gets the new method automatically, but you must manually add the method signature to the `<Model>Store` / `<Model>Service` interfaces inside their `// gen:begin/end` block in the port file, and the implementation in the service file.

Prefix any type with `*` to make it nullable — removes `NOT NULL` and `DEFAULT` from SQL.

Go naming follows standard initialisms automatically: `user_id` → `UserID`, `content_url` → `ContentURL`, `api_key` → `APIKey`.

---

## How it protects your work

Every generated block is wrapped with markers:

```go
// gen:begin Product
var ErrProductNotFound = errors.New("product not found")
var ErrProductConflict = errors.New("product conflict")

type Product struct {
    ID        string    `db:"id"          json:"id"`
    Name      string    `db:"name"        json:"name"`
    CreatedAt time.Time `db:"created_at"  json:"created_at"`
    UpdatedAt time.Time `db:"updated_at"  json:"updated_at"`
}
// gen:end Product
```

- `gen add` and `gen remove` only regenerate code **inside** markers
- Anything you write **outside** markers is never touched
- `gen destroy` refuses to delete files with user code outside markers
- Migrations are always append-only — no command ever deletes them

---

## Requirements

Must be run from inside a project with this structure:

```
internal/core/domain/
internal/core/ports/
internal/adapters/store/
```

**Driver auto-detection** (reads `go.mod`):
- `modernc.org/sqlite` or `mattn/go-sqlite3` → SQLite store (`database/sql`, `?` placeholders, `INTEGER`/`DATETIME` types)
- anything else → pgx store (`pgxpool`, `$N` placeholders, `UUID`/`TIMESTAMPTZ` types)

Matches the `boilerplate-pg` and `boilerplate-sql` layouts.

---

## Full workflow example

```bash
# 1. New model (client_id:ref:clients generates ListInvoicesByClientID everywhere)
gen scaffold Invoice number:string{20} amount:float paid:bool client_id:ref:clients

# 2. Wire up (2 manual lines)
#    app.go:    add InvoiceModule to fx.New()
#    router.go: add invoiceH *InvoiceHandler param + r.Mount("/api/invoices", invoiceH.Routes())

# 3. Run migrations
make migrate

# 4. Add a field later
gen add Invoice notes:*string due_date:*time
make migrate

# 5. Change your mind on a field
gen remove Invoice notes
make migrate

# 6. Scrap the whole model
gen destroy Invoice --force
# DB rollback:
goose down -dir ./internal/adapters/store/migrations
```

After `scaffold`, two things need manual wiring — `gen` cannot do them because they live in user-owned files with no markers:

**1. `internal/app/app.go` — add the module to `fx.New()`**
```go
fx.New(
    // ... existing modules
    InvoiceModule,   // ← add (generated in internal/app/wire_invoice.go)
)
```

`InvoiceModule` is an `fx.Options` block that wires the store interface adapter, service interface adapter, service constructor, and handler constructor — all provided to the fx container automatically.

**2. `internal/adapters/http/router.go` — add the handler param and mount the route**
```go
func NewRouter(
    authSvc  ports.AuthService,
    invoiceH *InvoiceHandler,   // ← add (fx injects from InvoiceModule)
    logger   *slog.Logger,
) http.Handler {
    ...
    r.Mount("/api/invoices", invoiceH.Routes())   // ← mount
    ...
}
```

The URL prefix, middleware group (public vs auth-required), and role guards are decisions only you can make — that's why `gen` stops here.
