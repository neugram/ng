// Generated file, do not edit.

package gowrap

import (
	"reflect"
	sync "sync"
)

var wrap_sync = &Pkg{
	Exports: map[string]reflect.Value{

		"Cond":      reflect.ValueOf(sync.Cond{}),
		"Locker":    reflect.ValueOf((*sync.Locker)(nil)),
		"Mutex":     reflect.ValueOf(sync.Mutex{}),
		"NewCond":   reflect.ValueOf(sync.NewCond),
		"Once":      reflect.ValueOf(sync.Once{}),
		"Pool":      reflect.ValueOf(sync.Pool{}),
		"RWMutex":   reflect.ValueOf(sync.RWMutex{}),
		"WaitGroup": reflect.ValueOf(sync.WaitGroup{}),
	},
}

func init() {
	Pkgs["sync"] = wrap_sync
}
