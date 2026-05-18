package main

import (
	"fmt"
	"strings"
)

// ── Marker helpers ────────────────────────────────────────────────────────────

func beginMarker(model string) string { return "// gen:begin " + model }
func endMarker(model string) string   { return "// gen:end " + model }

// wrapSection wraps inner content with gen:begin/end markers.
func wrapSection(model, inner string) string {
	return beginMarker(model) + "\n" + inner + endMarker(model) + "\n"
}

// findSection returns the byte range [start, end) of the full marker block
// (begin marker first char → char after end marker's newline).
// found == false if markers are absent.
func findSection(content, model string) (start, end int, found bool) {
	bm := beginMarker(model) + "\n"
	em := endMarker(model)

	si := strings.Index(content, bm)
	if si == -1 {
		return 0, 0, false
	}
	ei := strings.Index(content[si:], em)
	if ei == -1 {
		return 0, 0, false
	}
	ei += si + len(em)
	if ei < len(content) && content[ei] == '\n' {
		ei++
	}
	return si, ei, true
}

// replaceSection swaps the existing marker block for newSection.
// If no markers, appends newSection at the end.
func replaceSection(content, model, newSection string) string {
	si, ei, found := findSection(content, model)
	if !found {
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		return content + "\n" + newSection
	}
	return content[:si] + newSection + content[ei:]
}

// ── Domain ────────────────────────────────────────────────────────────────────

// genDomainSection builds the gen:begin/end block for the domain file.
// userFields must NOT include id / created_at / updated_at.
func genDomainSection(model string, userFields []Field) string {
	var sb strings.Builder

	// sentinel errors
	sb.WriteString(fmt.Sprintf("var Err%sNotFound = errors.New(%q)\n", model, toSnakeCase(model)+" not found"))
	sb.WriteString(fmt.Sprintf("var Err%sConflict = errors.New(%q)\n\n", model, toSnakeCase(model)+" conflict"))

	// struct
	sb.WriteString(fmt.Sprintf("type %s struct {\n", model))
	sb.WriteString("\tID        string    `db:\"id\"          json:\"id\"`\n")
	for _, f := range userFields {
		jsonTag := f.DBName
		if strings.HasPrefix(f.GoType, "*") {
			jsonTag = f.DBName + ",omitempty"
		}
		tags := fmt.Sprintf(`db:"%s" json:"%s"`, f.DBName, jsonTag)
		if f.RefTable != "" {
			tags += fmt.Sprintf(` ref:"%s"`, f.RefTable)
		}
		if f.ExtraTags != "" {
			tags += " " + f.ExtraTags
		}
		sb.WriteString(fmt.Sprintf("\t%-10s %-12s `%s`\n", f.GoName, f.GoType, tags))
	}
	sb.WriteString("\tCreatedAt time.Time `db:\"created_at\"  json:\"created_at\"`\n")
	sb.WriteString("\tUpdatedAt time.Time `db:\"updated_at\"  json:\"updated_at\"`\n")
	sb.WriteString("}\n")

	return wrapSection(model, sb.String())
}

// genDomainFile builds a full new domain file.
func genDomainFile(model string, userFields []Field) string {
	var sb strings.Builder
	sb.WriteString("package domain\n\n")
	sb.WriteString("import (\n\t\"errors\"\n\t\"time\"\n)\n\n")
	sb.WriteString(genDomainSection(model, userFields))
	return sb.String()
}

// ── Port ──────────────────────────────────────────────────────────────────────

func genPortSection(model string, userFields []Field) string {
	plural := pluralPascal(model)
	var sb strings.Builder

	// Store interface — explicit named methods so *store.Store satisfies it on both drivers
	sb.WriteString(fmt.Sprintf("type %sStore interface {\n", model))
	sb.WriteString(fmt.Sprintf("\tCreate%s(ctx context.Context, p domain.%s) (*domain.%s, error)\n", model, model, model))
	sb.WriteString(fmt.Sprintf("\t%sByID(ctx context.Context, id string) (*domain.%s, error)\n", model, model))
	sb.WriteString(fmt.Sprintf("\tUpdate%s(ctx context.Context, p domain.%s) (*domain.%s, error)\n", model, model, model))
	sb.WriteString(fmt.Sprintf("\tDelete%s(ctx context.Context, id string) error\n", model))
	sb.WriteString(fmt.Sprintf("\tList%s(ctx context.Context, limit, offset int) ([]domain.%s, error)\n", plural, model))
	for _, f := range userFields {
		if f.RefTable == "" {
			continue
		}
		param := strings.ToLower(f.GoName[:1]) + f.GoName[1:]
		sb.WriteString(fmt.Sprintf("\tList%sBy%s(ctx context.Context, %s string, limit, offset int) ([]domain.%s, error)\n", plural, f.GoName, param, model))
	}
	sb.WriteString("}\n\n")

	// Service interface
	sb.WriteString(fmt.Sprintf("type %sService interface {\n", model))
	sb.WriteString(fmt.Sprintf("\tCRUDService[domain.%s]\n", model))
	for _, f := range userFields {
		if f.RefTable == "" {
			continue
		}
		param := strings.ToLower(f.GoName[:1]) + f.GoName[1:]
		sb.WriteString(fmt.Sprintf("\tList%sBy%s(ctx context.Context, %s string, limit, offset int) ([]domain.%s, error)\n", plural, f.GoName, param, model))
	}
	sb.WriteString("}\n")

	return wrapSection(model, sb.String())
}


func genPortFile(module, model string, userFields []Field) string {
	var sb strings.Builder
	sb.WriteString("package ports\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"context\"\n\n")
	sb.WriteString(fmt.Sprintf("\t%q\n", module+"/internal/core/domain"))
	sb.WriteString(")\n\n")
	sb.WriteString(genPortSection(model, userFields))
	return sb.String()
}

// ── Service ───────────────────────────────────────────────────────────────────

func genServiceSection(model string, userFields []Field) string {
	plural := pluralPascal(model)
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("type %sService struct {\n", model))
	sb.WriteString(fmt.Sprintf("\tstore  ports.%sStore\n", model))
	sb.WriteString("\tlogger *slog.Logger\n")
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func New%sService(store ports.%sStore, logger *slog.Logger) *%sService {\n", model, model, model))
	sb.WriteString(fmt.Sprintf("\treturn &%sService{store: store, logger: logger}\n", model))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (s *%sService) Get(ctx context.Context, id string) (*domain.%s, error) {\n", model, model))
	sb.WriteString(fmt.Sprintf("\treturn s.store.%sByID(ctx, id)\n", model))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (s *%sService) List(ctx context.Context, limit, offset int) ([]domain.%s, error) {\n", model, model))
	sb.WriteString(fmt.Sprintf("\treturn s.store.List%s(ctx, limit, offset)\n", plural))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (s *%sService) Create(ctx context.Context, p domain.%s) (*domain.%s, error) {\n", model, model, model))
	sb.WriteString(fmt.Sprintf("\treturn s.store.Create%s(ctx, p)\n", model))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (s *%sService) Update(ctx context.Context, p domain.%s) (*domain.%s, error) {\n", model, model, model))
	sb.WriteString(fmt.Sprintf("\treturn s.store.Update%s(ctx, p)\n", model))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (s *%sService) Delete(ctx context.Context, id string) error {\n", model))
	sb.WriteString(fmt.Sprintf("\treturn s.store.Delete%s(ctx, id)\n", model))
	sb.WriteString("}\n")

	for _, f := range userFields {
		if f.RefTable == "" {
			continue
		}
		param := strings.ToLower(f.GoName[:1]) + f.GoName[1:]
		sb.WriteString(fmt.Sprintf("\nfunc (s *%sService) List%sBy%s(ctx context.Context, %s string, limit, offset int) ([]domain.%s, error) {\n", model, plural, f.GoName, param, model))
		sb.WriteString(fmt.Sprintf("\treturn s.store.List%sBy%s(ctx, %s, limit, offset)\n", plural, f.GoName, param))
		sb.WriteString("}\n")
	}

	return wrapSection(model, sb.String())
}

func genServiceFile(module, model string, userFields []Field) string {
	var sb strings.Builder
	sb.WriteString("package services\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"context\"\n")
	sb.WriteString("\t\"log/slog\"\n\n")
	sb.WriteString(fmt.Sprintf("\t%q\n", module+"/internal/core/domain"))
	sb.WriteString(fmt.Sprintf("\t%q\n", module+"/internal/core/ports"))
	sb.WriteString(")\n\n")
	sb.WriteString(genServiceSection(model, userFields))
	return sb.String()
}

// ── Handler ───────────────────────────────────────────────────────────────────

func genHandlerSection(module, model string) string {
	plural := pluralPascal(model)
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("type %sHandler struct {\n", model))
	sb.WriteString(fmt.Sprintf("\t*BaseHandler[domain.%s]\n", model))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func New%sHandler(svc ports.%sService, logger *slog.Logger) *%sHandler {\n", model, model, model))
	sb.WriteString(fmt.Sprintf("\treturn &%sHandler{\n", model))
	sb.WriteString(fmt.Sprintf("\t\tBaseHandler: NewBaseHandler[domain.%s](svc, logger, %q),\n", model, toSnakeCase(model)))
	sb.WriteString("\t}\n")
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func Register%sRoutes(h *%sHandler) func(chi.Router) {\n", model, model))
	sb.WriteString("\treturn func(r chi.Router) {\n")
	sb.WriteString(fmt.Sprintf("\t\tr.Mount(\"/api/%s\", h.Routes())\n", toSnakeCase(plural)))
	sb.WriteString("\t}\n")
	sb.WriteString("}\n")

	return wrapSection(model, sb.String())
}



func genHandlerFile(module, model string) string {
	var sb strings.Builder
	sb.WriteString("package http\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"log/slog\"\n\n")
	sb.WriteString(fmt.Sprintf("\t%q\n", module+"/internal/core/domain"))
	sb.WriteString(fmt.Sprintf("\t%q\n\n", module+"/internal/core/ports"))
	sb.WriteString("\t\"github.com/go-chi/chi/v5\"\n")
	sb.WriteString(")\n\n")
	sb.WriteString(genHandlerSection(module, model))
	return sb.String()
}


// ── Store ─────────────────────────────────────────────────────────────────────

func genStoreSection(model string, userFields []Field, driver string) string {
	switch driver {
	case "sqlite":
		return genStoreSectionSQLite(model, userFields)
	case "pgx":
		return genStoreSectionPgx(model, userFields)
	default:
		panic("unknown driver: " + driver)
	}
}

func genStoreSectionPgx(model string, userFields []Field) string {
	table := tableOf(model)
	plural := pluralPascal(model)
	selectCols := buildSelectCols(userFields)
	insertCols := colList(userFields)
	placeholders := placeholderList(len(userFields))
	updateSet := updateSetList(userFields)
	nArgs := len(userFields)

	var sb strings.Builder

	// Create
	sb.WriteString(fmt.Sprintf("func (s *Store) Create%s(ctx context.Context, p domain.%s) (*domain.%s, error) {\n", model, model, model))
	if nArgs == 0 {
		sb.WriteString(fmt.Sprintf("\trows, err := s.pool.Query(ctx,\n\t\t`INSERT INTO %s DEFAULT VALUES RETURNING %s`,\n\t)\n", table, selectCols))
	} else {
		sb.WriteString(fmt.Sprintf("\trows, err := s.pool.Query(ctx,\n\t\t`INSERT INTO %s (%s) VALUES (%s) RETURNING %s`,\n", table, insertCols, placeholders, selectCols))
		for _, f := range userFields {
			sb.WriteString(fmt.Sprintf("\t\tp.%s,\n", f.GoName))
		}
		sb.WriteString("\t)\n")
	}
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, \"Create%s\")\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\tresult, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[domain.%s])\n", model))
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString("\t\tvar pgErr *pgconn.PgError\n")
	sb.WriteString("\t\tif errors.As(err, &pgErr) && pgErr.Code == \"23505\" {\n")
	sb.WriteString(fmt.Sprintf("\t\t\treturn nil, domain.Err%sConflict\n", model))
	sb.WriteString("\t\t}\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, \"Create%s\")\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString("\treturn result, nil\n")
	sb.WriteString("}\n\n")

	// ByID
	sb.WriteString(fmt.Sprintf("func (s *Store) %sByID(ctx context.Context, id string) (*domain.%s, error) {\n", model, model))
	sb.WriteString(fmt.Sprintf("\trows, err := s.pool.Query(ctx,\n\t\t`SELECT %s FROM %s WHERE id = $1`,\n\t\tid,\n\t)\n", selectCols, table))
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, \"%sByID\")\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\tp, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[domain.%s])\n", model))
	sb.WriteString("\tif errors.Is(err, pgx.ErrNoRows) {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, domain.Err%sNotFound\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\treturn p, DecorateError(err, \"%sByID\")\n", model))
	sb.WriteString("}\n\n")

	// Update
	sb.WriteString(fmt.Sprintf("func (s *Store) Update%s(ctx context.Context, p domain.%s) (*domain.%s, error) {\n", model, model, model))
	if nArgs == 0 {
		sb.WriteString(fmt.Sprintf("\trows, err := s.pool.Query(ctx,\n\t\t`UPDATE %s SET updated_at = NOW() WHERE id = $1 RETURNING %s`,\n\t\tp.ID,\n\t)\n", table, selectCols))
	} else {
		sb.WriteString(fmt.Sprintf("\trows, err := s.pool.Query(ctx,\n\t\t`UPDATE %s SET %s, updated_at = NOW() WHERE id = $%d RETURNING %s`,\n", table, updateSet, nArgs+1, selectCols))
		for _, f := range userFields {
			sb.WriteString(fmt.Sprintf("\t\tp.%s,\n", f.GoName))
		}
		sb.WriteString("\t\tp.ID,\n\t)\n")
	}
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, \"Update%s\")\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\tresult, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[domain.%s])\n", model))
	sb.WriteString("\tif errors.Is(err, pgx.ErrNoRows) {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, domain.Err%sNotFound\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\treturn result, DecorateError(err, \"Update%s\")\n", model))
	sb.WriteString("}\n\n")

	// Delete
	sb.WriteString(fmt.Sprintf("func (s *Store) Delete%s(ctx context.Context, id string) error {\n", model))
	sb.WriteString(fmt.Sprintf("\ttag, err := s.pool.Exec(ctx,\n\t\t`DELETE FROM %s WHERE id = $1`,\n\t\tid,\n\t)\n", table))
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn DecorateError(err, \"Delete%s\")\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString("\tif tag.RowsAffected() == 0 {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn domain.Err%sNotFound\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString("\treturn nil\n")
	sb.WriteString("}\n\n")

	// List
	sb.WriteString(fmt.Sprintf("func (s *Store) List%s(ctx context.Context, limit, offset int) ([]domain.%s, error) {\n", plural, model))
	sb.WriteString(fmt.Sprintf("\trows, err := s.pool.Query(ctx,\n\t\t`SELECT %s FROM %s ORDER BY created_at DESC LIMIT $1 OFFSET $2`,\n\t\tlimit, offset,\n\t)\n", selectCols, table))
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, \"List%s\")\n", plural))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\treturn pgx.CollectRows(rows, pgx.RowToStructByName[domain.%s])\n", model))
	sb.WriteString("}\n")

	// ListBy<RefField>
	for _, f := range userFields {
		if f.RefTable == "" {
			continue
		}
		param := strings.ToLower(f.GoName[:1]) + f.GoName[1:]
		methodName := fmt.Sprintf("List%sBy%s", plural, f.GoName)
		sb.WriteString(fmt.Sprintf("\nfunc (s *Store) %s(ctx context.Context, %s string, limit, offset int) ([]domain.%s, error) {\n", methodName, param, model))
		sb.WriteString(fmt.Sprintf("\trows, err := s.pool.Query(ctx,\n\t\t`SELECT %s FROM %s WHERE %s = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,\n\t\t%s, limit, offset,\n\t)\n", selectCols, table, f.DBName, param))
		sb.WriteString("\tif err != nil {\n")
		sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, %q)\n", methodName))
		sb.WriteString("\t}\n")
		sb.WriteString(fmt.Sprintf("\treturn pgx.CollectRows(rows, pgx.RowToStructByName[domain.%s])\n", model))
		sb.WriteString("}\n")
	}

	return wrapSection(model, sb.String())
}




// ── SQLite store helpers ───────────────────────────────────────────────────────

func scanPtrList(userFields []Field) string {
	parts := []string{"&p.ID"}
	for _, f := range userFields {
		parts = append(parts, "&p."+f.GoName)
	}
	parts = append(parts, "&p.CreatedAt", "&p.UpdatedAt")
	return strings.Join(parts, ", ")
}

func insertColsSQLite(userFields []Field) string {
	cols := make([]string, 0, 1+len(userFields))
	cols = append(cols, "id")
	for _, f := range userFields {
		cols = append(cols, f.DBName)
	}
	return strings.Join(cols, ", ")
}

func questionMarks(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ", ")
}

func updateSetSQLite(fields []Field) string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = f.DBName + " = ?"
	}
	return strings.Join(parts, ", ")
}

// pgToSQLiteType maps a PostgreSQL SQL column definition to its SQLite equivalent.
// strings.NewReplacer does a single left-to-right pass; specific patterns must be
// listed before their substrings so the longer match wins at each position.
var sqliteTypeReplacer = strings.NewReplacer(
	"BIGINT NOT NULL DEFAULT 0", "INTEGER NOT NULL DEFAULT 0",
	"DOUBLE PRECISION NOT NULL DEFAULT 0", "REAL NOT NULL DEFAULT 0",
	"BOOLEAN NOT NULL DEFAULT FALSE", "INTEGER NOT NULL DEFAULT 0",
	"TIMESTAMPTZ NOT NULL DEFAULT NOW()", "DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP",
	"JSONB NOT NULL DEFAULT '{}'", "TEXT NOT NULL DEFAULT '{}'",
	"UUID NOT NULL REFERENCES", "TEXT NOT NULL REFERENCES",
	"UUID REFERENCES", "TEXT REFERENCES",
	"BIGINT", "INTEGER",
	"DOUBLE PRECISION", "REAL",
	"BOOLEAN", "INTEGER",
	"TIMESTAMPTZ", "DATETIME",
	"JSONB", "TEXT",
	"DEFAULT NOW()", "DEFAULT CURRENT_TIMESTAMP",
	"DEFAULT FALSE", "DEFAULT 0",
	"DEFAULT TRUE", "DEFAULT 1",
)

func pgToSQLiteType(s string) string {
	return sqliteTypeReplacer.Replace(s)
}

func genStoreSectionSQLite(model string, userFields []Field) string {
	table := tableOf(model)
	plural := pluralPascal(model)
	selectCols := buildSelectCols(userFields)
	scanPtrs := scanPtrList(userFields)

	var sb strings.Builder

	// Create
	sb.WriteString(fmt.Sprintf("func (s *Store) Create%s(ctx context.Context, p domain.%s) (*domain.%s, error) {\n", model, model, model))
	sb.WriteString("\tid := newID()\n")
	if len(userFields) == 0 {
		sb.WriteString(fmt.Sprintf("\t_, err := s.db.ExecContext(ctx,\n\t\t`INSERT INTO %s (id) VALUES (?)`,\n\t\tid,\n\t)\n", table))
	} else {
		sb.WriteString(fmt.Sprintf("\t_, err := s.db.ExecContext(ctx,\n\t\t`INSERT INTO %s (%s) VALUES (%s)`,\n\t\tid,\n", table, insertColsSQLite(userFields), questionMarks(1+len(userFields))))
		for _, f := range userFields {
			sb.WriteString(fmt.Sprintf("\t\tp.%s,\n", f.GoName))
		}
		sb.WriteString("\t)\n")
	}
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString("\t\tif strings.Contains(err.Error(), \"UNIQUE constraint failed\") {\n")
	sb.WriteString(fmt.Sprintf("\t\t\treturn nil, domain.Err%sConflict\n", model))
	sb.WriteString("\t\t}\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, \"Create%s\")\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\treturn s.%sByID(ctx, id)\n", model))
	sb.WriteString("}\n\n")

	// ByID
	sb.WriteString(fmt.Sprintf("func (s *Store) %sByID(ctx context.Context, id string) (*domain.%s, error) {\n", model, model))
	sb.WriteString(fmt.Sprintf("\trow := s.db.QueryRowContext(ctx,\n\t\t`SELECT %s FROM %s WHERE id = ?`,\n\t\tid,\n\t)\n", selectCols, table))
	sb.WriteString(fmt.Sprintf("\tvar p domain.%s\n", model))
	sb.WriteString(fmt.Sprintf("\terr := row.Scan(%s)\n", scanPtrs))
	sb.WriteString("\tif errors.Is(err, sql.ErrNoRows) {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, domain.Err%sNotFound\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\treturn &p, DecorateError(err, \"%sByID\")\n", model))
	sb.WriteString("}\n\n")

	// Update
	sb.WriteString(fmt.Sprintf("func (s *Store) Update%s(ctx context.Context, p domain.%s) (*domain.%s, error) {\n", model, model, model))
	if len(userFields) == 0 {
		sb.WriteString(fmt.Sprintf("\tres, err := s.db.ExecContext(ctx,\n\t\t`UPDATE %s SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`,\n\t\tp.ID,\n\t)\n", table))
	} else {
		sb.WriteString(fmt.Sprintf("\tres, err := s.db.ExecContext(ctx,\n\t\t`UPDATE %s SET %s, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,\n", table, updateSetSQLite(userFields)))
		for _, f := range userFields {
			sb.WriteString(fmt.Sprintf("\t\tp.%s,\n", f.GoName))
		}
		sb.WriteString("\t\tp.ID,\n\t)\n")
	}
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, \"Update%s\")\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString("\tif n, err2 := res.RowsAffected(); err2 == nil && n == 0 {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, domain.Err%sNotFound\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\treturn s.%sByID(ctx, p.ID)\n", model))
	sb.WriteString("}\n\n")

	// Delete
	sb.WriteString(fmt.Sprintf("func (s *Store) Delete%s(ctx context.Context, id string) error {\n", model))
	sb.WriteString(fmt.Sprintf("\tres, err := s.db.ExecContext(ctx,\n\t\t`DELETE FROM %s WHERE id = ?`,\n\t\tid,\n\t)\n", table))
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn DecorateError(err, \"Delete%s\")\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString("\tif n, err2 := res.RowsAffected(); err2 == nil && n == 0 {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn domain.Err%sNotFound\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString("\treturn nil\n")
	sb.WriteString("}\n\n")

	// List
	sb.WriteString(fmt.Sprintf("func (s *Store) List%s(ctx context.Context, limit, offset int) ([]domain.%s, error) {\n", plural, model))
	sb.WriteString(fmt.Sprintf("\trows, err := s.db.QueryContext(ctx,\n\t\t`SELECT %s FROM %s ORDER BY created_at DESC LIMIT ? OFFSET ?`,\n\t\tlimit, offset,\n\t)\n", selectCols, table))
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, \"List%s\")\n", plural))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\treturn collectRows(rows, func(r *sql.Rows) (domain.%s, error) {\n", model))
	sb.WriteString(fmt.Sprintf("\t\tvar p domain.%s\n", model))
	sb.WriteString(fmt.Sprintf("\t\terr := r.Scan(%s)\n", scanPtrs))
	sb.WriteString("\t\treturn p, err\n")
	sb.WriteString("\t})\n")
	sb.WriteString("}\n")

	// ListBy<RefField>
	for _, f := range userFields {
		if f.RefTable == "" {
			continue
		}
		param := strings.ToLower(f.GoName[:1]) + f.GoName[1:]
		methodName := fmt.Sprintf("List%sBy%s", plural, f.GoName)
		sb.WriteString(fmt.Sprintf("\nfunc (s *Store) %s(ctx context.Context, %s string, limit, offset int) ([]domain.%s, error) {\n", methodName, param, model))
		sb.WriteString(fmt.Sprintf("\trows, err := s.db.QueryContext(ctx,\n\t\t`SELECT %s FROM %s WHERE %s = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,\n\t\t%s, limit, offset,\n\t)\n", selectCols, table, f.DBName, param))
		sb.WriteString("\tif err != nil {\n")
		sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, %q)\n", methodName))
		sb.WriteString("\t}\n")
		sb.WriteString(fmt.Sprintf("\treturn collectRows(rows, func(r *sql.Rows) (domain.%s, error) {\n", model))
		sb.WriteString(fmt.Sprintf("\t\tvar p domain.%s\n", model))
		sb.WriteString(fmt.Sprintf("\t\terr := r.Scan(%s)\n", scanPtrs))
		sb.WriteString("\t\treturn p, err\n")
		sb.WriteString("\t})\n")
		sb.WriteString("}\n")
	}

	return wrapSection(model, sb.String())
}

func genStoreFile(module, model string, userFields []Field, driver string) string {
	var sb strings.Builder
	sb.WriteString("package store\n\n")
	if driver == "sqlite" {
		sb.WriteString("import (\n")
		sb.WriteString("\t\"context\"\n")
		sb.WriteString("\t\"database/sql\"\n")
		sb.WriteString("\t\"errors\"\n")
		sb.WriteString("\t\"strings\"\n\n")
		sb.WriteString(fmt.Sprintf("\t%q\n", module+"/internal/core/domain"))
		sb.WriteString(")\n\n")
	} else {
		sb.WriteString("import (\n")
		sb.WriteString("\t\"context\"\n")
		sb.WriteString("\t\"errors\"\n\n")
		sb.WriteString(fmt.Sprintf("\t%q\n\n", module+"/internal/core/domain"))
		sb.WriteString("\t\"github.com/jackc/pgx/v5\"\n")
		sb.WriteString("\t\"github.com/jackc/pgx/v5/pgconn\"\n")
		sb.WriteString(")\n\n")
	}
	sb.WriteString(genStoreSection(model, userFields, driver))
	return sb.String()
}

// ── Migration ─────────────────────────────────────────────────────────────────

func genCreateMigration(model string, userFields []Field, driver string) string {
	table := tableOf(model)
	var sb strings.Builder
	sb.WriteString("-- +goose Up\n")
	sb.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", table))
	if driver == "sqlite" {
		sb.WriteString(fmt.Sprintf("  %-14s TEXT        PRIMARY KEY,\n", "id"))
		for _, f := range userFields {
			sb.WriteString(fmt.Sprintf("  %-14s %s,\n", f.DBName, pgToSQLiteType(f.SQLType)))
		}
		sb.WriteString(fmt.Sprintf("  %-14s DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,\n", "created_at"))
		sb.WriteString(fmt.Sprintf("  %-14s DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP\n", "updated_at"))
	} else {
		sb.WriteString(fmt.Sprintf("  %-14s UUID        PRIMARY KEY DEFAULT uuidv7(),\n", "id"))
		for _, f := range userFields {
			sb.WriteString(fmt.Sprintf("  %-14s %s,\n", f.DBName, f.SQLType))
		}
		sb.WriteString(fmt.Sprintf("  %-14s TIMESTAMPTZ NOT NULL DEFAULT NOW(),\n", "created_at"))
		sb.WriteString(fmt.Sprintf("  %-14s TIMESTAMPTZ NOT NULL DEFAULT NOW()\n", "updated_at"))
	}
	sb.WriteString(");\n")
	if driver == "sqlite" {
		sb.WriteString(fmt.Sprintf("CREATE INDEX %s_created_at_idx ON %s (created_at DESC);\n", table, table))
		for _, f := range userFields {
			if f.Index {
				sb.WriteString(fmt.Sprintf("CREATE INDEX %s_%s_idx ON %s (%s);\n", table, f.DBName, table, f.DBName))
			}
		}
	} else {
		sb.WriteString(fmt.Sprintf("CREATE INDEX ON %s (created_at DESC);\n", table))
		for _, f := range userFields {
			if f.Index {
				sb.WriteString(fmt.Sprintf("CREATE INDEX ON %s (%s);\n", table, f.DBName))
			}
		}
	}
	sb.WriteString("\n-- +goose Down\n")
	sb.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS %s;\n", table))
	return sb.String()
}

func genAlterMigration(model string, addFields []Field, driver string) string {
	table := tableOf(model)
	var sb strings.Builder
	sb.WriteString("-- +goose Up\n")
	if driver == "sqlite" {
		for _, f := range addFields {
			sqlType := pgToSQLiteType(f.SQLType)
			isUnique := strings.Contains(sqlType, " UNIQUE")
			sqlType = strings.ReplaceAll(sqlType, " UNIQUE", "")
			sb.WriteString(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %-14s %s;\n", table, f.DBName, sqlType))
			if isUnique {
				sb.WriteString(fmt.Sprintf("CREATE UNIQUE INDEX %s_%s_unique_idx ON %s (%s);\n", table, f.DBName, table, f.DBName))
			}
		}
		for _, f := range addFields {
			if f.Index {
				sb.WriteString(fmt.Sprintf("CREATE INDEX %s_%s_idx ON %s (%s);\n", table, f.DBName, table, f.DBName))
			}
		}
		sb.WriteString("\n-- +goose Down\n")
		sb.WriteString("-- requires SQLite >= 3.35.0\n")
		for _, f := range addFields {
			sb.WriteString(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;\n", table, f.DBName))
		}
	} else {
		for _, f := range addFields {
			sb.WriteString(fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %-14s %s;\n", table, f.DBName, f.SQLType))
		}
		for _, f := range addFields {
			if f.Index {
				sb.WriteString(fmt.Sprintf("CREATE INDEX ON %s (%s);\n", table, f.DBName))
			}
		}
		sb.WriteString("\n-- +goose Down\n")
		for _, f := range addFields {
			sb.WriteString(fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;\n", table, f.DBName))
		}
	}
	return sb.String()
}

// ── fx wire module (one file per scaffolded model) ───────────────────────────

func genWireSection(model string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("var %sModule = fx.Module(%q,\n", model, toSnakeCase(model)))
	sb.WriteString("\tfx.Provide(\n")
	// Cast *store.Store to the port interface — works for both pgx and sqlite since
	// both boilerplates use a single *Store with named methods.
	sb.WriteString(fmt.Sprintf("\t\tfx.Annotate(func(st *store.Store) ports.%sStore { return st }),\n", model))
	sb.WriteString(fmt.Sprintf("\t\tfx.Annotate(services.New%sService, fx.As(new(ports.%sService))),\n", model, model))
	sb.WriteString(fmt.Sprintf("\t\thttpAdapter.New%sHandler,\n", model))
	sb.WriteString(fmt.Sprintf("\t\tfx.Annotate(httpAdapter.Register%sRoutes, fx.ResultTags(`group:\"routes\"`)),\n", model))
	sb.WriteString("\t),\n")
	sb.WriteString(")\n")
	return sb.String()
}

func genWireFile(module, model string) string {
	var sb strings.Builder
	sb.WriteString("package app\n\n")
	sb.WriteString("import (\n")
	sb.WriteString(fmt.Sprintf("\thttpAdapter %q\n", module+"/internal/adapters/http"))
	sb.WriteString(fmt.Sprintf("\t%q\n", module+"/internal/adapters/store"))
	sb.WriteString(fmt.Sprintf("\t%q\n", module+"/internal/core/ports"))
	sb.WriteString(fmt.Sprintf("\t%q\n", module+"/internal/core/services"))
	sb.WriteString("\t\"go.uber.org/fx\"\n")
	sb.WriteString(")\n\n")
	sb.WriteString(wrapSection(model, genWireSection(model)))
	return sb.String()
}


// ── HTTP JSON helpers (generated once per project) ────────────────────────────

// genJSONHelpersFile returns the content of internal/adapters/http/json.go.
// respondJSON marshals to a buffer before writing headers so a marshal error
// can still be reported as 500 (100go #53: handle errors, don't silently drop).
func genJSONHelpersFile() string {
	return `package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
)

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	if err := json.NewEncoder(buf).Encode(v); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}
`
}

// ── SQL builder helpers ───────────────────────────────────────────────────────

// buildSelectCols returns "id, col1, col2, ..., created_at, updated_at".
func buildSelectCols(userFields []Field) string {
	cols := []string{"id"}
	for _, f := range userFields {
		cols = append(cols, f.DBName)
	}
	cols = append(cols, "created_at", "updated_at")
	return strings.Join(cols, ", ")
}

func colList(fields []Field) string {
	cols := make([]string, len(fields))
	for i, f := range fields {
		cols[i] = f.DBName
	}
	return strings.Join(cols, ", ")
}

func placeholderList(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(parts, ", ")
}

func updateSetList(fields []Field) string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = fmt.Sprintf("%s = $%d", f.DBName, i+1)
	}
	return strings.Join(parts, ", ")
}
