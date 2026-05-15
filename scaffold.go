package main

import (
	"fmt"
	"os"
	"path/filepath"
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
		{p.portFile(model), genPortFile(p.Module, model)},
		{p.storeFile(model), genStoreFile(p.Module, model, userFields, p.Driver)},
		{p.serviceFile(model), genServiceFile(p.Module, model)},
		{p.handlerFile(model), genHandlerFile(p.Module, model)},
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

	fmt.Println()
	fmt.Println("next steps:")
	fmt.Printf("  1. Register %sStore in your App struct / wire up dependencies\n", model)
	fmt.Printf("  2. Run migrations: make migrate (or goose up)\n")
	fmt.Printf("  3. Add %sService to internal/core/services/ if needed\n", model)
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
