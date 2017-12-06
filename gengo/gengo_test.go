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
			"shell1",
			"shell2",
			"shell3",
			"shell4",
			"shell5",
			"export",
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
			"method1",
			"method2",
			"op1",
			"select1",
			"select2_error",
			"select3",
			"select4",
			"select5",
			"select7",
			"select8",
			"slice1",
			"switch10",
			"switch11",
			"switch12_error",
			"switch13_error",
			"switch1_error",
			"switch2_error",
			"switch3_error",
			"switch4_error",
			"switch5_error",
			"switch6",
			"switch7",
			"switch8",
			"switch9",
			"typeassert",
			"typeassert_error",
			"typeassert_panic",
			"typeswitch1",
			"typeswitch2",
			"typeswitch3",
			"typeswitch4",
			"typeswitch5",
			"typeswitch6",
			"typeswitch7",
			"typeswitch8",
			"typeswitch9_error",
			"varargs",
			"vardecl1_error",
			"vardecl2_error",
			"vardecl3_error",
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
