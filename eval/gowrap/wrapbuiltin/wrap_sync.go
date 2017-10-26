// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	sync "sync"
)

var wrap_sync = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"Cond":      reflect.ValueOf(reflect.TypeOf(sync.Cond{})),
		"Locker":    reflect.ValueOf(reflect.TypeOf((*sync.Locker)(nil)).Elem()),
		"Map":       reflect.ValueOf(reflect.TypeOf(sync.Map{})),
		"Mutex":     reflect.ValueOf(reflect.TypeOf(sync.Mutex{})),
		"NewCond":   reflect.ValueOf(sync.NewCond),
		"Once":      reflect.ValueOf(reflect.TypeOf(sync.Once{})),
		"Pool":      reflect.ValueOf(reflect.TypeOf(sync.Pool{})),
		"RWMutex":   reflect.ValueOf(reflect.TypeOf(sync.RWMutex{})),
		"WaitGroup": reflect.ValueOf(reflect.TypeOf(sync.WaitGroup{})),
	},
}

func init() {
	gowrap.Pkgs["sync"] = wrap_sync
}
