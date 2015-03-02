// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package grinder

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type EditBuffer struct {
	edits []edit
	src   *token.File
	text  string
}

func (b *EditBuffer) NumEdits() int {
	return len(b.edits)
}

func NewEditBuffer(pkg *Package, filename string, f *ast.File) *EditBuffer {
	// Find *token.File via package clause. Must match expected file name.
	// TODO(rsc): Handle mismatch gracefully (think yacc etc).
	src := pkg.FileSet.File(f.Package)
	if src.Name() != filename {
		panic("package statement not in source file")
	}

	return &EditBuffer{src: src, text: pkg.Src(filename)}
}

func (b *EditBuffer) tx(p token.Pos) int {
	return b.src.Offset(p)
}

const (
	Insert = 1 + iota
	Delete
)

type edit struct {
	start int
	end   int
	text  string
}

// End returns x.End() except that it works around buggy results from
// the implementation of *ast.LabeledStmt and *ast.EmptyStmt.
// The node x must be located within b's source file.
// See golang.org/issue/9979.
func (b *EditBuffer) End(x ast.Node) token.Pos {
	switch x := x.(type) {
	case *ast.LabeledStmt:
		if _, ok := x.Stmt.(*ast.EmptyStmt); ok {
			return x.Colon + 1
		}
		return b.End(x.Stmt)
	case *ast.EmptyStmt:
		i := b.tx(x.Semicolon)
		if strings.HasPrefix(b.text[i:], ";") {
			return x.Semicolon + 1
		}
		return x.Semicolon
	}
	return x.End()
}

// BeforeComments rewinds start past any blank lines or line comments
// and return the result. It does not rewind past leading blank lines:
// the returned position, if changed, is always the start of a non-blank line.
func (b *EditBuffer) BeforeComments(start token.Pos) token.Pos {
	i := b.tx(start)
	// Back up to newline.
	for i > 0 && (b.text[i-1] == ' ' || b.text[i-1] == '\t') {
		i--
	}
	if i > 0 && b.text[i-1] != '\n' {
		return start
	}

	// Go backward by lines.
	lastNonBlank := i
	for i > 0 {
		j := i - 1
		for j > 0 && b.text[j-1] != '\n' {
			j--
		}
		trim := strings.TrimSpace(b.text[j:i])
		if len(trim) > 0 && !strings.HasPrefix(trim, "//") {
			break
		}
		if len(trim) > 0 {
			lastNonBlank = j
		}
		i = j
	}
	return start - token.Pos(b.tx(start)-lastNonBlank)
}

func (b *EditBuffer) TextAt(start, end token.Pos) string {
	return string(b.text[b.tx(start):b.tx(end)])
}

func (b *EditBuffer) Insert(p token.Pos, text string) {
	b.edits = append(b.edits, edit{b.tx(p), b.tx(p), text})
}

func (b *EditBuffer) Replace(start, end token.Pos, text string) {
	b.edits = append(b.edits, edit{b.tx(start), b.tx(end), text})
}

func (b *EditBuffer) Delete(startp, endp token.Pos) {
	b.edits = append(b.edits, edit{b.tx(startp), b.tx(endp), ""})
}

func (b *EditBuffer) DeleteLine(startp, endp token.Pos) {
	start := b.tx(startp)
	end := b.tx(endp)
	i := end
	for i < len(b.text) && (b.text[i] == ' ' || b.text[i] == '\t' || b.text[i] == '\r') {
		i++
	}
	// delete comment too
	if i+2 < len(b.text) && b.text[i] == '/' && b.text[i+1] == '/' {
		j := strings.Index(b.text[i:], "\n")
		if j >= 0 {
			i += j
		}
	}
	if i == len(b.text) || b.text[i] == '\n' {
		end = i + 1
		i := start
		for i > 0 && (b.text[i-1] == ' ' || b.text[i-1] == '\t') {
			i--
		}
		if i == 0 || b.text[i-1] == '\n' {
			start = i
		}
	}
	b.edits = append(b.edits, edit{start, end, ""})
}

func (b *EditBuffer) CopyLine(startp, endp, insertp token.Pos) {
	start := b.tx(startp)
	end := b.tx(endp)
	i := end
	for i < len(b.text) && (b.text[i] == ' ' || b.text[i] == '\t' || b.text[i] == '\r') {
		i++
	}
	// copy comment too
	if i+2 < len(b.text) && b.text[i] == '/' && b.text[i+1] == '/' {
		j := strings.Index(b.text[i:], "\n")
		if j >= 0 {
			i += j
		}
	}
	if i == len(b.text) || b.text[i] == '\n' {
		end = i + 1
	}
	text := string(b.text[start:end])
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}

	insert := b.tx(insertp)
	j := insert
	for j > 0 && (b.text[j-1] == ' ' || b.text[j-1] == '\t') {
		j--
	}
	if j == 0 || b.text[j-1] == '\n' {
		text = string(b.text[j:insert]) + text
		insert = j
	}
	b.edits = append(b.edits, edit{insert, insert, text})
}

func (b *EditBuffer) Apply() string {
	sort.Sort(editsByStart(b.edits))
	var out []byte
	last := 0
	for _, e := range b.edits {
		//fmt.Printf("EDIT: %+v\n", e)
		if e.start < last {
			for _, e := range b.edits {
				fmt.Printf("%d,%d %q\n", e.start, e.end, e.text)
			}
			panic("overlapping edits")
		}
		out = append(out, b.text[last:e.start]...)
		out = append(out, e.text...)
		last = e.end
	}
	out = append(out, b.text[last:]...)
	return string(out)
}

type editsByStart []edit

func (x editsByStart) Len() int      { return len(x) }
func (x editsByStart) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x editsByStart) Less(i, j int) bool {
	if x[i].start != x[j].start {
		return x[i].start < x[j].start
	}
	if x[i].end != x[j].end {
		return x[i].end < x[j].end
	}
	return x[i].text < x[j].text
}

func Diff(old, new string) []byte {
	f1, err := ioutil.TempFile("", "go-fix")
	if err != nil {
		return []byte(fmt.Sprintf("writing temp file: %v\n", err))
	}
	defer os.Remove(f1.Name())
	defer f1.Close()

	f2, err := ioutil.TempFile("", "go-fix")
	if err != nil {
		return []byte(fmt.Sprintf("writing temp file: %v\n", err))
	}
	defer os.Remove(f2.Name())
	defer f2.Close()

	f1.Write([]byte(old))
	f2.Write([]byte(new))

	// Use git diff to get consistent output and also for the context after @@ lines.
	data, err := exec.Command("git", "diff", f1.Name(), f2.Name()).CombinedOutput()
	if len(data) > 0 {
		// diff exits with a non-zero status when the files don't match.
		// Ignore that failure as long as we get output.
		err = nil
	}
	if err != nil {
		return []byte(fmt.Sprintf("invoking git diff: %v\n%s", err, data))
	}
	// skip over diff header, since it is showing temporary file names
	i := bytes.Index(data, []byte("\n@@"))
	if i >= 0 {
		data = data[i+1:]
	}
	return data
}
