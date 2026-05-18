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

> **Shell quoting** — arguments containing `{}` or `[]` must be single-quoted in zsh/bash to prevent glob expansion:
> ```bash
> ./gen scaffold Product 'slug:string{50}[unique,index]'   # correct
> ./gen scaffold Product slug:string{50}[unique,index]      # zsh error: no matches found
> ```

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
gen scaffold Product 'name:string[unique]' price:float active:bool
gen scaffold Post    title:string body:text user_id:ref:users
gen scaffold Country 'code:string{2}[unique]' name:string
gen scaffold Order   amount:float 'ref_num:string{20}[unique,index]' client_id:ref:clients
```

---

### `gen add <Model> [field:type ...]`

Adds one or more fields to an existing model.

- Parses the existing struct via AST — reads current fields
- Skips fields that already exist (safe to re-run)
- **Preserves all non-gen struct tags** (`validate:`, custom tags, etc.) across rewrites
- Inserts new fields into the struct
- Regenerates all store CRUD SQL inside `// gen:begin` / `// gen:end` markers
- Creates an `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` migration

```bash
gen add Product description:*string category:string
gen add Post    'published:bool' 'published_at:*time' 'slug:string{200}[unique,index]'
```

---

### `gen remove <Model> <field> [field2 ...]`

Removes one or more fields from an existing model.

- Validates each field exists and is not auto-managed (`id`, `created_at`, `updated_at`)
- **Preserves all non-gen struct tags** on remaining fields
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

| Syntax | Go type | SQL (pgx) | SQL (SQLite) |
|--------|---------|-----------|--------------|
| `name:string` | `string` | `TEXT NOT NULL DEFAULT ''` | same |
| `name:*string` | `*string` | `TEXT` | same |
| `name:int` | `int64` | `BIGINT NOT NULL DEFAULT 0` | `INTEGER NOT NULL DEFAULT 0` |
| `name:*int` | `*int64` | `BIGINT` | `INTEGER` |
| `name:float` | `float64` | `DOUBLE PRECISION NOT NULL DEFAULT 0` | `REAL NOT NULL DEFAULT 0` |
| `name:*float` | `*float64` | `DOUBLE PRECISION` | `REAL` |
| `name:bool` | `bool` | `BOOLEAN NOT NULL DEFAULT FALSE` | `INTEGER NOT NULL DEFAULT 0` |
| `name:*bool` | `*bool` | `BOOLEAN` | `INTEGER` |
| `name:time` | `time.Time` | `TIMESTAMPTZ NOT NULL DEFAULT NOW()` | `DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP` |
| `name:*time` | `*time.Time` | `TIMESTAMPTZ` | `DATETIME` |
| `name:json` | `[]byte` | `JSONB NOT NULL DEFAULT '{}'` | `TEXT NOT NULL DEFAULT '{}'` |
| `name:string{n}` | `string` | `VARCHAR(n) NOT NULL DEFAULT ''` | same |
| `name:*string{n}` | `*string` | `VARCHAR(n)` | same |
| `name:ref:<table>` | `string` | `TEXT NOT NULL REFERENCES <table>(id) ON DELETE CASCADE` | same |
| `name:*ref:<table>` | `*string` | `TEXT REFERENCES <table>(id) ON DELETE SET NULL` | same |

Prefix any type with `*` to make it nullable — removes `NOT NULL` and `DEFAULT` from SQL.

Go naming follows standard initialisms automatically: `user_id` → `UserID`, `content_url` → `ContentURL`, `api_key` → `APIKey`.

---

## Modifiers

Append `[modifier,modifier]` after the type to add SQL constraints or indexes.

```bash
gen scaffold Product 'email:string[unique]' 'slug:string{100}[unique,index]' 'rank:int[index]'
```

| Modifier | Effect on column SQL | Effect on migration |
|----------|---------------------|---------------------|
| `unique` | appends `UNIQUE` to the column definition | — (UNIQUE implicitly creates a unique index in both engines) |
| `index` | — | emits a separate `CREATE INDEX` statement after the table |

**Both modifiers work on all field types and both drivers.** They compose freely:

```bash
'code:string{6}[unique]'          # VARCHAR(6) NOT NULL DEFAULT '' UNIQUE
'slug:string[unique,index]'        # TEXT NOT NULL DEFAULT '' UNIQUE  +  CREATE INDEX
'score:int[index]'                 # BIGINT NOT NULL DEFAULT 0        +  CREATE INDEX
'owner_id:ref:users[unique]'       # TEXT NOT NULL REFERENCES …       UNIQUE
```

**pgx index naming** — unnamed (`CREATE INDEX ON table (col)`) — Postgres auto-names them.  
**SQLite index naming** — named (`CREATE INDEX table_col_idx ON table (col)`).

---

## `ref:` fields generate query methods

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

**Note on `gen add` with `ref:` fields:** `gen add` only regenerates the store section (inside markers). If you add a `ref:` field via `gen add`, the store gets the new `ListBy` method automatically, but you must manually add the method signature to the port interfaces and service file inside their `// gen:begin/end` blocks.

---

## How it protects your work

Every generated block is wrapped with markers:

```go
// gen:begin Product
var ErrProductNotFound = errors.New("product not found")
var ErrProductConflict = errors.New("product conflict")

type Product struct {
    ID        string    `db:"id"          json:"id"`
    Name      string    `db:"name"        json:"name" validate:"required,min=2"`
    CreatedAt time.Time `db:"created_at"  json:"created_at"`
    UpdatedAt time.Time `db:"updated_at"  json:"updated_at"`
}
// gen:end Product
```

- `gen add` and `gen remove` only regenerate code **inside** markers
- Anything you write **outside** markers is never touched
- **Non-gen struct tags** (`validate:`, `form:`, custom tags) are read from the existing struct and re-emitted verbatim — adding a field or removing one never strips tags from untouched fields
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
# 1. New model
gen scaffold Invoice 'number:string{20}[unique]' amount:float paid:bool client_id:ref:clients

# 2. Wire up — one line in internal/app/app.go:
#    add InvoiceModule to fx.New()

# 3. Run migrations
make migrate

# 4. Add fields later — existing validate: tags on other fields are preserved
gen add Invoice notes:*string 'due_date:*time' 'priority:int[index]'
make migrate

# 5. Change your mind
gen remove Invoice notes
make migrate

# 6. Scrap the whole model
gen destroy Invoice --force
# DB rollback:
goose down -dir ./internal/adapters/store/migrations
```

After `scaffold`, one thing needs manual wiring — `gen` cannot do it because it lives in a user-owned file with no markers:

**`internal/app/app.go` — add the module to `fx.New()`**
```go
fx.New(
    // ... existing modules
    InvoiceModule,   // ← add (generated in internal/app/wire_invoice.go)
)
```

`InvoiceModule` is an `fx.Module` that wires the store interface adapter, service, and handler — all provided to the fx container automatically. Routes self-register via `group:"routes"`.
