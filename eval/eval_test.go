// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eval

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"neugram.io/ng/eval/environ"
	"neugram.io/ng/eval/shell"
	"neugram.io/ng/format"
	"neugram.io/ng/parser"
	"neugram.io/ng/stmt"
)

var exprTests = []struct {
	stmt string
	want interface{}
}{
	{"2+3*(x+y-2)", 23},
	{"func() int { return 7 }()", 7},
	{
		`func() int {
			if x > 2 && x < 500 {
				return z+1
			} else {
				return z-1
			}
		}()`,
		8,
	},
	{
		`func() int64 {
			x := 9
			x++
			if x > 5 {
				x = -x
			}
			return int64(x)
		}()`,
		int64(-10),
	},
	{
		`func() int {
			f := func() bool {
				x++
				return true
			}
			if x == 4 || f() {
				x += 4
			}
			if x == 1 && f() {
				x *= 4
			}
			return x
		}()`,
		8,
	},
	{
		`func() int {
			v := 2
			for i := 1; i < 4; i++ {
				v *= i
			}
			return v
		}()`,
		12,
	},
}

func mkBasicProgram() (*Program, error) {
	p := New("basic", nil)
	if _, err := p.Eval(mustParse("x := 4"), nil); err != nil {
		return nil, err
	}
	if _, err := p.Eval(mustParse("y := 5"), nil); err != nil {
		return nil, err
	}
	if _, err := p.Eval(mustParse("z := 7"), nil); err != nil {
		return nil, err
	}
	return p, nil
}

func TestExprs(t *testing.T) {
	for _, test := range exprTests {
		p, err := mkBasicProgram()
		if err != nil {
			t.Fatalf("mkBasicProgram: %v", err)
		}
		s := mustParse(test.stmt)
		res, err := p.Eval(s, nil)
		if err != nil {
			t.Errorf("Eval(%s) error: %v", format.Stmt(s), err)
		}
		if len(res) != 1 {
			t.Errorf("Eval(%s) want *big.Int, got multi-valued (%d) result: %v", format.Stmt(s), len(res), res)
			continue
		}
		fmt.Printf("Returning Eval: %#+v\n", res)
		switch want := test.want.(type) {
		case *big.Int:
			got, ok := res[0].Interface().(*big.Int)
			if !ok {
				t.Errorf("Eval(%s) want *big.Int, got: %s (%T)", format.Stmt(s), got, res[0].Interface())
				continue
			}
			if want.Cmp(got) != 0 {
				t.Errorf("Eval(%s)=%v, want %v", format.Stmt(s), got, want)
			}
		case *big.Float:
			got, ok := res[0].Interface().(*big.Float)
			if !ok {
				t.Errorf("Eval(%s) want *big.Float, got: %s (%T)", format.Stmt(s), got, res[0].Interface())
				continue
			}
			if want.Cmp(got) != 0 {
				t.Errorf("Eval(%s)=%v, want %v", format.Stmt(s), got, want)
			}
		default:
			got := res[0].Interface()
			if got != want {
				t.Errorf("Eval(%s)=%v, want %v", format.Stmt(s), got, want)
			}
		}
	}
}

func mustParse(src string) stmt.Stmt {
	expr, err := parser.ParseStmt([]byte(src))
	if err != nil {
		panic(fmt.Sprintf("mustParse(%q): %v", src, err))
	}
	return expr
}

var errRE = regexp.MustCompile(`ERROR: (.*)`)

func TestPrograms(t *testing.T) {
	files, err := filepath.Glob("testdata/*.ng")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("cannot find testdata")
	}
	origStdout := os.Stdout
	origStderr := os.Stderr
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	for _, file := range files {
		// For the file "testdata/name.ng", name the subtest "name".
		test := file[len("testdata/") : len(file)-3]
		t.Run(test, func(t *testing.T) {
			out, err := ioutil.TempFile("", test+".stdout.")
			if err != nil {
				t.Fatal(err)
			}

			shellState := &shell.State{
				Env:   environ.New(),
				Alias: environ.New(),
			}
			for _, s := range os.Environ() {
				i := strings.Index(s, "=")
				shellState.Env.Set(s[:i], s[i+1:])
			}

			os.Stdout = out
			os.Stderr = out
			err = EvalFile(file, shellState)
			os.Stdout = origStdout
			os.Stderr = origStderr
			if err2 := out.Close(); err2 != nil {
				t.Fatal(err2)
			}

			if strings.HasSuffix(test, "_panic") {
				if _, isPanic := err.(Panic); !isPanic {
					t.Fatalf("expected panic, got: %v", err)
				}
			} else if strings.HasSuffix(test, "_error") {
				// TODO: support multiple errors
				// TODO: check line numbers
				b, ferr := ioutil.ReadFile(file)
				if ferr != nil {
					t.Fatal(ferr)
				}
				match := errRE.FindStringSubmatch(string(b))
				if match == nil {
					t.Fatal("test has _error suffix but no ERROR directive")
				}
				wantStr := match[1]
				if !strings.Contains(err.Error(), wantStr) {
					t.Fatalf("want %q, got: %v", wantStr, err.Error())
				}
				return
			} else if err != nil {
				t.Fatal(err)
			}

			b, err := ioutil.ReadFile(out.Name())
			if err != nil {
				t.Fatal(err)
			}
			output := string(b)
			if !strings.HasSuffix(test, "_panic") && !strings.HasSuffix(output, "OK\n") {
				t.Logf("output:\n%s", output)
				t.Error("test missing OK")
			}
		})
	}
}
