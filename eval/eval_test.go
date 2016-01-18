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

	"neugram.io/eval/shell"
	"neugram.io/lang/stmt"
	"neugram.io/parser"
)

var exprTests = []struct {
	stmt string
	want *big.Int
}{
	{"2+3*(x+y-2)", big.NewInt(23)},
	{"func() num { return 7 }()", big.NewInt(7)},
	{
		// TODO: I believe our typechecking of this is woefully incomplete.
		// When the closure func() num is declared, it delcares a new num
		// parameter. But it closes over z, which inherits the num
		// parameter from the outer scope. For z to be silently used as a
		// num here, we are tying the two type parameters together. That's
		// kind-of a big deal.
		//
		`func() num {
			if x > 2 && x < 500 {
				return z+1
			} else {
				return z-1
			}
		}()`,
		big.NewInt(8),
	},
	{
		`func() num {
			x := 9
			x++
			if x > 5 {
				x = -x
			}
			return x
		}()`,
		big.NewInt(-10),
	},
	/* TODO: true {
		`func() num {
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
		big.NewInt(8),
	},*/
	{
		`func() num {
			v := 2
			for i := 1; i < 4; i++ {
				v *= i
			}
			return v
		}()`,
		big.NewInt(12),
	},
	/*{
		`func() val {
			v := 2
			for {
				v++
				break
				v++
			}
			return v
		}()`,
		big.NewInt(3),
	},*/
}

func mkBasicProgram() (*Program, error) {
	p := New()
	if _, _, err := p.Eval(mustParse("x := 4")); err != nil {
		return nil, err
	}
	if _, _, err := p.Eval(mustParse("y := 5")); err != nil {
		return nil, err
	}
	if _, _, err := p.Eval(mustParse("z := 7")); err != nil {
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
		res, _, err := p.Eval(s)
		if err != nil {
			t.Errorf("Eval(%s) error: %v", s.Sexp(), err)
		}
		if len(res) != 1 {
			t.Errorf("Eval(%s) want *big.Int, got multi-valued (%d) result: %v", s.Sexp(), len(res), res)
			continue
		}
		fmt.Printf("Returning Eval: %#+v\n", res)
		got, ok := res[0].(*big.Int)
		if !ok {
			t.Errorf("Eval(%s) want *big.Int, got: %s (%T)", s.Sexp(), got, got)
			continue
		}
		if test.want.Cmp(got) != 0 {
			t.Errorf("Eval(%s)=%s, want %s", s.Sexp(), got, test.want)
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
			if _, _, err := prg.Eval(s); err != nil {
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

	out, err := ioutil.TempFile("", "neugram.stdout.")
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	defer func() {
		out.Close()
		os.Stdout = stdout
	}()
	os.Stdout = out

	for _, file := range files {
		t.Logf("Testing program %q", file)
		contents, err := ioutil.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := out.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		if err := out.Truncate(0); err != nil {
			t.Fatal(err)
		}

		err = runProgram(contents)
		if strings.HasSuffix(file, "_panic.ng") {
			if _, isPanic := err.(Panic); !isPanic {
				t.Errorf("%s: expect panic, got: %v", file, err)
				continue
			}
		} else if err != nil {
			t.Errorf("%s:%v", file, err)
			continue
		}

		if _, err := out.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		b, err := ioutil.ReadAll(out)
		if err != nil {
			t.Fatal(err)
		}
		output := string(b)
		t.Logf("output: %s", output)
		if !strings.HasSuffix(file, "_panic.ng") && !strings.HasSuffix(output, "OK\n") {
			t.Errorf("%s missing OK", file)
		}
	}
}
