// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graph contains graph-related utility functions.
package graph

type bipartite struct {
	Edges map[int][]int
}

// NewBipartite returns a bipartite graph structure.
func NewBipartite() *bipartite {
	var g bipartite
	g.Edges = make(map[int][]int)
	return &g
}

// AddEdge adds an edge from x to y
func (g bipartite) AddEdge(x, y int) {
	if _, ok := g.Edges[x]; !ok {
		g.Edges[x] = []int{y}
	} else {
		g.Edges[x] = append(g.Edges[x], y)
	}
}

// inSlice returns true if the given value is in the list.
func inSlice(value int, slice []int) bool {
	for _, a := range slice {
		if value == a {
			return true
		}
	}
	return false
}

// matchingHelper returns true if maching for vertex u is possible.
func (g bipartite) matchingHelper(u int, match map[int]int, seen map[int]bool) bool {
	for _, dest := range g.Edges[u] {
		if seen[dest] {
			continue
		}

		seen[dest] = true
		if _, ok := match[dest]; !ok || g.matchingHelper(match[dest], match, seen) {
			match[dest] = u
			return true
		}
	}
	return false
}

// MaxMaching returns the max matching for the bipartite graph
func (g bipartite) MaxMatching() int {
	var dests []int
	for _, edges := range g.Edges {
		for _, dest := range edges {
			if !inSlice(dest, dests) {
				dests = append(dests, dest)
			}
		}
	}
	matchMap := make(map[int]int)
	visited := make(map[int]bool)
	count := 0
	for src := range g.Edges {
		for _, dest := range dests {
			visited[dest] = false
		}
		if g.matchingHelper(src, matchMap, visited) {
			count = count + 1
		}
	}
	return count
}
