// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package graph

import "testing"

func TestEmptyGraph(t *testing.T) {
	g := NewBipartite()

	if result := g.MaxMatching(); result != 0 {
		t.Errorf("Wrong number of max matching, expect 0 but got %d", result)
	}
}

func TestOneToOneGraph(t *testing.T) {
	g := NewBipartite()
	expect := 5

	for i := 0; i < 5; i++ {
		g.AddEdge(i, 4-i)
	}
	if result := g.MaxMatching(); result != expect {
		t.Errorf("Wrong number of max matching, expect %d but got %d", expect, result)
	}
}

func TestPartialOneToOneGraph(t *testing.T) {
	g := NewBipartite()
	expect := 2

	for i := 0; i < 3; i++ {
		g.AddEdge(i, 0)
		g.AddEdge(i, 1)
	}

	if result := g.MaxMatching(); result != expect {
		t.Errorf("Wrong number of max matching, expect %d but got %d", expect, result)
	}
}

func TestPartialOneToOneGraph2(t *testing.T) {
	g := NewBipartite()
	expect := 4

	for i := 0; i < 3; i++ {
		g.AddEdge(i, i)
		g.AddEdge(i, 2-i)
	}
	g.AddEdge(3, 3)

	if result := g.MaxMatching(); result != expect {
		t.Errorf("Wrong number of max matching, expect %d but got %d", expect, result)
	}
}
