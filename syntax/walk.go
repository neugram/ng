// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import (
	"fmt"
	"reflect"

	"neugram.io/ng/syntax/expr"
	"neugram.io/ng/syntax/stmt"
)

// Walk traverses a syntax tree, calling preFn and postFn for each node.
//
// If a preFn is provided it is called for each node before its children
// are traversed. If preFn returns false no children are traversed.
//
// If a postFn is provided it is called for each node after its children
// are traversed. TODO: If a postFn returns false traversal ends.
func Walk(root Node, preFn, postFn WalkFunc) (result Node) {
	type rootNode struct {
		Node
	}
	parent := &rootNode{Node: root}

	w := walker{preFn: preFn, postFn: postFn}
	w.walk(parent, root, "Node", nil)

	return root
}

// A WalkFunc is invoked by Walk when traversing nodes in a syntax tree.
//
// The return value determines how traversal will proceed, it is
// described in detail in the Walk function documentation.
type WalkFunc func(*Cursor) bool

// A Cursor describes a Node during a syntax tree traversal.
type Cursor struct {
	Node   Node   // the current Node
	Parent Node   // the parent of the current Node
	Name   string // name of the parent field containing the current Node

	iter *iterator
}

// TODO: Replace(Node), InsertAfter(Node), InsertBefore(Node), Delete()

type iterator struct {
	index int
}

type walker struct {
	preFn  WalkFunc
	postFn WalkFunc
	c      Cursor   // reusable Cursor
	iter   iterator // reusable iterator
}

func (w *walker) walk(parent, node Node, fieldName string, iter *iterator) {
	// typed nil -> untyped nil
	if v := reflect.ValueOf(node); v.Kind() == reflect.Ptr && v.IsNil() {
		node = nil
	}

	oldCursor := w.c
	w.c = Cursor{
		Node:   node,
		Parent: parent,
		Name:   fieldName,
		iter:   iter,
	}
	defer func() { w.c = oldCursor }()

	if w.preFn != nil && !w.preFn(&w.c) {
		return
	}

	switch node := node.(type) {
	case nil:
		// done

	case *File:
		w.walkSlice(node, "Stmts")

	case *stmt.Import:
		// done

	case *stmt.ImportSet:
		w.walkSlice(node, "Imports")

	case *stmt.TypeDecl:
		// done

	case *stmt.MethodikDecl:
		w.walkSlice(node, "Methods")

	case *stmt.Const:
		w.walkSlice(node, "Values")

	case *stmt.ConstSet:
		w.walkSlice(node, "Consts")

	case *stmt.Var:
		w.walkSlice(node, "Values")

	case *stmt.VarSet:
		w.walkSlice(node, "Vars")

	case *stmt.Assign:
		w.walkSlice(node, "Left")
		w.walkSlice(node, "Right")

	case *stmt.Block:
		w.walkSlice(node, "Stmts")

	case *stmt.If:
		w.walk(node, node.Init, "Init", nil)
		w.walk(node, node.Cond, "Cond", nil)
		w.walk(node, node.Body, "Body", nil)
		w.walk(node, node.Else, "Else", nil)

	case *stmt.For:
		w.walk(node, node.Init, "Init", nil)
		w.walk(node, node.Cond, "Cond", nil)
		w.walk(node, node.Post, "Post", nil)
		w.walk(node, node.Body, "Body", nil)

	case *stmt.Switch:
		w.walk(node, node.Init, "Init", nil)
		w.walk(node, node.Cond, "Cond", nil)
		w.walkSlice(node, "Cases")

	case stmt.SwitchCase:
		w.walkSlice(node, "Conds")
		w.walk(node, node.Body, "Body", nil)

	case *stmt.TypeSwitch:
		w.walk(node, node.Init, "Init", nil)
		w.walk(node, node.Assign, "Assign", nil)
		w.walkSlice(node, "Cases")

	case stmt.TypeSwitchCase:
		w.walk(node, node.Body, "Body", nil)

	case *stmt.Go:
		w.walk(node, node.Call, "Call", nil)

	case *stmt.Range:
		w.walk(node, node.Key, "Key", nil)
		w.walk(node, node.Val, "Val", nil)
		w.walk(node, node.Expr, "Expr", nil)
		w.walk(node, node.Body, "Body", nil)

	case *stmt.Return:
		w.walkSlice(node, "Exprs")

	case *stmt.Simple:
		w.walk(node, node.Expr, "Expr", nil)

	case *stmt.Send:
		w.walk(node, node.Chan, "Chan", nil)
		w.walk(node, node.Value, "Value", nil)

	case *stmt.Branch:

	case *stmt.Labeled:
		w.walk(node, node.Stmt, "Stmt", nil)

	case *stmt.Select:
		w.walkSlice(node, "Cases")

	case stmt.SelectCase:
		w.walk(node, node.Stmt, "Stmt", nil)
		w.walk(node, node.Body, "Body", nil)

	case *stmt.Bad:

	case *expr.Binary:
		w.walk(node, node.Left, "Left", nil)
		w.walk(node, node.Right, "Right", nil)

	case *expr.Unary:
		w.walk(node, node.Expr, "Expr", nil)

	case *expr.Bad:

	case *expr.Selector:
		w.walk(node, node.Left, "Left", nil)
		w.walk(node, node.Right, "Right", nil)

	case *expr.Slice:
		w.walk(node, node.Low, "Low", nil)
		w.walk(node, node.High, "High", nil)
		w.walk(node, node.Max, "Max", nil)

	case *expr.Index:
		w.walk(node, node.Left, "Left", nil)
		w.walkSlice(node, "Indicies")

	case *expr.TypeAssert:
		w.walk(node, node.Left, "Left", nil)

	case *expr.BasicLiteral:

	case *expr.FuncLiteral:
		if body, isStmt := node.Body.(*stmt.Block); isStmt {
			w.walk(node, body, "Body", nil)
		}

	case *expr.CompLiteral:
		w.walkSlice(node, "Keys")
		w.walkSlice(node, "Elements")

	case *expr.MapLiteral:
		w.walkSlice(node, "Keys")
		w.walkSlice(node, "Values")

	case *expr.SliceLiteral:
		w.walkSlice(node, "Elems")

	case *expr.TableLiteral:
		w.walkSlice(node, "ColNames")
		// TODO: handle rows

	case *expr.Type:

	case *expr.Ident:

	case *expr.Call:
		w.walk(node, node.Func, "Func", nil)
		w.walkSlice(node, "Args")

	case *expr.Range:
		w.walk(node, node.Start, "Start", nil)
		w.walk(node, node.End, "End", nil)
		w.walk(node, node.Exact, "Exact", nil)

	case *expr.ShellList:
		w.walkSlice(node, "AndOr")

	case *expr.ShellAndOr:
		w.walkSlice(node, "Pipeline")

	case *expr.ShellPipeline:
		w.walkSlice(node, "Cmd")

	case *expr.ShellCmd:
		w.walk(node, node.SimpleCmd, "SimpleCmd", nil)
		w.walk(node, node.Subshell, "Subshell", nil)

	case *expr.ShellSimpleCmd:
		w.walkSlice(node, "Redirect")
		w.walkSlice(node, "Assign")

	case *expr.ShellRedirect:

	case expr.ShellAssign:

	case *expr.Shell:
		w.walkSlice(node, "Cmds")

	default:
		panic(fmt.Sprintf("syntax.Walk: unknown node (type %T)", node))
	}

	if w.postFn != nil && !w.postFn(&w.c) {
		// TODO: implement abort
	}
}

func (w *walker) walkSlice(parent Node, fieldName string) {
	oldIter := w.iter
	defer func() { w.iter = oldIter }()

	w.iter.index = 0

	for {
		v := reflect.Indirect(reflect.ValueOf(parent)).FieldByName(fieldName)
		if w.iter.index >= v.Len() {
			break
		}

		var node Node
		if e := v.Index(w.iter.index); e.IsValid() {
			node = e.Interface().(Node)
		}

		w.walk(parent, node, fieldName, &w.iter)
		w.iter.index++
	}
}
