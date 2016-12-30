// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"neugram.io/lang/expr"
	"neugram.io/lang/stmt"
)

type sExpr interface {
	Sexp() string
}

func printToFile(x sExpr) (path string, err error) {
	f, err := ioutil.TempFile("", "neugram-diff-")
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

	str := ""
	if x != nil {
		str = x.Sexp()
	}
	if _, err := io.WriteString(f, str); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func DiffStmt(x, y stmt.Stmt) (string, error) {
	if EqualStmt(x, y) {
		return "", nil
	}
	return diffSexp(x, y)
}

func DiffExpr(x, y expr.Expr) (string, error) {
	if EqualExpr(x, y) {
		return "", nil
	}
	return diffSexp(x, y)
}

func diffSexp(x, y sExpr) (string, error) {
	fx, err := printToFile(x)
	if err != nil {
		return "", fmt.Errorf("diff print lhs error: %v", err)
	}
	defer os.Remove(fx)
	fy, err := printToFile(y)
	if err != nil {
		return "", fmt.Errorf("diff print rhs error: %v", err)
	}
	defer os.Remove(fy)

	b, _ := ioutil.ReadFile(fx)
	fmt.Printf("fx: %s\n", b)

	data, err := exec.Command("diff", "-U100", "-u", fx, fy).CombinedOutput()
	if err != nil && len(data) == 0 {
		// diff exits with a non-zero status when the files don't match.
		return "", fmt.Errorf("diff error: %v", err)
	}
	res := string(data)
	res = strings.Replace(res, fx, "/x", 1)
	res = strings.Replace(res, fy, "/y", 1)

	if res == "" {
		return "", fmt.Errorf("expressions not equal but empty diff. LHS: %s: %#+v", x.Sexp(), x)
	}
	return res, nil
}
