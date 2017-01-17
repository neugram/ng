// Generated file, do not edit.

package gowrap

import (
	"reflect"
	runtime "runtime"
)

var wrap_runtime = &Pkg{
	Exports: map[string]reflect.Value{

		"BlockProfile":            reflect.ValueOf(runtime.BlockProfile),
		"BlockProfileRecord":      reflect.ValueOf(runtime.BlockProfileRecord{}),
		"Breakpoint":              reflect.ValueOf(runtime.Breakpoint),
		"CPUProfile":              reflect.ValueOf(runtime.CPUProfile),
		"Caller":                  reflect.ValueOf(runtime.Caller),
		"Callers":                 reflect.ValueOf(runtime.Callers),
		"CallersFrames":           reflect.ValueOf(runtime.CallersFrames),
		"Compiler":                reflect.ValueOf(runtime.Compiler),
		"Error":                   reflect.ValueOf((*runtime.Error)(nil)),
		"Frame":                   reflect.ValueOf(runtime.Frame{}),
		"Frames":                  reflect.ValueOf(runtime.Frames{}),
		"Func":                    reflect.ValueOf(runtime.Func{}),
		"FuncForPC":               reflect.ValueOf(runtime.FuncForPC),
		"GC":                      reflect.ValueOf(runtime.GC),
		"GOARCH":                  reflect.ValueOf(runtime.GOARCH),
		"GOMAXPROCS":              reflect.ValueOf(runtime.GOMAXPROCS),
		"GOOS":                    reflect.ValueOf(runtime.GOOS),
		"GOROOT":                  reflect.ValueOf(runtime.GOROOT),
		"Goexit":                  reflect.ValueOf(runtime.Goexit),
		"GoroutineProfile":        reflect.ValueOf(runtime.GoroutineProfile),
		"Gosched":                 reflect.ValueOf(runtime.Gosched),
		"KeepAlive":               reflect.ValueOf(runtime.KeepAlive),
		"LockOSThread":            reflect.ValueOf(runtime.LockOSThread),
		"MemProfile":              reflect.ValueOf(runtime.MemProfile),
		"MemProfileRate":          reflect.ValueOf(runtime.MemProfileRate),
		"MemProfileRecord":        reflect.ValueOf(runtime.MemProfileRecord{}),
		"MemStats":                reflect.ValueOf(runtime.MemStats{}),
		"MutexProfile":            reflect.ValueOf(runtime.MutexProfile),
		"NumCPU":                  reflect.ValueOf(runtime.NumCPU),
		"NumCgoCall":              reflect.ValueOf(runtime.NumCgoCall),
		"NumGoroutine":            reflect.ValueOf(runtime.NumGoroutine),
		"ReadMemStats":            reflect.ValueOf(runtime.ReadMemStats),
		"ReadTrace":               reflect.ValueOf(runtime.ReadTrace),
		"SetBlockProfileRate":     reflect.ValueOf(runtime.SetBlockProfileRate),
		"SetCPUProfileRate":       reflect.ValueOf(runtime.SetCPUProfileRate),
		"SetCgoTraceback":         reflect.ValueOf(runtime.SetCgoTraceback),
		"SetFinalizer":            reflect.ValueOf(runtime.SetFinalizer),
		"SetMutexProfileFraction": reflect.ValueOf(runtime.SetMutexProfileFraction),
		"Stack":                   reflect.ValueOf(runtime.Stack),
		"StackRecord":             reflect.ValueOf(runtime.StackRecord{}),
		"StartTrace":              reflect.ValueOf(runtime.StartTrace),
		"StopTrace":               reflect.ValueOf(runtime.StopTrace),
		"ThreadCreateProfile":     reflect.ValueOf(runtime.ThreadCreateProfile),
		"TypeAssertionError":      reflect.ValueOf(runtime.TypeAssertionError{}),
		"UnlockOSThread":          reflect.ValueOf(runtime.UnlockOSThread),
		"Version":                 reflect.ValueOf(runtime.Version),
	},
}

func init() {
	Pkgs["runtime"] = wrap_runtime
}
