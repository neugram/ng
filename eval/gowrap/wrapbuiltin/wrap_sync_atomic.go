// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_sync_atomic "sync/atomic"
)

var pkg_wrap_sync_atomic = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"AddInt32":              reflect.ValueOf(wrap_sync_atomic.AddInt32),
		"AddInt64":              reflect.ValueOf(wrap_sync_atomic.AddInt64),
		"AddUint32":             reflect.ValueOf(wrap_sync_atomic.AddUint32),
		"AddUint64":             reflect.ValueOf(wrap_sync_atomic.AddUint64),
		"AddUintptr":            reflect.ValueOf(wrap_sync_atomic.AddUintptr),
		"CompareAndSwapInt32":   reflect.ValueOf(wrap_sync_atomic.CompareAndSwapInt32),
		"CompareAndSwapInt64":   reflect.ValueOf(wrap_sync_atomic.CompareAndSwapInt64),
		"CompareAndSwapPointer": reflect.ValueOf(wrap_sync_atomic.CompareAndSwapPointer),
		"CompareAndSwapUint32":  reflect.ValueOf(wrap_sync_atomic.CompareAndSwapUint32),
		"CompareAndSwapUint64":  reflect.ValueOf(wrap_sync_atomic.CompareAndSwapUint64),
		"CompareAndSwapUintptr": reflect.ValueOf(wrap_sync_atomic.CompareAndSwapUintptr),
		"LoadInt32":             reflect.ValueOf(wrap_sync_atomic.LoadInt32),
		"LoadInt64":             reflect.ValueOf(wrap_sync_atomic.LoadInt64),
		"LoadPointer":           reflect.ValueOf(wrap_sync_atomic.LoadPointer),
		"LoadUint32":            reflect.ValueOf(wrap_sync_atomic.LoadUint32),
		"LoadUint64":            reflect.ValueOf(wrap_sync_atomic.LoadUint64),
		"LoadUintptr":           reflect.ValueOf(wrap_sync_atomic.LoadUintptr),
		"StoreInt32":            reflect.ValueOf(wrap_sync_atomic.StoreInt32),
		"StoreInt64":            reflect.ValueOf(wrap_sync_atomic.StoreInt64),
		"StorePointer":          reflect.ValueOf(wrap_sync_atomic.StorePointer),
		"StoreUint32":           reflect.ValueOf(wrap_sync_atomic.StoreUint32),
		"StoreUint64":           reflect.ValueOf(wrap_sync_atomic.StoreUint64),
		"StoreUintptr":          reflect.ValueOf(wrap_sync_atomic.StoreUintptr),
		"SwapInt32":             reflect.ValueOf(wrap_sync_atomic.SwapInt32),
		"SwapInt64":             reflect.ValueOf(wrap_sync_atomic.SwapInt64),
		"SwapPointer":           reflect.ValueOf(wrap_sync_atomic.SwapPointer),
		"SwapUint32":            reflect.ValueOf(wrap_sync_atomic.SwapUint32),
		"SwapUint64":            reflect.ValueOf(wrap_sync_atomic.SwapUint64),
		"SwapUintptr":           reflect.ValueOf(wrap_sync_atomic.SwapUintptr),
		"Value":                 reflect.ValueOf(reflect.TypeOf(wrap_sync_atomic.Value{})),
	},
}

func init() {
	if gowrap.Pkgs["sync/atomic"] == nil {
		gowrap.Pkgs["sync/atomic"] = pkg_wrap_sync_atomic
	}
}
