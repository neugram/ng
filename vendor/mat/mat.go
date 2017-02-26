// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat

// TODO: rewrite this in Neugram
// TODO: use 'num' type parameter instead of float64

type Matrix interface {
	At(i, j int) float64
	Set(i, j int, v float64)
	Add(m Matrix) Matrix
}

func New(i, j int) Matrix {
	return &memMatrix{Rows: i, Cols: j, Stride: j, Data: make([]float64, i*j)}
}

type memMatrix struct {
	Rows   int
	Cols   int
	Stride int
	Data   []float64
}

func (m *memMatrix) offset(i, j int) int {
	if i < 0 || i >= m.Rows || j < 0 || j >= m.Cols {
		panic("matrix position out of bounds")
	}
	return i*m.Stride + j
}

func (m *memMatrix) At(i, j int) float64     { return m.Data[m.offset(i, j)] }
func (m *memMatrix) Set(i, j int, v float64) { m.Data[m.offset(i, j)] = v }
func (m *memMatrix) Add(m2 Matrix) Matrix {
	m3 := New(m.Rows, m.Cols)
	// Silly implementation, this is not meant for serious programs.
	for i := 0; i < m.Rows; i++ {
		for j := 0; j < m.Cols; j++ {
			m3.Set(i, j, m.At(i, j)+m2.At(i, j))
		}
	}
	return m3
}
