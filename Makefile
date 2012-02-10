# Copyright (c) 2011 CZ.NIC z.s.p.o. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# blame: jnml, labs.nic.cz

all: parser.go

parser.go: parser.y
	go tool yacc -o parser.go parser.y
	sed -i -e 's|//line.*||' parser.go
	gofmt -w parser.go

clean:
	rm -f parser.go y.output *~
