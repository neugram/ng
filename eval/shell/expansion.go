// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package shell

import (
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
	"unicode"
)

func expansion(argv1 []string, params paramset) ([]string, error) {
	var err error
	var argv2 []string

	for _, expander := range expanders {
		for _, arg := range argv1 {
			if arg[0] == '\'' || arg[0] == '"' {
				argv2 = append(argv2, arg)
				continue
			}
			argv2, err = expander(argv2, arg, params)
			if err != nil {
				return nil, err
			}
		}
		argv1 = argv2
		argv2 = nil
	}

	for i, arg := range argv1 {
		s, e := arg[0], arg[len(arg)-1]
		if s == '\'' && e == '\'' {
			argv1[i] = arg[1 : len(arg)-1]
		} else if s == '"' && e == '"' {
			v, err := ExpandParams(arg, params)
			if err != nil {
				return nil, err
			}
			v = v[1 : len(arg)-1]
			v = quoteUnescaper.Replace(v)
			argv1[i] = v
		}
	}

	return argv1, nil
}

var quoteUnescaper = strings.NewReplacer(`\"`, `"`, "\\`", "`")

var expanders = []func([]string, string, paramset) ([]string, error){
	braceExpand,
	tildeExpand,
	paramExpand,
	pathsExpand,
}

// brace expansion (for example: "c{d,e}" becomes "cd ce")
func braceExpand(src []string, arg string, _ paramset) (res []string, err error) {
	res = src
	i1 := indexUnquoted(arg, '{')
	if i1 == -1 {
		return append(res, arg), nil
	}
	i2 := indexUnquoted(arg[i1:], '}')
	if i2 == -1 {
		return append(res, arg), nil
	} else {
		prefix, suffix := arg[:i1], arg[i1+i2+1:]
		arg = arg[i1+1 : i1+i2]
		for len(arg) > 0 {
			c := indexUnquoted(arg, ',')
			if c == -1 {
				res, _ = braceExpand(res, prefix+arg+suffix, nil)
				break
			}
			res, _ = braceExpand(res, prefix+arg[:c]+suffix, nil)
			arg = arg[c+1:]
		}
	}
	return res, nil
}

func ExpandTilde(arg string) (res string, err error) {
	if !strings.HasPrefix(arg, "~") {
		return arg, nil
	}
	name := arg[1:]
	for i, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			name = name[:i]
			break
		}
	}
	var u *user.User
	if len(name) == 0 {
		u, err = user.Current()
	} else {
		u, err = user.Lookup(name)
	}
	if err != nil {
		if _, ok := err.(user.UnknownUserError); ok {
			return arg, nil
		}
		return "", fmt.Errorf("expanding %s: %v", arg, err)
	}
	return u.HomeDir + arg[1+len(name):], nil
}

// tilde expansion (important: cd ~, cd ~/foo, less so: cd ~user1)
func tildeExpand(src []string, arg string, params paramset) (res []string, err error) {
	expanded, err := ExpandTilde(arg)
	if err != nil {
		return nil, err
	}
	return append(src, expanded), nil
}

// ExpandParams expands $ variables.
func ExpandParams(arg string, params paramset) (string, error) {
	for {
		i1 := indexParam(arg)
		if i1 == -1 {
			break
		}
		var r rune
		i2 := -1
		for i2, r = range arg[i1+1:] {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				i2--
				break
			}
		}
		if i2 == -1 {
			return "", fmt.Errorf("invalid $ parameter: %q", arg)
		}
		end := i1 + 1 + i2 + 1
		name := arg[i1+1 : end]
		val := params.Get(name)
		arg = arg[:i1] + val + arg[end:]
	}
	return arg, nil
}

// param expansion ($x, $PATH, ${x}, long tail of questionable sh features)
func paramExpand(src []string, arg string, params paramset) (res []string, err error) {
	res = src
	expanded, err := ExpandParams(arg, params)
	if err != nil {
		return nil, err
	}
	return append(src, expanded), nil
}

// paths expansion (*, ?, [)
func pathsExpand(src []string, arg string, params paramset) (res []string, err error) {
	res = src
	if !strings.ContainsAny(arg, "*?[") {
		return append(res, arg), nil
	}
	// TODO to support interior quoting (like ab"*".c) this will need a rewrite.
	matches, err := filepath.Glob(arg)
	if err != nil {
		return nil, err
	}
	return append(res, matches...), nil
}

// indexUnquoted returns the index of the first unquoted Unicode code
// point r, or -1. A code point r is quoted if it is directly preceded
// by a '\' or enclosed in "" or ''.
func indexUnquoted(s string, r rune) int {
	prevSlash := false
	inBlock := rune(-1)
	for i, v := range s {
		if inBlock != -1 {
			if v == inBlock {
				inBlock = -1
			}
			continue
		}

		if !prevSlash {
			switch v {
			case r:
				return i
			case '\'', '"':
				inBlock = v
			}
		}

		prevSlash = v == '\\'
	}

	return -1
}

// indexParam returns the index of the first $ not quoted with '' or \, or -1.
func indexParam(s string) int {
	prevSlash := false
	inQuote := false
	for i, v := range s {
		if inQuote {
			if v == '\'' {
				inQuote = false
			}
			continue
		}

		if !prevSlash {
			switch v {
			case '$':
				return i
			case '\'':
				inQuote = true
			}
		}

		prevSlash = v == '\\'
	}

	return -1
}
