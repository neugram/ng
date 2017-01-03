// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run genwrap.go fmt
//go:generate go run genwrap.go os
//go:generate go run genwrap.go io
//go:generate go run genwrap.go sync
//go:generate go run genwrap.go bytes

package gowrap // import "neugram.io/eval/gowrap"
import "reflect"

var Pkgs = make(map[string]*Pkg)

type Pkg struct {
	// TODO: ExportData
	Exports map[string]reflect.Value
}
