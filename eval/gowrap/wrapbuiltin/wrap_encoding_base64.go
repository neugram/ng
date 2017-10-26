// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	encoding_base64 "encoding/base64"
)

var wrap_encoding_base64 = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"CorruptInputError": reflect.ValueOf(reflect.TypeOf(encoding_base64.CorruptInputError(0))),
		"Encoding":          reflect.ValueOf(reflect.TypeOf(encoding_base64.Encoding{})),
		"NewDecoder":        reflect.ValueOf(encoding_base64.NewDecoder),
		"NewEncoder":        reflect.ValueOf(encoding_base64.NewEncoder),
		"NewEncoding":       reflect.ValueOf(encoding_base64.NewEncoding),
		"NoPadding":         reflect.ValueOf(encoding_base64.NoPadding),
		"RawStdEncoding":    reflect.ValueOf(encoding_base64.RawStdEncoding),
		"RawURLEncoding":    reflect.ValueOf(encoding_base64.RawURLEncoding),
		"StdEncoding":       reflect.ValueOf(encoding_base64.StdEncoding),
		"StdPadding":        reflect.ValueOf(encoding_base64.StdPadding),
		"URLEncoding":       reflect.ValueOf(encoding_base64.URLEncoding),
	},
}

func init() {
	gowrap.Pkgs["encoding/base64"] = wrap_encoding_base64
}
