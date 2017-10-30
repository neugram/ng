// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package frametest

import (
	"math/big"
	"reflect"
	"testing"

	"neugram.io/ng/frame"
	"neugram.io/ng/frame/memframe"
)

var memPresidents = memframe.NewLiteral(
	[]string{"ID", "Name", "Term1", "Term2"},
	[][]interface{}{
		{1, "George Washington", 1789, 1792},
		{2, "John Adams", 1797, 0},
		{3, "Thomas Jefferson", 1800, 1804},
		{4, "James Madison", 1808, 1812},
		{5, "James Monroe", 1816, 1820},
		{6, "John Quincy Adams", 1824, 0},
		{7, "Andrew Jackson", 1828, 1832},
		{8, "Martin Van Buren", 1836, 0},
		{9, "William Henry Harrison", 1840, 0},
		{10, "John Tyler", 1840, 0},
		{11, "James K. Polk", 1844, 0},
		{12, "Zachary Taylor", 1848, 0},
		{13, "Millard Fillmore", 1848, 0},
		{14, "Franklin Pierce", 1852, 0},
		{15, "James Buchanan", 1856, 0},
		{16, "Abraham Lincoln", 1860, 1864},
		{17, "Andrew Johnson", 1864, 0},
		{18, "Ulysses S. Grant", 1868, 1872},
	},
)

func LoadPresidents(t *testing.T, f frame.Frame) {
	h, err := frame.Len(f)
	if err != nil {
		t.Fatal(err)
	}
	if h != 0 {
		t.Errorf("want zero height, got %d", h)
	}
	wantCols := []string{"ID", "Name", "Term1", "Term2"}
	if cols := f.Cols(); !reflect.DeepEqual(cols, wantCols) {
		t.Errorf("cols: %v, want %v", cols, wantCols)
	}

	if _, err := frame.Copy(f, memPresidents); err != nil {
		t.Fatal(err)
	}

	numPres, err := frame.Len(memPresidents)
	if err != nil {
		t.Fatal(err)
	}

	h, err = frame.Len(f)
	if err != nil {
		t.Fatal(err)
	}
	if h != numPres {
		t.Errorf("f.Len() = %d, want %d", h, numPres)
	}

	var term2 int64
	if err := f.Get(3, 0, &term2); err != nil {
		t.Errorf("Get(3, 0) error: %v", err)
	}
	if term2 != 1792 {
		t.Errorf("Get(3, 0) Washington second term %d, want 1792", term2)
	}
	term2Big := big.NewInt(0)
	if err := f.Get(3, 0, term2Big); err != nil {
		t.Errorf("Get(3, 0) error: %v", err)
	}
	if term2Big.Cmp(big.NewInt(1792)) != 0 {
		t.Errorf("Get(3, 0) Washington second term (big.Int) %d, want 1792", term2Big)
	}

	var name string
	if err := f.Get(1, 0, &name); err != nil {
		t.Errorf("Get(1, 0) error: %v", err)
	}
	if want := "George Washington"; name != want {
		t.Errorf("Get(1, 0) = %q, want %q", name, want)
	}

	getTests := []struct {
		y    int
		name string
	}{
		{0, "George Washington"},
		{1, "John Adams"},
		{15, "Abraham Lincoln"},
		{3, "James Madison"},
		{0, "George Washington"},
		{17, "Ulysses S. Grant"},
	}
	for _, test := range getTests {
		var id, term1, term2 int64
		var name string
		if err := f.Get(0, test.y, &id, &name, &term1, &term2); err != nil {
			t.Errorf("Get(0, %d) error: %v", test.y, err)
			continue
		}
		if test.name != name {
			t.Errorf("Get(0, %d): %q, want %q", test.y, name, test.name)
		}
	}

	// Test padding on cached path.
	if err := f.Get(3, 2, &term2); err != nil {
		t.Errorf("Get(3, 1) error: %v", err)
	}
	if term2 != 1804 {
		t.Errorf("Get(3, 1) Jefferson second term %d, want 1804", term2)
	}

	fs := frame.Slice(f, 1, 2, 9, 3)
	if got, want := fs.Cols(), []string{"Name", "Term1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Slice cols=%v, want %v", got, want)
	}
	var term1 int64
	if err := fs.Get(0, 0, &name, &term1); err != nil {
		t.Errorf("Slice Get(0, 0) error: %v", err)
	}
	if want := "John Tyler"; name != want {
		t.Errorf("Slice Get(0, 0) name=%q, want %q", name, want)
	}

	/*
		{10, "John Tyler", 1840, 0},
		{11, "James K. Polk", 1844, 0},
		{12, "Zachary Taylor", 1848, 0},
	*/

}
