// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package memframe

import (
	"testing"

	"numgrad.io/frame"
)

func TestMemory(t *testing.T) {
	fm := New(6, 4)
	var f frame.Frame = fm
	h, err := fm.Len()
	if err != nil {
		t.Fatal(err)
	}
	if h != 4 {
		t.Fatalf("Len(): %d, want %d", h, 4)
	}
	if w := len(f.Cols()); w != 6 {
		t.Fatalf("len(f.Cols())=%d, want %d", w, 6)
	}
	if err := fm.Set(2, 1, 2.1); err != nil {
		t.Errorf("Set error: %v", err)
	}
	var v interface{}
	if err := fm.Get(2, 1, &v); err != nil {
		t.Fatal(err)
	}
	if v != 2.1 {
		t.Errorf("Get(2, 1) = %v, want %v", v, 2.1)
	}

	f = frame.Slice(f, 2, 3, 1, 1)
	fm, ok := f.(*Memory)
	if !ok {
		t.Fatalf("slice produced wrong type: %T", f)
	}
	h, err = fm.Len()
	if err != nil {
		t.Fatal(err)
	}
	if h != 1 {
		t.Fatalf("slice Len(): %d, want %d", h, 1)
	}
	if w := len(f.Cols()); w != 3 {
		t.Fatalf("slice width: %d, want %d", w, 3)
	}
	v = 0
	if err := f.Get(0, 0, &v); err != nil {
		t.Fatal(err)
	}
	if v != 2.1 {
		t.Errorf("slice Get(0, 0) = %v, want %v", v, 2.1)
	}
}
