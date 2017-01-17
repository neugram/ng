// Generated file, do not edit.

package gowrap

import (
	"reflect"
	sync_atomic "sync/atomic"
)

var wrap_sync_atomic = &Pkg{
	Exports: map[string]reflect.Value{

		"AddInt32":              reflect.ValueOf(sync_atomic.AddInt32),
		"AddInt64":              reflect.ValueOf(sync_atomic.AddInt64),
		"AddUint32":             reflect.ValueOf(sync_atomic.AddUint32),
		"AddUint64":             reflect.ValueOf(sync_atomic.AddUint64),
		"AddUintptr":            reflect.ValueOf(sync_atomic.AddUintptr),
		"CompareAndSwapInt32":   reflect.ValueOf(sync_atomic.CompareAndSwapInt32),
		"CompareAndSwapInt64":   reflect.ValueOf(sync_atomic.CompareAndSwapInt64),
		"CompareAndSwapPointer": reflect.ValueOf(sync_atomic.CompareAndSwapPointer),
		"CompareAndSwapUint32":  reflect.ValueOf(sync_atomic.CompareAndSwapUint32),
		"CompareAndSwapUint64":  reflect.ValueOf(sync_atomic.CompareAndSwapUint64),
		"CompareAndSwapUintptr": reflect.ValueOf(sync_atomic.CompareAndSwapUintptr),
		"LoadInt32":             reflect.ValueOf(sync_atomic.LoadInt32),
		"LoadInt64":             reflect.ValueOf(sync_atomic.LoadInt64),
		"LoadPointer":           reflect.ValueOf(sync_atomic.LoadPointer),
		"LoadUint32":            reflect.ValueOf(sync_atomic.LoadUint32),
		"LoadUint64":            reflect.ValueOf(sync_atomic.LoadUint64),
		"LoadUintptr":           reflect.ValueOf(sync_atomic.LoadUintptr),
		"StoreInt32":            reflect.ValueOf(sync_atomic.StoreInt32),
		"StoreInt64":            reflect.ValueOf(sync_atomic.StoreInt64),
		"StorePointer":          reflect.ValueOf(sync_atomic.StorePointer),
		"StoreUint32":           reflect.ValueOf(sync_atomic.StoreUint32),
		"StoreUint64":           reflect.ValueOf(sync_atomic.StoreUint64),
		"StoreUintptr":          reflect.ValueOf(sync_atomic.StoreUintptr),
		"SwapInt32":             reflect.ValueOf(sync_atomic.SwapInt32),
		"SwapInt64":             reflect.ValueOf(sync_atomic.SwapInt64),
		"SwapPointer":           reflect.ValueOf(sync_atomic.SwapPointer),
		"SwapUint32":            reflect.ValueOf(sync_atomic.SwapUint32),
		"SwapUint64":            reflect.ValueOf(sync_atomic.SwapUint64),
		"SwapUintptr":           reflect.ValueOf(sync_atomic.SwapUintptr),
		"Value":                 reflect.ValueOf(sync_atomic.Value{}),
	},
}

func init() {
	Pkgs["sync/atomic"] = wrap_sync_atomic
}
