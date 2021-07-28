// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"reflect"
	"sort"

	"chromiumos/tast/errors"
	pb "chromiumos/tast/services/cros/firmware"
)

// allGBBFlags has all the GBB Flags in sorted order.
var allGBBFlags []pb.GBBFlag

// faftClearingGBBFlags has all the GBB Flags in sorted order, except for DISABLE_EC_SOFTWARE_SYNC
var faftClearingGBBFlags []pb.GBBFlag

func init() {
	for _, v := range pb.GBBFlag_value {
		allGBBFlags = append(allGBBFlags, pb.GBBFlag(v))
		if pb.GBBFlag(v) != pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC {
			faftClearingGBBFlags = append(faftClearingGBBFlags, pb.GBBFlag(v))
		}
	}
	sort.Slice(allGBBFlags, func(i, j int) bool { return allGBBFlags[i] < allGBBFlags[j] })
	sort.Slice(faftClearingGBBFlags, func(i, j int) bool { return faftClearingGBBFlags[i] < faftClearingGBBFlags[j] })
}

// AllGBBFlags returns all the GBB Flags in order by their int values.
func AllGBBFlags() []pb.GBBFlag {
	return allGBBFlags
}

// FAFTGBBFlags returns the flags that faft sets in firmware_test.py before starting a test.
func FAFTGBBFlags() []pb.GBBFlag {
	return []pb.GBBFlag{pb.GBBFlag_FAFT_KEY_OVERIDE, pb.GBBFlag_ENTER_TRIGGERS_TONORM}
}

// FAFTClearingGBBFlags returns the flags that faft clears in firmware_test.py before starting a test.
func FAFTClearingGBBFlags() []pb.GBBFlag {
	return faftClearingGBBFlags
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

// GBBFlagsVerifyExpected if the `want` flag state has been applied to `current`.
func GBBFlagsVerifyExpected(want, current pb.GBBFlagsState) error {
	want = canonicalGBBFlagsState(want)
	current = canonicalGBBFlagsState(current)

	wantClear := makeFlagsMap(want.Clear)
	wantSet := makeFlagsMap(want.Set)
	currentSet := makeFlagsMap(current.Set)

	for _, f := range allGBBFlags {
		_, inWantClear := wantClear[f]
		_, inWantSet := wantSet[f]
		_, inCurrentSet := currentSet[f]
		if inWantClear && inCurrentSet {
			return errors.Errorf("GBB flag %s incorrect, got: set, want: cleared", f)
		}
		if inWantSet && !inCurrentSet {
			return errors.Errorf("GBB flag %s incorrect, got: cleared, want: set", f)
		}
	}
	return nil

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
