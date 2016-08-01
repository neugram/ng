// Generated file, do not edit.

package gowrap

import (
	"fmt"
	"reflect"
)

var wrap_fmt = &Pkg{
	Exports: map[string]reflect.Value{

		"Errorf":     reflect.ValueOf(fmt.Errorf),
		"Formatter":  reflect.ValueOf((*fmt.Formatter)(nil)),
		"Fprint":     reflect.ValueOf(fmt.Fprint),
		"Fprintf":    reflect.ValueOf(fmt.Fprintf),
		"Fprintln":   reflect.ValueOf(fmt.Fprintln),
		"Fscan":      reflect.ValueOf(fmt.Fscan),
		"Fscanf":     reflect.ValueOf(fmt.Fscanf),
		"Fscanln":    reflect.ValueOf(fmt.Fscanln),
		"GoStringer": reflect.ValueOf((*fmt.GoStringer)(nil)),
		"Print":      reflect.ValueOf(fmt.Print),
		"Printf":     reflect.ValueOf(fmt.Printf),
		"Println":    reflect.ValueOf(fmt.Println),
		"Scan":       reflect.ValueOf(fmt.Scan),
		"ScanState":  reflect.ValueOf((*fmt.ScanState)(nil)),
		"Scanf":      reflect.ValueOf(fmt.Scanf),
		"Scanln":     reflect.ValueOf(fmt.Scanln),
		"Scanner":    reflect.ValueOf((*fmt.Scanner)(nil)),
		"Sprint":     reflect.ValueOf(fmt.Sprint),
		"Sprintf":    reflect.ValueOf(fmt.Sprintf),
		"Sprintln":   reflect.ValueOf(fmt.Sprintln),
		"Sscan":      reflect.ValueOf(fmt.Sscan),
		"Sscanf":     reflect.ValueOf(fmt.Sscanf),
		"Sscanln":    reflect.ValueOf(fmt.Sscanln),
		"State":      reflect.ValueOf((*fmt.State)(nil)),
		"Stringer":   reflect.ValueOf((*fmt.Stringer)(nil)),
	},
}

func init() {
	Pkgs["fmt"] = wrap_fmt
}
