package main

import (
	"fmt"
	"strings"
)

// Field holds parsed input like "name:string", "code:string{2}", "user_id:ref:users".
type Field struct {
	GoName   string // PascalCase Go identifier (e.g. FullName)
	DBName   string // snake_case db tag      (e.g. full_name)
	GoType   string // Go type string         (e.g. string, *int64, time.Time)
	SQLType  string // Full SQL column def    (e.g. TEXT NOT NULL DEFAULT '')
	RefTable string // set when type is ref:table (e.g. "users")
}

type typeRow struct {
	goType  string
	sqlFull string // NOT NULL + DEFAULT included
	sqlNull string // nullable (no constraint)
}

var typeMappings = map[string]typeRow{
	"string":  {"string", "TEXT NOT NULL DEFAULT ''", "TEXT"},
	"text":    {"string", "TEXT NOT NULL DEFAULT ''", "TEXT"},
	"int":     {"int64", "BIGINT NOT NULL DEFAULT 0", "BIGINT"},
	"int64":   {"int64", "BIGINT NOT NULL DEFAULT 0", "BIGINT"},
	"float":   {"float64", "DOUBLE PRECISION NOT NULL DEFAULT 0", "DOUBLE PRECISION"},
	"float64": {"float64", "DOUBLE PRECISION NOT NULL DEFAULT 0", "DOUBLE PRECISION"},
	"bool":    {"bool", "BOOLEAN NOT NULL DEFAULT FALSE", "BOOLEAN"},
	"time":    {"time.Time", "TIMESTAMPTZ NOT NULL DEFAULT NOW()", "TIMESTAMPTZ"},
	"json":    {"[]byte", "JSONB NOT NULL DEFAULT '{}'", "JSONB"},
}

func parseField(raw string) (Field, error) {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Field{}, fmt.Errorf("invalid field %q: expected name:type", raw)
	}
	name := parts[0]
	typ := strings.ToLower(strings.TrimSpace(parts[1]))

	nullable := strings.HasPrefix(typ, "*")
	if nullable {
		typ = typ[1:]
	}

	// ── ref:table ──────────────────────────────────────────────────────────
	if strings.HasPrefix(typ, "ref:") {
		refTable := typ[4:]
		if refTable == "" {
			return Field{}, fmt.Errorf("ref requires a table name: %q (e.g. user_id:ref:users)", raw)
		}
		sqlType := fmt.Sprintf("TEXT NOT NULL REFERENCES %s(id) ON DELETE CASCADE", refTable)
		goType := "string"
		if nullable {
			sqlType = fmt.Sprintf("TEXT REFERENCES %s(id) ON DELETE SET NULL", refTable)
			goType = "*string"
		}
		return Field{
			GoName:   toPascalCase(name),
			DBName:   toSnakeCase(name),
			GoType:   goType,
			SQLType:  sqlType,
			RefTable: refTable,
		}, nil
	}

	// ── string{n} / text{n} ───────────────────────────────────────────────
	if i := strings.Index(typ, "{"); i != -1 {
		j := strings.Index(typ, "}")
		if j == -1 || j <= i {
			return Field{}, fmt.Errorf("invalid type syntax %q: expected string{n}", typ)
		}
		n := typ[i+1 : j]
		base := typ[:i]
		if base != "string" && base != "text" && base != "varchar" {
			return Field{}, fmt.Errorf("length modifier only valid for string/text/varchar, got %q", base)
		}
		sqlType := fmt.Sprintf("VARCHAR(%s) NOT NULL DEFAULT ''", n)
		goType := "string"
		if nullable {
			sqlType = fmt.Sprintf("VARCHAR(%s)", n)
			goType = "*string"
		}
		return Field{
			GoName:  toPascalCase(name),
			DBName:  toSnakeCase(name),
			GoType:  goType,
			SQLType: sqlType,
		}, nil
	}

	// ── standard types ────────────────────────────────────────────────────
	row, ok := typeMappings[typ]
	if !ok {
		return Field{}, fmt.Errorf("unknown type %q — valid: string, int, float, bool, time, json, ref:<table>, string{n}", typ)
	}

	goType := row.goType
	sqlType := row.sqlFull
	if nullable {
		goType = "*" + goType
		sqlType = row.sqlNull
	}

	return Field{
		GoName:  toPascalCase(name),
		DBName:  toSnakeCase(name),
		GoType:  goType,
		SQLType: sqlType,
	}, nil
}

func parseFields(args []string) ([]Field, error) {
	var fields []Field
	for _, a := range args {
		f, err := parseField(a)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	return fields, nil
}

// isAutoField returns true for fields managed automatically (never in INSERT/UPDATE params).
func isAutoField(dbName string) bool {
	switch dbName {
	case "id", "created_at", "updated_at", "deleted_at":
		return true
	}
	return false
}

// goTypeToSQLType converts a parsed Go type string back to a SQL type string (best-effort).
// Used when re-reading existing structs for gen add — ref tables are lost here (falls back to TEXT).
func goTypeToSQLType(goType string) string {
	nullable := strings.HasPrefix(goType, "*")
	base := strings.TrimPrefix(goType, "*")
	var sqlBase string
	switch base {
	case "string":
		sqlBase = "TEXT"
	case "int64":
		sqlBase = "BIGINT"
	case "float64":
		sqlBase = "DOUBLE PRECISION"
	case "bool":
		sqlBase = "BOOLEAN"
	case "time.Time":
		sqlBase = "TIMESTAMPTZ"
	case "[]byte":
		sqlBase = "JSONB"
	default:
		sqlBase = "TEXT"
	}
	if nullable {
		return sqlBase
	}
	switch sqlBase {
	case "TEXT":
		return "TEXT NOT NULL DEFAULT ''"
	case "BIGINT":
		return "BIGINT NOT NULL DEFAULT 0"
	case "DOUBLE PRECISION":
		return "DOUBLE PRECISION NOT NULL DEFAULT 0"
	case "BOOLEAN":
		return "BOOLEAN NOT NULL DEFAULT FALSE"
	case "TIMESTAMPTZ":
		return "TIMESTAMPTZ NOT NULL DEFAULT NOW()"
	case "JSONB":
		return "JSONB NOT NULL DEFAULT '{}'"
	}
	return sqlBase
}
