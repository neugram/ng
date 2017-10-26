// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	fmt "fmt"
)

var wrap_fmt = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"Errorf":     reflect.ValueOf(fmt.Errorf),
		"Formatter":  reflect.ValueOf(reflect.TypeOf((*fmt.Formatter)(nil)).Elem()),
		"Fprint":     reflect.ValueOf(fmt.Fprint),
		"Fprintf":    reflect.ValueOf(fmt.Fprintf),
		"Fprintln":   reflect.ValueOf(fmt.Fprintln),
		"Fscan":      reflect.ValueOf(fmt.Fscan),
		"Fscanf":     reflect.ValueOf(fmt.Fscanf),
		"Fscanln":    reflect.ValueOf(fmt.Fscanln),
		"GoStringer": reflect.ValueOf(reflect.TypeOf((*fmt.GoStringer)(nil)).Elem()),
		"Print":      reflect.ValueOf(fmt.Print),
		"Printf":     reflect.ValueOf(fmt.Printf),
		"Println":    reflect.ValueOf(fmt.Println),
		"Scan":       reflect.ValueOf(fmt.Scan),
		"ScanState":  reflect.ValueOf(reflect.TypeOf((*fmt.ScanState)(nil)).Elem()),
		"Scanf":      reflect.ValueOf(fmt.Scanf),
		"Scanln":     reflect.ValueOf(fmt.Scanln),
		"Scanner":    reflect.ValueOf(reflect.TypeOf((*fmt.Scanner)(nil)).Elem()),
		"Sprint":     reflect.ValueOf(fmt.Sprint),
		"Sprintf":    reflect.ValueOf(fmt.Sprintf),
		"Sprintln":   reflect.ValueOf(fmt.Sprintln),
		"Sscan":      reflect.ValueOf(fmt.Sscan),
		"Sscanf":     reflect.ValueOf(fmt.Sscanf),
		"Sscanln":    reflect.ValueOf(fmt.Sscanln),
		"State":      reflect.ValueOf(reflect.TypeOf((*fmt.State)(nil)).Elem()),
		"Stringer":   reflect.ValueOf(reflect.TypeOf((*fmt.Stringer)(nil)).Elem()),
	},
}

func init() {
	gowrap.Pkgs["fmt"] = wrap_fmt
}
