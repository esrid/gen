package main

import (
	"fmt"
	"os"
	"strings"
)

func runRemove(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: gen remove <Model> <field> [field2 ...]")
	}
	model := args[0]
	if !isValidModel(model) {
		return fmt.Errorf("model name %q must start with an uppercase letter", model)
	}

	toRemove := make(map[string]bool)
	for _, name := range args[1:] {
		db := toSnakeCase(name)
		if isAutoField(db) {
			return fmt.Errorf("cannot remove auto-managed field %q (id, created_at, updated_at, deleted_at)", db)
		}
		toRemove[db] = true
	}

	p, err := detectProject()
	if err != nil {
		return err
	}

	domainPath := p.domainFile(model)
	if _, err := os.Stat(domainPath); os.IsNotExist(err) {
		return fmt.Errorf("domain file not found: %s\nRun: gen scaffold %s first", relPath(domainPath), model)
	}

	parsed, err := readStructFields(domainPath, model)
	if err != nil {
		return fmt.Errorf("parse %s: %w", relPath(domainPath), err)
	}

	// Validate requested removals exist.
	for db := range toRemove {
		found := false
		for _, pf := range parsed {
			if pf.DBName == db {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("field %q not found in %s struct", db, model)
		}
	}

	// Fields being dropped (for migration down).
	var dropped []ParsedField
	for _, pf := range parsed {
		if toRemove[pf.DBName] {
			dropped = append(dropped, pf)
		}
	}

	// Remaining user fields (auto fields already excluded by isAutoField check in mergeUserFields).
	var remaining []Field
	for _, pf := range parsed {
		if isAutoField(pf.DBName) || toRemove[pf.DBName] {
			continue
		}
		remaining = append(remaining, Field{
			GoName:  pf.GoName,
			DBName:  pf.DBName,
			GoType:  pf.GoType,
			SQLType: goTypeToSQLType(pf.GoType),
		})
	}

	fmt.Printf("remove %d field(s) from %s  module=%s\n\n", len(toRemove), model, p.Module)
	for _, pf := range dropped {
		fmt.Printf("  drop  %s (%s)\n", pf.DBName, pf.GoType)
	}
	fmt.Println()

	// 1. Update domain file.
	if err := updateDomainFile(domainPath, model, remaining, nil); err != nil {
		return fmt.Errorf("update domain: %w", err)
	}
	fmt.Printf("  update %s\n", relPath(domainPath))

	// 2. Update store file.
	storePath := p.storeFile(model)
	if err := updateStoreFile(storePath, p.Module, model, remaining); err != nil {
		return fmt.Errorf("update store: %w", err)
	}

	// 3. Create DROP COLUMN migration.
	n, err := p.nextMigrationNumber()
	if err != nil {
		return err
	}
	migPath := p.migrationFile(n, "remove_from_"+toSnakeCase(model))
	migContent := genDropMigration(model, dropped)
	if _, err := createIfAbsent(migPath, migContent); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("next step: run migrations (make migrate / goose up)")
	return nil
}

func genDropMigration(model string, dropped []ParsedField) string {
	table := tableOf(model)
	var sb strings.Builder
	sb.WriteString("-- +goose Up\n")
	for _, pf := range dropped {
		sb.WriteString(fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;\n", table, pf.DBName))
	}
	sb.WriteString("\n-- +goose Down\n")
	for _, pf := range dropped {
		sb.WriteString(fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %-14s %s;\n",
			table, pf.DBName, goTypeToSQLType(pf.GoType)))
	}
	return sb.String()
}
