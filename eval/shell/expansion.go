// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package shell

import "strings"

func expansion(argv1 []string, params Params) ([]string, error) {
	var err error
	var argv2 []string
	for _, expander := range expanders {
		for _, arg := range argv1 {
			argv2, err = expander(argv2, arg, params)
			if err != nil {
				return nil, err
			}
		}
		argv1 = argv2
		argv2 = nil
	}

	return argv1, nil
}

var expanders = []func([]string, string, Params) ([]string, error){
	braceExpand,
	tildeExpand,
	paramExpand,
	pathsExpand,
}

// brace expansion (for example: "c{d,e}" becomes "cd ce")
func braceExpand(src []string, arg string, _ Params) (res []string, err error) {
	res = src
	i1 := strings.IndexRune(arg, '{')
	if i1 == -1 {
		return append(res, arg), nil
	}
	i2 := strings.IndexRune(arg[i1:], '}')
	if i2 == -1 {
		return append(res, arg), nil
	} else {
		prefix, suffix := arg[:i1], arg[i1+i2+1:]
		arg = arg[i1+1 : i1+i2]
		for len(arg) > 0 {
			c := strings.IndexRune(arg, ',')
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

// tilde expansion (important: cd ~, cd ~/foo, less so: cd ~user1)
func tildeExpand(src []string, arg string, params Params) (res []string, err error) {
	// TODO
	return append(src, arg), nil
}

// param expansion ($x, $PATH, ${x}, long tail of questionable sh features)
func paramExpand(src []string, arg string, params Params) (res []string, err error) {
	// TODO
	return append(src, arg), nil
}

// paths expansion (*, ?, [)
func pathsExpand(src []string, arg string, params Params) (res []string, err error) {
	// TODO
	return append(src, arg), nil
}
