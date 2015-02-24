// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/ast"

	"rsc.io/grind/block"
	"rsc.io/grind/grinder"
)

func DeleteUnusedLabels(ctxt *grinder.Context, pkg *grinder.Package) {
	grinder.GrindFuncDecls(ctxt, pkg, func(ctxt *grinder.Context, pkg *grinder.Package, edit *grinder.EditBuffer, fn *ast.FuncDecl) {
		if fn.Body == nil {
			return
		}
		blocks := block.Build(pkg.FileSet, fn.Body)
		ast.Inspect(fn.Body, func(x ast.Node) bool {
			switch x := x.(type) {
			case *ast.LabeledStmt:
				if len(blocks.Goto[x.Label.Name])+len(blocks.Break[x.Label.Name])+len(blocks.Continue[x.Label.Name]) == 0 {
					edit.DeleteLine(x.Pos(), x.Colon+1)
				}
			case ast.Expr:
				return false
			}
			return true
		})
	})
}
