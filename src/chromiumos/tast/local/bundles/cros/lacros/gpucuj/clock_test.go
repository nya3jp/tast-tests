// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gpucuj

import (
	"reflect"
	"testing"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace/github.com/google/perfetto/perfetto_proto"
)

func TestMergeClockSet(t *testing.T) {
	for _, tt := range []struct {
		cs1 *clockSet
		cs2 *clockSet
		res *clockSet
	}{
		// Merge two empty sets.
		{
			cs1: newClockSet(),
			cs2: newClockSet(),
			res: newClockSet(),
		},
		// Only one thing in cs1's clock map.
		{
			cs1: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk: 1,
					},
				},
				offsetGraph: map[clockPair]int64{},
			},
			cs2: newClockSet(),
			res: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk: 1,
					},
				},
				offsetGraph: map[clockPair]int64{},
			},
		},
		// Only one thing in cs2's clock map.
		{
			cs1: newClockSet(),
			cs2: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk: 1,
					},
				},
				offsetGraph: map[clockPair]int64{},
			},
			res: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk: 1,
					},
				},
				offsetGraph: map[clockPair]int64{},
			},
		},
		// One thing in each clock map, non-conflicting.
		{
			cs1: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk: 1,
					},
				},
				offsetGraph: map[clockPair]int64{},
			},
			cs2: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					2: {
						clk: 2,
					},
				},
				offsetGraph: map[clockPair]int64{},
			},
			res: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk: 1,
					},
					2: {
						clk: 2,
					},
				},
				offsetGraph: map[clockPair]int64{},
			},
		},
		// One thing in each clock map, but conflicting.
		{
			cs1: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk: 1,
					},
				},
				offsetGraph: map[clockPair]int64{},
			},
			cs2: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk: 1,
					},
				},
				offsetGraph: map[clockPair]int64{},
			},
			res: nil,
		},
		// Only one thing in cs1's offset graph.
		{
			cs1: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{},
				offsetGraph: map[clockPair]int64{
					{1, 2}: 1,
				},
			},
			cs2: newClockSet(),
			res: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{},
				offsetGraph: map[clockPair]int64{
					{1, 2}: 1,
				},
			},
		},
		// Only one thing in cs2's offset graph.
		{
			cs1: newClockSet(),
			cs2: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{},
				offsetGraph: map[clockPair]int64{
					{1, 2}: 1,
				},
			},
			res: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{},
				offsetGraph: map[clockPair]int64{
					{1, 2}: 1,
				},
			},
		},
		// One thing in each offset graph, non-conflicting.
		{
			cs1: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{},
				offsetGraph: map[clockPair]int64{
					{1, 2}: 1,
				},
			},
			cs2: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{},
				offsetGraph: map[clockPair]int64{
					{2, 3}: 1,
				},
			},
			res: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{},
				offsetGraph: map[clockPair]int64{
					{1, 2}: 1,
					{2, 3}: 1,
				},
			},
		},
		// One thing in each offset graph, but conflicting.
		{
			cs1: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{},
				offsetGraph: map[clockPair]int64{
					{1, 2}: 1,
				},
			},
			cs2: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{},
				offsetGraph: map[clockPair]int64{
					{1, 2}: 1,
				},
			},
			res: nil,
		},
	} {
		merged, err := mergeClockSets(tt.cs1, tt.cs2)
		if err != nil {
			if tt.res != nil {
				t.Errorf("got error %v, want %v", err, tt.res)
			}
		} else if tt.res == nil {
			if err == nil {
				t.Errorf("got %v, want error", merged)

			}
		} else if !reflect.DeepEqual(merged, tt.res) {
			t.Errorf("got %v, want %v", merged, tt.res)
		}
	}
}

func TestClockSetDomainConversion(t *testing.T) {
	for _, tt := range []struct {
		cs      *clockSet
		stID    clockID
		enID    clockID
		enNS    uint64
		succeed bool
	}{
		// Test identity.
		{
			cs: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk:   1,
						curNS: 0,
					},
				},
				offsetGraph: map[clockPair]int64{},
			},
			stID:    clockID(1),
			enID:    clockID(1),
			enNS:    0,
			succeed: true,
		},
		{
			cs: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk:   1,
						curNS: 55,
					},
				},
				offsetGraph: map[clockPair]int64{},
			},
			stID:    clockID(1),
			enID:    clockID(1),
			enNS:    55,
			succeed: true,
		},
		// Test path of length 1.
		{
			cs: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk:   1,
						curNS: 55,
					},
					2: {
						clk:   2,
						curNS: 55,
					},
				},
				offsetGraph: map[clockPair]int64{
					{1, 2}: 10,
				},
			},
			stID:    clockID(1),
			enID:    clockID(2),
			enNS:    65,
			succeed: true,
		},
		// Test missing stID
		{
			cs: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk:   1,
						curNS: 55,
					},
					2: {
						clk:   2,
						curNS: 55,
					},
				},
				offsetGraph: map[clockPair]int64{
					{1, 2}: 10,
				},
			},
			stID:    clockID(3),
			enID:    clockID(2),
			enNS:    0,
			succeed: false,
		},
		// Test missing enID
		{
			cs: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk:   1,
						curNS: 55,
					},
					2: {
						clk:   2,
						curNS: 55,
					},
				},
				offsetGraph: map[clockPair]int64{
					{1, 2}: 10,
				},
			},
			stID:    clockID(1),
			enID:    clockID(3),
			enNS:    0,
			succeed: false,
		},
		// Test path of length 2
		{
			cs: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk:   1,
						curNS: 55,
					},
					2: {
						clk:   2,
						curNS: 55,
					},
					3: {
						clk:   3,
						curNS: 55,
					},
				},
				offsetGraph: map[clockPair]int64{
					{1, 3}: 10,
					{3, 2}: -20,
				},
			},
			stID:    clockID(1),
			enID:    clockID(2),
			enNS:    45,
			succeed: true,
		},
		// Test path in more complicated graph
		{
			cs: &clockSet{
				defClk: clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
				clocks: map[clockID]*clock{
					1: {
						clk:   1,
						curNS: 55,
					},
					2: {
						clk:   2,
						curNS: 55,
					},
					3: {
						clk:   3,
						curNS: 55,
					},
					4: {
						clk:   4,
						curNS: 55,
					},
					5: {
						clk:   5,
						curNS: 55,
					},
					6: {
						clk:   5,
						curNS: 55,
					},
				},
				offsetGraph: map[clockPair]int64{
					{1, 3}: 10,
					{1, 2}: -20,
					{3, 4}: -20,
					{2, 4}: 10,
					{4, 5}: -10,
					{4, 6}: 10,
				},
			},
			stID:    clockID(1),
			enID:    clockID(5),
			enNS:    35,
			succeed: true,
		},
	} {
		enNS, err := tt.cs.timeNSInDomain(tt.stID, tt.enID)
		if err != nil {
			if tt.succeed {
				t.Errorf("got error %v, want %d", err, tt.enNS)
			}
		} else if !tt.succeed {
			if err == nil {
				t.Errorf("got %d, want error", enNS)

			}
		} else if enNS != tt.enNS {
			t.Errorf("got %d, want %d", enNS, tt.enNS)
		}
	}
}
