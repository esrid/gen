package main

import (
	"fmt"
	"os"
	"strings"
)

func runMethod(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: gen method <Model> <MethodName> \"<params> -> <returns>\"\n  e.g.  gen method Restaurant RestaurantByUserID \"userID:string -> *Restaurant,error\"")
	}

	model := args[0]
	methodName := args[1]
	sigRaw := strings.Join(args[2:], " ")

	parts := strings.SplitN(sigRaw, "->", 2)
	if len(parts) != 2 {
		return fmt.Errorf("signature must contain '->' separator\n  e.g. \"userID:string -> *Restaurant,error\"")
	}

	paramsPart := strings.TrimSpace(parts[0])
	returnsPart := strings.TrimSpace(parts[1])

	goParams, callArgs, err := parseMethodParams(paramsPart)
	if err != nil {
		return err
	}

	goReturns, err := parseMethodReturns(returnsPart)
	if err != nil {
		return err
	}

	paramStr := "ctx context.Context"
	if goParams != "" {
		paramStr += ", " + goParams
	}
	stubCallArgs := "ctx"
	if callArgs != "" {
		stubCallArgs += ", " + callArgs
	}

	ifaceMethod := fmt.Sprintf("\t%s(%s) %s\n", methodName, paramStr, goReturns)

	proj, err := detectProject()
	if err != nil {
		return err
	}

	if err := addMethodToPortFile(proj, model, ifaceMethod); err != nil {
		return err
	}
	if err := addMethodToServiceFile(proj, model, methodName, paramStr, goReturns, stubCallArgs); err != nil {
		return err
	}

	snake := toSnakeCase(model)
	fmt.Printf("added %s to ports/%s.go (%sStore + %sService)\n", methodName, snake, model, model)
	fmt.Printf("added forwarding stub to services/%s_service.go\n", snake)
	fmt.Printf("\nnext: implement %s in adapters/store/%s_store.go\n", methodName, snake)
	return nil
}

// parseMethodParams parses "userID:string, name:*string" into Go param list and call-arg list.
func parseMethodParams(raw string) (goParams, callArgs string, err error) {
	if raw == "" {
		return "", "", nil
	}
	var goList, nameList []string
	for _, p := range splitOnComma(raw) {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		idx := strings.LastIndex(p, ":")
		if idx == -1 {
			return "", "", fmt.Errorf("param %q must be name:type (e.g. userID:string)", p)
		}
		name := strings.TrimSpace(p[:idx])
		typ := strings.TrimSpace(p[idx+1:])
		goList = append(goList, name+" "+typ)
		nameList = append(nameList, name)
	}
	return strings.Join(goList, ", "), strings.Join(nameList, ", "), nil
}

// parseMethodReturns parses "*Restaurant,error" into a Go return expression.
func parseMethodReturns(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("returns must not be empty")
	}
	var goTypes []string
	for _, t := range splitOnComma(raw) {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		goTypes = append(goTypes, domainQualify(t))
	}
	if len(goTypes) == 0 {
		return "", fmt.Errorf("returns must not be empty")
	}
	if len(goTypes) == 1 {
		return goTypes[0], nil
	}
	return "(" + strings.Join(goTypes, ", ") + ")", nil
}

// domainQualify adds a domain. prefix to PascalCase types that aren't builtins.
func domainQualify(t string) string {
	prefix := ""
	for strings.HasPrefix(t, "*") || strings.HasPrefix(t, "[]") {
		if strings.HasPrefix(t, "[]") {
			prefix += "[]"
			t = t[2:]
		} else {
			prefix += "*"
			t = t[1:]
		}
	}
	builtins := map[string]bool{
		"error": true, "bool": true, "string": true,
		"int": true, "int32": true, "int64": true,
		"float32": true, "float64": true,
		"byte": true, "rune": true, "any": true,
	}
	if builtins[t] {
		return prefix + t
	}
	if len(t) > 0 && t[0] >= 'A' && t[0] <= 'Z' {
		return prefix + "domain." + t
	}
	return prefix + t
}

// splitOnComma splits on commas (no generics assumed in domain method signatures).
func splitOnComma(s string) []string {
	return strings.Split(s, ",")
}

// addMethodToPortFile inserts ifaceMethod into both XStore and XService interfaces.
func addMethodToPortFile(proj *Project, model, ifaceMethod string) error {
	path := proj.portFile(model)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("port file not found (%s) — run gen scaffold first", path)
	}
	content := string(data)

	// Guard: skip if method already present
	sigLine := strings.TrimSpace(ifaceMethod)
	if strings.Contains(content, sigLine) {
		return fmt.Errorf("method signature already present in %s", path)
	}

	for _, iface := range []string{
		fmt.Sprintf("type %sStore interface {", model),
		fmt.Sprintf("type %sService interface {", model),
	} {
		content, err = insertIntoInterface(content, iface, ifaceMethod)
		if err != nil {
			return fmt.Errorf("could not modify %s in %s: %w", iface, path, err)
		}
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// insertIntoInterface inserts method before the closing } of the named interface.
func insertIntoInterface(content, ifaceDecl, method string) (string, error) {
	idx := strings.Index(content, ifaceDecl)
	if idx == -1 {
		return content, fmt.Errorf("%s not found", ifaceDecl)
	}
	openIdx := strings.Index(content[idx:], "{")
	if openIdx == -1 {
		return content, fmt.Errorf("no opening brace after %s", ifaceDecl)
	}
	after := content[idx+openIdx+1:]
	closeIdx := strings.Index(after, "\n}")
	if closeIdx == -1 {
		return content, fmt.Errorf("no closing brace for %s", ifaceDecl)
	}
	insertPos := idx + openIdx + 1 + closeIdx + 1 // points at the `}` character
	return content[:insertPos] + method + content[insertPos:], nil
}

// addMethodToServiceFile appends a forwarding stub after gen:end Model.
func addMethodToServiceFile(proj *Project, model, methodName, paramStr, returnStr, callArgs string) error {
	path := proj.serviceFile(model)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("service file not found (%s) — run gen scaffold first", path)
	}
	content := string(data)

	stub := fmt.Sprintf("\nfunc (s *%sService) %s(%s) %s {\n\treturn s.store.%s(%s)\n}\n",
		model, methodName, paramStr, returnStr, methodName, callArgs)

	// Guard: skip if already present
	if strings.Contains(content, fmt.Sprintf("func (s *%sService) %s(", model, methodName)) {
		return fmt.Errorf("method %s already present in %s", methodName, path)
	}

	marker := endMarker(model)
	endIdx := strings.LastIndex(content, marker)
	if endIdx != -1 {
		lineEnd := strings.Index(content[endIdx:], "\n")
		if lineEnd != -1 {
			insertPos := endIdx + lineEnd + 1
			content = content[:insertPos] + stub + content[insertPos:]
		} else {
			content += stub
		}
	} else {
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += stub
	}

	return os.WriteFile(path, []byte(content), 0644)
}
