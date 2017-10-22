// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	unicode_utf8 "unicode/utf8"
)

var wrap_unicode_utf8 = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"DecodeLastRune":         reflect.ValueOf(unicode_utf8.DecodeLastRune),
		"DecodeLastRuneInString": reflect.ValueOf(unicode_utf8.DecodeLastRuneInString),
		"DecodeRune":             reflect.ValueOf(unicode_utf8.DecodeRune),
		"DecodeRuneInString":     reflect.ValueOf(unicode_utf8.DecodeRuneInString),
		"EncodeRune":             reflect.ValueOf(unicode_utf8.EncodeRune),
		"FullRune":               reflect.ValueOf(unicode_utf8.FullRune),
		"FullRuneInString":       reflect.ValueOf(unicode_utf8.FullRuneInString),
		"MaxRune":                reflect.ValueOf(unicode_utf8.MaxRune),
		"RuneCount":              reflect.ValueOf(unicode_utf8.RuneCount),
		"RuneCountInString":      reflect.ValueOf(unicode_utf8.RuneCountInString),
		"RuneError":              reflect.ValueOf(unicode_utf8.RuneError),
		"RuneLen":                reflect.ValueOf(unicode_utf8.RuneLen),
		"RuneSelf":               reflect.ValueOf(unicode_utf8.RuneSelf),
		"RuneStart":              reflect.ValueOf(unicode_utf8.RuneStart),
		"UTFMax":                 reflect.ValueOf(unicode_utf8.UTFMax),
		"Valid":                  reflect.ValueOf(unicode_utf8.Valid),
		"ValidRune":              reflect.ValueOf(unicode_utf8.ValidRune),
		"ValidString":            reflect.ValueOf(unicode_utf8.ValidString),
	},
}

func init() {
	gowrap.Pkgs["unicode/utf8"] = wrap_unicode_utf8
}
