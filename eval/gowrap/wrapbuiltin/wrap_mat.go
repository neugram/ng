// TODO(crawshaw): mat is temporary, remove it

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	"mat"
)

func init() {
	gowrap.Pkgs["mat"] = &gowrap.Pkg{
		Exports: map[string]reflect.Value{
			"Matrix": reflect.ValueOf((*mat.Matrix)(nil)),
			"New":    reflect.ValueOf(mat.New),
		},
	}
}
