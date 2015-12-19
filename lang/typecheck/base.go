// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package typecheck

import "neugram.io/lang/tipe"

var Universe = &Scope{Objs: universeObjs}

var universeObjs = map[string]*Obj{
	"true":  &Obj{Kind: ObjVar, Type: tipe.Bool},
	"false": &Obj{Kind: ObjVar, Type: tipe.Bool},
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
