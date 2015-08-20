// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"numgrad.io/lang/expr"
)

func Print(w io.Writer, expr expr.Expr) error { // TODO remove
	_, err := io.WriteString(w, expr.Sexp())
	return err
}

func printToFile(x expr.Expr) (path string, err error) {
	f, err := ioutil.TempFile("", "numgrad-diff-")
	if err != nil {
		return "", err
	}
	defer func() {
		err2 := f.Close()
		if err == nil {
			err = err2
		}
		if err != nil {
			os.Remove(f.Name())
		}
	}()

	if err := Print(f, x); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func Diff(x, y expr.Expr) string {
	if EqualExpr(x, y) {
		return ""
	}
	fx, err := printToFile(x)
	if err != nil {
		return "diff print lhs error: " + err.Error()
	}
	defer os.Remove(fx)
	fy, err := printToFile(y)
	if err != nil {
		return "diff print rhs error: " + err.Error()
	}
	defer os.Remove(fy)

	b, _ := ioutil.ReadFile(fx)
	fmt.Printf("fx: %s\n", b)

	data, err := exec.Command("diff", "-U100", "-u", fx, fy).CombinedOutput()
	if err != nil && len(data) == 0 {
		// diff exits with a non-zero status when the files don't match.
		return "diff error: " + err.Error()
	}
	res := string(data)
	res = strings.Replace(res, fx, "/x", 1)
	res = strings.Replace(res, fy, "/y", 1)

	if res == "" {
		return fmt.Sprintf("expressions not equal but empty diff. LHS: %s: %#+v", x.Sexp(), x)
	}
	return res
}
