package driver

import (
	"fmt"
	"reflect"
	"strings"

	"abc/internal/ast"
	"abc/internal/token"
)

// dumpNode writes an indented representation of an AST to b.
func dumpNode(b *strings.Builder, v interface{}, depth int) {
	if v == nil {
		return
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	indent := strings.Repeat("  ", depth)
	switch rv.Kind() {
	case reflect.Struct:
		name := rv.Type().Name()
		if _, ok := v.(ast.Stmt); ok || name != "" {
			// print a short header with the most interesting fields
			line := indent + name
			if p, ok := v.(ast.Node); ok {
				line += fmt.Sprintf(" @%d:%d", p.Pos().Line, p.Pos().Col)
			}
			for _, f := range []string{"Name", "Op", "Label", "Value", "Text", "Mode", "Kind"} {
				if fv := rv.FieldByName(f); fv.IsValid() && fv.Kind() != reflect.Slice && fv.Kind() != reflect.Struct && fv.Kind() != reflect.Ptr {
					line += fmt.Sprintf(" %s=%v", f, fv.Interface())
				}
			}
			b.WriteString(line + "\n")
		}
		for i := 0; i < rv.NumField(); i++ {
			fv := rv.Field(i)
			if !fv.CanInterface() {
				continue
			}
			if fv.Kind() == reflect.Ptr && fv.IsNil() {
				continue
			}
			ft := rv.Type().Field(i)
			if ft.Name == "At" || ft.PkgPath != "" {
				continue
			}
			if fv.Kind() == reflect.Slice && fv.Len() == 0 {
				continue
			}
			if fv.Kind() == reflect.Interface && fv.IsNil() {
				continue
			}
			if fv.Kind() == reflect.String && fv.String() == "" {
				continue
			}
			dumpNode(b, fv.Interface(), depth+1)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			dumpNode(b, rv.Index(i).Interface(), depth)
		}
	case reflect.Ptr:
		dumpNode(b, rv.Elem().Interface(), depth)
	default:
		b.WriteString(indent + fmt.Sprint(rv.Interface()) + "\n")
	}
	_ = token.EOF
}
