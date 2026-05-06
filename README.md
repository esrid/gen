# gen

Code generator for hexagonal Go/pgx projects.

Reads your existing code before touching anything. Never overwrites work outside generated sections.

## Install

```bash
cd /path/to/gen
go install .
```

Run from your project root (where `go.mod` lives). Only works inside projects with the expected hexagonal structure.

---

## Commands

### `gen scaffold <Model> [field:type ...]`

Creates all layers for a new model in one shot:

| File | Path |
|------|------|
| Domain struct + error var | `internal/core/domain/<model>.go` |
| Store + Service interfaces | `internal/core/ports/<model>.go` |
| Store CRUD methods | `internal/adapters/store/<model>_store.go` |
| Service (delegates to store) | `internal/core/services/<model>_service.go` |
| HTTP handler (JSON REST) | `internal/adapters/http/<model>_handler.go` |
| CREATE TABLE migration | `internal/adapters/store/migrations/<N>_create_<model>.sql` |

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

Prefix any type with `*` to make it nullable — removes `NOT NULL` and `DEFAULT` from SQL.

Go naming follows standard initialisms automatically: `user_id` → `UserID`, `content_url` → `ContentURL`, `api_key` → `APIKey`.

---

## How it protects your work

Every generated block is wrapped with markers:

```go
// gen:begin Product
var ErrProductNotFound = errors.New("product not found")

type Product struct {
    ID        string    `db:"id"`
    Name      string    `db:"name"`
    CreatedAt time.Time `db:"created_at"`
    UpdatedAt time.Time `db:"updated_at"`
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

Targets **pgx/v5** + **goose** migrations — matches the `boilerplate-pg` layout.

---

## Full workflow example

```bash
# 1. New model
gen scaffold Invoice number:string{20} amount:float paid:bool client_id:ref:clients

# 2. Wire up in your App struct, router, etc.

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

After `scaffold`, three things need manual wiring — `gen` cannot do them because they live in user-owned files with no markers:

**1. `internal/adapters/store/store.go` or wherever your store is composed**
```go
// nothing to add — Store methods are on the existing *Store type automatically
```

**2. `internal/app/app.go` — inject the service**
```go
invoiceSvc := services.NewInvoiceService(store, logger)
```

**3. `internal/adapters/http/router.go` — add the param and mount the route**
```go
func NewRouter(
    authSvc ports.AuthService,
    invoiceSvc ports.InvoiceService,   // ← add
    logger *slog.Logger,
) http.Handler {
    ...
    r.Mount("/api/invoices", NewInvoiceHandler(invoiceSvc, logger).Routes())  // ← mount
    ...
}
```

The URL prefix, middleware group (public vs auth-required), and whether to use `r.Mount` or `r.Group` are decisions only you can make — that's why `gen` stops here.
