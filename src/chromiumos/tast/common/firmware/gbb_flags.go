// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/exec"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
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

// FAFTGBBFlags returns the flags that faft sets before starting a test.
func FAFTGBBFlags() []pb.GBBFlag {
	return []pb.GBBFlag{pb.GBBFlag_RUNNING_FAFT}
}

// RebootRequiredGBBFlags returns flags that require a DUT reboot after they are changed.
func RebootRequiredGBBFlags() []pb.GBBFlag {
	return []pb.GBBFlag{pb.GBBFlag_FORCE_DEV_SWITCH_ON, pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC, pb.GBBFlag_FORCE_DEV_BOOT_USB}
}

// GBBFlagsStatesEqual determines if 2 GBBFlagsState are the same.
func GBBFlagsStatesEqual(a, b *pb.GBBFlagsState) bool {
	canonicalA := canonicalGBBFlagsState(a)
	canonicalB := canonicalGBBFlagsState(b)

	return reflect.DeepEqual(canonicalA.Clear, canonicalB.Clear) && reflect.DeepEqual(canonicalA.Set, canonicalB.Set)
}

// GBBFlagsChanged determines if any of the flags definitely have changed between a and b.
func GBBFlagsChanged(a, b *pb.GBBFlagsState, flags []pb.GBBFlag) bool {
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

// getGBBFlagsInt gets the flags that are set as an integer.
func getGBBFlagsInt(ctx context.Context, dut *dut.DUT) (uint32, error) {
	out, err := dut.Conn().CommandContext(ctx, "/usr/share/vboot/bin/get_gbb_flags.sh").Output(exec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "get_gbb_flags.sh")
	}
	re, err := regexp.Compile(`Chrome ?OS GBB set flags: (0x[0-9a-fA-F]+)`)
	if err != nil {
		return 0, errors.Wrap(err, "parse gbb regex")
	}
	matches := re.FindSubmatch(out)
	if matches == nil {
		return 0, errors.Errorf("failed to find gbb flags in %s", string(out))
	}
	currentGBB64, err := strconv.ParseUint(string(matches[1]), 0, 32)
	if err != nil {
		return 0, errors.Wrapf(err, "parse gbb %q", string(matches[1]))
	}
	return uint32(currentGBB64), nil
}

// GetGBBFlags gets the flags that are cleared and set.
func GetGBBFlags(ctx context.Context, dut *dut.DUT) (*pb.GBBFlagsState, error) {
	currentGBB, err := getGBBFlagsInt(ctx, dut)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Current GBB flags = %#x", currentGBB)
	return &pb.GBBFlagsState{
		Clear: calcGBBFlags(^currentGBB),
		Set:   calcGBBFlags(currentGBB),
	}, nil
}

// ClearAndSetGBBFlags clears and sets specified GBB flags, leaving the rest unchanged.
func ClearAndSetGBBFlags(ctx context.Context, dut *dut.DUT, state *pb.GBBFlagsState) error {
	state = canonicalGBBFlagsState(state)
	currentGBB, err := getGBBFlagsInt(ctx, dut)
	if err != nil {
		return err
	}
	clearMask := calcGBBMask(state.Clear)
	setMask := calcGBBMask(state.Set)
	testing.ContextLogf(ctx, "Current GBB flags = %#x, want clear %#x, set %#x", currentGBB, clearMask, setMask)
	newGBB := (currentGBB & ^clearMask) | setMask
	if newGBB != currentGBB {
		testing.ContextLogf(ctx, "Setting GBB flags = %#x", newGBB)
		if err := dut.Conn().CommandContext(ctx, "/usr/share/vboot/bin/set_gbb_flags.sh", fmt.Sprintf("%#x", newGBB)).Run(exec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "set_gbb_flags.sh")
		}
	} else {
		testing.ContextLog(ctx, "No GBB change required")
	}
	return nil
}

// SetGBBFlags ignores the previous GBB flags and sets them to the specified flags.
func SetGBBFlags(ctx context.Context, dut *dut.DUT, flags []pb.GBBFlag) error {
	setMask := calcGBBMask(flags)
	testing.ContextLogf(ctx, "Setting GBB flags = %#x", setMask)
	if err := dut.Conn().CommandContext(ctx, "/usr/share/vboot/bin/set_gbb_flags.sh", fmt.Sprintf("%#x", setMask)).Run(exec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "set_gbb_flags.sh")
	}
	return nil
}

// calcGBBFlags interprets mask as a GBBFlag bit mask and returns the set flags.
func calcGBBFlags(mask uint32) []pb.GBBFlag {
	var res []pb.GBBFlag
	for _, pos := range AllGBBFlags() {
		if mask&(0x0001<<pos) != 0 {
			res = append(res, pb.GBBFlag(pos))
		}
	}
	return res
}

// calcGBBMask returns the bit mask corresponding to the list of GBBFlags.
func calcGBBMask(flags []pb.GBBFlag) uint32 {
	var mask uint32
	for _, f := range flags {
		mask |= 0x0001 << f
	}
	return mask
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
func canonicalGBBFlagsState(s *pb.GBBFlagsState) *pb.GBBFlagsState {
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

	return &pb.GBBFlagsState{Clear: canonicalClear, Set: canonicalSet}
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

// GBBAddFlag modifies `s` to add all flags in `flags`.
func GBBAddFlag(s *pb.GBBFlagsState, flags ...pb.GBBFlag) {
	s.Set = append(s.Set, flags...)
	newS := canonicalGBBFlagsState(s)
	s.Set = newS.Set
	s.Clear = newS.Clear
}

// CopyGBBFlags returns a new GBBFlagsState that is a copy of `s`.
func CopyGBBFlags(s *pb.GBBFlagsState) *pb.GBBFlagsState {
	// Depends on the behavior of canonicalGBBFlagsState to always return a copy with new Set & Clear arrays.
	ret := canonicalGBBFlagsState(s)
	return ret
}

// GBBFlagsContains returns true if `s` contains the requested GBB flag `flag`.
func GBBFlagsContains(s *pb.GBBFlagsState, flag pb.GBBFlag) bool {
	for _, f := range s.Set {
		if f == flag {
			return true
		}
	}
	return false
}
