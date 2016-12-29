// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kr/pretty"

	"neugram.io/eval/environ"
	"neugram.io/eval/shell"
	"neugram.io/lang/stmt"
	"neugram.io/parser"
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
	/*{
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
	},*/
}

func mkBasicProgram() (*Program, error) {
	p := New()
	if _, err := p.Eval(mustParse("x := 4")); err != nil {
		return nil, err
	}
	if _, err := p.Eval(mustParse("y := 5")); err != nil {
		return nil, err
	}
	if _, err := p.Eval(mustParse("z := 7")); err != nil {
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
		res, err := p.Eval(s)
		if err != nil {
			t.Errorf("Eval(%s) error: %v", pretty.Sprint(s), err)
		}
		if len(res) != 1 {
			t.Errorf("Eval(%s) want *big.Int, got multi-valued (%d) result: %v", pretty.Sprint(s), len(res), res)
			continue
		}
		fmt.Printf("Returning Eval: %#+v\n", res)
		switch want := test.want.(type) {
		case *big.Int:
			got, ok := res[0].Interface().(*big.Int)
			if !ok {
				t.Errorf("Eval(%s) want *big.Int, got: %s (%T)", pretty.Sprint(s), got, res[0].Interface())
				continue
			}
			if want.Cmp(got) != 0 {
				t.Errorf("Eval(%s)=%v, want %v", pretty.Sprint(s), got, want)
			}
		case *big.Float:
			got, ok := res[0].Interface().(*big.Float)
			if !ok {
				t.Errorf("Eval(%s) want *big.Float, got: %s (%T)", pretty.Sprint(s), got, res[0].Interface())
				continue
			}
			if want.Cmp(got) != 0 {
				t.Errorf("Eval(%s)=%v, want %v", pretty.Sprint(s), got, want)
			}
		default:
			got := res[0].Interface()
			if got != want {
				t.Errorf("Eval(%s)=%v, want %v", pretty.Sprint(s), got, want)
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

// TODO: this should probably be part of package eval.
func runProgram(b []byte) error {
	p := parser.New()
	prg := New()

	lines := bytes.Split(b, []byte{'\n'})

	// TODO: position information in the parser will replace i.
	for i, line := range lines {
		res := p.ParseLine(line)
		if len(res.Errs) > 0 {
			return fmt.Errorf("%d: %v", i+1, res.Errs[0])
		}
		for _, s := range res.Stmts {
			if _, err := prg.Eval(s); err != nil {
				if _, isPanic := err.(Panic); isPanic {
					return err
				}
				return fmt.Errorf("%d: %v", i+1, err)
			}
		}
		for _, cmd := range res.Cmds {
			j := &shell.Job{
				Cmd:    cmd,
				Stdin:  os.Stdin,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
			}
			if err := j.Start(); err != nil {
				return err
			}
			done, err := j.Wait()
			if err != nil {
				return err
			}
			if !done {
				break // TODO not right, instead we should just have one cmd, not Cmds here.
			}
		}
	}
	return nil
}

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

	shell.Env = environ.New()
	for _, s := range os.Environ() {
		i := strings.Index(s, "=")
		shell.Env.Set(s[:i], s[i+1:])
	}
	shell.Alias = environ.New()

	for _, file := range files {
		contents, err := ioutil.ReadFile(file)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if len(contents) == 0 {
			t.Fatalf("%s: empty testdata", file)
		}

		out, err := ioutil.TempFile("", filepath.Base(file)+".stdout.")
		if err != nil {
			t.Fatal(err)
		}

		os.Stdout = out
		os.Stderr = out
		err = runProgram(contents)
		os.Stdout = origStdout
		os.Stderr = origStderr
		if err := out.Close(); err != nil {
			t.Fatal(err)
		}

		if strings.HasSuffix(file, "_panic.ng") {
			if _, isPanic := err.(Panic); !isPanic {
				t.Errorf("%s: expect panic, got: %v", file, err)
				continue
			}
		} else if err != nil {
			t.Errorf("%s:%v", file, err)
			continue
		}

		b, err := ioutil.ReadFile(out.Name())
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		output := string(b)
		if !strings.HasSuffix(file, "_panic.ng") && !strings.HasSuffix(output, "OK\n") {
			t.Logf("Testing program %q, output:\n%s", file, output)
			t.Errorf("%s missing OK", file)
		}
	}
}
