// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"strconv"
	"unicode"

	"neugram.io/ng/syntax/expr"
	"neugram.io/ng/syntax/token"
)

func (p *Parser) parseShellList() *expr.ShellList {
	andor := p.parseShellAndOr()
	if andor == nil {
		return nil
	}
	l := &expr.ShellList{
		AndOr: []*expr.ShellAndOr{andor},
	}
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
		if !p.interactive {
			p.next()
		}
	}
	return l
}

func (p *Parser) parseShellAndOr() *expr.ShellAndOr {
	pl := p.parseShellPipeline()
	if pl == nil {
		return nil
	}
	l := &expr.ShellAndOr{
		Pipeline: []*expr.ShellPipeline{pl},
	}
	for p.s.Token == token.LogicalAnd || p.s.Token == token.LogicalOr {
		l.Sep = append(l.Sep, p.s.Token)
		p.next()
		l.Pipeline = append(l.Pipeline, p.parseShellPipeline())
	}
	return l
}

func (p *Parser) parseShellPipeline() *expr.ShellPipeline {
	bang := false
	if p.s.Token == token.Not {
		bang = true
		p.next()
	}
	cmd := p.parseShellCmd()
	if cmd == nil {
		return nil
	}
	l := &expr.ShellPipeline{
		Bang: bang,
		Cmd:  []*expr.ShellCmd{cmd},
	}
	for p.s.Token == token.ShellPipe {
		p.next()
		l.Cmd = append(l.Cmd, p.parseShellCmd())
	}
	return l
}

func (p *Parser) parseShellCmd() (l *expr.ShellCmd) {
	if p.s.Token == token.LeftParen {
		p.next()
		l = &expr.ShellCmd{
			Subshell: p.parseShellList(),
		}
		p.expect(token.RightParen)
		p.next()
	} else {
		simplecmd := p.parseShellSimpleCmd()
		if simplecmd != nil {
			l = &expr.ShellCmd{
				SimpleCmd: simplecmd,
			}
		}
	}
	return l
}

func isAssignment(word string) (k, v string) {
	for i, r := range word {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			if r == '=' {
				return word[:i], word[i+1:]
			}
			return "", ""
		}
	}
	return "", ""
}

func (p *Parser) parseShellSimpleCmd() (l *expr.ShellSimpleCmd) {
	for {
		w, r := p.maybeParseShellRedirect()
		if r == nil {
			if w == "" {
				if p.s.Token == token.ShellWord {
					w = p.s.Literal.(string)
					p.next()
				} else {
					return l
				}
			}
		}
		if l == nil {
			l = &expr.ShellSimpleCmd{}
		}
		if r != nil {
			l.Redirect = append(l.Redirect, r)
		} else {
			if len(l.Args) == 0 {
				if k, v := isAssignment(w); k != "" {
					l.Assign = append(l.Assign, expr.ShellAssign{
						Key:   k,
						Value: v,
					})
					continue
				}
			}
			l.Args = append(l.Args, w)
		}
	}
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
