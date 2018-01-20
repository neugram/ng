// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	sh "mvdan.cc/sh/syntax"

	"neugram.io/ng/syntax/expr"
	"neugram.io/ng/syntax/token"
)

func (p *Parser) parseShellList() *expr.ShellList {
	shellParser := sh.NewParser(sh.StopAt("$$"), sh.Variant(sh.LangPOSIX))
	shellSrc := p.s.src[p.s.off:]
	list := &expr.ShellList{}
	var end int
	// parse and translate statements, for as long as we don't reach
	// any of:
	//  * a shell parse error
	//  * EOF / $$
	//  * a statement ended with no semicolon (e.g. newline)
	err := shellParser.Stmts(bytes.NewReader(shellSrc), func(stmt *sh.Stmt) bool {
		list.AndOr = append(list.AndOr, translateAndOr(stmt))
		end = int(stmt.End().Offset())
		return stmt.Semicolon != sh.Pos{}
	})
	if err != nil {
		panic(err)
	}
	p.s.off += end
	p.s.next()
	p.s.skipWhitespace() // trigger needSrc if we found a newline
	if p.s.r == '$' && p.s.src[p.s.off] == '$' {
		// we have $$, so get token.Shell to stop
		p.next()
		if p.s.r == '\n' {
			// TODO(mvdan): why is this necessary?
			p.s.off--
			p.s.r = ';'
		}
	} else {
		// we don't have a $$ - hand the rune back to
		// the shell lexer
		p.s.off -= int(p.s.lastWidth)
	}
	if len(list.AndOr) == 0 {
		return nil
	}
	return list
}

func translateAndOr(stmt *sh.Stmt) *expr.ShellAndOr {
	ao := &expr.ShellAndOr{Background: stmt.Background}
	for {
		binCmd, ok := stmt.Cmd.(*sh.BinaryCmd)
		if !ok || (binCmd.Op != sh.AndStmt && binCmd.Op != sh.OrStmt) {
			break
		}
		stmt = binCmd.X
		pipeline := translatePipeline(binCmd.Y)
		sep := token.LogicalAnd
		if binCmd.Op == sh.OrStmt {
			sep = token.LogicalOr
		}
		// reverse order
		defer func() {
			ao.Pipeline = append(ao.Pipeline, pipeline)
			ao.Sep = append(ao.Sep, sep)
		}()
	}
	ao.Pipeline = append(ao.Pipeline, translatePipeline(stmt))
	return ao
}

func translatePipeline(stmt *sh.Stmt) *expr.ShellPipeline {
	p := &expr.ShellPipeline{}
	for {
		binCmd, ok := stmt.Cmd.(*sh.BinaryCmd)
		if !ok || binCmd.Op != sh.Pipe {
			break
		}
		stmt = binCmd.X
		cmd := translateCmd(binCmd.Y)
		// reverse order
		defer func() { p.Cmd = append(p.Cmd, cmd) }()
	}
	p.Cmd = append(p.Cmd, translateCmd(stmt))
	return p
}

func translateCmd(stmt *sh.Stmt) *expr.ShellCmd {
	switch x := stmt.Cmd.(type) {
	case *sh.CallExpr:
		return &expr.ShellCmd{SimpleCmd: &expr.ShellSimpleCmd{
			Assign:   translateAssigns(x.Assigns),
			Args:     stringifyWords(x.Args),
			Redirect: translateRedirs(stmt.Redirs),
		}}
	case *sh.Subshell:
		cmd := &expr.ShellCmd{Subshell: &expr.ShellList{}}
		for _, stmt := range x.Stmts {
			cmd.Subshell.AndOr = append(cmd.Subshell.AndOr, translateAndOr(stmt))
		}
		return cmd
	default:
		panic(fmt.Sprintf("TODO: sh.Command type %T", x))
	}
}

func translateAssigns(assigns []*sh.Assign) []expr.ShellAssign {
	if len(assigns) == 0 {
		return nil
	}
	as := make([]expr.ShellAssign, len(assigns))
	for i, assign := range assigns {
		as[i] = expr.ShellAssign{
			Key:   assign.Name.Value,
			Value: stringifyWord(assign.Value),
		}
	}
	return as
}

func translateRedirs(redirs []*sh.Redirect) []*expr.ShellRedirect {
	if len(redirs) == 0 {
		return nil
	}
	rs := make([]*expr.ShellRedirect, len(redirs))
	for i, redir := range redirs {
		tok := token.Greater
		switch redir.Op {
		case sh.AppOut:
			tok = token.TwoGreater
		case sh.DplOut:
			tok = token.GreaterAnd
		}
		rs[i] = &expr.ShellRedirect{
			Token:    tok,
			Filename: stringifyWord(redir.Word),
		}
		if redir.N != nil {
			if n, _ := strconv.Atoi(redir.N.Value); n > 0 {
				rs[i].Number = &n
			}
		}
	}
	return rs
}

func stringifyWords(words []*sh.Word) []string {
	strs := make([]string, len(words))
	for i, word := range words {
		strs[i] = stringifyWord(word)
	}
	return strs
}

func stringifyWord(word *sh.Word) string {
	var buf bytes.Buffer
	printer := sh.NewPrinter()
	// a bit hacky, to print just one word without any
	// indentation etc
	f := &sh.File{}
	f.Stmts = append(f.Stmts, &sh.Stmt{Cmd: &sh.CallExpr{
		Args: []*sh.Word{word},
	}})
	printer.Print(&buf, f)
	s := buf.String()
	s = strings.TrimPrefix(s, "\\\n")
	return strings.Trim(s, "\n\t ")
}
