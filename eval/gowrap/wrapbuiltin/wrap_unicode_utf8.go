// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_unicode_utf8 "unicode/utf8"
)

var pkg_wrap_unicode_utf8 = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"DecodeLastRune":         reflect.ValueOf(wrap_unicode_utf8.DecodeLastRune),
		"DecodeLastRuneInString": reflect.ValueOf(wrap_unicode_utf8.DecodeLastRuneInString),
		"DecodeRune":             reflect.ValueOf(wrap_unicode_utf8.DecodeRune),
		"DecodeRuneInString":     reflect.ValueOf(wrap_unicode_utf8.DecodeRuneInString),
		"EncodeRune":             reflect.ValueOf(wrap_unicode_utf8.EncodeRune),
		"FullRune":               reflect.ValueOf(wrap_unicode_utf8.FullRune),
		"FullRuneInString":       reflect.ValueOf(wrap_unicode_utf8.FullRuneInString),
		"MaxRune":                reflect.ValueOf(wrap_unicode_utf8.MaxRune),
		"RuneCount":              reflect.ValueOf(wrap_unicode_utf8.RuneCount),
		"RuneCountInString":      reflect.ValueOf(wrap_unicode_utf8.RuneCountInString),
		"RuneError":              reflect.ValueOf(wrap_unicode_utf8.RuneError),
		"RuneLen":                reflect.ValueOf(wrap_unicode_utf8.RuneLen),
		"RuneSelf":               reflect.ValueOf(wrap_unicode_utf8.RuneSelf),
		"RuneStart":              reflect.ValueOf(wrap_unicode_utf8.RuneStart),
		"UTFMax":                 reflect.ValueOf(wrap_unicode_utf8.UTFMax),
		"Valid":                  reflect.ValueOf(wrap_unicode_utf8.Valid),
		"ValidRune":              reflect.ValueOf(wrap_unicode_utf8.ValidRune),
		"ValidString":            reflect.ValueOf(wrap_unicode_utf8.ValidString),
	},
}

func init() {
	if gowrap.Pkgs["unicode/utf8"] == nil {
		gowrap.Pkgs["unicode/utf8"] = pkg_wrap_unicode_utf8
	}
}
