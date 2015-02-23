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
*/
package main
