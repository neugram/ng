// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"neugram.io/lang/token"
)

func completer(mode, line string) []string {
	switch mode {
	case "ng":
		return completerNg(line)
	case "sh":
		return completerSh(line)
	default:
		panic("ng: unknown completer: " + mode)
	}
}

func completerNg(line string) []string {
	if strings.TrimSpace(line) == "" {
		return nil
	}
	// TODO match on word not line.
	// TODO walk the scope for possible names.
	var res []string
	for keyword := range token.Keywords {
		if strings.HasPrefix(keyword, line) {
			res = append(res, keyword)
		}
	}
	return res
}

func completerSh(line string) (res []string) {
	i := strings.LastIndexByte(line, ' ')
	i2 := strings.LastIndexByte(line, '=')
	if i2 > i {
		i = i2
	}
	if i == -1 {
		return completePath(line, true)
	}
	prefix := line[i+1:]
	if prefix == "" {
		return completePath("", false)
	}
	// TODO: prefix="dir/$V" and prefix="--flag=$V" should complete var
	switch prefix[0] {
	case '$':
		return prepend(line[:i+1], completeVar(prefix))
	case '-':
		return prepend(line[:i+1], completeFlag(prefix, line))
	default:
		return prepend(line[:i+1], completePath(prefix, false))
	}
}

func prepend(prefix string, matches []string) []string {
	for i, m := range matches {
		matches[i] = prefix + m
	}
	return matches
}

func completeVar(prefix string) (res []string) {
	return res // TODO
}

func completeFlag(prefix, line string) (res []string) {
	return res // TODO
}

func completePath(prefix string, mustBeExec bool) (res []string) {
	// TODO expand $ variables
	dirPath, filePath := filepath.Split(prefix)
	if dirPath == "" {
		dirPath = "."
	}
	dir, err := os.Open(dirPath)
	if err != nil {
		return []string{}
	}

	var fi []os.FileInfo
	for {
		potentials, err := dir.Readdir(64)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "ng: %v\n", err)
			return []string{}
		}
		// TODO: can we use directory order to skip some calls?
		for _, info := range potentials {
			if filePath == "" && strings.HasPrefix(info.Name(), ".") {
				continue
			}
			if mustBeExec && !info.IsDir() && info.Mode()&0111 == 0 {
				continue
			}
			if strings.HasPrefix(info.Name(), filePath) {
				fi = append(fi, info)
			}
		}
	}

	dirPrefix := prefix[:len(prefix)-len(filePath)]
	for _, info := range fi {
		if info.IsDir() {
			res = append(res, dirPrefix+info.Name()+"/")
		} else {
			p := dirPrefix + info.Name()
			if len(fi) == 1 {
				p += " "
			}
			res = append(res, p)
		}
	}
	return res
}
