// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Grind polishes Go programs.

Usage:
	grind [-diff] [-v] packagepath...

Grind rewrites the source files in the named packages.
When grind rewrites a file, it prints a line to standard
error giving the name of the file and the rewrites applied.

As a special case, if the arguments are a list of Go source files,
they are considered to make up a single package, which
is then rewritten.

If the -diff flag is set, no files are rewritten.
Instead grind prints the differences a rewrite would introduce.

Grind does not make backup copies of the files that it edits.
Instead, use a version control system's ``diff'' functionality to inspect
the changes that grind makes before committing them.

Grind is a work in progress. More rewrites are planned.
The initial use case for grind is cleaning up Go code that looks
like C code.

Dead Code Elimination

Grind removes unreachable (dead) code. Code is considered
unreachable if it is preceded by a terminating statement
(see golang.org/ref/spec#Terminating_statements) and
is not the target of a goto statement.

Limitation: The analysis treats all block, if, for, switch, and
select statements as non-terminating. In the future it may
implement the more refined definitions given in the Go spec.

Var Declaration Placement

Grind moves var declarations as close as possible to their uses.
When possible, it combines the declaration with the initialization,
and it splits disjoint uses of a single variable into multiple variables.

For example, consider:

	var i int

	...

	for i = 0; i < 10; i++ {
		use(i)
	}

	...

	for i = 0; i < 10; i++ {
		otherUse(i)
	}

Grind deletes the declaration and rewrites both loop initializers
to use a combined declaration and assignment (i := 0).

Limitation: Grind does not move variable declarations into loop bodies.
It may in the future, provided it can determine that the variable
is dead on entry to the body and that the variable does not
escape (causing heap allocation inside the loop).

Limitation: Grind considers variables that appear in closures off limits.
A more sophisticated analysis might consider them in the future.

*/
package main
