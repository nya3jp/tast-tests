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

// New returns a StringSet of given strings.
func New(strs []string) StringSet {
	ret := StringSet{}
	for _, s := range strs {
		ret[s] = struct{}{}
	}
	return ret
}

// Union unions StringSet a and b.
func (a StringSet) Union(b StringSet) StringSet {
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
func (a StringSet) Diff(b StringSet) StringSet {
	ret := StringSet{}
	for k := range a {
		if _, ok := b[k]; !ok {
			ret[k] = struct{}{}
		}
	}
	return ret
}

// Intersect returns the intersection of StringSet a and b.
func (a StringSet) Intersect(b StringSet) StringSet {
	ret := StringSet{}
	for k := range a {
		if _, ok := b[k]; ok {
			ret[k] = struct{}{}
		}
	}
	return ret
}

// Has returns true if s is in StringSet a.
func (a StringSet) Has(k string) bool {
	_, ok := a[k]
	return ok
}

// Equal returns true if elements of a and b are the same.
func (a StringSet) Equal(b StringSet) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// Slice returns an sorted array of its elements.
func (a StringSet) Slice() []string {
	var keys []string
	for k := range a {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// String implements the Stringer interface for StringSet.
func (a StringSet) String() string {
	return fmt.Sprintf("StringSet(%q)", a.Slice())
}
