// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package memframe

import (
	"fmt"

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

func (d *Memory) offset(x, y int) int               { return y*d.Stride + x }
func (d *Memory) Get(x, y int) (interface{}, error) { return d.Data[d.offset(x, y)], nil }
func (d *Memory) Size() (width, height int)         { return d.Width, d.Height }
func (d *Memory) ColumnNames() []string             { return d.ColName }

func (d *Memory) Set(x, y int, value interface{}) error {
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
