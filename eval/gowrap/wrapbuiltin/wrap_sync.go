// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	sync "sync"
)

var wrap_sync = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"Cond":      reflect.ValueOf(sync.Cond{}),
		"Locker":    reflect.ValueOf((*sync.Locker)(nil)),
		"Map":       reflect.ValueOf(sync.Map{}),
		"Mutex":     reflect.ValueOf(sync.Mutex{}),
		"NewCond":   reflect.ValueOf(sync.NewCond),
		"Once":      reflect.ValueOf(sync.Once{}),
		"Pool":      reflect.ValueOf(sync.Pool{}),
		"RWMutex":   reflect.ValueOf(sync.RWMutex{}),
		"WaitGroup": reflect.ValueOf(sync.WaitGroup{}),
	},
}

func init() {
	gowrap.Pkgs["sync"] = wrap_sync
}
