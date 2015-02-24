// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gotoinline

import (
	"testing"

	"rsc.io/grind/grinder"
	"rsc.io/grind/grindtest"
)

func TestGrind(t *testing.T) {
	grindtest.TestGlob(t, "testdata/grind-*.go", []grinder.Func{Grind})
}
