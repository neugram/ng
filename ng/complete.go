// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"neugram.io/eval/shell"
	"neugram.io/lang/token"
)

func completer(mode, line string, pos int) (prefix string, completions []string, suffix string) {
	switch mode {
	case "ng":
		return completerNg(line, pos)
	case "sh":
		return completerSh(line, pos)
	default:
		panic("ng: unknown completer: " + mode)
	}
}

func completerNg(line string, pos int) (prefix string, completions []string, suffix string) {
	if pos != len(line) { // TODO mid-line matching
		return line, nil, ""
	}
	if strings.TrimSpace(line) == "" {
		return line, nil, ""
	}
	// TODO match on word not line.
	// TODO walk the scope for possible names.
	var res []string
	for keyword := range token.Keywords {
		if strings.HasPrefix(keyword, line) {
			res = append(res, keyword)
		}
	}
	return "", res, ""
}

func completerSh(line string, pos int) (prefix string, completions []string, suffix string) {
	if pos != len(line) { // TODO mid-line matching
		return line, nil, ""
	}

	i := strings.LastIndexByte(line, ' ')
	i2 := strings.LastIndexByte(line, '=')
	if i2 > i {
		i = i2
	}
	if i == -1 {
		prefix, completions = completePath(line, true)
		return prefix, completions, ""
	}
	prefix, word := line[:i+1], line[i+1:]
	if word != "" {
		// TODO: word="dir/$V" and word="--flag=$V" should complete var
		switch word[0] {
		case '$':
			return prefix, completeVar(word), ""
		case '-':
			return prefix, completeFlag(word, line), ""
		}
	}
	resPrefix, completions := completePath(word, false)
	return prefix + resPrefix, completions, ""
}

func completeVar(prefix string) (res []string) {
	res = shell.Env.Keys(prefix[1:])
	for i, s := range res {
		res[i] = "$" + s + " "
	}
	return res
}

func completeFlag(prefix, line string) (res []string) {
	return res // TODO
}

func completePath(prefix string, mustBeExec bool) (resPrefix string, res []string) {
	// TODO expand $ variables
	dirPath, filePath := filepath.Split(prefix)
	if dirPath == "" {
		dirPath = "."
	}
	dir, err := os.Open(dirPath)
	if err != nil {
		return prefix, []string{}
	}

	var fi []os.FileInfo
	for {
		potentials, err := dir.Readdir(64)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "ng: %v\n", err)
			return prefix, []string{}
		}
		// TODO: can we use directory order to skip some calls?
		for _, info := range potentials {
			if filePath == "" && strings.HasPrefix(info.Name(), ".") {
				continue
			}
			if !strings.HasPrefix(info.Name(), filePath) {
				continue
			}
			// Follow symlink.
			info, err := os.Stat(filepath.Join(dirPath, info.Name()))
			if err != nil {
				fmt.Fprintf(os.Stderr, "ng: %v\n", err)
				continue
			}
			if info.Name() == "a_file" {
				fmt.Printf("a_file: mustBeExec=%v, mode=0x%x\n", mustBeExec, info.Mode())
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
			res = append(res, info.Name()+"/")
		} else {
			p := info.Name()
			if len(fi) == 1 {
				p += " "
			}
			res = append(res, p)
		}
	}
	return dirPrefix, res
}
