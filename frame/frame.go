// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package frame

import (
	"errors"
	"fmt"
	"io"

	"numgrad.io/eval"
	"numgrad.io/lang/expr"
)

// A Frame is a two-dimensional data set.
//
// The typical Frame has a small number of named columns and a
// potentially large number of rows. It is inspired by (and indeed
// may be implemented by) an SQL table. However, it is possible to
// have a Frame without column names, and some implementations make
// large columns as cheap as large rows.
//
// Transforming a frame (slicing, aggregating, arithmetic, etc) is
// intended to be cheap. In some sense, transformations are lazily
// evaluated.
//
// As a performance optimization, a Frame may implement any of the
// following methods to provide implementation-specific versions of
// the common frame package functions. Users should not use these
// directly, instead preferring the frame.Fn(f) version for any Fn:
//
//	ColumnNames() []string
//	ColumnType(x int) Type
//	Permute(cols []int) Frame
//	Slice(x, xlen, y, ylen int) Frame
//	Set(x, y int, vals ...interface{}) error
//	Transpose() Frame
//	CopyFrom(src Frame) (n int, err error)
//	CopyTo(dst Frame) (n int, err error)
//	Accumulate(g Grouping) (Frame, error)
//	Len() (int, error)
//
// Maybe TODO:
//	Slice(Rectangle) Frame
//	Read(dst interface{}, col, off int) error // dst is []T.
type Frame interface {
	Cols() []string

	// TODO io.EOF for y out of range, io.ErrUnexpectedEOF for x out of range?
	Get(x, y int, dst ...interface{}) error
}

func Copy(dst, src Frame) (n int, err error) {
	dstf, ok := dst.(interface {
		CopyFrom(src Frame) (n int, err error)
	})
	if ok {
		return dstf.CopyFrom(src)
	}
	srcf, ok := src.(interface {
		CopyTo(dst Frame) (n int, err error)
	})
	if ok {
		return srcf.CopyTo(dst)
	}

	set, ok := dst.(interface {
		Set(x, y int, vals ...interface{}) error
	})
	if !ok {
		return 0, errors.New("frame.Copy: dst Frame does not implement Set")
	}

	cols := len(src.Cols())
	if dstCols := len(dst.Cols()); dstCols != cols {
		return 0, fmt.Errorf("frame.Copy: dst has %d columns, src has %d", dstCols, cols)
	}

	row := make([]interface{}, cols)
	rowp := make([]interface{}, len(row))
	for i := range row {
		rowp[i] = &row[i]
	}
	// TODO if src provides a Len, set backwards to allow impls to create space efficiently
	y := 0
	for {
		err := src.Get(0, y, rowp...)
		if err == io.EOF {
			break
		}
		if err != nil {
			return y, err
		}
		if err := set.Set(0, y, row...); err != nil {
			return y, err
		}
		y++
	}
	return y, nil
}

// Slice slices the Frame.
//
// A ylen of -1 means maximize the length. That is, on a 2x2 Frame,
// Slice(f2x2, 0, 2, 1, -1) produces a 2x1 Frame. This is not intended
// to be an implementation of Python's negative indexes, it is simply
// necessary as it can be expensive or impossible to determine the Len
// of a frame.
func Slice(f Frame, x, xlen, y, ylen int) Frame {
	fr, ok := f.(interface {
		Slice(x, xlen, y, ylen int) Frame
	})
	if ok {
		return fr.Slice(x, xlen, y, ylen)
	}
	panic("TODO Slice")
}

func Transpose(f Frame) Frame {
	fr, ok := f.(interface {
		Transpose() Frame
	})
	if ok {
		return fr.Transpose()
	}
	panic("TODO Transpose")
}

// TODO: consider baking the By field directly into the Frame.
// Then: x.groupby("Col0", "Col2").fold(sum) could work.
type Grouping struct {
	By   []int             // column index to group by, in order
	Func map[int]expr.Expr // column index -> binary operator
}

func Accumulate(f Frame, g Grouping) (Frame, error) {
	fr, ok := f.(interface {
		Accumulate(g Grouping) (Frame, error)
	})
	if ok {
		return fr.Accumulate(g)
	}
	panic("TODO Aggregate")
}

func Len(f Frame) (int, error) {
	fr, ok := f.(interface {
		Len() (int, error)
	})
	if ok {
		return fr.Len()
	}
	y := 0
	for {
		var v interface{}
		err := f.Get(0, y, &v)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		y++
	}
	return y, nil
}

func Filter(f Frame, s *eval.Scope, e expr.Expr) (Frame, error) {
	fr, ok := f.(interface {
		Filter(s *eval.Scope, e expr.Expr) (Frame, error)
	})
	if ok {
		return fr.Filter(s, e)
	}
	panic("TODO Filter")
}

// Want:
// SQL-like powers over frames.
// Filtering by expression.
// Copyable
// Materializable
