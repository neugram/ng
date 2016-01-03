// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

//go:generate go run genwrap.go fmt
//go:generate go run genwrap.go os
//go:generate go run genwrap.go sync

package gowrap // import "neugram.io/eval/gowrap"

var Pkgs = make(map[string]*Pkg)

type Pkg struct {
	// TODO: ExportData
	Exports map[string]interface{}
}
