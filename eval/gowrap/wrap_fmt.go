// Generated file, do not edit.

package gowrap

import "fmt"

var wrap_fmt = &Pkg{
	Exports: map[string]interface{}{

		"Errorf":     fmt.Errorf,
		"Formatter":  fmt.Formatter(nil),
		"Fprint":     fmt.Fprint,
		"Fprintf":    fmt.Fprintf,
		"Fprintln":   fmt.Fprintln,
		"Fscan":      fmt.Fscan,
		"Fscanf":     fmt.Fscanf,
		"Fscanln":    fmt.Fscanln,
		"GoStringer": fmt.GoStringer(nil),
		"Print":      fmt.Print,
		"Printf":     fmt.Printf,
		"Println":    fmt.Println,
		"Scan":       fmt.Scan,
		"ScanState":  fmt.ScanState(nil),
		"Scanf":      fmt.Scanf,
		"Scanln":     fmt.Scanln,
		"Scanner":    fmt.Scanner(nil),
		"Sprint":     fmt.Sprint,
		"Sprintf":    fmt.Sprintf,
		"Sprintln":   fmt.Sprintln,
		"Sscan":      fmt.Sscan,
		"Sscanf":     fmt.Sscanf,
		"Sscanln":    fmt.Sscanln,
		"State":      fmt.State(nil),
		"Stringer":   fmt.Stringer(nil),
	},
}

func init() {
	Pkgs["fmt"] = wrap_fmt
}
