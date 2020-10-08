// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graph contains graph-related utility functions.
package graph

type bipartite struct {
	edge map[int][]int
}

// NewBipartite returns a bipartite graph structure.
func NewBipartite() *bipartite {
	var g bipartite
	g.edge = make(map[int][]int)
	return &g
}

// AddEdge adds an edge from x to y. Note that x would be source and y would be sink. Those two number are treated differently even though their number is the same. In other word, AddEdge(1, 1) is not a self-loop but an edge from source #1 to sink #1.
func (g bipartite) AddEdge(x, y int) {
	if edges, ok := g.edge[x]; !ok {
		g.edge[x] = []int{y}
	} else {
		g.edge[x] = append(edges, y)
	}
}

// matchingHelper returns true if matching for vertex u is possible.
func (g bipartite) matchingHelper(u int, match map[int]int, seen map[int]bool) bool {
	for _, dest := range g.edge[u] {
		if seen[dest] {
			continue
		}

		seen[dest] = true
		if matched, ok := match[dest]; !ok || g.matchingHelper(matched, match, seen) {
			match[dest] = u
			return true
		}
	}
	return false
}

// MaxMatching returns the max number of matchings in the bipartite graph.
func (g bipartite) MaxMatching() int {
	matchMap := make(map[int]int)
	count := 0
	for src := range g.edge {
		if g.matchingHelper(src, matchMap, map[int]bool{}) {
			count = count + 1
		}
	}
	return count
}
