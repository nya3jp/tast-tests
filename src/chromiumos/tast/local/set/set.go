// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package set provides utility set operations.
package set

// DiffStringSlice returns a - b (where - is  the set difference operator).
// In other words, it returns all elements of |a| that are not in |b|.
func DiffStringSlice(a, b []string) []string {
	om := make(map[string]bool, len(b))
	for _, p := range b {
		om[p] = true
	}

	var out []string
	for _, p := range a {
		if !om[p] {
			out = append(out, p)
		}
	}
	return out
}
