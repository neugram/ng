// Copyright 2016 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type file struct {
	name      string
	dir, exec bool
}

type completeTest struct {
	line string
	want []string
}

var emptyDir = []file{}

var emptyTests = []completeTest{
	{line: "", want: nil},
	{line: "ls ", want: nil},
}

var justFilesDir = []file{
	{name: "unique"},
	{name: "file1"},
	{name: "file2"},
}

var justFilesTests = []completeTest{
	{line: "ls u", want: []string{"ls unique "}},
	{line: "ls file1", want: []string{"ls file1 "}},
	{line: "ls f", want: []string{
		"ls file1",
		"ls file2",
	}},
	{line: "find . -name=un", want: []string{"find . -name=unique "}},
}

var hierarchyDir = []file{
	{name: "hierarchy", dir: true},
	{name: "hierarchy/d1", dir: true},
	{name: "hierarchy/d2", dir: true},
	{name: "hierarchy/d2/d2d1", dir: true},
	{name: "hierarchy/d2/d2f1"},
	{name: "hierarchy/f1"},
	{name: "hierarchy/f2"},
	{name: "hierarchy/and1"},
	{name: "hierarchy/and2", dir: true},
	{name: "hierarchy/e1", exec: true},
	{name: "hierarchy/e2", exec: true},
}

var hierarchyTests = []completeTest{
	{line: "ls h", want: []string{"ls hierarchy/"}},
	{line: "ls hierarchy", want: []string{"ls hierarchy/"}},
	{line: "ls hierarchy/", want: []string{
		"ls hierarchy/and1",
		"ls hierarchy/and2/",
		"ls hierarchy/d1/",
		"ls hierarchy/d2/",
		"ls hierarchy/e1",
		"ls hierarchy/e2",
		"ls hierarchy/f1",
		"ls hierarchy/f2",
	}},
	{line: "ls hierarchy/d", want: []string{
		"ls hierarchy/d1/",
		"ls hierarchy/d2/",
	}},
	{line: "ls hierarchy/f", want: []string{
		"ls hierarchy/f1",
		"ls hierarchy/f2",
	}},
	{line: "ls hierarchy/an", want: []string{
		"ls hierarchy/and1",
		"ls hierarchy/and2/",
	}},
	{line: "./hierarchy/f", want: nil},
	{line: "./h", want: []string{"./hierarchy/"}},
	{line: "./hierarchy/e", want: []string{
		"./hierarchy/e1",
		"./hierarchy/e2",
	}},
}

func testCompleteSh(t *testing.T, testName string, files []file, tests []completeTest) {
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("%s: %v", testName, err)
	}
	defer os.Chdir(oldwd)
	dir, err := ioutil.TempDir("", "ngcompletetest")
	if err != nil {
		t.Fatalf("%s: %v", testName, err)
	}
	defer os.RemoveAll(dir)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("%s: %v", testName, err)
	}
	for _, f := range files {
		if f.dir {
			if err := os.MkdirAll(filepath.Join(dir, f.name), 0755); err != nil {
				t.Fatalf("%s: %v", testName, err)
			}
		}
	}
	for _, f := range files {
		if f.dir {
			continue
		}
		perm := os.FileMode(0644)
		if f.exec {
			perm = os.FileMode(0755)
		}
		path := filepath.Join(dir, f.name)
		name := []byte(f.name + " contents")
		if err := ioutil.WriteFile(path, name, perm); err != nil {
			t.Fatalf("%s: %v", testName, err)
		}
	}

	for _, test := range tests {
		got := completerSh(test.line)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: %q: got=%v, want=%v", testName, test.line, got, test.want)
		}
	}
}

func TestCompleteShell(t *testing.T) {
	testCompleteSh(t, "empty", emptyDir, emptyTests)
	testCompleteSh(t, "justFiles", justFilesDir, justFilesTests)
	testCompleteSh(t, "hierarchy", hierarchyDir, hierarchyTests)
}
