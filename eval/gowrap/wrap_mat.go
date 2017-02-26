package gowrap

import (
	"reflect"

	"mat"
)

func init() {
	Pkgs["mat"] = &Pkg{
		Exports: map[string]reflect.Value{
			"Matrix": reflect.ValueOf((*mat.Matrix)(nil)),
			"New":    reflect.ValueOf(mat.New),
		},
	}
}
