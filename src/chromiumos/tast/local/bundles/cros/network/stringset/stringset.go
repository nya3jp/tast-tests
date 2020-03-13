// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package stringset defines basic operation of set of strings.
package stringset

import (
	"fmt"
	"sort"
)

// StringSet is a set of strings.
type StringSet map[string]struct{}

// NewStringSet returns a StringSet of given strings.
func NewStringSet(strs []string) StringSet {
	ret := StringSet{}
	for _, s := range strs {
		ret[s] = struct{}{}
	}
	return ret
}

// Union unions StringSet a and b.
func Union(a, b StringSet) StringSet {
	ret := StringSet{}
	for k := range a {
		ret[k] = struct{}{}
	}
	for k := range b {
		ret[k] = struct{}{}
	}
	return ret
}

// Diff returns the difference of StringSet a and b.
func Diff(a, b StringSet) StringSet {
	ret := StringSet{}
	for k := range a {
		if _, ok := b[k]; !ok {
			ret[k] = struct{}{}
		}
	}
	return ret
}

// Interset returns the intersection of StringSet a and b.
func Interset(a, b StringSet) StringSet {
	ret := StringSet{}
	for k := range a {
		if _, ok := b[k]; ok {
			ret[k] = struct{}{}
		}
	}
	return ret
}

// String implements the String interface for StringSet.
func (s StringSet) String() string {
	var keys []string
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Sprintf("StringSet(%q)", keys)
}
