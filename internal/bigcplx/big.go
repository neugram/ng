// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bigcplx

import "math/big"

type Complex struct {
	Real *big.Float
	Imag *big.Float
}

func New(v complex128) *Complex {
	return &Complex{
		Real: big.NewFloat(real(v)),
		Imag: big.NewFloat(imag(v)),
	}
}

func (c *Complex) Set(x *Complex) *Complex {
	c.Real.Set(x.Real)
	c.Imag.Set(x.Imag)
	return c
}

func (c *Complex) SetComplex128(x complex128) *Complex {
	c.Real.SetFloat64(real(x))
	c.Imag.SetFloat64(imag(x))
	return c
}

func (c *Complex) String() string {
	return c.Real.String() + " + " + c.Imag.String() + "i"
}
