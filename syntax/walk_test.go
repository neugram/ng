// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax_test

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"neugram.io/ng/parser"
	"neugram.io/ng/syntax"
)

func TestWalk(t *testing.T) {
	files, err := filepath.Glob("../eval/testdata/*.ng")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("cannot find testdata")
	}

	for _, file := range files {
		file := file
		test := file[len("testdata/") : len(file)-3]
		t.Run(test, func(t *testing.T) {
			source, err := ioutil.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(file)
			f, err := p.Parse(source)
			if err != nil {
				if strings.HasSuffix(test, "_error") {
					return // probably a parser test
				}
				t.Fatal(err)
			}

			preCount, postCount := 0, 0
			preFn := func(c *syntax.Cursor) bool {
				preCount++
				if c.Name == "" {
					t.Errorf("cursor has no name on %v", c.Node)
				}
				if c.Parent == nil {
					t.Errorf("cursor has no parent on %v", c.Node)
				}
				return true
			}
			postFn := func(c *syntax.Cursor) bool {
				postCount++
				return true
			}
			if res := syntax.Walk(f, preFn, postFn); res != f {
				t.Errorf("Walk returned %v, not original %v", res, f)
			}
			if preCount != postCount {
				t.Errorf("Walk has imbalanced pre/post counts: %d/%d", preCount, postCount)
			}
			if preCount == 0 {
				t.Errorf("Walk didn't visit anything")
			}
		})
	}
}
