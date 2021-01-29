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

type clock struct {
	clk         clockID
	multiplier  uint64 // Clock units to nanoseconds conversion factor.
	incremental bool
	curNS       uint64
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
	return c.clk >= 1 && c.clk <= 63
}

type clockPair struct {
	st clockID
	en clockID
}

type clockData struct {
	defClk      clockID
	clocks      map[clockID]*clock
	offsetGraph map[clockPair]int64
}

func newClockData() *clockData {
	return &clockData{
		defClk:      clockID(trace.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
		clocks:      make(map[clockID]*clock),
		offsetGraph: make(map[clockPair]int64),
	}
}

func mergeClockData(cd1, cd2 *clockData) (*clockData, error) {
	cd := newClockData()

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

type seqData struct {
	uuid                  uint64
	anMap                 map[uint64]string // Annotation ID to name map.
	evMap                 map[uint64]string // Event ID to name map.
	trackMap              map[uint64]string // Track UUID to name map.
	shouldSkipIncremental bool
	cd                    *clockData
}

func newSeqData() *seqData {
	return &seqData{
		uuid:                  0,
		anMap:                 make(map[uint64]string),
		evMap:                 make(map[uint64]string),
		trackMap:              make(map[uint64]string),
		shouldSkipIncremental: true, // True until we see the first clear incremental state packet.
		cd:                    newClockData(),
	}
}

func (sd *seqData) clearIncrementalState() {
	sd.uuid = 0
	sd.anMap = make(map[uint64]string)
	sd.evMap = make(map[uint64]string)
	sd.trackMap = make(map[uint64]string)
	sd.shouldSkipIncremental = false
}

type traceAnalyzer struct {
	seqMap map[uint32]*seqData // Map from trusted packet sequence ID to current default track ID.
	cd     *clockData
	tr     *trace.Trace
}

func newTraceAnalyzer(tr *trace.Trace) (*traceAnalyzer, error) {
	return &traceAnalyzer{
		tr: tr,
	}, nil
}

func (ta *traceAnalyzer) getTimeNSInDomain(p *trace.TracePacket, enID clockID) (uint64, error) {
	cd, err := mergeClockData(ta.cd, ta.seqMap[p.GetTrustedPacketSequenceId()].cd)
	if err != nil {
		return 0, err
	}

	type node struct {
		clk clockID
		ns  int64
	}

	stID := ta.getClockID(p)
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

func (ta *traceAnalyzer) getClockID(p *trace.TracePacket) clockID {
	clk := ta.seqMap[p.GetTrustedPacketSequenceId()].cd.defClk
	if p.TimestampClockId != nil {
		clk = clockID(*p.TimestampClockId)
	} else if p.GetChromeEvents() != nil || p.GetChromeMetadata() != nil {
		// Chrome will not set TimestampClockId but mean MONOTONIC timestamps.
		clk = clockID(trace.BuiltinClock_BUILTIN_CLOCK_MONOTONIC)
	}

	return clk
}

type namer interface {
	GetNameIid() uint64
	GetName() string
}

func (ta *traceAnalyzer) getEventName(p *trace.TracePacket, n namer) string {
	if n.GetNameIid() != 0 {
		return ta.seqMap[p.GetTrustedPacketSequenceId()].evMap[n.GetNameIid()]
	}
	return n.GetName()
}

func (ta *traceAnalyzer) getAnnotationName(p *trace.TracePacket, n namer) string {
	if n.GetNameIid() != 0 {
		return ta.seqMap[p.GetTrustedPacketSequenceId()].anMap[n.GetNameIid()]
	}
	return n.GetName()
}

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
			sd.cd.defClk = clockID(*pdef.TimestampClockId)
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

			if c.isBuiltin() {
				ta.cd.clocks[c.clk] = c
			} else {
				sd.cd.clocks[c.clk] = c
			}
		}
		for _, c1 := range clocks {
			for _, c2 := range clocks {
				cd := sd.cd // Only put this in the built-in clock offset map if both clocks are built-in.
				if c1.isBuiltin() && c2.isBuiltin() {
					cd = ta.cd
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
		if d.Name != nil {
			sd.trackMap[*d.Uuid] = *d.Name
		}
	}

	// Update current timestamp:
	clk := ta.getClockID(p)
	var cd *clockData
	if _, ok := ta.cd.clocks[clk]; ok {
		cd = ta.cd
	} else if _, ok := sd.cd.clocks[clk]; ok {
		cd = sd.cd
	}

	if c, ok := cd.clocks[clk]; ok {
		if c.incremental {
			c.curNS += c.multiplier * p.GetTimestamp()
		} else {
			c.curNS = c.multiplier * p.GetTimestamp()
		}
	} else {
		return nil, false, errors.Errorf("unknown clock with ID %d", clk)
	}

	return sd, false, nil
}

func (ta *traceAnalyzer) parseTrackEvents(visit func(ts uint64, trackName string, sd *seqData, p *trace.TracePacket, flowID uint64, newFlow bool) error) error {
	ta.seqMap = make(map[uint32]*seqData)
	ta.cd = newClockData()

	for _, p := range ta.tr.Packet {
		sd, skip, err := ta.parseState(p)
		if err != nil {
			return err
		}
		if skip {
			continue
		}

		if d := p.GetTrackEvent(); d != nil {
			ts, err := ta.getTimeNSInDomain(p, clockID(trace.BuiltinClock_BUILTIN_CLOCK_BOOTTIME))
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
