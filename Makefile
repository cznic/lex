# Copyright 2009 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

include $(GOROOT)/src/Make.inc

TARG=github.com/cznic/lex

GOFILES=\
	dfa.go\
    lex.go\
	parser.go\
	ranges.go\
	rule.go\
	scanner.go\
	stateset.go\

CLEANFILES += y.output

include $(GOROOT)/src/Make.pkg

parser.go: parser.y
	goyacc -o parser.go parser.y
	sed -i -e 's|//line.*||' parser.go
	gofmt -w parser.go
