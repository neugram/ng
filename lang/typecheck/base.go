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
	"alias": &Obj{Kind: ObjVar, Type: &tipe.Map{Key: tipe.String, Value: tipe.String}},
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
}

func init() {
	var basic = []tipe.Basic{
		tipe.Bool,
		tipe.Byte, // TODO: actually an alias for Int8
		tipe.Rune,
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
		tipe.Float32,
		tipe.Float64,
	}
	for _, t := range basic {
		Universe.Objs[string(t)] = &Obj{Kind: ObjType, Type: t}
	}
}
