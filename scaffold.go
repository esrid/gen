package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runScaffold(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gen scaffold <Model> [field:type ...]")
	}
	model := args[0]
	if !isValidModel(model) {
		return fmt.Errorf("model name %q must start with an uppercase letter", model)
	}

	userFields, err := parseFields(args[1:])
	if err != nil {
		return err
	}

	p, err := detectProject()
	if err != nil {
		return err
	}

	n, err := p.nextMigrationNumber()
	if err != nil {
		return err
	}

	fmt.Printf("scaffold %s  module=%s\n\n", model, p.Module)

	type genFile struct {
		path    string
		content string
	}
	codeFiles := []genFile{
		{p.domainFile(model), genDomainFile(model, userFields)},
		{p.portFile(model), genPortFile(p.Module, model, userFields)},
		{p.storeFile(model), genStoreFile(p.Module, model, userFields, p.Driver)},
		{p.serviceFile(model), genServiceFile(p.Module, model, userFields)},
		{p.handlerFile(model), genHandlerFile(p.Module, model)},
		{p.wireFile(model), genWireFile(p.Module, model)},
	}

	// Ensure JSON helpers exist before creating the handler (generated handler
	// calls respondJSON/decodeJSON which live in this file).
	if _, err := createIfAbsent(p.jsonHelpersFile(), genJSONHelpersFile()); err != nil {
		return err
	}

	anyCreated := false
	for _, f := range codeFiles {
		created, err := createIfAbsent(f.path, f.content)
		if err != nil {
			return err
		}
		if created {
			anyCreated = true
		}
	}

	if anyCreated {
		migPath := p.migrationFile(n, "create_"+toSnakeCase(model))
		if _, err := createIfAbsent(migPath, genCreateMigration(model, userFields, p.Driver)); err != nil {
			return err
		}
	} else {
		fmt.Printf("  skip   migration (all files already exist)\n")
	}

	lower := strings.ToLower(model[:1]) + model[1:]
	plural := tableOf(model)

	fmt.Println()
	fmt.Println("next steps:")
	fmt.Printf("  1. Add %sModule to fx.New() in app.go  — 1 line\n", model)
	fmt.Printf("  2. In router.go, add to NewRouter params + body:\n")
	fmt.Printf("       %sH *%sHandler\n", lower, model)
	fmt.Printf("       r.Mount(\"/api/%s\", %sH.Routes())\n", plural, lower)
	fmt.Printf("  3. Run migrations:  make migrate  (or: goose up)\n")

	var refFields []Field
	for _, f := range userFields {
		if f.RefTable != "" {
			refFields = append(refFields, f)
		}
	}
	if len(refFields) > 0 {
		fmt.Println()
		fmt.Println("ref-field queries generated (store + service + port):")
		pluralPascalName := pluralPascal(model)
		for _, f := range refFields {
			param := strings.ToLower(f.GoName[:1]) + f.GoName[1:]
			fmt.Printf("  List%sBy%s(ctx, %s string, limit, offset int)\n", pluralPascalName, f.GoName, param)
		}
	}

	return nil
}

// createIfAbsent writes path only if it does not already exist.
// Returns (true, nil) when created, (false, nil) when skipped.
func createIfAbsent(path, content string) (created bool, err error) {
	rel := relPath(path)
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("  skip   %s  (already exists)\n", rel)
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, err
	}
	fmt.Printf("  create %s\n", rel)
	return true, nil
}

func isValidModel(s string) bool {
	return len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z'
}

func relPath(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(wd, path)
	if err != nil {
		return path
	}
	return rel
}
