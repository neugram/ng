// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package gowrap

var Pkgs = make(map[string]*Pkg)

type Pkg struct {
	// TODO: ExportData
	Exports map[string]interface{}
}
