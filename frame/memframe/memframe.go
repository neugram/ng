// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package memframe

import (
	"errors"
	"fmt"
	"io"

	"numgrad.io/frame"
)

/*
type Int struct {
	Data   []big.Int
	Stride int
	Rect   Rectangle
}

type Float64 struct {
	Data   []float64
	Stride int
	Rect   Rectangle
}
*/

type Memory struct {
	ColName []string
	//ColType []frame.Type TODO
	Data   []interface{}
	Stride int
	Width  int
	Height int
}

func New(width, height int) *Memory {
	return &Memory{
		ColName: make([]string, width),
		Data:    make([]interface{}, width*height),
		Stride:  width,
		Width:   width,
		Height:  height,
	}
}

func NewLiteral(colName []string, data [][]interface{}) *Memory {
	d := New(len(colName), len(data))
	d.ColName = append([]string{}, colName...)
	for i, row := range data {
		if len(row) != len(colName) {
			panic(fmt.Sprintf("memframe.NewLiteral: row %d length is %d, want %d", i, len(row), len(colName)))
		}
		copy(d.Data[i*d.Stride:], row)
	}
	return d
}

var errPtrNil = errors.New("pointer is nil")

func assign(dst, src interface{}) error {
	if dst, ok := dst.(*interface{}); ok {
		*dst = src
		return nil
	}

	fmt.Printf("memframe.assign: dst=%#+v, %T\n", dst, dst)

	switch src := src.(type) {
	case string:
		switch dst := dst.(type) {
		case *string:
			if dst == nil {
				return errPtrNil
			}
			*dst = src
			return nil
		}
		// TODO case []byte?
	case nil:
	}

	return fmt.Errorf("memframe assign TODO")
}

func (d *Memory) offset(x, y int) int { return y*d.Stride + x }
func (d *Memory) Get(x, y int, dst ...interface{}) error {
	if y >= d.Height {
		return io.EOF
	}
	for i, dst := range dst {
		// TODO use frame.Type?
		if err := assign(dst, d.Data[d.offset(x+i, y)]); err != nil {
			return fmt.Errorf("memframe: Get(%d, %d, ... %d:%T): %v", x, y, i, dst, err)
		}
	}
	return nil
}
func (d *Memory) Size() (width, height int) { return d.Width, d.Height }
func (d *Memory) Cols() []string            { return d.ColName }

func (d *Memory) Set(x, y int, value interface{}) error {
	// TODO check for valid types
	d.Data[d.offset(x, y)] = value
	return nil
}

func (d *Memory) Slice(x, xlen, y, ylen int) frame.Frame {
	return &Memory{
		ColName: d.ColName[x : x+xlen],
		Data:    d.Data[d.offset(x, y):],
		Stride:  d.Stride,
		Width:   xlen,
		Height:  ylen,
	}
}
