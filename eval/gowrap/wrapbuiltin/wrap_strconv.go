// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_strconv "strconv"
)

var pkg_wrap_strconv = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"AppendBool":               reflect.ValueOf(wrap_strconv.AppendBool),
		"AppendFloat":              reflect.ValueOf(wrap_strconv.AppendFloat),
		"AppendInt":                reflect.ValueOf(wrap_strconv.AppendInt),
		"AppendQuote":              reflect.ValueOf(wrap_strconv.AppendQuote),
		"AppendQuoteRune":          reflect.ValueOf(wrap_strconv.AppendQuoteRune),
		"AppendQuoteRuneToASCII":   reflect.ValueOf(wrap_strconv.AppendQuoteRuneToASCII),
		"AppendQuoteRuneToGraphic": reflect.ValueOf(wrap_strconv.AppendQuoteRuneToGraphic),
		"AppendQuoteToASCII":       reflect.ValueOf(wrap_strconv.AppendQuoteToASCII),
		"AppendQuoteToGraphic":     reflect.ValueOf(wrap_strconv.AppendQuoteToGraphic),
		"AppendUint":               reflect.ValueOf(wrap_strconv.AppendUint),
		"Atoi":                     reflect.ValueOf(wrap_strconv.Atoi),
		"CanBackquote":             reflect.ValueOf(wrap_strconv.CanBackquote),
		"ErrRange":                 reflect.ValueOf(wrap_strconv.ErrRange),
		"ErrSyntax":                reflect.ValueOf(wrap_strconv.ErrSyntax),
		"FormatBool":               reflect.ValueOf(wrap_strconv.FormatBool),
		"FormatFloat":              reflect.ValueOf(wrap_strconv.FormatFloat),
		"FormatInt":                reflect.ValueOf(wrap_strconv.FormatInt),
		"FormatUint":               reflect.ValueOf(wrap_strconv.FormatUint),
		"IntSize":                  reflect.ValueOf(wrap_strconv.IntSize),
		"IsGraphic":                reflect.ValueOf(wrap_strconv.IsGraphic),
		"IsPrint":                  reflect.ValueOf(wrap_strconv.IsPrint),
		"Itoa":                     reflect.ValueOf(wrap_strconv.Itoa),
		"NumError":                 reflect.ValueOf(reflect.TypeOf(wrap_strconv.NumError{})),
		"ParseBool":                reflect.ValueOf(wrap_strconv.ParseBool),
		"ParseFloat":               reflect.ValueOf(wrap_strconv.ParseFloat),
		"ParseInt":                 reflect.ValueOf(wrap_strconv.ParseInt),
		"ParseUint":                reflect.ValueOf(wrap_strconv.ParseUint),
		"Quote":                    reflect.ValueOf(wrap_strconv.Quote),
		"QuoteRune":                reflect.ValueOf(wrap_strconv.QuoteRune),
		"QuoteRuneToASCII":         reflect.ValueOf(wrap_strconv.QuoteRuneToASCII),
		"QuoteRuneToGraphic":       reflect.ValueOf(wrap_strconv.QuoteRuneToGraphic),
		"QuoteToASCII":             reflect.ValueOf(wrap_strconv.QuoteToASCII),
		"QuoteToGraphic":           reflect.ValueOf(wrap_strconv.QuoteToGraphic),
		"Unquote":                  reflect.ValueOf(wrap_strconv.Unquote),
		"UnquoteChar":              reflect.ValueOf(wrap_strconv.UnquoteChar),
	},
}

func init() {
	if gowrap.Pkgs["strconv"] == nil {
		gowrap.Pkgs["strconv"] = pkg_wrap_strconv
	}
}
