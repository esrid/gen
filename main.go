package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "scaffold":
		err = runScaffold(os.Args[2:])
	case "add":
		err = runAdd(os.Args[2:])
	case "remove":
		err = runRemove(os.Args[2:])
	case "destroy":
		err = runDestroy(os.Args[2:])
	case "method":
		err = runMethod(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`gen - code generator for hexagonal Go/pgx projects

Commands:
  gen scaffold <Model> [field:type ...]                   create domain, port, store, service, handler, migration
  gen add      <Model> [field:type ...]                   add fields to existing model
  gen remove   <Model> <field> [field2 ...]               remove fields from existing model
  gen destroy  <Model> [--force]                          remove all generated files (confirms before deleting)
  gen method   <Model> <MethodName> "<params> -> <returns>"  add custom method to port interfaces + service stub

Types:
  string *string   int *int   float *float   bool *bool   time *time
  string{n}        VARCHAR(n)
  ref:<table>      TEXT REFERENCES <table>(id) ON DELETE CASCADE
  *ref:<table>     nullable foreign key

Examples:
  gen scaffold Product name:string price:float active:bool
  gen scaffold Post title:string body:text user_id:ref:users
  gen scaffold Country code:string{2} name:string
  gen add Product description:*string tags:json
  gen method Product FindBySlug "slug:string -> *Product,error"
  gen method Order ListByUser "userID:string -> []Order,error"

Notes:
  - Auto-adds: id (UUID uuid7), created_at, updated_at
  - Code wrapped in // gen:begin <Model> / // gen:end <Model> — safe to add code outside
  - gen add only modifies marked sections + creates ALTER TABLE migration
  - gen destroy asks confirmation, skips files with user code outside markers
  - gen method adds signature to both XStore+XService interfaces; you implement the store method manually
  - Run from project root (where go.mod lives)
  - Install: cd /path/to/gen && go install .
`)
}
