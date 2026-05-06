package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func runDestroy(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gen destroy <Model> [--force]")
	}
	model := args[0]
	if !isValidModel(model) {
		return fmt.Errorf("model name %q must start with an uppercase letter", model)
	}

	force := false
	for _, a := range args[1:] {
		if a == "--force" || a == "-f" {
			force = true
		}
	}

	p, err := detectProject()
	if err != nil {
		return err
	}

	type candidate struct {
		path      string
		label     string
		safe      bool   // true = only generated content, safe to delete
		skipReason string
	}

	files := []struct{ path, label string }{
		{p.domainFile(model), "domain"},
		{p.portFile(model), "port"},
		{p.storeFile(model), "store"},
		{p.serviceFile(model), "service"},
		{p.handlerFile(model), "handler"},
	}

	var candidates []candidate
	for _, f := range files {
		if _, err := os.Stat(f.path); os.IsNotExist(err) {
			continue
		}
		safe, reason := isEntirelyGenerated(f.path, model)
		candidates = append(candidates, candidate{
			path:       f.path,
			label:      f.label,
			safe:       safe,
			skipReason: reason,
		})
	}

	if len(candidates) == 0 {
		fmt.Println("nothing to destroy — no generated files found")
		return nil
	}

	fmt.Printf("destroy %s  module=%s\n\n", model, p.Module)

	var toDelete []candidate
	for _, c := range candidates {
		if c.safe {
			fmt.Printf("  delete %s  %s\n", c.label, relPath(c.path))
			toDelete = append(toDelete, c)
		} else {
			fmt.Printf("  skip   %s  %s  (%s)\n", c.label, relPath(c.path), c.skipReason)
		}
	}

	fmt.Println()
	fmt.Printf("  ⚠  migrations NOT removed — run goose down manually if needed\n")
	fmt.Printf("     dir: internal/adapters/store/migrations/\n")

	if len(toDelete) == 0 {
		fmt.Println("\nnothing safe to delete")
		return nil
	}

	if !force {
		fmt.Printf("\n%d file(s) will be deleted. Continue? [y/N] ", len(toDelete))
		reader := bufio.NewReader(os.Stdin)
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "y" && ans != "yes" {
			fmt.Println("aborted")
			return nil
		}
	}

	for _, c := range toDelete {
		if err := os.Remove(c.path); err != nil {
			fmt.Fprintf(os.Stderr, "  error removing %s: %v\n", c.label, err)
			continue
		}
		fmt.Printf("  removed %s\n", relPath(c.path))
	}
	return nil
}

// isEntirelyGenerated checks whether path contains only generated content for model.
// "Only generated" means: after removing the gen:begin/end section the remaining
// content is just package declaration + import block (no user functions/types).
func isEntirelyGenerated(path, model string) (safe bool, reason string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, "unreadable"
	}
	content := string(data)

	si, ei, found := findSection(content, model)
	if !found {
		return false, "no gen markers — file may be hand-written"
	}

	// What's left after removing the generated section?
	remainder := content[:si] + content[ei:]
	remainder = strings.TrimSpace(remainder)

	// Safe if remainder has no func/type definitions outside the markers.
	if strings.Contains(remainder, "\nfunc ") || strings.Contains(remainder, "\ntype ") {
		return false, "contains user-defined code outside markers"
	}
	return true, ""
}
