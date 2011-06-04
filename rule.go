// Copyright (c) 2011 CZ.NIC z.s.p.o. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// blame: jnml, labs.nic.cz


package lex


import (
	"bytes"
	"fmt"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"unicode"
	"utf8"
)


var (
	nameFirst = &unicode.RangeTable{R16: []unicode.Range16{{'A', 'Z', 1}, {'_', '_', 1}, {'a', 'z', 1}}}
	nameNext  = &unicode.RangeTable{R16: []unicode.Range16{{'-', '-', 1}, {'0', '9', 1}, {'A', 'Z', 1}, {'_', '_', 1}, {'a', 'z', 1}}}
)


func parsePattern(pos token.Position, src string, stack map[string]bool) (pattern, re, action string, bol, eol bool) {
	p := &pat{src: src, re: bytes.NewBuffer(nil), stack: stack}

	defer func() {
		if e := recover(); e != nil {
			pos.Column += p.pos
			logErr(fmt.Sprintf(`%s - "%s^%s" - %s`, pos, src[:p.pos], src[p.pos:], e.(os.Error)))
		}
	}()

	p.parseExpr(0)
	pattern, re = src[:p.pos], p.re.String()
	bol, eol = p.bol, p.eol
	switch b := p.current(); b {
	case 0:
		return
	case ' ', '\t':
		p.move()
		action = src[p.pos:]
		return
	}
	panic(os.NewError("syntax error"))
}


type pat struct {
	src      string
	pos      int
	delta    int
	re       *bytes.Buffer
	stack    map[string]bool
	bol, eol bool
}


func (p *pat) current() (y int) {
	if i := p.pos; i < len(p.src) {
		if !bits32 {
			return int(p.src[i])
		}

		y, p.delta = utf8.DecodeRuneInString(p.src[i:])
		return
	}

	return 0
}


func (p *pat) eof(whiteIsEof bool) bool {
	b := p.current()
	return b == 0 || whiteIsEof && (b == ' ' || b == '\t')
}


func (p *pat) move() {
	if p.pos < len(p.src) {
		if !bits32 {
			p.pos++
		} else {
			p.pos += p.delta
		}
	}
	return
}


func (p *pat) accept(b int) bool {
	if b == p.current() {
		p.move()
		return true
	}

	return false
}


func (p *pat) parseExpr(nest int) {
	ok := false
	for !p.eof(true) {
		p.parseAlt(nest)
		ok = true
		if !p.accept('|') {
			break
		}

		p.re.WriteRune('|')
	}
	if ok {
		return
	}

	panic(os.NewError(`expected "alernative"`))
}


func (p *pat) parseAlt(nest int) {
	ok := false
	for p.current() != 0 {
		if !p.parseTerm(nest) {
			break
		}

		ok = true
	}
	if ok {
		return
	}

	panic(os.NewError(`expected "term"`))
}


func (p *pat) parseTerm(nest int) (ok bool) {
	ok = true
	switch b := p.current(); b {
	default:
		p.re.WriteRune(b)
		p.move()
	case '$':
		p.move()
		if p.pos != len(p.src) && p.current() != ' ' && p.current() != '\t' { // not an assertion
			p.re.WriteString(`\$`)
		} else {
			p.re.WriteString(`(\n|\x00)`)
			p.eol = true
		}
	case '^':
		p.move()
		if p.pos != 1 { // not an assertion
			p.re.WriteString(`\^`)
		} else {
			p.bol = true
		}
	case '/':
		panic(os.NewError("trailing context not supported"))
	case '.':
		p.move()
		if !bits32 {
			p.re.WriteString("[\x01-\x09\x0b-\u00ff]")
		} else {
			p.re.WriteString("[\x01-\x09\x0b-\U0010ffff]")
		}
	case '+', '*', '?':
		panic(fmt.Errorf("unexpected metachar %q", string(b)))
	case '\\':
		switch b := p.mustParseChar(false); b {
		default:
			p.re.WriteString(regexp.QuoteMeta(string(b)))
		case 0:
			p.re.WriteString("\\x00")
		}
	case 0, '|', ' ', '\t':
		return false
	case ')':
		if nest == 0 {
			panic(os.NewError(`unexpected ")"`))
		}
		return false
	case '(':
		p.re.WriteRune(b)
		p.move()
		p.parseExpr(nest + 1)
		p.expect(')')
		p.re.WriteRune(')')
	case '[':
		p.re.WriteRune(b)
		p.move()
		if p.accept('^') {
			p.re.WriteString("^\\x00-\\x00")
		}
		for {
			a := p.mustParseChar(false)
			p.re.WriteString(regexp.QuoteMeta(string(a)))
			if p.accept('-') {
				p.re.WriteRune('-')
				b := p.mustParseChar(false)
				if b < a {
					panic(fmt.Errorf(`invalid range bounds ordering in bracket expression "%s-%s"`, string(a), string(b)))
				}
				p.re.WriteString(regexp.QuoteMeta(string(b)))
			}
			if p.accept(']') {
				break
			}
		}
		p.re.WriteRune(']')
	case '{':
		p.move()
		if !unicode.Is(nameFirst, p.current()) {
			p.re.WriteRune('{')
			break
		}

		name := ""
		for {
			b := p.current()
			if !unicode.Is(nameNext, b) {
				break
			}
			p.move()
			name += string(b)
		}
		p.expect('}')
		if _, ok := defs[name]; !ok {
			panic(fmt.Errorf("%q undefined", name))
		}

		if re, ok := defRE[name]; ok {
			p.re.WriteString(re)
			break
		}

		if p.stack[name] {
			panic(fmt.Errorf("recursive definition %q", name))
		}

		p.stack[name] = true
		//TODO support assertions in definitions also?
		_, re, _, _, _ := parsePattern(defPos[name], defs[name], p.stack)
		re = "(" + re + ")"
		defRE[name] = re
		p.re.WriteString(re)
	case '"':
		p.move()
		lit := ""
	outer:
		for {
			switch b := p.current(); b {
			default:
				lit += string(b)
				p.move()
			case 0, '\n', '\r':
				panic(fmt.Errorf("unterminated quoted pattern"))
			case '\\':
				p.move()
				if p.current() == '"' {
					p.move()
					lit += "\""
				} else {
					lit += "\\"
				}
			case '"':
				p.move()
				break outer
			}
		}
		lit = "(" + regexp.QuoteMeta(lit) + ")"
		p.re.WriteString(lit)
	}

	// postfix ops
	switch b := p.current(); b {
	case '+', '*', '?':
		p.re.WriteRune(b)
		p.move()
	}

	return
}


func (p *pat) mustParseChar(whiteIsEof bool) (b int) {
	if p.eof(whiteIsEof) {
		panic(fmt.Errorf("unexpected regexp end"))
	}

	b = p.parseChar()
	p.move()
	return
}


func (p *pat) parseChar() (b int) {
	if b = p.current(); b != '\\' {
		return
	}

	p.move()
	switch b = p.current(); b {
	default:
		return
	case 'a':
		return '\a'
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	case 'v':
		return '\v'
	case 'x':
		s := ""
		for i := 0; i < 2; i++ {
			if p.eof(true) {
				panic(os.NewError("unexpected regexp end"))
			}
			p.move()
			s += string(p.current())
		}
		n, err := strconv.Btoui64(s, 16)
		if err != nil {
			panic(err)
		}

		return int(n)
	case 'u':
		s := ""
		for i := 0; i < 4; i++ {
			if p.eof(true) {
				panic(os.NewError("unexpected regexp end"))
			}
			p.move()
			s += string(p.current())
		}
		n, err := strconv.Btoui64(s, 16)
		if err != nil {
			panic(err)
		}

		return int(n)
	case 'U':
		s := ""
		for i := 0; i < 8; i++ {
			if p.eof(true) {
				panic(os.NewError("unexpected regexp end"))
			}
			p.move()
			s += string(p.current())
		}
		n, err := strconv.Btoui64(s, 16)
		if err != nil {
			panic(err)
		}

		return int(n)
	case '0', '1', '2', '3', '4', '5', '6', '7':
		s := ""
		for b = p.current(); (len(s) < 3 || bits32 && len(s) < 7) && b >= '0' && b <= '7'; b = p.current() {
			s += string(b)
			p.move()
		}
		n, err := strconv.Btoui64(s, 8)
		if err != nil {
			panic(err)
		}

		if !bits32 && n > 255 {
			panic(fmt.Errorf("octal literal %s out of byte range", s))
		}

		p.pos--
		return int(n)
	}

	panic("unreachable")
}


func (p *pat) expect(b int) {
	if !p.accept(b) {
		panic(fmt.Errorf("expected %q, got %q", string(b), string(p.current())))
	}
}


func moreAction(s string) {
	n := len(rules) - 1
	rules[n].action += "\n" + s
}

func addStartSet(s string) bool {
	if _, ok := defStarts[s]; ok {
		return false
	}

	iStarts[s] = len(iStarts)
	defStarts[s], unrefStarts[s] = true, true
	return true
}
