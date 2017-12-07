// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package environ

import (
	"sort"
	"strings"
	"sync"
)

type Environ struct {
	mu sync.Mutex
	m  map[string]string
}

func New() *Environ {
	return &Environ{m: make(map[string]string)}
}

func NewFrom(vals []string) *Environ {
	env := New()
	for _, s := range vals {
		i := strings.Index(s, "=")
		env.Set(s[:i], s[i+1:])
	}
	return env
}

func (e *Environ) GetVal(key interface{}) interface{} { return e.Get(key.(string)) }
func (e *Environ) SetVal(key, val interface{})        { e.Set(key.(string), val.(string)) }

func (e *Environ) Get(key string) string {
	e.mu.Lock()
	v := e.m[key]
	e.mu.Unlock()
	return v
}

func (e *Environ) Set(key, value string) {
	e.mu.Lock()
	e.m[key] = value
	e.mu.Unlock()
}

func (e *Environ) List() []string {
	e.mu.Lock()
	l := make([]string, 0, len(e.m))
	for k, v := range e.m {
		l = append(l, k+"="+v)
	}
	e.mu.Unlock()
	sort.Strings(l)
	return l
}

func (e *Environ) Keys(prefix string) []string {
	var res []string
	e.mu.Lock()
	for k := range e.m {
		if strings.HasPrefix(k, prefix) {
			res = append(res, k)
		}
	}
	e.mu.Unlock()
	sort.Strings(res)
	return res
}
