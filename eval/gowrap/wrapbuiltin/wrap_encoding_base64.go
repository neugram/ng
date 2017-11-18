// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_encoding_base64 "encoding/base64"
)

var pkg_wrap_encoding_base64 = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"CorruptInputError": reflect.ValueOf(reflect.TypeOf(wrap_encoding_base64.CorruptInputError(0))),
		"Encoding":          reflect.ValueOf(reflect.TypeOf(wrap_encoding_base64.Encoding{})),
		"NewDecoder":        reflect.ValueOf(wrap_encoding_base64.NewDecoder),
		"NewEncoder":        reflect.ValueOf(wrap_encoding_base64.NewEncoder),
		"NewEncoding":       reflect.ValueOf(wrap_encoding_base64.NewEncoding),
		"NoPadding":         reflect.ValueOf(wrap_encoding_base64.NoPadding),
		"RawStdEncoding":    reflect.ValueOf(wrap_encoding_base64.RawStdEncoding),
		"RawURLEncoding":    reflect.ValueOf(wrap_encoding_base64.RawURLEncoding),
		"StdEncoding":       reflect.ValueOf(wrap_encoding_base64.StdEncoding),
		"StdPadding":        reflect.ValueOf(wrap_encoding_base64.StdPadding),
		"URLEncoding":       reflect.ValueOf(wrap_encoding_base64.URLEncoding),
	},
}

func init() {
	if gowrap.Pkgs["encoding/base64"] == nil {
		gowrap.Pkgs["encoding/base64"] = pkg_wrap_encoding_base64
	}
}
