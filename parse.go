package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

// ParsedField is a field read from an existing Go struct via AST.
type ParsedField struct {
	GoName   string
	DBName   string
	GoType   string
	RefTable string // from ref:"table" struct tag
}

// readStructFields parses filename and returns the fields of structName.
// Returns nil if the file does not exist or the struct is not found.
func readStructFields(filename, structName string) ([]ParsedField, error) {
	src, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		// Return empty rather than hard-fail on unparseable user code.
		return nil, nil
	}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != structName {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			return collectParsedFields(st), nil
		}
	}
	return nil, nil
}

func collectParsedFields(st *ast.StructType) []ParsedField {
	var out []ParsedField
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			continue // embedded
		}
		pf := ParsedField{
			GoName: f.Names[0].Name,
			GoType: exprStr(f.Type),
		}
		if f.Tag != nil {
			pf.DBName = structTagValue(f.Tag.Value, "db")
			pf.RefTable = structTagValue(f.Tag.Value, "ref")
		}
		if pf.DBName == "" {
			pf.DBName = toSnakeCase(pf.GoName)
		}
		out = append(out, pf)
	}
	return out
}

func exprStr(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return "*" + exprStr(v.X)
	case *ast.SelectorExpr:
		return exprStr(v.X) + "." + v.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprStr(v.Elt)
	}
	return "any"
}

func structTagValue(tagLit, key string) string {
	tag := strings.Trim(tagLit, "`")
	prefix := key + `:"`
	for _, part := range strings.Fields(tag) {
		if strings.HasPrefix(part, prefix) {
			return strings.Trim(strings.TrimPrefix(part, prefix), `"`)
		}
	}
	return ""
}
