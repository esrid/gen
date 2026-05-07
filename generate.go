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

	// sentinel error
	sb.WriteString(fmt.Sprintf("var Err%sNotFound = errors.New(%q)\n\n", model, toSnakeCase(model)+" not found"))

	// struct
	sb.WriteString(fmt.Sprintf("type %s struct {\n", model))
	sb.WriteString("\tID        string    `db:\"id\"`\n")
	for _, f := range userFields {
		sb.WriteString(fmt.Sprintf("\t%-10s %-12s `db:\"%s\"`\n", f.GoName, f.GoType, f.DBName))
	}
	sb.WriteString("\tCreatedAt time.Time `db:\"created_at\"`\n")
	sb.WriteString("\tUpdatedAt time.Time `db:\"updated_at\"`\n")
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

func genPortSection(model string) string {
	plural := pluralPascal(model)
	var sb strings.Builder

	// Store interface
	sb.WriteString(fmt.Sprintf("type %sStore interface {\n", model))
	sb.WriteString(fmt.Sprintf("\tCreate%s(ctx context.Context, p domain.%s) (*domain.%s, error)\n", model, model, model))
	sb.WriteString(fmt.Sprintf("\t%sByID(ctx context.Context, id string) (*domain.%s, error)\n", model, model))
	sb.WriteString(fmt.Sprintf("\tUpdate%s(ctx context.Context, p domain.%s) error\n", model, model))
	sb.WriteString(fmt.Sprintf("\tDelete%s(ctx context.Context, id string) error\n", model))
	sb.WriteString(fmt.Sprintf("\tList%s(ctx context.Context) ([]domain.%s, error)\n", plural, model))
	sb.WriteString("}\n\n")

	// Service interface
	sb.WriteString(fmt.Sprintf("type %sService interface {\n", model))
	sb.WriteString(fmt.Sprintf("\tCreate%s(ctx context.Context, p domain.%s) (*domain.%s, error)\n", model, model, model))
	sb.WriteString(fmt.Sprintf("\tGet%s(ctx context.Context, id string) (*domain.%s, error)\n", model, model))
	sb.WriteString(fmt.Sprintf("\tUpdate%s(ctx context.Context, p domain.%s) error\n", model, model))
	sb.WriteString(fmt.Sprintf("\tDelete%s(ctx context.Context, id string) error\n", model))
	sb.WriteString(fmt.Sprintf("\tList%s(ctx context.Context) ([]domain.%s, error)\n", plural, model))
	sb.WriteString("}\n")

	return wrapSection(model, sb.String())
}

func genPortFile(module, model string) string {
	var sb strings.Builder
	sb.WriteString("package ports\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"context\"\n\n")
	sb.WriteString(fmt.Sprintf("\t%q\n", module+"/internal/core/domain"))
	sb.WriteString(")\n\n")
	sb.WriteString(genPortSection(model))
	return sb.String()
}

// ── Service ───────────────────────────────────────────────────────────────────

func genServiceSection(_ string, model string) string {
	plural := pluralPascal(model)
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("type %sService struct {\n", model))
	sb.WriteString(fmt.Sprintf("\tstore  ports.%sStore\n", model))
	sb.WriteString("\tlogger *slog.Logger\n")
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func New%sService(store ports.%sStore, logger *slog.Logger) *%sService {\n", model, model, model))
	sb.WriteString(fmt.Sprintf("\treturn &%sService{store: store, logger: logger}\n", model))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (s *%sService) Create%s(ctx context.Context, p domain.%s) (*domain.%s, error) {\n", model, model, model, model))
	sb.WriteString(fmt.Sprintf("\treturn s.store.Create%s(ctx, p)\n", model))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (s *%sService) Get%s(ctx context.Context, id string) (*domain.%s, error) {\n", model, model, model))
	sb.WriteString(fmt.Sprintf("\treturn s.store.%sByID(ctx, id)\n", model))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (s *%sService) Update%s(ctx context.Context, p domain.%s) error {\n", model, model, model))
	sb.WriteString(fmt.Sprintf("\treturn s.store.Update%s(ctx, p)\n", model))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (s *%sService) Delete%s(ctx context.Context, id string) error {\n", model, model))
	sb.WriteString(fmt.Sprintf("\treturn s.store.Delete%s(ctx, id)\n", model))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (s *%sService) List%s(ctx context.Context) ([]domain.%s, error) {\n", model, plural, model))
	sb.WriteString(fmt.Sprintf("\treturn s.store.List%s(ctx)\n", plural))
	sb.WriteString("}\n")

	return wrapSection(model, sb.String())
}

func genServiceFile(module, model string) string {
	var sb strings.Builder
	sb.WriteString("package services\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"context\"\n")
	sb.WriteString("\t\"log/slog\"\n\n")
	sb.WriteString(fmt.Sprintf("\t%q\n", module+"/internal/core/domain"))
	sb.WriteString(fmt.Sprintf("\t%q\n", module+"/internal/core/ports"))
	sb.WriteString(")\n\n")
	sb.WriteString(genServiceSection(module, model))
	return sb.String()
}

// ── Handler ───────────────────────────────────────────────────────────────────

func genHandlerSection(_ string, model string) string {
	lower := strings.ToLower(model[:1]) + model[1:]
	plural := pluralPascal(model)
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("type %sHandler struct {\n", model))
	sb.WriteString(fmt.Sprintf("\tsvc    ports.%sService\n", model))
	sb.WriteString("\tlogger *slog.Logger\n")
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func New%sHandler(svc ports.%sService, logger *slog.Logger) *%sHandler {\n", model, model, model))
	sb.WriteString(fmt.Sprintf("\treturn &%sHandler{svc: svc, logger: logger}\n", model))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (h *%sHandler) Routes() http.Handler {\n", model))
	sb.WriteString("\tr := chi.NewRouter()\n")
	sb.WriteString("\tr.Get(\"/\", h.handleList)\n")
	sb.WriteString("\tr.Post(\"/\", h.handleCreate)\n")
	sb.WriteString("\tr.Get(\"/{id}\", h.handleGet)\n")
	sb.WriteString("\tr.Put(\"/{id}\", h.handleUpdate)\n")
	sb.WriteString("\tr.Delete(\"/{id}\", h.handleDelete)\n")
	sb.WriteString("\treturn r\n")
	sb.WriteString("}\n\n")

	// handleList
	sb.WriteString(fmt.Sprintf("func (h *%sHandler) handleList(w http.ResponseWriter, r *http.Request) {\n", model))
	sb.WriteString(fmt.Sprintf("\titems, err := h.svc.List%s(r.Context())\n", plural))
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\th.logger.ErrorContext(r.Context(), \"list %s\", \"err\", err)\n", toSnakeCase(model)))
	sb.WriteString("\t\thttp.Error(w, \"internal error\", http.StatusInternalServerError)\n")
	sb.WriteString("\t\treturn\n")
	sb.WriteString("\t}\n")
	sb.WriteString("\trespondJSON(w, http.StatusOK, items)\n")
	sb.WriteString("}\n\n")

	// handleCreate
	sb.WriteString(fmt.Sprintf("func (h *%sHandler) handleCreate(w http.ResponseWriter, r *http.Request) {\n", model))
	sb.WriteString(fmt.Sprintf("\tvar %s domain.%s\n", lower, model))
	sb.WriteString(fmt.Sprintf("\tif !decodeJSON(w, r, &%s) {\n", lower))
	sb.WriteString("\t\treturn\n")
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\tresult, err := h.svc.Create%s(r.Context(), %s)\n", model, lower))
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\th.logger.ErrorContext(r.Context(), \"create %s\", \"err\", err)\n", toSnakeCase(model)))
	sb.WriteString("\t\thttp.Error(w, err.Error(), http.StatusUnprocessableEntity)\n")
	sb.WriteString("\t\treturn\n")
	sb.WriteString("\t}\n")
	sb.WriteString("\trespondJSON(w, http.StatusCreated, result)\n")
	sb.WriteString("}\n\n")

	// handleGet
	sb.WriteString(fmt.Sprintf("func (h *%sHandler) handleGet(w http.ResponseWriter, r *http.Request) {\n", model))
	sb.WriteString("\tid := chi.URLParam(r, \"id\")\n")
	sb.WriteString(fmt.Sprintf("\titem, err := h.svc.Get%s(r.Context(), id)\n", model))
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString("\t\thttp.Error(w, \"not found\", http.StatusNotFound)\n")
	sb.WriteString("\t\treturn\n")
	sb.WriteString("\t}\n")
	sb.WriteString("\trespondJSON(w, http.StatusOK, item)\n")
	sb.WriteString("}\n\n")

	// handleUpdate
	sb.WriteString(fmt.Sprintf("func (h *%sHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {\n", model))
	sb.WriteString("\tid := chi.URLParam(r, \"id\")\n")
	sb.WriteString(fmt.Sprintf("\tvar %s domain.%s\n", lower, model))
	sb.WriteString(fmt.Sprintf("\tif !decodeJSON(w, r, &%s) {\n", lower))
	sb.WriteString("\t\treturn\n")
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\t%s.ID = id\n", lower))
	sb.WriteString(fmt.Sprintf("\tif err := h.svc.Update%s(r.Context(), %s); err != nil {\n", model, lower))
	sb.WriteString(fmt.Sprintf("\t\th.logger.ErrorContext(r.Context(), \"update %s\", \"id\", id, \"err\", err)\n", toSnakeCase(model)))
	sb.WriteString("\t\thttp.Error(w, err.Error(), http.StatusUnprocessableEntity)\n")
	sb.WriteString("\t\treturn\n")
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\trespondJSON(w, http.StatusOK, %s)\n", lower))
	sb.WriteString("}\n\n")

	// handleDelete
	sb.WriteString(fmt.Sprintf("func (h *%sHandler) handleDelete(w http.ResponseWriter, r *http.Request) {\n", model))
	sb.WriteString("\tid := chi.URLParam(r, \"id\")\n")
	sb.WriteString(fmt.Sprintf("\tif err := h.svc.Delete%s(r.Context(), id); err != nil {\n", model))
	sb.WriteString(fmt.Sprintf("\t\th.logger.ErrorContext(r.Context(), \"delete %s\", \"id\", id, \"err\", err)\n", toSnakeCase(model)))
	sb.WriteString("\t\thttp.Error(w, err.Error(), http.StatusBadRequest)\n")
	sb.WriteString("\t\treturn\n")
	sb.WriteString("\t}\n")
	sb.WriteString("\tw.WriteHeader(http.StatusNoContent)\n")
	sb.WriteString("}\n")

	return wrapSection(model, sb.String())
}

func genHandlerFile(module, model string) string {
	var sb strings.Builder
	sb.WriteString("package http\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"log/slog\"\n")
	sb.WriteString("\t\"net/http\"\n\n")
	sb.WriteString(fmt.Sprintf("\t%q\n", module+"/internal/core/domain"))
	sb.WriteString(fmt.Sprintf("\t%q\n\n", module+"/internal/core/ports"))
	sb.WriteString("\t\"github.com/go-chi/chi/v5\"\n")
	sb.WriteString(")\n\n")
	sb.WriteString(genHandlerSection(module, model))
	return sb.String()
}

// ── Store ─────────────────────────────────────────────────────────────────────

func genStoreSection(model string, userFields []Field) string {
	table := tableOf(model)
	plural := pluralPascal(model)

	selectCols := buildSelectCols(userFields)
	insertCols := colList(userFields)
	placeholders := placeholderList(len(userFields))
	updateSet := updateSetList(userFields)
	nArgs := len(userFields)

	var sb strings.Builder

	// ── Create ──
	sb.WriteString(fmt.Sprintf("func (s *Store) Create%s(ctx context.Context, p domain.%s) (*domain.%s, error) {\n", model, model, model))
	if len(userFields) == 0 {
		sb.WriteString(fmt.Sprintf("\trows, err := s.pool.Query(ctx,\n"))
		sb.WriteString(fmt.Sprintf("\t\t`INSERT INTO %s DEFAULT VALUES RETURNING %s`,\n", table, selectCols))
	} else {
		sb.WriteString(fmt.Sprintf("\trows, err := s.pool.Query(ctx,\n"))
		sb.WriteString(fmt.Sprintf("\t\t`INSERT INTO %s (%s) VALUES (%s) RETURNING %s`,\n", table, insertCols, placeholders, selectCols))
		for _, f := range userFields {
			sb.WriteString(fmt.Sprintf("\t\tp.%s,\n", f.GoName))
		}
	}
	sb.WriteString("\t)\n")
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, \"Create%s\")\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\tresult, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[domain.%s])\n", model))
	sb.WriteString(fmt.Sprintf("\treturn result, DecorateError(err, \"Create%s\")\n", model))
	sb.WriteString("}\n\n")

	// ── ByID ──
	sb.WriteString(fmt.Sprintf("func (s *Store) %sByID(ctx context.Context, id string) (*domain.%s, error) {\n", model, model))
	sb.WriteString("\trows, err := s.pool.Query(ctx,\n")
	sb.WriteString(fmt.Sprintf("\t\t`SELECT %s FROM %s WHERE id = $1`,\n", selectCols, table))
	sb.WriteString("\t\tid,\n")
	sb.WriteString("\t)\n")
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, \"%sByID\")\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\tp, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[domain.%s])\n", model))
	sb.WriteString("\tif errors.Is(err, pgx.ErrNoRows) {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, domain.Err%sNotFound\n", model))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\treturn p, DecorateError(err, \"%sByID\")\n", model))
	sb.WriteString("}\n\n")

	// ── Update ──
	sb.WriteString(fmt.Sprintf("func (s *Store) Update%s(ctx context.Context, p domain.%s) error {\n", model, model))
	sb.WriteString("\t_, err := s.pool.Exec(ctx,\n")
	if len(userFields) == 0 {
		sb.WriteString(fmt.Sprintf("\t\t`UPDATE %s SET updated_at = NOW() WHERE id = $1`,\n", table))
		sb.WriteString("\t\tp.ID,\n")
	} else {
		sb.WriteString(fmt.Sprintf("\t\t`UPDATE %s SET %s, updated_at = NOW() WHERE id = $%d`,\n", table, updateSet, nArgs+1))
		for _, f := range userFields {
			sb.WriteString(fmt.Sprintf("\t\tp.%s,\n", f.GoName))
		}
		sb.WriteString("\t\tp.ID,\n")
	}
	sb.WriteString("\t)\n")
	sb.WriteString(fmt.Sprintf("\treturn DecorateError(err, \"Update%s\")\n", model))
	sb.WriteString("}\n\n")

	// ── Delete ──
	sb.WriteString(fmt.Sprintf("func (s *Store) Delete%s(ctx context.Context, id string) error {\n", model))
	sb.WriteString("\t_, err := s.pool.Exec(ctx,\n")
	sb.WriteString(fmt.Sprintf("\t\t`DELETE FROM %s WHERE id = $1`,\n", table))
	sb.WriteString("\t\tid,\n")
	sb.WriteString("\t)\n")
	sb.WriteString(fmt.Sprintf("\treturn DecorateError(err, \"Delete%s\")\n", model))
	sb.WriteString("}\n\n")

	// ── List ──
	sb.WriteString(fmt.Sprintf("func (s *Store) List%s(ctx context.Context) ([]domain.%s, error) {\n", plural, model))
	sb.WriteString("\trows, err := s.pool.Query(ctx,\n")
	sb.WriteString(fmt.Sprintf("\t\t`SELECT %s FROM %s ORDER BY created_at DESC`,\n", selectCols, table))
	sb.WriteString("\t)\n")
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString(fmt.Sprintf("\t\treturn nil, DecorateError(err, \"List%s\")\n", plural))
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\treturn pgx.CollectRows(rows, pgx.RowToStructByName[domain.%s])\n", model))
	sb.WriteString("}\n")

	return wrapSection(model, sb.String())
}

func genStoreFile(module, model string, userFields []Field) string {
	var sb strings.Builder
	sb.WriteString("package store\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"context\"\n")
	sb.WriteString("\t\"errors\"\n\n")
	sb.WriteString(fmt.Sprintf("\t%q\n\n", module+"/internal/core/domain"))
	sb.WriteString("\t\"github.com/jackc/pgx/v5\"\n")
	sb.WriteString(")\n\n")
	sb.WriteString(genStoreSection(model, userFields))
	return sb.String()
}

// ── Migration ─────────────────────────────────────────────────────────────────

func genCreateMigration(model string, userFields []Field) string {
	table := tableOf(model)
	var sb strings.Builder
	sb.WriteString("-- +goose Up\n")
	sb.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", table))
	sb.WriteString(fmt.Sprintf("  %-14s TEXT        PRIMARY KEY DEFAULT uuidv7()::text,\n", "id"))
	for _, f := range userFields {
		sb.WriteString(fmt.Sprintf("  %-14s %s,\n", f.DBName, f.SQLType))
	}
	sb.WriteString(fmt.Sprintf("  %-14s TIMESTAMPTZ NOT NULL DEFAULT NOW(),\n", "created_at"))
	sb.WriteString(fmt.Sprintf("  %-14s TIMESTAMPTZ NOT NULL DEFAULT NOW()\n", "updated_at"))
	sb.WriteString(");\n")
	sb.WriteString("\n-- +goose Down\n")
	sb.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS %s;\n", table))
	return sb.String()
}

func genAlterMigration(model string, addFields []Field) string {
	table := tableOf(model)
	var sb strings.Builder
	sb.WriteString("-- +goose Up\n")
	for _, f := range addFields {
		sb.WriteString(fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %-14s %s;\n", table, f.DBName, f.SQLType))
	}
	sb.WriteString("\n-- +goose Down\n")
	for _, f := range addFields {
		sb.WriteString(fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;\n", table, f.DBName))
	}
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
