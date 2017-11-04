// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

import (
	"go/constant"

	"neugram.io/ng/tipe"
)

var Universe = &Scope{Objs: universeObjs}

var errorType = &tipe.Interface{
	Methods: map[string]*tipe.Func{
		"Error": &tipe.Func{
			Results: &tipe.Tuple{
				Elems: []tipe.Type{tipe.String},
			},
		},
	},
}

var universeObjs = map[string]*Obj{
	"true":  &Obj{Kind: ObjConst, Type: tipe.UntypedBool, Decl: constant.MakeBool(true)},
	"false": &Obj{Kind: ObjConst, Type: tipe.UntypedBool, Decl: constant.MakeBool(false)},
	"nil":   &Obj{Kind: ObjVar, Type: tipe.UntypedNil},
	"env":   &Obj{Kind: ObjVar, Type: &tipe.Map{Key: tipe.String, Value: tipe.String}},
	"alias": &Obj{Kind: ObjVar, Type: &tipe.Map{Key: tipe.String, Value: tipe.String}},
	"error": &Obj{
		Kind: ObjType,
		Type: errorType,
	},
	"print": &Obj{
		Kind: ObjVar,
		Type: &tipe.Func{
			Params: &tipe.Tuple{Elems: []tipe.Type{
				&tipe.Slice{Elem: &tipe.Interface{}},
			}},
			Variadic: true,
		},
	},
	"printf": &Obj{
		Kind: ObjVar,
		Type: &tipe.Func{
			Params: &tipe.Tuple{Elems: []tipe.Type{
				tipe.String,
				&tipe.Slice{Elem: &tipe.Interface{}},
			}},
			Variadic: true,
		},
	},
	"errorf": &Obj{
		Kind: ObjVar,
		Type: &tipe.Func{
			Params: &tipe.Tuple{Elems: []tipe.Type{
				tipe.String,
				&tipe.Slice{Elem: &tipe.Interface{}},
			}},
			Results:  &tipe.Tuple{Elems: []tipe.Type{errorType}},
			Variadic: true,
		},
	},
	"append":  &Obj{Kind: ObjVar, Type: tipe.Append},
	"cap":     &Obj{Kind: ObjVar, Type: tipe.Cap},
	"close":   &Obj{Kind: ObjVar, Type: tipe.Close},
	"copy":    &Obj{Kind: ObjVar, Type: tipe.Copy},
	"delete":  &Obj{Kind: ObjVar, Type: tipe.Delete},
	"len":     &Obj{Kind: ObjVar, Type: tipe.Len},
	"make":    &Obj{Kind: ObjVar, Type: tipe.Make},
	"new":     &Obj{Kind: ObjVar, Type: tipe.New},
	"panic":   &Obj{Kind: ObjVar, Type: tipe.Panic},
	"recover": &Obj{Kind: ObjVar, Type: tipe.Recover},
	"complex": &Obj{Kind: ObjVar, Type: tipe.ComplexFunc},
	"real":    &Obj{Kind: ObjVar, Type: tipe.Real},
	"imag":    &Obj{Kind: ObjVar, Type: tipe.Imag},
}

func init() {
	var basic = []tipe.Basic{
		tipe.Bool,
		tipe.Integer,
		tipe.Float,
		tipe.Complex,
		tipe.String,
		tipe.Int,
		tipe.Int8,
		tipe.Int16,
		tipe.Int32,
		tipe.Int64,
		tipe.Uint,
		tipe.Uint8,
		tipe.Uint16,
		tipe.Uint32,
		tipe.Uint64,
		tipe.Uintptr,
		tipe.Float32,
		tipe.Float64,
		tipe.Complex64,
		tipe.Complex128,
		tipe.UnsafePointer,
	}
	for _, t := range basic {
		Universe.Objs[string(t)] = &Obj{Kind: ObjType, Type: t}
	}
	Universe.Objs["byte"] = &Obj{Kind: ObjType, Type: tipe.Byte}
	Universe.Objs["rune"] = &Obj{Kind: ObjType, Type: tipe.Rune}
}
