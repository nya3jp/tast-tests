// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package set provides utility set operations.
package set

// DiffStringSlice returns b - a (where - is  the set difference operator).
// In other words, it returns all elements of |b| that are not in |a|.
func DiffStringSlice(b, a []string) []string {
	om := make(map[string]bool, len(a))
	for _, p := range a {
		om[p] = true
	}

	var out []string
	for _, p := range b {
		if !om[p] {
			out = append(out, p)
		}
	}
	return out
}
