package main

import (
	"strings"
	"unicode"
)

// toSnakeCase converts "FullName" / "fullName" / "full_name" → "full_name".
func toSnakeCase(s string) string {
	runes := []rune(s)
	var out []rune
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && !unicode.IsUpper(runes[i-1]) {
			out = append(out, '_')
		}
		out = append(out, unicode.ToLower(r))
	}
	return string(out)
}

// Common Go initialisms — these are capitalised in full.
var initialisms = map[string]string{
	"id":    "ID",
	"url":   "URL",
	"urls":  "URLs",
	"uri":   "URI",
	"http":  "HTTP",
	"https": "HTTPS",
	"api":   "API",
	"json":  "JSON",
	"xml":   "XML",
	"db":    "DB",
	"sql":   "SQL",
	"ssh":   "SSH",
	"tls":   "TLS",
	"ip":    "IP",
	"rpc":   "RPC",
	"eof":   "EOF",
}

// toPascalCase converts "full_name" / "fullName" / "FullName" → "FullName".
// Respects common Go initialisms: "language_id" → "LanguageID".
func toPascalCase(s string) string {
	parts := strings.Split(toSnakeCase(s), "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		if upper, ok := initialisms[p]; ok {
			b.WriteString(upper)
			continue
		}
		b.WriteRune(unicode.ToUpper(rune(p[0])))
		b.WriteString(p[1:])
	}
	return b.String()
}

// toPlural applies simple English pluralization to a snake_case word.
func toPlural(s string) string {
	if strings.HasSuffix(s, "y") && len(s) > 1 {
		vowels := "aeiou"
		if !strings.ContainsRune(vowels, rune(s[len(s)-2])) {
			return s[:len(s)-1] + "ies"
		}
	}
	for _, suf := range []string{"ch", "sh", "ss", "x", "z"} {
		if strings.HasSuffix(s, suf) {
			return s + "es"
		}
	}
	if !strings.HasSuffix(s, "s") {
		return s + "s"
	}
	return s
}

// pluralPascal returns the PascalCase plural of a model name (e.g. "Product" → "Products").
func pluralPascal(model string) string {
	return toPascalCase(toPlural(toSnakeCase(model)))
}

// tableOf returns the snake_case plural table name for a model (e.g. "WordAssociation" → "word_associations").
func tableOf(model string) string {
	return toPlural(toSnakeCase(model))
}
