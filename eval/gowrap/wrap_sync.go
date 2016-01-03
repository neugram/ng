// Generated file, do not edit.

package gowrap

import "sync"

var wrap_sync = &Pkg{
	Exports: map[string]interface{}{

		"Cond":      sync.Cond{},
		"Locker":    sync.Locker(nil),
		"Mutex":     sync.Mutex{},
		"NewCond":   sync.NewCond,
		"Once":      sync.Once{},
		"Pool":      sync.Pool{},
		"RWMutex":   sync.RWMutex{},
		"WaitGroup": sync.WaitGroup{},
	},
}

func init() {
	Pkgs["sync"] = wrap_sync
}
