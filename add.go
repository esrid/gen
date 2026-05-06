package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runAdd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: gen add <Model> [field:type ...]")
	}
	model := args[0]
	if !isValidModel(model) {
		return fmt.Errorf("model name %q must start with an uppercase letter", model)
	}

	newFields, err := parseFields(args[1:])
	if err != nil {
		return err
	}
	if len(newFields) == 0 {
		return fmt.Errorf("no fields specified")
	}

	p, err := detectProject()
	if err != nil {
		return err
	}

	domainPath := p.domainFile(model)
	if _, err := os.Stat(domainPath); os.IsNotExist(err) {
		return fmt.Errorf("domain file not found: %s\nRun: gen scaffold %s first", relPath(domainPath), model)
	}

	// Parse existing struct to find current user fields.
	parsed, err := readStructFields(domainPath, model)
	if err != nil {
		return fmt.Errorf("parse %s: %w", relPath(domainPath), err)
	}

	// Build a set of existing DB column names to detect duplicates.
	existing := make(map[string]bool, len(parsed))
	for _, pf := range parsed {
		existing[pf.DBName] = true
	}

	var addFields []Field
	for _, f := range newFields {
		if existing[f.DBName] {
			fmt.Printf("  skip   field %q (already present)\n", f.DBName)
			continue
		}
		addFields = append(addFields, f)
	}
	if len(addFields) == 0 {
		fmt.Println("nothing to add — all fields already exist")
		return nil
	}

	// Merge: all current user fields + new ones (preserving existing types).
	allUserFields := mergeUserFields(parsed, addFields)

	fmt.Printf("add %d field(s) to %s  module=%s\n\n", len(addFields), model, p.Module)

	// 1. Update domain file.
	if err := updateDomainFile(domainPath, model, allUserFields, addFields); err != nil {
		return fmt.Errorf("update domain: %w", err)
	}
	fmt.Printf("  update %s\n", relPath(domainPath))

	// 2. Update store file.
	storePath := p.storeFile(model)
	if err := updateStoreFile(storePath, p.Module, model, allUserFields); err != nil {
		return fmt.Errorf("update store: %w", err)
	}

	// 3. Create ALTER TABLE migration.
	n, err := p.nextMigrationNumber()
	if err != nil {
		return err
	}
	migPath := p.migrationFile(n, "add_to_"+toSnakeCase(model))
	if _, err := createIfAbsent(migPath, genAlterMigration(model, addFields)); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("next step: run migrations (make migrate / goose up)")
	return nil
}

// mergeUserFields builds a Field slice from existing parsed fields + new ones.
// Auto fields (id, created_at, etc.) in parsed are silently skipped.
func mergeUserFields(parsed []ParsedField, add []Field) []Field {
	var out []Field
	for _, pf := range parsed {
		if isAutoField(pf.DBName) {
			continue
		}
		out = append(out, Field{
			GoName:  pf.GoName,
			DBName:  pf.DBName,
			GoType:  pf.GoType,
			SQLType: goTypeToSQLType(pf.GoType),
		})
	}
	out = append(out, add...)
	return out
}

// updateDomainFile rewrites (or inserts) the gen:begin/end section for model.
// allUserFields = all user fields after merge (for marker-based regen).
// addFields     = only the newly added fields (for no-marker struct insertion).
func updateDomainFile(path, model string, allUserFields, addFields []Field) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(raw)

	_, _, hasMarkers := findSection(content, model)
	if hasMarkers {
		updated := replaceSection(content, model, genDomainSection(model, allUserFields))
		return os.WriteFile(path, []byte(updated), 0o644)
	}

	// No markers: insert only the new fields before the struct's closing brace.
	updated, err := insertBeforeStructClose(content, model, addFields)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}

// insertBeforeStructClose finds "type ModelName struct {" and inserts fields
// just before the matching closing brace.
func insertBeforeStructClose(content, model string, fields []Field) (string, error) {
	needle := "type " + model + " struct {"
	idx := strings.Index(content, needle)
	if idx == -1 {
		return "", fmt.Errorf("struct %s not found", model)
	}
	// Walk from the opening brace to find the matching '}'.
	braceStart := strings.Index(content[idx:], "{") + idx
	depth, closeAt := 0, -1
	for i := braceStart; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				closeAt = i
			}
		}
		if closeAt != -1 {
			break
		}
	}
	if closeAt == -1 {
		return "", fmt.Errorf("no closing brace for struct %s", model)
	}
	var ins strings.Builder
	for _, f := range fields {
		ins.WriteString(fmt.Sprintf("\t%s %s `db:\"%s\"`\n", f.GoName, f.GoType, f.DBName))
	}
	return content[:closeAt] + ins.String() + content[closeAt:], nil
}

// updateStoreFile regenerates the gen:begin/end section in the store file.
// If the file has no markers, it prints a notice instead of touching user code.
// If the file does not exist, it is created from scratch.
func updateStoreFile(path, module, model string, allUserFields []Field) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		content := genStoreFile(module, model, allUserFields)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Printf("  create %s\n", relPath(path))
		return nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(raw)

	_, _, hasMarkers := findSection(content, model)
	if !hasMarkers {
		fmt.Printf("  notice %s has no gen markers — update SQL queries manually\n", relPath(path))
		fmt.Printf("         new columns: %s\n", func() string {
			names := make([]string, len(allUserFields))
			for i, f := range allUserFields {
				names[i] = f.DBName
			}
			return strings.Join(names, ", ")
		}())
		return nil
	}

	updated := replaceSection(content, model, genStoreSection(model, allUserFields))
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return err
	}
	fmt.Printf("  update %s\n", relPath(path))
	return nil
}
