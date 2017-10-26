// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	strconv "strconv"
)

var wrap_strconv = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"AppendBool":               reflect.ValueOf(strconv.AppendBool),
		"AppendFloat":              reflect.ValueOf(strconv.AppendFloat),
		"AppendInt":                reflect.ValueOf(strconv.AppendInt),
		"AppendQuote":              reflect.ValueOf(strconv.AppendQuote),
		"AppendQuoteRune":          reflect.ValueOf(strconv.AppendQuoteRune),
		"AppendQuoteRuneToASCII":   reflect.ValueOf(strconv.AppendQuoteRuneToASCII),
		"AppendQuoteRuneToGraphic": reflect.ValueOf(strconv.AppendQuoteRuneToGraphic),
		"AppendQuoteToASCII":       reflect.ValueOf(strconv.AppendQuoteToASCII),
		"AppendQuoteToGraphic":     reflect.ValueOf(strconv.AppendQuoteToGraphic),
		"AppendUint":               reflect.ValueOf(strconv.AppendUint),
		"Atoi":                     reflect.ValueOf(strconv.Atoi),
		"CanBackquote":             reflect.ValueOf(strconv.CanBackquote),
		"ErrRange":                 reflect.ValueOf(strconv.ErrRange),
		"ErrSyntax":                reflect.ValueOf(strconv.ErrSyntax),
		"FormatBool":               reflect.ValueOf(strconv.FormatBool),
		"FormatFloat":              reflect.ValueOf(strconv.FormatFloat),
		"FormatInt":                reflect.ValueOf(strconv.FormatInt),
		"FormatUint":               reflect.ValueOf(strconv.FormatUint),
		"IntSize":                  reflect.ValueOf(strconv.IntSize),
		"IsGraphic":                reflect.ValueOf(strconv.IsGraphic),
		"IsPrint":                  reflect.ValueOf(strconv.IsPrint),
		"Itoa":                     reflect.ValueOf(strconv.Itoa),
		"NumError":                 reflect.ValueOf(reflect.TypeOf(strconv.NumError{})),
		"ParseBool":                reflect.ValueOf(strconv.ParseBool),
		"ParseFloat":               reflect.ValueOf(strconv.ParseFloat),
		"ParseInt":                 reflect.ValueOf(strconv.ParseInt),
		"ParseUint":                reflect.ValueOf(strconv.ParseUint),
		"Quote":                    reflect.ValueOf(strconv.Quote),
		"QuoteRune":                reflect.ValueOf(strconv.QuoteRune),
		"QuoteRuneToASCII":         reflect.ValueOf(strconv.QuoteRuneToASCII),
		"QuoteRuneToGraphic":       reflect.ValueOf(strconv.QuoteRuneToGraphic),
		"QuoteToASCII":             reflect.ValueOf(strconv.QuoteToASCII),
		"QuoteToGraphic":           reflect.ValueOf(strconv.QuoteToGraphic),
		"Unquote":                  reflect.ValueOf(strconv.Unquote),
		"UnquoteChar":              reflect.ValueOf(strconv.UnquoteChar),
	},
}

func init() {
	gowrap.Pkgs["strconv"] = wrap_strconv
}
