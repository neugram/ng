// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_runtime "runtime"
)

var pkg_wrap_runtime = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"BlockProfile":            reflect.ValueOf(wrap_runtime.BlockProfile),
		"BlockProfileRecord":      reflect.ValueOf(reflect.TypeOf(wrap_runtime.BlockProfileRecord{})),
		"Breakpoint":              reflect.ValueOf(wrap_runtime.Breakpoint),
		"CPUProfile":              reflect.ValueOf(wrap_runtime.CPUProfile),
		"Caller":                  reflect.ValueOf(wrap_runtime.Caller),
		"Callers":                 reflect.ValueOf(wrap_runtime.Callers),
		"CallersFrames":           reflect.ValueOf(wrap_runtime.CallersFrames),
		"Compiler":                reflect.ValueOf(wrap_runtime.Compiler),
		"Error":                   reflect.ValueOf(reflect.TypeOf((*wrap_runtime.Error)(nil)).Elem()),
		"Frame":                   reflect.ValueOf(reflect.TypeOf(wrap_runtime.Frame{})),
		"Frames":                  reflect.ValueOf(reflect.TypeOf(wrap_runtime.Frames{})),
		"Func":                    reflect.ValueOf(reflect.TypeOf(wrap_runtime.Func{})),
		"FuncForPC":               reflect.ValueOf(wrap_runtime.FuncForPC),
		"GC":                      reflect.ValueOf(wrap_runtime.GC),
		"GOARCH":                  reflect.ValueOf(wrap_runtime.GOARCH),
		"GOMAXPROCS":              reflect.ValueOf(wrap_runtime.GOMAXPROCS),
		"GOOS":                    reflect.ValueOf(wrap_runtime.GOOS),
		"GOROOT":                  reflect.ValueOf(wrap_runtime.GOROOT),
		"Goexit":                  reflect.ValueOf(wrap_runtime.Goexit),
		"GoroutineProfile":        reflect.ValueOf(wrap_runtime.GoroutineProfile),
		"Gosched":                 reflect.ValueOf(wrap_runtime.Gosched),
		"KeepAlive":               reflect.ValueOf(wrap_runtime.KeepAlive),
		"LockOSThread":            reflect.ValueOf(wrap_runtime.LockOSThread),
		"MemProfile":              reflect.ValueOf(wrap_runtime.MemProfile),
		"MemProfileRate":          reflect.ValueOf(&wrap_runtime.MemProfileRate).Elem(),
		"MemProfileRecord":        reflect.ValueOf(reflect.TypeOf(wrap_runtime.MemProfileRecord{})),
		"MemStats":                reflect.ValueOf(reflect.TypeOf(wrap_runtime.MemStats{})),
		"MutexProfile":            reflect.ValueOf(wrap_runtime.MutexProfile),
		"NumCPU":                  reflect.ValueOf(wrap_runtime.NumCPU),
		"NumCgoCall":              reflect.ValueOf(wrap_runtime.NumCgoCall),
		"NumGoroutine":            reflect.ValueOf(wrap_runtime.NumGoroutine),
		"ReadMemStats":            reflect.ValueOf(wrap_runtime.ReadMemStats),
		"ReadTrace":               reflect.ValueOf(wrap_runtime.ReadTrace),
		"SetBlockProfileRate":     reflect.ValueOf(wrap_runtime.SetBlockProfileRate),
		"SetCPUProfileRate":       reflect.ValueOf(wrap_runtime.SetCPUProfileRate),
		"SetCgoTraceback":         reflect.ValueOf(wrap_runtime.SetCgoTraceback),
		"SetFinalizer":            reflect.ValueOf(wrap_runtime.SetFinalizer),
		"SetMutexProfileFraction": reflect.ValueOf(wrap_runtime.SetMutexProfileFraction),
		"Stack":                   reflect.ValueOf(wrap_runtime.Stack),
		"StackRecord":             reflect.ValueOf(reflect.TypeOf(wrap_runtime.StackRecord{})),
		"StartTrace":              reflect.ValueOf(wrap_runtime.StartTrace),
		"StopTrace":               reflect.ValueOf(wrap_runtime.StopTrace),
		"ThreadCreateProfile":     reflect.ValueOf(wrap_runtime.ThreadCreateProfile),
		"TypeAssertionError":      reflect.ValueOf(reflect.TypeOf(wrap_runtime.TypeAssertionError{})),
		"UnlockOSThread":          reflect.ValueOf(wrap_runtime.UnlockOSThread),
		"Version":                 reflect.ValueOf(wrap_runtime.Version),
	},
}

func init() {
	if gowrap.Pkgs["runtime"] == nil {
		gowrap.Pkgs["runtime"] = pkg_wrap_runtime
	}
}
