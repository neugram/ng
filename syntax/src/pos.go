// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package src provides source code position tracking.
package src

import "fmt"

// Pos is a position in a source file.
type Pos struct {
	Filename string // path as provided by the user
	Line     int32  // line number, valid values start at 1
	Column   int16
}

func (p Pos) String() string {
	if p.Filename == "" && p.Line == 0 {
		return "<unknown line>"
	}
	if p.Column == 0 {
		return fmt.Sprintf("%s:%d", p.Filename, p.Line)
	} else {
		return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
	}
}
