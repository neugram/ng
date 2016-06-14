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
	linkto    string
	dir, exec bool
}

type completeTest struct {
	line       string
	wantPrefix string
	want       []string
}

var emptyDir = []file{}

var emptyTests = []completeTest{
	{line: "", want: nil},
	{line: "ls ", wantPrefix: "ls ", want: nil},
}

var justFilesDir = []file{
	{name: "unique"},
	{name: "file1"},
	{name: "file2"},
	{name: "cats"},
	{name: "more_cats", linkto: "cats"},
}

var justFilesTests = []completeTest{
	{
		line:       "ls u",
		wantPrefix: "ls ",
		want:       []string{"unique "},
	},
	{
		line:       "ls file1",
		wantPrefix: "ls ",
		want:       []string{"file1 "},
	},
	{
		line:       "ls f",
		wantPrefix: "ls ",
		want:       []string{"file1", "file2"},
	},
	{
		line:       "find . -name=un",
		wantPrefix: "find . -name=",
		want:       []string{"unique "},
	},
	{line: "cat"},
	{
		line:       "ls more_",
		wantPrefix: "ls ",
		want:       []string{"more_cats "},
	},
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
	{name: "more_h", linkto: "hierarchy"},
}

var hierarchyTests = []completeTest{
	{
		line:       "ls h",
		wantPrefix: "ls ",
		want:       []string{"hierarchy/"},
	},
	{
		line:       "ls more_",
		wantPrefix: "ls ",
		want:       []string{"more_h/"},
	},
	{
		line:       "ls hierarchy",
		wantPrefix: "ls ",
		want:       []string{"hierarchy/"},
	},
	{
		line:       "ls hierarchy/",
		wantPrefix: "ls hierarchy/",
		want: []string{
			"and1",
			"and2/",
			"d1/",
			"d2/",
			"e1",
			"e2",
			"f1",
			"f2",
		},
	},
	{
		line:       "ls hierarchy/d",
		wantPrefix: "ls hierarchy/",
		want: []string{
			"d1/",
			"d2/",
		},
	},
	{
		line:       "ls hierarchy/f",
		wantPrefix: "ls hierarchy/",
		want: []string{
			"f1",
			"f2",
		},
	},
	{
		line:       "ls hierarchy/an",
		wantPrefix: "ls hierarchy/",
		want: []string{
			"and1",
			"and2/",
		},
	},
	{
		line:       "./hierarchy/f",
		wantPrefix: "./hierarchy/",
	},
	{
		line:       "./h",
		wantPrefix: "./",
		want:       []string{"hierarchy/"},
	},
	{
		line:       "./hierarchy/e",
		wantPrefix: "./hierarchy/",
		want: []string{
			"e1",
			"e2",
		},
	},
	{
		line:       "hierarchy/f1 ",
		wantPrefix: "hierarchy/f1 ",
		want: []string{
			"hierarchy/",
			"more_h/",
		},
	},
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
		if f.linkto != "" {
			if err := os.Symlink(f.linkto, f.name); err != nil {
				t.Fatalf("%s: %v", testName, err)
			}
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
		gotPrefix, got, _ := completerSh(test.line, len(test.line))
		if gotPrefix != test.wantPrefix {
			t.Errorf("%s: %q: gotPrefix=%v, wantPrefix=%v", testName, test.line, gotPrefix, test.wantPrefix)
		}
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
