// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_strings "strings"
)

var pkg_wrap_strings = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"Compare":        reflect.ValueOf(wrap_strings.Compare),
		"Contains":       reflect.ValueOf(wrap_strings.Contains),
		"ContainsAny":    reflect.ValueOf(wrap_strings.ContainsAny),
		"ContainsRune":   reflect.ValueOf(wrap_strings.ContainsRune),
		"Count":          reflect.ValueOf(wrap_strings.Count),
		"EqualFold":      reflect.ValueOf(wrap_strings.EqualFold),
		"Fields":         reflect.ValueOf(wrap_strings.Fields),
		"FieldsFunc":     reflect.ValueOf(wrap_strings.FieldsFunc),
		"HasPrefix":      reflect.ValueOf(wrap_strings.HasPrefix),
		"HasSuffix":      reflect.ValueOf(wrap_strings.HasSuffix),
		"Index":          reflect.ValueOf(wrap_strings.Index),
		"IndexAny":       reflect.ValueOf(wrap_strings.IndexAny),
		"IndexByte":      reflect.ValueOf(wrap_strings.IndexByte),
		"IndexFunc":      reflect.ValueOf(wrap_strings.IndexFunc),
		"IndexRune":      reflect.ValueOf(wrap_strings.IndexRune),
		"Join":           reflect.ValueOf(wrap_strings.Join),
		"LastIndex":      reflect.ValueOf(wrap_strings.LastIndex),
		"LastIndexAny":   reflect.ValueOf(wrap_strings.LastIndexAny),
		"LastIndexByte":  reflect.ValueOf(wrap_strings.LastIndexByte),
		"LastIndexFunc":  reflect.ValueOf(wrap_strings.LastIndexFunc),
		"Map":            reflect.ValueOf(wrap_strings.Map),
		"NewReader":      reflect.ValueOf(wrap_strings.NewReader),
		"NewReplacer":    reflect.ValueOf(wrap_strings.NewReplacer),
		"Reader":         reflect.ValueOf(reflect.TypeOf(wrap_strings.Reader{})),
		"Repeat":         reflect.ValueOf(wrap_strings.Repeat),
		"Replace":        reflect.ValueOf(wrap_strings.Replace),
		"Replacer":       reflect.ValueOf(reflect.TypeOf(wrap_strings.Replacer{})),
		"Split":          reflect.ValueOf(wrap_strings.Split),
		"SplitAfter":     reflect.ValueOf(wrap_strings.SplitAfter),
		"SplitAfterN":    reflect.ValueOf(wrap_strings.SplitAfterN),
		"SplitN":         reflect.ValueOf(wrap_strings.SplitN),
		"Title":          reflect.ValueOf(wrap_strings.Title),
		"ToLower":        reflect.ValueOf(wrap_strings.ToLower),
		"ToLowerSpecial": reflect.ValueOf(wrap_strings.ToLowerSpecial),
		"ToTitle":        reflect.ValueOf(wrap_strings.ToTitle),
		"ToTitleSpecial": reflect.ValueOf(wrap_strings.ToTitleSpecial),
		"ToUpper":        reflect.ValueOf(wrap_strings.ToUpper),
		"ToUpperSpecial": reflect.ValueOf(wrap_strings.ToUpperSpecial),
		"Trim":           reflect.ValueOf(wrap_strings.Trim),
		"TrimFunc":       reflect.ValueOf(wrap_strings.TrimFunc),
		"TrimLeft":       reflect.ValueOf(wrap_strings.TrimLeft),
		"TrimLeftFunc":   reflect.ValueOf(wrap_strings.TrimLeftFunc),
		"TrimPrefix":     reflect.ValueOf(wrap_strings.TrimPrefix),
		"TrimRight":      reflect.ValueOf(wrap_strings.TrimRight),
		"TrimRightFunc":  reflect.ValueOf(wrap_strings.TrimRightFunc),
		"TrimSpace":      reflect.ValueOf(wrap_strings.TrimSpace),
		"TrimSuffix":     reflect.ValueOf(wrap_strings.TrimSuffix),
	},
}

func init() {
	if gowrap.Pkgs["strings"] == nil {
		gowrap.Pkgs["strings"] = pkg_wrap_strings
	}
}
