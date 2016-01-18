// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package typecheck

import "neugram.io/lang/tipe"

var Universe = &Scope{Objs: universeObjs}

var universeObjs = map[string]*Obj{
	"true":  &Obj{Kind: ObjVar, Type: tipe.Bool}, // TODO UntypedBool?
	"false": &Obj{Kind: ObjVar, Type: tipe.Bool},
	"nil":   &Obj{Kind: ObjVar, Type: tipe.UntypedNil},
	"env":   &Obj{Kind: ObjVar, Type: &tipe.Map{Key: tipe.String, Value: tipe.String}},
	"error": &Obj{
		Kind: ObjType,
		Type: &tipe.Interface{
			Methods: map[string]*tipe.Func{
				"Error": &tipe.Func{
					Results: &tipe.Tuple{
						Elems: []tipe.Type{tipe.String},
					},
				},
			},
		},
	},
	"print": &Obj{
		Kind: ObjVar,
		Type: &tipe.Func{
			Params: &tipe.Tuple{Elems: []tipe.Type{
				&tipe.Table{Type: &tipe.Interface{}},
			}},
			Variadic: true,
		},
	},
	"printf": &Obj{
		Kind: ObjVar,
		Type: &tipe.Func{
			Params: &tipe.Tuple{Elems: []tipe.Type{
				tipe.String,
				&tipe.Table{Type: &tipe.Interface{}},
			}},
			Variadic: true,
		},
	},
	"panic": &Obj{
		Kind: ObjVar,
		Type: &tipe.Func{
			Params: &tipe.Tuple{Elems: []tipe.Type{tipe.String}},
		},
	},
}

func init() {
	var basic = []tipe.Basic{
		tipe.Bool,
		tipe.Byte,
		tipe.Rune,
		tipe.Integer,
		tipe.Float,
		tipe.Complex,
		tipe.String,
		tipe.Int64,
		tipe.Float32,
		tipe.Float64,
		tipe.GoInt,
	}
	for _, t := range basic {
		Universe.Objs[string(t)] = &Obj{Kind: ObjType, Type: t}
	}
}
