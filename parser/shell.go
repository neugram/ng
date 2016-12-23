// Copyright 2016 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"strconv"
	"unicode"

	"neugram.io/lang/expr"
	"neugram.io/lang/token"
)

func (p *Parser) parseShellList() *expr.ShellList {
	l := &expr.ShellList{}
	l.AndOr = append(l.AndOr, p.parseShellAndOr())
	for p.s.Token == token.Ref || p.s.Token == token.Semicolon {
		if p.s.Token == token.Ref {
			l.AndOr[len(l.AndOr)-1].Background = true
		}
		p.next()
		if p.s.Token == token.ShellNewline || p.s.Token == token.Shell {
			break
		}
		l.AndOr = append(l.AndOr, p.parseShellAndOr())
	}
	if p.s.Token == token.ShellNewline {
		p.next()
	}
	return l
}

func (p *Parser) parseShellAndOr() *expr.ShellAndOr {
	l := &expr.ShellAndOr{}
	l.Pipeline = append(l.Pipeline, p.parseShellPipeline())
	for p.s.Token == token.LogicalAnd || p.s.Token == token.LogicalOr {
		l.Sep = append(l.Sep, p.s.Token)
		p.next()
		l.Pipeline = append(l.Pipeline, p.parseShellPipeline())
	}
	return l
}

func (p *Parser) parseShellPipeline() *expr.ShellPipeline {
	l := &expr.ShellPipeline{}
	if p.s.Token == token.Not {
		l.Bang = true
		p.next()
	}
	l.Cmd = append(l.Cmd, p.parseShellCmd())
	//fmt.Printf("parseShellPipeline, after first parseShellCmd, p.s.Token=%s\n", p.s.Token)
	for p.s.Token == token.ShellPipe {
		p.next()
		l.Cmd = append(l.Cmd, p.parseShellCmd())
	}
	return l
}

func (p *Parser) parseShellCmd() *expr.ShellCmd {
	//fmt.Printf("parseShellCmd, p.s.Token=%s\n", p.s.Token)
	l := &expr.ShellCmd{}
	if p.s.Token == token.LeftParen {
		p.next()
		l.Subshell = p.parseShellList()
		p.expect(token.RightParen)
		p.next()
	} else {
		l.SimpleCmd = p.parseShellSimpleCmd()
	}
	return l
}

func isAssignment(word string) (k, v string) {
	for i, r := range word {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			if r == '=' {
				return word[:i], word[i:]
			}
			return "", ""
		}
	}
	return "", ""
}

func (p *Parser) parseShellSimpleCmd() *expr.ShellSimpleCmd {
	l := &expr.ShellSimpleCmd{}
	for {
		w, r := p.maybeParseShellRedirect()
		if r != nil {
			//fmt.Printf("Redirect: %v\n", r)
			l.Redirect = append(l.Redirect, r)
			continue
		} else if w == "" {
			switch p.s.Token {
			case token.ShellWord:
				w = p.s.Literal.(string)
				p.next()
			//case token.ShellNewline:
			//p.next()
			//return l
			default:
				return l
			}
		}

		if k, v := isAssignment(w); k != "" {
			l.Assign = append(l.Assign, expr.ShellAssign{
				Key:   k,
				Value: v,
			})
		} else {
			l.Args = append(l.Args, w)
		}
	}
	return l
}

func (p *Parser) maybeParseShellRedirect() (string, *expr.ShellRedirect) {
	//fmt.Printf("maybeParseShellRedirect p.s.Token=%s\n", p.s.Token)
	lit := ""
	number := (*int)(nil)
	if p.s.Token == token.ShellWord {
		lit = p.s.Literal.(string)
		p.next()
		i, err := strconv.Atoi(lit)
		if err != nil {
			return lit, nil
		}
		number = &i
	}
	switch p.s.Token {
	case token.Less, token.Greater, token.GreaterAnd, token.AndGreater, token.TwoGreater: // TODO: <&
	default:
		return lit, nil
	}
	l := &expr.ShellRedirect{
		Number: number,
		Token:  p.s.Token,
	}
	p.next()
	if p.expect(token.ShellWord) {
		l.Filename = p.s.Literal.(string)
		p.next()
	}
	return "", l
}
