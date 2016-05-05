// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// +build !windows

package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"neugram.io/eval/environ"
)

func findExec(name string) error {
	fi, err := os.Stat(name)
	if err != nil {
		return err
	}
	if fi.IsDir() || fi.Mode()&0111 == 0 {
		return fmt.Errorf("%q is not an executable", name)
	}
	return nil
}

func findExecInPath(name string, env *environ.Environ) (string, error) {
	if strings.Contains(name, "/") {
		err := findExec(name)
		if err == nil {
			return name, nil
		}
		return "", err
	}

	path := filepath.SplitList(env.Get("PATH"))
	if len(path) == 0 {
		return "", fmt.Errorf("cannot find %q, no PATH", name)
	}

	for _, dir := range path {
		if dir == "" {
			dir = "."
		}
		file := dir + "/" + name
		if err := findExec(file); err == nil {
			return file, nil
		}
	}
	return "", fmt.Errorf("cannot find %q in PATH", name)
}
