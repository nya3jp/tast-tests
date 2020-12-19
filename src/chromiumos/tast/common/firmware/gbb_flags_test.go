// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"reflect"
	"testing"

	pb "chromiumos/tast/services/cros/firmware"
)

// flags converts a list of ints to a slice of pb.GBBFlags.
func flags(f ...int) []pb.GBBFlag {
	var fs []pb.GBBFlag
	for _, i := range f {
		fs = append(fs, pb.GBBFlag(i))
	}
	return fs
}

// state constructs a GBBFlagsState.
func state(clear, set []pb.GBBFlag) pb.GBBFlagsState {
	return pb.GBBFlagsState{Clear: clear, Set: set}
}

// compare returns true if the slices are equal.
func compare(a, b []pb.GBBFlag) bool {
	// Bizzarely DeepEqual considers empty slices to be unequal, I couldn't find any doc on this!
	if len(a) == 0 {
		if len(b) == 0 {
			return true
		}
		return false
	}
	return reflect.DeepEqual(a, b)
}

func TestCanonicalGBBFlagState(t *testing.T) {
	states := []struct {
		a pb.GBBFlagsState
		c pb.GBBFlagsState
	}{
		{state(flags(), flags()), state(flags(), flags())},
		{state(flags(0), flags()), state(flags(0), flags())},
		{state(flags(), flags(0)), state(flags(), flags(0))},
		{state(flags(1, 2, 3), flags(4, 5, 6)), state(flags(1, 2, 3), flags(4, 5, 6))},
		{state(flags(3, 2, 1), flags(6, 5, 4)), state(flags(1, 2, 3), flags(4, 5, 6))},
	}

	for _, s := range states {
		cA := canonicalGBBFlagsState(s.a)
		if !compare(cA.Clear, s.c.Clear) {
			t.Errorf("Clear incorrect for canonical %v: \nwant\n%v\ngot\n%v\n\n", s.a.Clear, s.c.Clear, cA.Clear)
		}
		if !compare(cA.Set, s.c.Set) {
			t.Errorf("Set incorrect for canonical %v: \nwant\n%v\ngot\n%v\n\n", s.a.Set, s.c.Set, cA.Set)
		}
	}
}

func TestGBBFlagsStatesEqual(t *testing.T) {
	states := []struct {
		a    pb.GBBFlagsState
		b    pb.GBBFlagsState
		want bool
	}{
		{state(flags(), flags()), state(flags(), flags()), true},
		{state(flags(0), flags()), state(flags(0), flags()), true},
		{state(flags(), flags(0)), state(flags(), flags(0)), true},
		{state(flags(1, 2, 3), flags(4, 5, 6)), state(flags(1, 2, 3), flags(4, 5, 6)), true},
		{state(flags(3, 2, 1), flags(6, 5, 4)), state(flags(1, 2, 3), flags(4, 5, 6)), true},
		{state(flags(3, 2, 1), flags(6, 5, 4)), state(flags(3, 2, 1), flags(6, 5, 4)), true},
		{state(flags(), flags()), state(flags(0), flags()), false},
		{state(flags(0), flags()), state(flags(), flags(0)), false},
		{state(flags(0), flags(0)), state(flags(0), flags(1)), false},
		{state(flags(0), flags(0)), state(flags(1), flags(0)), false},
		{state(flags(0, 1), flags(0, 1)), state(flags(0, 1), flags(0, 2)), false},
	}

	for _, s := range states {
		got := GBBFlagsStatesEqual(s.a, s.b)
		if got != s.want {
			t.Errorf("Comparing\n%+v\nand\n%+v\nwant %v, got %v", s.a, s.b, s.want, got)
		}
	}
}

func TestGBBFlagsChanged(t *testing.T) {
	states := []struct {
		a    pb.GBBFlagsState
		b    pb.GBBFlagsState
		f    []pb.GBBFlag
		want bool
	}{
		{state(flags(), flags()), state(flags(), flags()), flags(0), false},
		{state(flags(0), flags(1)), state(flags(1), flags(0)), flags(2), false},
		{state(flags(0), flags()), state(flags(0), flags()), flags(0), false},
		{state(flags(), flags(0)), state(flags(), flags(0)), flags(0), false},
		{state(flags(), flags(0)), state(flags(), flags(0)), flags(0), false},
		{state(flags(0), flags()), state(flags(1), flags()), flags(0), false},
		{state(flags(0), flags()), state(flags(), flags(0)), flags(0), true},
		{state(flags(), flags(0)), state(flags(0), flags()), flags(0), true},
		{state(flags(), flags(0)), state(flags(0), flags()), flags(0), true},
		{state(flags(0, 1), flags(0, 1, 2)), state(flags(0, 2), flags()), flags(0), true},
	}
	for _, s := range states {
		got := GBBFlagsChanged(s.a, s.b, s.f)
		if got != s.want {
			t.Errorf("Flags %v changed from\n%v\nto%v\nwant %v, got %v", s.f, s.a, s.b, s.want, got)
		}
	}
}

func TestAllGBBFlags(t *testing.T) {
	var all []int
	for i := 0; i < len(pb.GBBFlag_value); i++ {
		all = append(all, i)
	}
	want := flags(all...)
	got := AllGBBFlags()
	if !compare(want, got) {
		t.Errorf("All flags\nwant\n%v\ngot\n%v", want, got)
	}
}
