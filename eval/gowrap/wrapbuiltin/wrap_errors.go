// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	errors "errors"
)

var wrap_errors = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"New": reflect.ValueOf(errors.New),
	},
}

func init() {
	gowrap.Pkgs["errors"] = wrap_errors
}
