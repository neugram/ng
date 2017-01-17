// Generated file, do not edit.

package gowrap

import (
	errors "errors"
	"reflect"
)

var wrap_errors = &Pkg{
	Exports: map[string]reflect.Value{

		"New": reflect.ValueOf(errors.New),
	},
}

func init() {
	Pkgs["errors"] = wrap_errors
}
