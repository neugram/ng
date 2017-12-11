// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gengo_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"neugram.io/ng/gengo"
)

func TestGeneratedPrograms(t *testing.T) {
	files, err := filepath.Glob("../eval/testdata/*.ng")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("cannot find testdata")
	}

	for _, file := range files {
		file := file
		test := file[len("../eval/testdata/") : len(file)-3]
		exclude := []string{ // TODO remove this list
			"import3",
			"vec",
			"complex",
			"ellipsis",
			"error6",
			"error7",
			"import3_error",
			"import4",
			"import5",
			"import8",
			"op1",
			"slice1",
			"array2",
		}
		donotrun := false
		for _, ex := range exclude {
			if test == ex {
				donotrun = true
			}
		}
		if donotrun {
			continue
		}
		t.Run(test, func(t *testing.T) {
			t.Parallel()
			res, err := gengo.GenGo(file, "main")
			if err != nil {
				if strings.HasSuffix(test, "_error") {
					return
				}
				t.Fatal(err)
			}

			f, err := ioutil.TempFile("", "gengo-"+test)
			if err != nil {
				t.Fatal(err)
			}
			tmpgo := f.Name() + ".go"
			f.Close()
			os.Remove(f.Name())
			f, err = os.Create(tmpgo)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			if _, err := f.Write(res); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			binname := strings.TrimSuffix(f.Name(), ".go")

			cmd := exec.Command("go", "build", "-o", binname, f.Name())
			out, err := cmd.CombinedOutput()
			if err != nil {
				if strings.HasSuffix(test, "_error") {
					return
				}
				t.Fatalf("failed to build: %v\n%s", err, out)
			}
			defer os.Remove(binname)

			cmd = exec.Command(binname)
			out, err = cmd.CombinedOutput()
			if err != nil {
				if strings.HasSuffix(test, "_error") {
					return
				}
				if strings.HasSuffix(test, "_panic") {
					// TODO: check errors make sense
					return
				}
				t.Fatalf("failed to run: %v\n%s", err, out)
			}
			if !bytes.HasSuffix(out, []byte("OK\n")) {
				t.Logf("output:\n%s", out)
				t.Error("test missing OK")
			}
		})
	}
}
