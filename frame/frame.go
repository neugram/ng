package frame

import (
	"fmt"

	"numgrad.io/parser"
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
//	Set(x, y int, value interface{}) error
//	Transpose() Frame
//	CopyFrom(src Frame) error
//	Accumulate(g Grouping) (Frame, error)
//
// Maybe TODO:
//	Slice(Rectangle) Frame
//	Read(dst interface{}, col, off int) error // dst is []T.
type Frame interface {
	Get(x, y int) (interface{}, error)
	Size() (width, height int)
}

func ColumnNames(f Frame) []string {
	fr, ok := f.(interface {
		ColumnNames() []string
	})
	if ok {
		return fr.ColumnNames()
	}
	w, _ := f.Size()
	names := make([]string, w)
	for i := range names {
		names[i] = fmt.Sprintf("_%d", i)
	}
	return names
}

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
	By   []int               // column index to group by, in order
	Func map[int]parser.Expr // column index -> binary operator
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

// Want:
// SQL-like powers over frames.
// Filtering by expression.
// Copyable
// Materializable
