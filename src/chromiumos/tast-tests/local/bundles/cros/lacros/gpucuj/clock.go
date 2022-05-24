// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gpucuj tests GPU CUJ tests on lacros Chrome and ChromeOS Chrome.
package gpucuj

import (
	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace/github.com/google/perfetto/perfetto_proto"

	"chromiumos/tast/errors"
)

type clockID uint32

// clock describes the current state of a clock domain. The information is taken
// from clock snapshots in the traces.
// See https://source.chromium.org/chromium/chromium/src/+/main:third_party/perfetto/protos/perfetto/trace/clock_snapshot.proto
type clock struct {
	clk         clockID
	multiplier  uint64 // Clock units to nanoseconds conversion factor.
	incremental bool   // Whether timestamps for this clock should be meausured as deltas from the last trace packet's timestamp.
	curNS       uint64 // The current time for this clock.
}

func newClockFromSnapshot(dc *perfetto_proto.ClockSnapshot_Clock) *clock {
	multiplier := uint64(1)
	if dc.GetUnitMultiplierNs() > 1 {
		multiplier = dc.GetUnitMultiplierNs()
	}
	return &clock{
		clk:         clockID(dc.GetClockId()),
		multiplier:  multiplier,
		incremental: dc.GetIsIncremental(),
		curNS:       multiplier * dc.GetTimestamp(),
	}
}

func (c *clock) isBuiltin() bool {
	// See https://source.chromium.org/chromium/chromium/src/+/main:third_party/perfetto/protos/perfetto/common/builtin_clock.proto
	return c.clk >= 1 && c.clk <= 63
}

type clockPair struct {
	st clockID
	en clockID
}

// clockSet holds a set of clocks data about them, such as the transformation between their clock domains.
// There are both global clocks, and clocks local to a packet sequence, which is why this is used as part of
// the packet sequence data.
type clockSet struct {
	defClk clockID            // Default clock to be used if the clock is unspecified in a TracePacket.
	clocks map[clockID]*clock // Mapping between clock IDs and the clock domain data.
	// Periodic snapshots define time offsets between clock domains. This stores the most up-to-date offset
	// between pairs of clock domains. The code in this file assumes that any path from clock domain A->B will produce
	// the same offset value. This should be true conceptually. Practically there may be a small
	// amount of error.
	offsetGraph map[clockPair]int64
}

func newClockSet() *clockSet {
	return &clockSet{
		defClk:      clockID(perfetto_proto.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
		clocks:      make(map[clockID]*clock),
		offsetGraph: make(map[clockPair]int64),
	}
}

// mergeClockSets produces a combined clockSet from cs1 and cs2, which are not modified.
// This is necessary to merge global and local (to a packet sequence) clock information.
// This means that the clocks in clockSet.clocks and offsetGraph should be mutually exclusive
// between cs1 and cs2, since they should be local and global clocks.
func mergeClockSets(cs1, cs2 *clockSet) (*clockSet, error) {
	cs := newClockSet()

	for k, v := range cs1.clocks {
		cs.clocks[k] = v
	}
	for k, v := range cs2.clocks {
		if _, ok := cs.clocks[k]; ok {
			return nil, errors.New("duplicated clock (bug)")
		}
		cs.clocks[k] = v
	}

	for k, v := range cs1.offsetGraph {
		cs.offsetGraph[k] = v
	}
	for k, v := range cs2.offsetGraph {
		if _, ok := cs.offsetGraph[k]; ok {
			return nil, errors.New("duplicated clock offset (bug)")
		}
		cs.offsetGraph[k] = v
	}

	return cs, nil
}

// timeNSInDomain returns the time in nanoseconds of the clock identified by stID in the clock
// domain of the clock identified by enID.
func (cs *clockSet) timeNSInDomain(stID, enID clockID) (uint64, error) {
	type node struct {
		clk clockID
		ns  int64
	}

	st, ok := cs.clocks[stID]
	if !ok {
		return 0, errors.Errorf("could not find starting clock %d", stID)
	}

	// Start at st.curNS. We don't look at any intermediate clock's curNS value, because it could be arbitrarily old
	// (used to store the last timestamp for incremental clocks). Instead, we use the offset from the offset graph.
	q := []node{{clk: stID, ns: int64(st.curNS)}}
	seen := make(map[clockID]bool)
	for len(q) > 0 {
		n := q[0]
		q = q[1:]

		if n.clk == enID {
			return uint64(n.ns), nil
		}

		if seen[n.clk] {
			continue
		}
		seen[n.clk] = true

		for pair, offset := range cs.offsetGraph {
			if pair.st != n.clk {
				continue
			}
			q = append(q, node{clk: pair.en, ns: n.ns + offset})
		}
	}
	return 0, errors.Errorf("could not find path to clock %d", enID)
}
