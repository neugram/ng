// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ngcore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"neugram.io/ng/eval/gowrap"
	"neugram.io/ng/syntax/shell"
	"neugram.io/ng/syntax/token"
)

func (s *Session) Completer(mode, line string, pos int) (prefix string, completions []string, suffix string) {
	switch mode {
	case "ng":
		return s.completerNg(line, pos)
	case "sh":
		return s.completerSh(line, pos)
	default:
		panic("ngcore: unknown completer: " + mode)
	}
}

func (s *Session) completerNg(line string, pos int) (prefix string, completions []string, suffix string) {
	if pos != len(line) { // TODO mid-line matching
		return line, nil, ""
	}
	if strings.TrimSpace(line) == "" {
		return line, nil, ""
	}
	// TODO: split line according to neugram semantics
	// TODO: split line but preserve integrity of e.g. 'foo := "string with a space"'
	words := strings.Split(line, " ")
	word := words[len(words)-1]
	var res []string

	root := func(name string) (reflect.Value, bool) {
		for scope := s.Program.Cur; scope != nil; scope = scope.Parent {
			if scope.VarName == name {
				return scope.Var, true
			}
		}
		return reflect.Value{}, false
	}

	var get func(toks []string, rv reflect.Value) (reflect.Value, bool)
	get = func(toks []string, rv reflect.Value) (reflect.Value, bool) {
		if len(toks) == 1 {
			return rv, true
		}
		next := toks[0]
		switch v := rv.Interface().(type) {
		case *gowrap.Pkg:
			for k, v := range v.Exports {
				if k == next {
					return get(toks[1:], v)
				}
			}
		default:
			rt := rv.Type()
			switch rt.Kind() {
			case reflect.Struct:
				for i := 0; i < rt.NumField(); i++ {
					ft := rt.Field(i)
					if ft.Name == next {
						return get(toks[1:], rv.Field(i))
					}
				}
			}
			for i := 0; i < rt.NumMethod(); i++ {
				meth := rt.Method(i)
				if meth.Name == next {
					return get(toks[1:], rv.Method(i))
				}
			}
			return reflect.Value{}, false
		}
		return reflect.Value{}, false
	}

	find := func(toks []string) (reflect.Value, bool) {
		rv, ok := root(toks[0])
		if !ok {
			return reflect.Value{}, false
		}
		return get(toks[1:], rv)
	}

	switch {
	case strings.Contains(word, "."):
		toks := strings.Split(word, ".")
		rv, ok := find(toks)
		if ok {
			last := toks[len(toks)-1]
			name := strings.Join(toks[:len(toks)-1], ".")
			switch v := rv.Interface().(type) {
			case *gowrap.Pkg:
				for k, v := range v.Exports {
					if strings.HasPrefix(k, last) {
						if v.Kind() == reflect.Func {
							k += "("
						}
						res = append(res, name+"."+k)
					}
				}
			default:
				rt := rv.Type()
				for i := 0; i < rt.NumMethod(); i++ {
					meth := rt.Method(i)
					if strings.HasPrefix(meth.Name, last) {
						res = append(res, name+"."+meth.Name+"(")
					}
				}
				switch rt.Kind() {
				case reflect.Struct:
					for i := 0; i < rt.NumField(); i++ {
						ftname := rt.Field(i).Name
						if strings.HasPrefix(ftname, last) {
							res = append(res, name+"."+ftname)
						}
					}
				}
			}
		}

	default:
		for keyword := range token.Keywords {
			if strings.HasPrefix(keyword, word) {
				res = append(res, keyword)
			}
		}
		for scope := s.Program.Cur; scope != nil; scope = scope.Parent {
			if strings.HasPrefix(scope.VarName, word) {
				res = append(res, scope.VarName)
			}
		}
		res = append(res, s.Program.Types.TypesWithPrefix(word)...)
	}

	prefix = strings.Join(words[:len(words)-1], " ")
	if prefix != "" {
		prefix += " "
	}
	return prefix, res, ""
}

func (s *Session) completerSh(line string, pos int) (prefix string, completions []string, suffix string) {
	if pos != len(line) { // TODO mid-line matching
		return line, nil, ""
	}

	mustBeExec := false
	i := strings.LastIndexByte(line, ' ')
	i2 := strings.LastIndexByte(line, '=')
	if i2 > i {
		i = i2
	}
	if i == -1 {
		//i = 0
		mustBeExec = true
		//prefix, completions = completePath(line, true)
		//return prefix, completions, ""
	}
	prefix, word := line[:i+1], line[i+1:]
	if word != "" && word[0] == '-' {
		// TODO: word="--flag=$V" should complete var
		return prefix, s.completeFlag(word, line), ""
	}
	resPrefix, completions := s.completePath(word, mustBeExec)
	return prefix + resPrefix, completions, ""
}

func (s *Session) completeFlag(prefix, line string) (res []string) {
	return res // TODO
}

func (s *Session) completePath(prefix string, mustBeExec bool) (resPrefix string, res []string) {
	dirPath, filePath := filepath.Split(prefix)
	dirPrefix := dirPath
	if dirPath == "" {
		dirPath = "."
	} else {
		var err error
		dirPath, err = shell.ExpandTilde(dirPath)
		if err != nil {
			return prefix, []string{}
		}
		dirPath, err = shell.ExpandParams(dirPath, s.ShellState.Env)
		if err != nil {
			return prefix, []string{}
		}
	}
	if len(filePath) > 0 && filePath[0] == '$' {
		res = s.ShellState.Env.Keys(filePath[1:])
		for i, s := range res {
			res[i] = "$" + s + " "
		}
		return dirPrefix, res
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
			fmt.Fprintf(s.Stderr, "ng: %v\n", err)
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
				fmt.Fprintf(s.Stderr, "ng: %v\n", err)
				continue
			}
			if info.Name() == "a_file" {
				fmt.Fprintf(s.Stdout, "a_file: mustBeExec=%v, mode=0x%x\n", mustBeExec, info.Mode())
			}
			if mustBeExec && !info.IsDir() && info.Mode()&0111 == 0 {
				continue
			}
			if strings.HasPrefix(info.Name(), filePath) {
				fi = append(fi, info)
			}
		}
	}

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
	sort.Strings(res)
	return dirPrefix, res
}
