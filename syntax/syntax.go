// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package syntax defines an Abstract Syntax Tree, an AST, for Neugram.
//
// Nodes in the AST are represented by Node objects. The particular nodes
// for expressions, statements, and types are defined in the respective
// packages:
//
//	syntax/expr
//	syntax/stmt
//	syntax/tipe
//
package syntax

import "neugram.io/ng/syntax/src"

// A Node is a node in the syntax tree.
type Node interface {
	Pos() src.Pos
}
