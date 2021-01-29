// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gpucuj tests GPU CUJ tests on lacros Chrome and Chrome OS Chrome.
package gpucuj

import (
	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"

	"chromiumos/tast/errors"
)

type clockID uint32

// clock describes the current state of a clock domain. The information is taken
// from clock snapshots in the traces.
// See https://source.chromium.org/chromium/chromium/src/+/master:third_party/perfetto/protos/perfetto/trace/clock_snapshot.proto
type clock struct {
	clk         clockID
	multiplier  uint64 // Clock units to nanoseconds conversion factor.
	incremental bool   // Whether timestamps for this clock should be meausured as deltas from the last trace packet's timestamp.
	curNS       uint64 // The current time for this clock.
}

func newClockFromSnapshot(dc *trace.ClockSnapshot_Clock) *clock {
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
	// See https://source.chromium.org/chromium/chromium/src/+/master:third_party/perfetto/protos/perfetto/common/builtin_clock.proto
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
	// between pairs of clock domains.
	offsetGraph map[clockPair]int64
}

func newClockSet() *clockSet {
	return &clockSet{
		defClk:      clockID(trace.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
		clocks:      make(map[clockID]*clock),
		offsetGraph: make(map[clockPair]int64),
	}
}

// mergeClockSets produces a combined clockSet from cd1 and cd2, which are not modified.
// This is necessary to merge global and local (to a packet sequence) clock information.
// This means that the clocks in clockSet.clocks and offsetGraph should be mutually exclusive
// between cd1 and cd2, since they should be local and global clocks.
func mergeClockSets(cd1, cd2 *clockSet) (*clockSet, error) {
	cd := newClockSet()

	for k, v := range cd1.clocks {
		cd.clocks[k] = v
	}
	for k, v := range cd2.clocks {
		if _, ok := cd.clocks[k]; ok {
			return nil, errors.New("duplicated clock (bug)")
		}
		cd.clocks[k] = v
	}

	for k, v := range cd1.offsetGraph {
		cd.offsetGraph[k] = v
	}
	for k, v := range cd2.offsetGraph {
		if _, ok := cd.offsetGraph[k]; ok {
			return nil, errors.New("duplicated clock offset (bug)")
		}
		cd.offsetGraph[k] = v
	}

	return cd, nil
}

// seqData holds state for each packet sequence. This state is built incrementally while parsing the
// trace, so seqData must only be used incrementally while parsing - i.e. each packet needs a different seqData.
type seqData struct {
	uuid                  uint64
	anMap                 map[uint64]string // Annotation ID to name map.
	evMap                 map[uint64]string // Event ID to name map.
	trackMap              map[uint64]string // Track UUID to name map.
	shouldSkipIncremental bool
	cs                    *clockSet
}

func newSeqData() *seqData {
	return &seqData{
		uuid:                  0,
		anMap:                 make(map[uint64]string),
		evMap:                 make(map[uint64]string),
		trackMap:              make(map[uint64]string),
		shouldSkipIncremental: true, // True until we see the first clear incremental state packet.
		cs:                    newClockSet(),
	}
}

// clearIncrementalState clears a subset of seqData that should be cleared when incremental state is
// explicitly asked to be cleared, or when a broken incremental state is detected.
func (sd *seqData) clearIncrementalState() {
	sd.uuid = 0
	sd.anMap = make(map[uint64]string)
	sd.evMap = make(map[uint64]string)
	sd.trackMap = make(map[uint64]string)
	sd.shouldSkipIncremental = false
}

type traceAnalyzer struct {
	seqMap map[uint32]*seqData // Map from trusted packet sequence ID to the state for this sequence.
	cs     *clockSet
	tr     *trace.Trace
}

func newTraceAnalyzer(tr *trace.Trace) *traceAnalyzer {
	return &traceAnalyzer{
		tr: tr,
	}
}

// timeNSInDomain returns the time in nanoseconds of the TracePacket p, in the clock domain of the
// clock identified by the clockID enID.
func (ta *traceAnalyzer) timeNSInDomain(p *trace.TracePacket, enID clockID) (uint64, error) {
	cd, err := mergeClockSets(ta.cs, ta.seqMap[p.GetTrustedPacketSequenceId()].cs)
	if err != nil {
		return 0, err
	}

	type node struct {
		clk clockID
		ns  int64
	}

	stID := ta.clockID(p)
	q := []node{{clk: stID, ns: int64(cd.clocks[stID].curNS)}}
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

		for pair, offset := range cd.offsetGraph {
			if pair.st != n.clk {
				continue
			}
			q = append(q, node{clk: pair.en, ns: n.ns + offset})
		}
	}
	return 0, errors.Errorf("could not find path to clock %d", enID)
}

func (ta *traceAnalyzer) clockID(p *trace.TracePacket) clockID {
	clk := ta.seqMap[p.GetTrustedPacketSequenceId()].cs.defClk
	if p.TimestampClockId != nil {
		clk = clockID(*p.TimestampClockId)
	} else if p.GetChromeEvents() != nil || p.GetChromeMetadata() != nil {
		// Chrome will not set TimestampClockId but mean MONOTONIC timestamps.
		clk = clockID(trace.BuiltinClock_BUILTIN_CLOCK_MONOTONIC)
	}

	return clk
}

// namer is an interface to generalise trace objects that have a name and a name IID.
type namer interface {
	GetNameIid() uint64
	GetName() string
}

func (ta *traceAnalyzer) eventName(p *trace.TracePacket, n namer) string {
	if n.GetNameIid() != 0 {
		return ta.seqMap[p.GetTrustedPacketSequenceId()].evMap[n.GetNameIid()]
	}
	return n.GetName()
}

func (ta *traceAnalyzer) annotationName(p *trace.TracePacket, n namer) string {
	if n.GetNameIid() != 0 {
		return ta.seqMap[p.GetTrustedPacketSequenceId()].anMap[n.GetNameIid()]
	}
	return n.GetName()
}

// parseState takes a TracePacket and updates (and returns) the data associated with the sequence
// the packet is from. It also returns whether this packet should be skipped, which can happen
// if the packet requires incremental state but the incremental state is broken.
func (ta *traceAnalyzer) parseState(p *trace.TracePacket) (*seqData, bool, error) {
	seqID := p.GetTrustedPacketSequenceId()

	if _, ok := ta.seqMap[seqID]; !ok {
		ta.seqMap[seqID] = newSeqData()
	}
	sd := ta.seqMap[seqID]

	if p.PreviousPacketDropped != nil && *p.PreviousPacketDropped {
		sd.shouldSkipIncremental = true // Dropped packet means skipping incremental packets until next clear.
	}

	// Only set incremental state once it has been cleared. Otherwise, we should not accumulate it.
	if p.GetSequenceFlags()&uint32(trace.TracePacket_SEQ_INCREMENTAL_STATE_CLEARED) != 0 {
		sd.clearIncrementalState()
	}

	// Non-incremental state should still be processed, even if incremental state is broken.
	if pdef := p.GetTracePacketDefaults(); pdef != nil {
		if tdef := pdef.GetTrackEventDefaults(); tdef != nil && tdef.TrackUuid != nil {
			sd.uuid = *tdef.TrackUuid
		}
		if pdef.TimestampClockId != nil {
			sd.cs.defClk = clockID(*pdef.TimestampClockId)
		}
	}

	if d := p.GetInternedData(); d != nil {
		for _, en := range d.EventNames {
			sd.evMap[*en.Iid] = *en.Name
		}
		for _, dn := range d.DebugAnnotationNames {
			sd.anMap[*dn.Iid] = *dn.Name
		}
	}

	if d := p.GetClockSnapshot(); d != nil {
		var clocks []*clock
		for _, dc := range d.Clocks {
			c := newClockFromSnapshot(dc)
			clocks = append(clocks, c)

			// Keep global clocks in the global clockSet, and local in the sequential data.
			if c.isBuiltin() {
				ta.cs.clocks[c.clk] = c
			} else {
				sd.cs.clocks[c.clk] = c
			}
		}
		for _, c1 := range clocks {
			for _, c2 := range clocks {
				cd := sd.cs // Only put this in the built-in clock offset map if both clocks are built-in.
				if c1.isBuiltin() && c2.isBuiltin() {
					cd = ta.cs
				}
				cd.offsetGraph[clockPair{st: c1.clk, en: c2.clk}] = int64(c2.curNS) - int64(c1.curNS)
			}
		}
	}

	// If our incremental state is broken and this packet requires incremental state, skip it.
	if sd.shouldSkipIncremental && p.GetSequenceFlags()&uint32(trace.TracePacket_SEQ_NEEDS_INCREMENTAL_STATE) != 0 {
		return sd, true, nil
	}

	// Accumulate incremental state:
	if d := p.GetTrackDescriptor(); d != nil {
		if d.Thread != nil && d.Thread.ThreadName != nil {
			sd.trackMap[*d.Uuid] = *d.Thread.ThreadName
		}
		// If name is specified, use it. Otherwise, use the thread name as a default.
		// See https://source.chromium.org/chromium/chromium/src/+/master:third_party/perfetto/protos/perfetto/trace/track_event/track_descriptor.proto
		if d.Name != nil {
			sd.trackMap[*d.Uuid] = *d.Name
		}
	}

	// Update current timestamp:
	clk := ta.clockID(p)
	var cd *clockSet
	if _, ok := ta.cs.clocks[clk]; ok {
		cd = ta.cs
	} else if _, ok := sd.cs.clocks[clk]; ok {
		cd = sd.cs
	}

	c, ok := cd.clocks[clk]
	if !ok {
		return nil, false, errors.Errorf("unknown clock with ID %d", clk)
	}
	// Update clock domain current time.
	if c.incremental {
		c.curNS += c.multiplier * p.GetTimestamp()
	} else {
		c.curNS = c.multiplier * p.GetTimestamp()
	}

	return sd, false, nil
}

// parseTrackEvents will call the callback visit with information about each track event and the current state.
// The callback takes these arguments:
//   ts: Timestamp of the track event.
//   trackName: Name of the track the event is from. This often corresponds to the thread name.
//   sd: The sequential data associated with the sequence this track event is on.
//   p: The trace packet this track event comes from.
//   flowID: The ID of the flow sequence this packet is on, or 0 if it is not on a flow sequence.
//   newFlow: True iff this packet is the start of a new flow sequence (with ID flowID)
func (ta *traceAnalyzer) parseTrackEvents(visit func(ts uint64, trackName string, sd *seqData, p *trace.TracePacket, flowID uint64, newFlow bool) error) error {
	ta.seqMap = make(map[uint32]*seqData)
	ta.cs = newClockSet()

	for _, p := range ta.tr.Packet {
		sd, skip, err := ta.parseState(p)
		if err != nil {
			return err
		}
		if skip {
			continue
		}

		if d := p.GetTrackEvent(); d != nil {
			ts, err := ta.timeNSInDomain(p, clockID(trace.BuiltinClock_BUILTIN_CLOCK_BOOTTIME))
			if err != nil {
				return err
			}

			var flowID uint64
			var newFlow bool
			if d.LegacyEvent != nil && d.LegacyEvent.BindId != nil {
				flowID = *d.LegacyEvent.BindId
				newFlow = *d.LegacyEvent.FlowDirection == trace.TrackEvent_LegacyEvent_FLOW_OUT
			}

			uuid := sd.uuid
			if d.TrackUuid != nil {
				uuid = *d.TrackUuid
			}

			visit(ts, sd.trackMap[uuid], sd, p, flowID, newFlow)
		}
	}

	return nil
}
