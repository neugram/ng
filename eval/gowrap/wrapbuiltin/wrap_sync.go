// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_sync "sync"
)

var pkg_wrap_sync = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"Cond":      reflect.ValueOf(reflect.TypeOf(wrap_sync.Cond{})),
		"Locker":    reflect.ValueOf(reflect.TypeOf((*wrap_sync.Locker)(nil)).Elem()),
		"Map":       reflect.ValueOf(reflect.TypeOf(wrap_sync.Map{})),
		"Mutex":     reflect.ValueOf(reflect.TypeOf(wrap_sync.Mutex{})),
		"NewCond":   reflect.ValueOf(wrap_sync.NewCond),
		"Once":      reflect.ValueOf(reflect.TypeOf(wrap_sync.Once{})),
		"Pool":      reflect.ValueOf(reflect.TypeOf(wrap_sync.Pool{})),
		"RWMutex":   reflect.ValueOf(reflect.TypeOf(wrap_sync.RWMutex{})),
		"WaitGroup": reflect.ValueOf(reflect.TypeOf(wrap_sync.WaitGroup{})),
	},
}

func init() {
	if gowrap.Pkgs["sync"] == nil {
		gowrap.Pkgs["sync"] = pkg_wrap_sync
	}
}
