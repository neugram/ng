// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	"fmt"
	gotypes "go/types"

	"neugram.io/eval/gowrap"
	"neugram.io/lang/tipe"
)

type GoPkg struct {
	Type  *tipe.Package
	GoPkg *gotypes.Package
	Wrap  *gowrap.Pkg
}

type GoFunc struct {
	Type *tipe.Func
	Func interface{}
}

func (f GoFunc) call(args ...interface{}) (res []interface{}, err error) {
	return nil, fmt.Errorf("Call GoFunc TODO")
}
