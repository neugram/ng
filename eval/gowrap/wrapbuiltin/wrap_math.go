// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_math "math"
)

var pkg_wrap_math = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"Abs":                    reflect.ValueOf(wrap_math.Abs),
		"Acos":                   reflect.ValueOf(wrap_math.Acos),
		"Acosh":                  reflect.ValueOf(wrap_math.Acosh),
		"Asin":                   reflect.ValueOf(wrap_math.Asin),
		"Asinh":                  reflect.ValueOf(wrap_math.Asinh),
		"Atan":                   reflect.ValueOf(wrap_math.Atan),
		"Atan2":                  reflect.ValueOf(wrap_math.Atan2),
		"Atanh":                  reflect.ValueOf(wrap_math.Atanh),
		"Cbrt":                   reflect.ValueOf(wrap_math.Cbrt),
		"Ceil":                   reflect.ValueOf(wrap_math.Ceil),
		"Copysign":               reflect.ValueOf(wrap_math.Copysign),
		"Cos":                    reflect.ValueOf(wrap_math.Cos),
		"Cosh":                   reflect.ValueOf(wrap_math.Cosh),
		"Dim":                    reflect.ValueOf(wrap_math.Dim),
		"E":                      reflect.ValueOf(wrap_math.E),
		"Erf":                    reflect.ValueOf(wrap_math.Erf),
		"Erfc":                   reflect.ValueOf(wrap_math.Erfc),
		"Exp":                    reflect.ValueOf(wrap_math.Exp),
		"Exp2":                   reflect.ValueOf(wrap_math.Exp2),
		"Expm1":                  reflect.ValueOf(wrap_math.Expm1),
		"Float32bits":            reflect.ValueOf(wrap_math.Float32bits),
		"Float32frombits":        reflect.ValueOf(wrap_math.Float32frombits),
		"Float64bits":            reflect.ValueOf(wrap_math.Float64bits),
		"Float64frombits":        reflect.ValueOf(wrap_math.Float64frombits),
		"Floor":                  reflect.ValueOf(wrap_math.Floor),
		"Frexp":                  reflect.ValueOf(wrap_math.Frexp),
		"Gamma":                  reflect.ValueOf(wrap_math.Gamma),
		"Hypot":                  reflect.ValueOf(wrap_math.Hypot),
		"Ilogb":                  reflect.ValueOf(wrap_math.Ilogb),
		"Inf":                    reflect.ValueOf(wrap_math.Inf),
		"IsInf":                  reflect.ValueOf(wrap_math.IsInf),
		"IsNaN":                  reflect.ValueOf(wrap_math.IsNaN),
		"J0":                     reflect.ValueOf(wrap_math.J0),
		"J1":                     reflect.ValueOf(wrap_math.J1),
		"Jn":                     reflect.ValueOf(wrap_math.Jn),
		"Ldexp":                  reflect.ValueOf(wrap_math.Ldexp),
		"Lgamma":                 reflect.ValueOf(wrap_math.Lgamma),
		"Ln10":                   reflect.ValueOf(wrap_math.Ln10),
		"Ln2":                    reflect.ValueOf(wrap_math.Ln2),
		"Log":                    reflect.ValueOf(wrap_math.Log),
		"Log10":                  reflect.ValueOf(wrap_math.Log10),
		"Log10E":                 reflect.ValueOf(wrap_math.Log10E),
		"Log1p":                  reflect.ValueOf(wrap_math.Log1p),
		"Log2":                   reflect.ValueOf(wrap_math.Log2),
		"Log2E":                  reflect.ValueOf(wrap_math.Log2E),
		"Logb":                   reflect.ValueOf(wrap_math.Logb),
		"Max":                    reflect.ValueOf(wrap_math.Max),
		"MaxFloat32":             reflect.ValueOf(wrap_math.MaxFloat32),
		"MaxFloat64":             reflect.ValueOf(wrap_math.MaxFloat64),
		"MaxInt16":               reflect.ValueOf(wrap_math.MaxInt16),
		"MaxInt32":               reflect.ValueOf(wrap_math.MaxInt32),
		"MaxInt64":               reflect.ValueOf(wrap_math.MaxInt64),
		"MaxInt8":                reflect.ValueOf(wrap_math.MaxInt8),
		"MaxUint16":              reflect.ValueOf(wrap_math.MaxUint16),
		"MaxUint32":              reflect.ValueOf(wrap_math.MaxUint32),
		"MaxUint64":              reflect.ValueOf(uint64(wrap_math.MaxUint64)),
		"MaxUint8":               reflect.ValueOf(wrap_math.MaxUint8),
		"Min":                    reflect.ValueOf(wrap_math.Min),
		"MinInt16":               reflect.ValueOf(wrap_math.MinInt16),
		"MinInt32":               reflect.ValueOf(wrap_math.MinInt32),
		"MinInt64":               reflect.ValueOf(wrap_math.MinInt64),
		"MinInt8":                reflect.ValueOf(wrap_math.MinInt8),
		"Mod":                    reflect.ValueOf(wrap_math.Mod),
		"Modf":                   reflect.ValueOf(wrap_math.Modf),
		"NaN":                    reflect.ValueOf(wrap_math.NaN),
		"Nextafter":              reflect.ValueOf(wrap_math.Nextafter),
		"Nextafter32":            reflect.ValueOf(wrap_math.Nextafter32),
		"Phi":                    reflect.ValueOf(wrap_math.Phi),
		"Pi":                     reflect.ValueOf(wrap_math.Pi),
		"Pow":                    reflect.ValueOf(wrap_math.Pow),
		"Pow10":                  reflect.ValueOf(wrap_math.Pow10),
		"Remainder":              reflect.ValueOf(wrap_math.Remainder),
		"Signbit":                reflect.ValueOf(wrap_math.Signbit),
		"Sin":                    reflect.ValueOf(wrap_math.Sin),
		"Sincos":                 reflect.ValueOf(wrap_math.Sincos),
		"Sinh":                   reflect.ValueOf(wrap_math.Sinh),
		"SmallestNonzeroFloat32": reflect.ValueOf(wrap_math.SmallestNonzeroFloat32),
		"SmallestNonzeroFloat64": reflect.ValueOf(wrap_math.SmallestNonzeroFloat64),
		"Sqrt":                   reflect.ValueOf(wrap_math.Sqrt),
		"Sqrt2":                  reflect.ValueOf(wrap_math.Sqrt2),
		"SqrtE":                  reflect.ValueOf(wrap_math.SqrtE),
		"SqrtPhi":                reflect.ValueOf(wrap_math.SqrtPhi),
		"SqrtPi":                 reflect.ValueOf(wrap_math.SqrtPi),
		"Tan":                    reflect.ValueOf(wrap_math.Tan),
		"Tanh":                   reflect.ValueOf(wrap_math.Tanh),
		"Trunc":                  reflect.ValueOf(wrap_math.Trunc),
		"Y0":                     reflect.ValueOf(wrap_math.Y0),
		"Y1":                     reflect.ValueOf(wrap_math.Y1),
		"Yn":                     reflect.ValueOf(wrap_math.Yn),
	},
}

func init() {
	if gowrap.Pkgs["math"] == nil {
		gowrap.Pkgs["math"] = pkg_wrap_math
	}
}
