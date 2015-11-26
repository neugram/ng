// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package gowrap

import "fmt"

// TODO: generate this file

var wrap_fmt = &Pkg{
	Exports: map[string]interface{}{
		"Println": fmt.Println,
		"Printf":  fmt.Printf,
	},
}

func init() {
	Pkgs["fmt"] = wrap_fmt
}
