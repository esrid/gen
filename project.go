package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Project holds the detected project root and module name.
type Project struct {
	Root   string
	Module string
	Driver string // "pgx" or "sqlite"
}

// requiredDirs are the directories that must exist for a project to be recognised.
var requiredDirs = []string{
	filepath.Join("internal", "core", "domain"),
	filepath.Join("internal", "core", "ports"),
	filepath.Join("internal", "adapters", "store"),
}

// detectProject walks up from the current directory to find a go.mod that sits
// inside a hexagonal project (must have internal/core/domain, ports, store).
func detectProject() (*Project, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if err := checkProjectStructure(dir); err != nil {
				return nil, err
			}
			mod, driver, err := readGoMod(filepath.Join(dir, "go.mod"))
			if err != nil {
				return nil, err
			}
			return &Project{Root: dir, Module: mod, Driver: driver}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("no go.mod found — run gen from your project root")
		}
		dir = parent
	}
}

// checkProjectStructure verifies the hexagonal directory layout exists.
func checkProjectStructure(root string) error {
	var missing []string
	for _, d := range requiredDirs {
		if _, err := os.Stat(filepath.Join(root, d)); os.IsNotExist(err) {
			missing = append(missing, d)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"project at %s does not look like a hexagonal Go project\n  missing dirs: %s\n  gen expects: internal/core/domain, internal/core/ports, internal/adapters/store",
			root, strings.Join(missing, ", "),
		)
	}
	return nil
}

func readGoMod(modPath string) (module, driver string, err error) {
	data, err := os.ReadFile(modPath)
	if err != nil {
		return "", "", err
	}
	content := string(data)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			module = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			break
		}
	}
	if module == "" {
		return "", "", fmt.Errorf("module directive not found in %s", modPath)
	}
	driver = "pgx"
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		// strip leading "require " for single-line require directives
		check := strings.TrimPrefix(trimmed, "require ")
		if strings.HasPrefix(check, "modernc.org/sqlite") || strings.HasPrefix(check, "mattn/go-sqlite3") {
			driver = "sqlite"
			break
		}
	}
	return module, driver, nil
}

// nextMigrationNumber scans the migrations directory and returns max+1.
func (p *Project) nextMigrationNumber() (int, error) {
	dir := filepath.Join(p.Root, "internal", "adapters", "store", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	max := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		part, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

// File path helpers.

func (p *Project) domainFile(model string) string {
	return filepath.Join(p.Root, "internal", "core", "domain", toSnakeCase(model)+".go")
}

func (p *Project) portFile(model string) string {
	return filepath.Join(p.Root, "internal", "core", "ports", toSnakeCase(model)+".go")
}

func (p *Project) storeFile(model string) string {
	return filepath.Join(p.Root, "internal", "adapters", "store", toSnakeCase(model)+"_store.go")
}

func (p *Project) serviceFile(model string) string {
	return filepath.Join(p.Root, "internal", "core", "services", toSnakeCase(model)+"_service.go")
}

func (p *Project) handlerFile(model string) string {
	return filepath.Join(p.Root, "internal", "adapters", "http", toSnakeCase(model)+"_handler.go")
}

func (p *Project) jsonHelpersFile() string {
	return filepath.Join(p.Root, "internal", "adapters", "http", "json.go")
}

func (p *Project) migrationFile(n int, label string) string {
	return filepath.Join(p.Root, "internal", "adapters", "store", "migrations",
		fmt.Sprintf("%05d_%s.sql", n, toSnakeCase(label)))
}
