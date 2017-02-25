// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The methodpool is an awful hack for creating Go types with methods.
//
// A type with a method is pulled out of the pool and embedded in a
// struct created with reflect.StructOf.
//
// These method structs appear first in the struct definition and
// have no size, so a pointer to them is a pointer to the real value.
// Each of these method structs from the pool is only used for one
// new type, so a global map lets us work out the real type of the
// value.
//
// This imposes two limits on a methodik that should not exist:
//	1. methods can only be defined on struct types
//	2. only a limited number of method signatures are possible
//
// All of this can be replaced by either reflect.NamedOf (which is
// https://golang.org/issue/16522), or by generating then compiling
// Go to a plugin and loading that. Hopefully darwin plugin support
// will be ready for 1.9, and this can be retired.
package eval

import (
	"fmt"
	"reflect"
	"sync"
	"unsafe"
)

type methodMap struct {
	typ  reflect.Type  // embedding type created by reflect.StructOf
	impl reflect.Value // func to call implementing type
}

var methodPool = struct {
	mu     sync.Mutex
	used   map[reflect.Type]methodMap
	unused map[string][]reflect.Type
}{
	used: make(map[reflect.Type]methodMap),
	unused: map[string][]reflect.Type{
		"Read": {
			reflect.TypeOf(MethodPoolRead1{}),
			reflect.TypeOf(MethodPoolRead2{}),
			reflect.TypeOf(MethodPoolRead3{}),
		},
		"Write": {
			reflect.TypeOf(MethodPoolWrite1{}),
			reflect.TypeOf(MethodPoolWrite2{}),
			reflect.TypeOf(MethodPoolWrite3{}),
		},
	},
}

func methodPoolAssign(name string, fnType reflect.Type, fnImpl reflect.Value) reflect.Type {
	methodPool.mu.Lock()
	defer methodPool.mu.Unlock()
	avail := methodPool.unused[name]
	if len(avail) == 0 {
		panic(fmt.Sprintf("no method pool entries available for %s, %s", name, fnType))
	}
	embType := avail[0]
	/* TODO if got := embType.Method(0).Type; got != fnType {
		panic(fmt.Sprintf("method pool entry for %s has signature %s, want %s", name, got, fnType))
	}*/
	methodPool.unused[name] = avail[1:]
	methodPool.used[embType] = methodMap{
		typ:  embType,
		impl: fnImpl,
	}
	return embType
}

func callRead(m unsafe.Pointer, embType reflect.Type, b []byte) (n int, err error) {
	methodPool.mu.Lock()
	mapping := methodPool.used[embType]
	methodPool.mu.Unlock()

	v := reflect.NewAt(mapping.typ, m)
	res := mapping.impl.Call([]reflect.Value{v, reflect.ValueOf(b)})
	n = res[0].Interface().(int)
	if errv := res[1].Interface(); errv != nil {
		err = errv.(error)
	}
	return n, err
}

func callWrite(m unsafe.Pointer, embType reflect.Type, b []byte) (n int, err error) {
	methodPool.mu.Lock()
	mapping := methodPool.used[embType]
	methodPool.mu.Unlock()

	v := reflect.NewAt(mapping.typ, m)
	res := mapping.impl.Call([]reflect.Value{v, reflect.ValueOf(b)})
	n = res[0].Interface().(int)
	if errv := res[1].Interface(); errv != nil {
		err = errv.(error)
	}
	return n, err

}

type MethodPoolRead1 struct{}
type MethodPoolRead2 struct{}
type MethodPoolRead3 struct{}

func (m MethodPoolRead1) Read(b []byte) (n int, err error) {
	return callRead(unsafe.Pointer(&m), reflect.TypeOf(m), b)
}
func (m MethodPoolRead2) Read(b []byte) (n int, err error) {
	return callRead(unsafe.Pointer(&m), reflect.TypeOf(m), b)
}
func (m MethodPoolRead3) Read(b []byte) (n int, err error) {
	return callRead(unsafe.Pointer(&m), reflect.TypeOf(m), b)
}

type MethodPoolWrite1 struct{}
type MethodPoolWrite2 struct{}
type MethodPoolWrite3 struct{}

func (m MethodPoolWrite1) Write(b []byte) (n int, err error) {
	return callWrite(unsafe.Pointer(&m), reflect.TypeOf(m), b)
}
func (m MethodPoolWrite2) Write(b []byte) (n int, err error) {
	return callWrite(unsafe.Pointer(&m), reflect.TypeOf(m), b)
}
func (m MethodPoolWrite3) Write(b []byte) (n int, err error) {
	return callWrite(unsafe.Pointer(&m), reflect.TypeOf(m), b)
}
