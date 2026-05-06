# gen

Code generator for hexagonal Go/pgx projects.

Reads your existing code before touching anything. Never overwrites work outside generated sections.

## Install

```bash
cd /path/to/gen
go install .
```

## Commands

### `gen scaffold <Model> [field:type ...]`

Creates all layers for a new model:

| File | Path |
|------|------|
| Domain struct | `internal/core/domain/<model>.go` |
| Port interfaces | `internal/core/ports/<model>.go` |
| Store (CRUD) | `internal/adapters/store/<model>_store.go` |
| Service | `internal/core/services/<model>_service.go` |
| HTTP handler | `internal/adapters/http/<model>_handler.go` |
| Migration | `internal/adapters/store/migrations/<N>_create_<model>.sql` |

Auto-adds `id TEXT DEFAULT uuidv7()::text`, `created_at`, `updated_at` to every model.

**Skips any file that already exists** — never overwrites.

```bash
gen scaffold Product name:string price:float active:bool
gen scaffold Post    title:string body:text user_id:ref:users
gen scaffold Country code:string{2} name:string
```

---

### `gen add <Model> [field:type ...]`

Adds fields to an existing model.

1. Parses `internal/core/domain/<model>.go` via AST — reads current struct fields
2. Skips fields that already exist (safe to re-run)
3. Inserts new fields into the struct
4. Regenerates store CRUD SQL inside `// gen:begin` / `// gen:end` markers
5. Creates an `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` migration

```bash
gen add Product description:*string category:string
gen add Post     published:bool published_at:*time
```

---

### `gen destroy <Model> [--force]`

Removes generated files for a model.

- Asks for confirmation unless `--force` / `-f` is passed
- **Skips files that contain user code outside `// gen:begin/end` markers**
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
| `name:bool` | `bool` | `BOOLEAN NOT NULL DEFAULT FALSE` |
| `name:time` | `time.Time` | `TIMESTAMPTZ NOT NULL DEFAULT NOW()` |
| `name:*time` | `*time.Time` | `TIMESTAMPTZ` |
| `name:json` | `[]byte` | `JSONB NOT NULL DEFAULT '{}'` |
| `name:string{n}` | `string` | `VARCHAR(n) NOT NULL DEFAULT ''` |
| `name:ref:<table>` | `string` | `TEXT NOT NULL REFERENCES <table>(id) ON DELETE CASCADE` |
| `name:*ref:<table>` | `*string` | `TEXT REFERENCES <table>(id) ON DELETE SET NULL` |

Go naming follows standard initialisms: `user_id` → `UserID`, `content_url` → `ContentURL`.

---

## How it protects your work

Generated sections are wrapped with markers:

```go
// gen:begin Product
type Product struct { ... }
// gen:end Product
```

- `gen add` only regenerates code **inside** markers
- Code you write **outside** markers is never touched
- `gen destroy` refuses to delete files that have user code outside markers
- Migrations are append-only — `gen destroy` never removes them

---

## Requirements

Must be run from inside a project that has:

```
internal/core/domain/
internal/core/ports/
internal/adapters/store/
```

Uses **goose** migrations and **pgx/v5** — matches the `boilerplate-pg` layout.

---

## Example workflow

```bash
# New model from scratch
gen scaffold Invoice number:string{20} amount:float paid:bool client_id:ref:clients

# Add a field later
gen add Invoice notes:*string due_date:*time

# Changed your mind
gen destroy Invoice   # removes Go files, keeps migration
# then: goose down -dir ./internal/adapters/store/migrations
```

After scaffold, wire up manually:
1. Register `NewInvoiceStore(pool)` in your `App` struct
2. Register `NewInvoiceService(store, logger)` 
3. Mount `NewInvoiceHandler(svc, logger).Routes()` in your router
4. Run `make migrate` (or `goose up`)
# gen
