// Copyright (c) 2011 CZ.NIC z.s.p.o. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// blame: jnml, labs.nic.cz

package lex

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

var trace *bool

func init() {
	hook = true
	trace = flag.Bool("trace", false, "allow test runtime error stack traces")
}

func TestLineSeparators(t *testing.T) {
	const text = `
%{
package main

import (
    "bufio"
    "log"
    "strconv"
)

type yylexer struct{
    src     *bufio.Reader
    buf     []byte
    empty   bool
    current byte
}

func newLexer(src *bufio.Reader) (y *yylexer) {
    y = &yylexer{src: src}
    if b, err := src.ReadByte(); err == nil {
        y.current = b
    }
    return
}

func (y *yylexer) getc() byte {
    if y.current != 0 {
        y.buf = append(y.buf, y.current)
    }
    y.current = 0
    if b, err := y.src.ReadByte(); err == nil {
        y.current = b
    }
    return y.current
}

func (y yylexer) Error(e string) {
    log.Fatal(e)
}

func (y *yylexer) Lex(lval *yySymType) int {
    var err error
    c := y.current
    if y.empty {
        c, y.empty = y.getc(), false
    }
%}

%yyc c
%yyn c = y.getc()

D  [0-9]+
E  [eE][-+]?{D}
F  {D}"."{D}?{E}?|{D}{E}?|"."{D}{E}?

%%
    y.buf = y.buf[:0]

[ \t\r]+

{F}
    if lval.value, err = strconv.ParseFloat(string(y.buf), 64); err != nil {
        log.Fatal(err)
    }

    return NUM

	// https://github.com/cznic/golex/issues/1
a[ \-*]
b[ -*]
c[ \-*]
d[\+\-\*]


%%
    y.empty = true
    return int(c)
}`
	text2 := strings.Replace(text, "\n", "\r\n", -1)
	if !(len(text2) > len(text)) {
		t.Fatal()
	}

	if !*trace {
		defer func() {
			if e := recover(); e != nil {
				t.Fatal(e)
			}
		}()
	}

	src := bytes.NewBufferString(text2)
	if _, err := NewL("test", src, false, false); err != nil {
		t.Fatal(err)
	}
}
