// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_errors "errors"
)

var pkg_wrap_errors = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"New": reflect.ValueOf(wrap_errors.New),
	},
}

func init() {
	if gowrap.Pkgs["errors"] == nil {
		gowrap.Pkgs["errors"] = pkg_wrap_errors
	}
}
