// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_fmt "fmt"
)

var pkg_wrap_fmt = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"Errorf":     reflect.ValueOf(wrap_fmt.Errorf),
		"Formatter":  reflect.ValueOf(reflect.TypeOf((*wrap_fmt.Formatter)(nil)).Elem()),
		"Fprint":     reflect.ValueOf(wrap_fmt.Fprint),
		"Fprintf":    reflect.ValueOf(wrap_fmt.Fprintf),
		"Fprintln":   reflect.ValueOf(wrap_fmt.Fprintln),
		"Fscan":      reflect.ValueOf(wrap_fmt.Fscan),
		"Fscanf":     reflect.ValueOf(wrap_fmt.Fscanf),
		"Fscanln":    reflect.ValueOf(wrap_fmt.Fscanln),
		"GoStringer": reflect.ValueOf(reflect.TypeOf((*wrap_fmt.GoStringer)(nil)).Elem()),
		"Print":      reflect.ValueOf(wrap_fmt.Print),
		"Printf":     reflect.ValueOf(wrap_fmt.Printf),
		"Println":    reflect.ValueOf(wrap_fmt.Println),
		"Scan":       reflect.ValueOf(wrap_fmt.Scan),
		"ScanState":  reflect.ValueOf(reflect.TypeOf((*wrap_fmt.ScanState)(nil)).Elem()),
		"Scanf":      reflect.ValueOf(wrap_fmt.Scanf),
		"Scanln":     reflect.ValueOf(wrap_fmt.Scanln),
		"Scanner":    reflect.ValueOf(reflect.TypeOf((*wrap_fmt.Scanner)(nil)).Elem()),
		"Sprint":     reflect.ValueOf(wrap_fmt.Sprint),
		"Sprintf":    reflect.ValueOf(wrap_fmt.Sprintf),
		"Sprintln":   reflect.ValueOf(wrap_fmt.Sprintln),
		"Sscan":      reflect.ValueOf(wrap_fmt.Sscan),
		"Sscanf":     reflect.ValueOf(wrap_fmt.Sscanf),
		"Sscanln":    reflect.ValueOf(wrap_fmt.Sscanln),
		"State":      reflect.ValueOf(reflect.TypeOf((*wrap_fmt.State)(nil)).Elem()),
		"Stringer":   reflect.ValueOf(reflect.TypeOf((*wrap_fmt.Stringer)(nil)).Elem()),
	},
}

func init() {
	if gowrap.Pkgs["fmt"] == nil {
		gowrap.Pkgs["fmt"] = pkg_wrap_fmt
	}
}
