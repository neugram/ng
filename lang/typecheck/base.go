// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package typecheck

import "neugram.io/lang/tipe"

var base = &Scope{Objs: make(map[string]*Obj)}

func init() {
	var basic = []tipe.Basic{
		tipe.Bool,
		tipe.Integer,
		tipe.Float,
		tipe.Complex,
		tipe.String,
		tipe.Int64,
		tipe.Float32,
		tipe.Float64,
	}
	for _, t := range basic {
		base.Objs[string(t)] = &Obj{Kind: ObjType, Type: t}
	}
}
