// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gotoinline

import (
	"go/ast"
	"go/token"

	"rsc.io/grind/block"
	"rsc.io/grind/grinder"
)

func Grind(ctxt *grinder.Context, pkg *grinder.Package) {
	grinder.GrindFuncDecls(ctxt, pkg, grindFunc)
}

type targetBlock struct {
	start      token.Pos
	endLabel   token.Pos
	end        token.Pos
	code       string
	needReturn bool
	needGoto   string
	short      bool
	dead       bool
}

func grindFunc(ctxt *grinder.Context, pkg *grinder.Package, edit *grinder.EditBuffer, fn *ast.FuncDecl) {
	if fn.Body == nil {
		return
	}
	blocks := block.Build(pkg.FileSet, fn.Body)
	for labelname, gotos := range blocks.Goto {
		target, ok := findTargetBlock(pkg, edit, fn, blocks, labelname)
		if ok && (len(gotos) == 1 && target.dead || target.short) {
			for _, g := range gotos {
				code := target.code
				if target.needReturn {
					// NOTE: Should really check to see if function results are shadowed.
					// If we screw up, the code won't compile, so we can put it off.
					code += "; return"
				}
				if target.needGoto != "" {
					code += "; goto " + target.needGoto
				}
				edit.Replace(g.Pos(), g.End(), code)
			}
			if len(gotos) == 1 && target.dead {
				edit.Delete(target.start, target.end)
			} else {
				edit.DeleteLine(target.start, target.endLabel)
			}
			return
		}
	}
}

func findTargetBlock(pkg *grinder.Package, edit *grinder.EditBuffer, fn *ast.FuncDecl, blocks *block.Graph, labelname string) (target targetBlock, ok bool) {
	lstmt := blocks.Label[labelname]
	if lstmt == nil {
		return
	}

	var list []ast.Stmt
	switch x := blocks.Map[lstmt].Root.(type) {
	default:
		return
	case *ast.BlockStmt:
		list = x.List
	case *ast.CommClause:
		list = x.Body
	case *ast.CaseClause:
		list = x.Body
	}

	ulstmt := grinder.Unlabel(lstmt)
	for i := 0; i < len(list); i++ {
		if grinder.Unlabel(list[i]) == ulstmt {
			// Found statement. Find extent of block.
			end := i
			for ; ; end++ {
				if end >= len(list) {
					// List ended without terminating statement.
					// Unless this is the top-most block, we can't hoist this code.
					if blocks.Map[lstmt].Root != fn.Body {
						return
					}
					// Top-most block. Implicit return at end of list.
					target.needReturn = true
					break
				}
				if end > i && grinder.IsGotoTarget(blocks, list[end]) {
					target.needGoto = list[end].(*ast.LabeledStmt).Label.Name
					break
				}
				if grinder.IsTerminatingStmt(blocks, list[end]) {
					end++
					break
				}
			}
			if end <= i {
				return
			}
			target.dead = i > 0 && grinder.IsTerminatingStmt(blocks, list[i-1])
			target.start = lstmt.Pos()
			target.endLabel = lstmt.Colon + 1
			target.end = edit.End(list[end-1])
			target.code = edit.TextAt(lstmt.Colon+1, target.end)
			target.short = end == i+1 && (isReturn(grinder.Unlabel(list[i])) || isEmpty(grinder.Unlabel(list[i])) && target.needReturn)
			return target, true
		}
	}
	return
}

func isReturn(x ast.Stmt) bool {
	_, ok := x.(*ast.ReturnStmt)
	return ok
}

func isEmpty(x ast.Stmt) bool {
	_, ok := x.(*ast.EmptyStmt)
	return ok
}
