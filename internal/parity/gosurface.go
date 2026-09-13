package parity

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var markerHelpers = map[string]int{
	"noYesSmile":       1,
	"noYesSmileVolume": 1,
	"sourceCheck":      0,
}

var locatorNames = map[string]bool{
	"t":   true,
	"loc": true,
}

var keyboardTypes = map[string]bool{
	"KeyboardButton":       true,
	"InlineKeyboardButton": true,
}

func LoadLocale(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	messages := map[string]string{}
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	locale := make(map[string]string, len(messages))
	for key, value := range messages {
		locale[strings.TrimPrefix(key, ".")] = strings.TrimPrefix(value, ".")
	}
	return locale, nil
}

func GoButtonLiterals(dir string, locale map[string]string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	var out []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok || !isKeyboardType(literal.Type) {
				return true
			}
			out = append(out, keyboardLabels(literal, locale)...)
			return false
		})
	}
	return out, nil
}

func isKeyboardType(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		return keyboardTypes[typed.Sel.Name]
	case *ast.ArrayType:
		return isKeyboardType(typed.Elt)
	case *ast.StarExpr:
		return isKeyboardType(typed.X)
	}
	return false
}

func keyboardLabels(literal *ast.CompositeLit, locale map[string]string) []string {
	var out []string
	for _, element := range literal.Elts {
		switch typed := element.(type) {
		case *ast.KeyValueExpr:
			if key, ok := typed.Key.(*ast.Ident); ok && key.Name == "Text" {
				out = append(out, labelValues(typed.Value, locale)...)
				continue
			}
			if nested, ok := typed.Value.(*ast.CompositeLit); ok {
				out = append(out, keyboardLabels(nested, locale)...)
			}
		case *ast.CompositeLit:
			out = append(out, keyboardLabels(typed, locale)...)
		}
	}
	return out
}

func labelValues(expr ast.Expr, locale map[string]string) []string {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		if typed.Kind != token.STRING {
			return nil
		}
		if value, err := strconv.Unquote(typed.Value); err == nil {
			return []string{value}
		}
	case *ast.BinaryExpr:
		if typed.Op != token.ADD {
			return nil
		}
		out := labelValues(typed.X, locale)
		return append(out, labelValues(typed.Y, locale)...)
	case *ast.ParenExpr:
		return labelValues(typed.X, locale)
	case *ast.CallExpr:
		return callLabelValues(typed, locale)
	}
	return nil
}

func callLabelValues(call *ast.CallExpr, locale map[string]string) []string {
	name := ""
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		name = fun.Name
	case *ast.SelectorExpr:
		name = fun.Sel.Name
	}

	if index, ok := markerHelpers[name]; ok && index < len(call.Args) {
		return labelValues(call.Args[index], locale)
	}
	if name == "Sprintf" && len(call.Args) > 0 {
		return labelValues(call.Args[0], locale)
	}
	if locatorNames[name] && len(call.Args) > 0 {
		if values := labelValues(call.Args[0], locale); len(values) > 0 {
			if value, ok := locale[values[0]]; ok {
				return []string{value}
			}
		}
	}
	return nil
}
