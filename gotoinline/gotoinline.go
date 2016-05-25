// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gotoinline

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"go/types"

	"rsc.io/grind/block"
	"rsc.io/grind/grinder"
)

var debug = false

func Grind(ctxt *grinder.Context, pkg *grinder.Package) {
	grinder.GrindFuncDecls(ctxt, pkg, grindFunc)
}

type targetBlock struct {
	comment    token.Pos
	start      token.Pos
	endLabel   token.Pos
	end        token.Pos
	code       string
	needReturn bool
	needGoto   string
	short      bool
	dead       bool
	objs       []types.Object
}

func grindFunc(ctxt *grinder.Context, pkg *grinder.Package, edit *grinder.EditBuffer, fn *ast.FuncDecl) {
	if fn.Name.Name == "evconst" {
		old := debug
		debug = true
		defer func() { debug = old }()
	}

	if pkg.TypesError != nil {
		// Without scoping information, we can't be sure code moves are okay.
		fmt.Printf("%s: cannot inline gotos without type information\n", fn.Name)
		return
	}

	if fn.Body == nil {
		return
	}
	blocks := block.Build(pkg.FileSet, fn.Body)
	for labelname, gotos := range blocks.Goto {
		target, ok := findTargetBlock(pkg, edit, fn, blocks, labelname)
		if debug {
			println("TARGET", ok, labelname, len(gotos), target.dead, target.short)
		}
		if ok && (len(gotos) == 1 && target.dead || target.short) {
			numReplaced := 0
			for _, g := range gotos {
				code := edit.TextAt(target.comment, target.start) + target.code
				if !objsMatch(pkg, fn, g.Pos(), target.objs, target.start, target.end) {
					if debug {
						println("OBJS DO NOT MATCH")
					}
					// Cannot inline code here; needed identifiers have different meanings.
					continue
				}
				if target.needReturn {
					// NOTE: Should really check to see if function results are shadowed.
					// If we screw up, the code won't compile, so we can put it off.
					code += "; return"
				}
				if target.needGoto != "" {
					code += "; goto " + target.needGoto
				}
				edit.Replace(g.Pos(), g.End(), code)
				numReplaced++
			}
			if numReplaced == len(gotos) {
				if len(gotos) == 1 && target.dead {
					edit.Delete(target.comment, target.end)
				} else {
					edit.DeleteLine(target.start, target.endLabel)
				}
			}
			// The code we move might itself have gotos to inline,
			// and we can't make that change until we get new line
			// number position, so return after each label change.
			if numReplaced > 0 {
				return
			}
		}
	}
}

func findTargetBlock(pkg *grinder.Package, edit *grinder.EditBuffer, fn *ast.FuncDecl, blocks *block.Graph, labelname string) (target targetBlock, ok bool) {
	if debug {
		println("FINDTARGET", labelname)
	}
	lstmt := blocks.Label[labelname]
	if lstmt == nil {
		return
	}

	list := grinder.BlockList(blocks.Map[lstmt].Root)
	if list == nil {
		return
	}

	ulstmt := grinder.Unlabel(lstmt)
	for i := 0; i < len(list); i++ {
		if grinder.Unlabel(list[i]) == ulstmt {
			// Found statement. Find extent of block.
			if debug {
				println("FOUND")
			}
			end := i
			for ; ; end++ {
				if end >= len(list) {
					if debug {
						println("EARLY END")
					}
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
					if debug {
						println("FOUND TARGET")
					}
					target.needGoto = list[end].(*ast.LabeledStmt).Label.Name
					break
				}
				if grinder.IsTerminatingStmt(blocks, list[end]) {
					if debug {
						println("TERMINATING")
					}
					end++
					break
				}
			}
			if end <= i {
				if debug {
					println("NOTHING")
				}
				return
			}
			if debug {
				println("OK")
			}
			target.dead = i > 0 && grinder.IsTerminatingStmt(blocks, list[i-1])
			target.start = lstmt.Pos()
			target.comment = edit.BeforeComments(target.start)
			target.endLabel = lstmt.Colon + 1
			target.end = edit.End(list[end-1])
			target.code = strings.TrimSpace(edit.TextAt(lstmt.Colon+1, target.end))
			target.short = end == i+1 && (isReturn(grinder.Unlabel(list[i])) || isEmpty(grinder.Unlabel(list[i])) && target.needReturn)
			target.objs = gatherObjs(pkg, fn, lstmt.Pos(), list[i:end])
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

func gatherObjs(pkg *grinder.Package, fn *ast.FuncDecl, start token.Pos, list []ast.Stmt) []types.Object {
	seen := make(map[types.Object]bool)
	var objs []types.Object
	addObj := func(obj types.Object) {
		if obj == nil || seen[obj] {
			return
		}
		switch obj := obj.(type) {
		case *types.Label:
			return
		case *types.Var:
			if obj.IsField() {
				return
			}
		}
		seen[obj] = true
		objs = append(objs, obj)
	}
	ignore := make(map[*ast.Ident]bool)
	for _, stmt := range list {
		ast.Inspect(stmt, func(x ast.Node) bool {
			switch x := x.(type) {
			case *ast.SelectorExpr:
				ignore[x.Sel] = true
			case *ast.Ident:
				if !ignore[x] {
					addObj(pkg.Info.Uses[x])
				}
			case *ast.ReturnStmt:
				if len(x.Results) == 0 && fn.Type.Results != nil {
					for _, field := range fn.Type.Results.List {
						for _, id := range field.Names {
							if pkg.Info.Defs[id] == nil {
								break
							}
							addObj(pkg.Info.Defs[id])
						}
					}
				}
			}
			return true
		})
	}
	return objs
}

func objsMatch(pkg *grinder.Package, fn *ast.FuncDecl, pos token.Pos, objs []types.Object, start, end token.Pos) bool {
	for _, obj := range objs {
		if start < obj.Pos() && obj.Pos() < end {
			// declaration is in code being moved
			return true
		}
		if pkg.LookupAtPos(fn, pos, obj.Name()) != obj {
			if debug {
				println("OBJ MISMATCH", obj.Name(), pkg.LookupAtPos(fn, pos, obj.Name()), obj)
			}
			return false
		}
	}
	return true
}
