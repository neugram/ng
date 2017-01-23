// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// L0:
//go:generate go run genwrap.go errors
//go:generate go run genwrap.go io
//go:generate go run genwrap.go runtime
//go:generate go run genwrap.go sync
//go:generate go run genwrap.go sync/atomic

// L1:
//go:generate go run genwrap.go math
//go:generate go run genwrap.go strconv
//go:generate go run genwrap.go unicode/utf8

// L2:
// TODO do we want bufio? maybe write our own version.
//go:generate go run genwrap.go bytes
//go:generate go run genwrap.go path
//go:generate go run genwrap.go strings
//go:generate go run genwrap.go unicode

// L3:
//go:generate go run genwrap.go encoding/base64
//go:generate go run genwrap.go encoding/binary

// OS:
//go:generate go run genwrap.go os
// TODO go:generate go run genwrap.go path/filepath

// L4:
//go:generate go run genwrap.go fmt
//go:generate go run genwrap.go time

package gowrap // import "neugram.io/ng/eval/gowrap"
import "reflect"

var Pkgs = make(map[string]*Pkg)

type Pkg struct {
	// TODO: ExportData
	Exports map[string]reflect.Value
}
