// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_bytes "bytes"
)

var pkg_wrap_bytes = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"Buffer":          reflect.ValueOf(reflect.TypeOf(wrap_bytes.Buffer{})),
		"Compare":         reflect.ValueOf(wrap_bytes.Compare),
		"Contains":        reflect.ValueOf(wrap_bytes.Contains),
		"ContainsAny":     reflect.ValueOf(wrap_bytes.ContainsAny),
		"ContainsRune":    reflect.ValueOf(wrap_bytes.ContainsRune),
		"Count":           reflect.ValueOf(wrap_bytes.Count),
		"Equal":           reflect.ValueOf(wrap_bytes.Equal),
		"EqualFold":       reflect.ValueOf(wrap_bytes.EqualFold),
		"ErrTooLarge":     reflect.ValueOf(&wrap_bytes.ErrTooLarge).Elem(),
		"Fields":          reflect.ValueOf(wrap_bytes.Fields),
		"FieldsFunc":      reflect.ValueOf(wrap_bytes.FieldsFunc),
		"HasPrefix":       reflect.ValueOf(wrap_bytes.HasPrefix),
		"HasSuffix":       reflect.ValueOf(wrap_bytes.HasSuffix),
		"Index":           reflect.ValueOf(wrap_bytes.Index),
		"IndexAny":        reflect.ValueOf(wrap_bytes.IndexAny),
		"IndexByte":       reflect.ValueOf(wrap_bytes.IndexByte),
		"IndexFunc":       reflect.ValueOf(wrap_bytes.IndexFunc),
		"IndexRune":       reflect.ValueOf(wrap_bytes.IndexRune),
		"Join":            reflect.ValueOf(wrap_bytes.Join),
		"LastIndex":       reflect.ValueOf(wrap_bytes.LastIndex),
		"LastIndexAny":    reflect.ValueOf(wrap_bytes.LastIndexAny),
		"LastIndexByte":   reflect.ValueOf(wrap_bytes.LastIndexByte),
		"LastIndexFunc":   reflect.ValueOf(wrap_bytes.LastIndexFunc),
		"Map":             reflect.ValueOf(wrap_bytes.Map),
		"MinRead":         reflect.ValueOf(wrap_bytes.MinRead),
		"NewBuffer":       reflect.ValueOf(wrap_bytes.NewBuffer),
		"NewBufferString": reflect.ValueOf(wrap_bytes.NewBufferString),
		"NewReader":       reflect.ValueOf(wrap_bytes.NewReader),
		"Reader":          reflect.ValueOf(reflect.TypeOf(wrap_bytes.Reader{})),
		"Repeat":          reflect.ValueOf(wrap_bytes.Repeat),
		"Replace":         reflect.ValueOf(wrap_bytes.Replace),
		"Runes":           reflect.ValueOf(wrap_bytes.Runes),
		"Split":           reflect.ValueOf(wrap_bytes.Split),
		"SplitAfter":      reflect.ValueOf(wrap_bytes.SplitAfter),
		"SplitAfterN":     reflect.ValueOf(wrap_bytes.SplitAfterN),
		"SplitN":          reflect.ValueOf(wrap_bytes.SplitN),
		"Title":           reflect.ValueOf(wrap_bytes.Title),
		"ToLower":         reflect.ValueOf(wrap_bytes.ToLower),
		"ToLowerSpecial":  reflect.ValueOf(wrap_bytes.ToLowerSpecial),
		"ToTitle":         reflect.ValueOf(wrap_bytes.ToTitle),
		"ToTitleSpecial":  reflect.ValueOf(wrap_bytes.ToTitleSpecial),
		"ToUpper":         reflect.ValueOf(wrap_bytes.ToUpper),
		"ToUpperSpecial":  reflect.ValueOf(wrap_bytes.ToUpperSpecial),
		"Trim":            reflect.ValueOf(wrap_bytes.Trim),
		"TrimFunc":        reflect.ValueOf(wrap_bytes.TrimFunc),
		"TrimLeft":        reflect.ValueOf(wrap_bytes.TrimLeft),
		"TrimLeftFunc":    reflect.ValueOf(wrap_bytes.TrimLeftFunc),
		"TrimPrefix":      reflect.ValueOf(wrap_bytes.TrimPrefix),
		"TrimRight":       reflect.ValueOf(wrap_bytes.TrimRight),
		"TrimRightFunc":   reflect.ValueOf(wrap_bytes.TrimRightFunc),
		"TrimSpace":       reflect.ValueOf(wrap_bytes.TrimSpace),
		"TrimSuffix":      reflect.ValueOf(wrap_bytes.TrimSuffix),
	},
}

func init() {
	if gowrap.Pkgs["bytes"] == nil {
		gowrap.Pkgs["bytes"] = pkg_wrap_bytes
	}
}
