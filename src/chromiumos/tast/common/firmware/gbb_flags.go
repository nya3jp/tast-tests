// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"reflect"
	"sort"

	pb "chromiumos/tast/services/cros/firmware"
)

// allGBBFlags has all the GBB Flags in sorted order.
var allGBBFlags []pb.GBBFlag

func init() {
	for _, v := range pb.GBBFlag_value {
		allGBBFlags = append(allGBBFlags, pb.GBBFlag(v))
	}
	sort.Slice(allGBBFlags, func(i, j int) bool { return allGBBFlags[i] < allGBBFlags[j] })
}

// AllGBBFlags returns all the GBB Flags in order by their int values.
func AllGBBFlags() []pb.GBBFlag {
	return allGBBFlags
}

// FAFTGBBFlags returns the flags that faft sets in firmware_test.py before starting a test.
func FAFTGBBFlags() []pb.GBBFlag {
	return []pb.GBBFlag{pb.GBBFlag_FAFT_KEY_OVERIDE, pb.GBBFlag_ENTER_TRIGGERS_TONORM}
}

// RebootRequiredGBBFlags returns flags that require a DUT reboot after they are changed.
func RebootRequiredGBBFlags() []pb.GBBFlag {
	return []pb.GBBFlag{pb.GBBFlag_FORCE_DEV_SWITCH_ON, pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC}
}

// GBBFlagsStatesEqual determines if 2 GBBFlagsState are the same.
func GBBFlagsStatesEqual(a, b pb.GBBFlagsState) bool {
	canonicalA := canonicalGBBFlagsState(a)
	canonicalB := canonicalGBBFlagsState(b)

	return reflect.DeepEqual(canonicalA.Clear, canonicalB.Clear) && reflect.DeepEqual(canonicalA.Set, canonicalB.Set)
}

// GBBFlagsChanged determines if any of the flags definitely have changed between a and b.
func GBBFlagsChanged(a, b pb.GBBFlagsState, flags []pb.GBBFlag) bool {
	a = canonicalGBBFlagsState(a)
	b = canonicalGBBFlagsState(b)

	aClear := makeFlagsMap(a.Clear)
	aSet := makeFlagsMap(a.Set)
	bClear := makeFlagsMap(b.Clear)
	bSet := makeFlagsMap(b.Set)

	for _, f := range flags {
		_, inAClear := aClear[f]
		_, inASet := aSet[f]
		_, inBClear := bClear[f]
		_, inBSet := bSet[f]
		if (inAClear && inBSet) || (inASet && inBClear) {
			return true
		}
	}
	return false
}

// makeFlagsMap converts a slice of GBBFlags into a map for easy lookup.
func makeFlagsMap(f []pb.GBBFlag) map[pb.GBBFlag]bool {
	m := make(map[pb.GBBFlag]bool)
	for _, f := range f {
		m[f] = true
	}
	return m
}

// canonicalGBBFlagsState standardizes the GBBFlagsState so that they can be more readily compared.  In particular, a flag in both Set and Clear will be deleted from Clear.  The flags are also sorted.
func canonicalGBBFlagsState(s pb.GBBFlagsState) pb.GBBFlagsState {
	setMap := makeFlagsMap(s.Set)
	clearMap := makeFlagsMap(s.Clear)

	var canonicalClear []pb.GBBFlag
	var canonicalSet []pb.GBBFlag

	for _, v := range allGBBFlags {
		if _, sOk := setMap[v]; sOk {
			canonicalSet = append(canonicalSet, v)
		} else if _, cOk := clearMap[v]; cOk {
			canonicalClear = append(canonicalClear, v)
		}
	}

	return pb.GBBFlagsState{Clear: canonicalClear, Set: canonicalSet}
}

// GBBToggle adds `flag` to `flags` if it is missing, or removes it if it is present. Returns a new list, and does not modify the `flags` slice.
func GBBToggle(flags []pb.GBBFlag, flag pb.GBBFlag) []pb.GBBFlag {
	var ret []pb.GBBFlag
	found := false
	for _, v := range flags {
		if v == flag {
			found = true
		} else {
			ret = append(ret, v)
		}
	}
	if !found {
		ret = append(ret, flag)
	}
	return ret
}
