package main

import (
	"errors"
	"fmt"
	"io/ioutil"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"
	"github.com/golang/protobuf/proto"
)

type clock struct {
	clockID     uint32
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
		clockID:     dc.GetClockId(),
		multiplier:  multiplier,
		incremental: dc.GetIsIncremental(),
		curNS:       multiplier * dc.GetTimestamp(),
	}
}

func (c *clock) isBuiltin() bool {
	return c.clockID >= 1 && c.clockID <= 63
}

type clockPair struct {
	st uint32
	en uint32
}

type clockData struct {
	defClock    uint32
	clocks      map[uint32]*clock
	offsetGraph map[clockPair]int64
}

func newClockData() *clockData {
	return &clockData{
		defClock:    uint32(trace.BuiltinClock_BUILTIN_CLOCK_BOOTTIME),
		clocks:      make(map[uint32]*clock),
		offsetGraph: make(map[clockPair]int64),
	}
}

func mergeClockData(cd1 *clockData, cd2 *clockData) (*clockData, error) {
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
	// Map from trusted packet sequence ID to current default track ID.
	seqMap map[uint32]*seqData
	cd     *clockData
	tr     *trace.Trace
}

func newTraceAnalyzer(tr *trace.Trace) (*traceAnalyzer, error) {
	return &traceAnalyzer{
		seqMap: make(map[uint32]*seqData),
		cd:     newClockData(),
		tr:     tr,
	}, nil
}

func (ta *traceAnalyzer) getTimeNSInDomain(p *trace.TracePacket, enID uint32) (uint64, error) {
	cd, err := mergeClockData(ta.cd, ta.seqMap[p.GetTrustedPacketSequenceId()].cd)
	if err != nil {
		return 0, err
	}

	type node struct {
		clockID uint32
		ns      int64
	}

	stID := ta.getClockID(p)
	q := []node{{clockID: stID, ns: int64(cd.clocks[stID].curNS)}}
	seen := make(map[uint32]bool)
	for len(q) > 0 {
		n := q[0]
		q = q[1:]

		if n.clockID == enID {
			return uint64(n.ns), nil
		}

		if seen[n.clockID] {
			continue
		}
		seen[n.clockID] = true

		for pair, offset := range cd.offsetGraph {
			if pair.st != n.clockID {
				continue
			}
			q = append(q, node{clockID: pair.en, ns: n.ns + offset})
		}
	}
	return 0, fmt.Errorf("could not find path to clock %d", enID)
}

func (ta *traceAnalyzer) getClockID(p *trace.TracePacket) uint32 {
	clockID := ta.seqMap[p.GetTrustedPacketSequenceId()].cd.defClock
	if p.TimestampClockId != nil {
		clockID = *p.TimestampClockId
	} else if p.GetChromeEvents() != nil || p.GetChromeMetadata() != nil {
		// Chrome will not set TimestampClockId but mean MONOTONIC timestamps.
		clockID = uint32(trace.BuiltinClock_BUILTIN_CLOCK_MONOTONIC)
	}

	return clockID
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
			sd.cd.defClock = *pdef.TimestampClockId
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
				ta.cd.clocks[c.clockID] = c
			} else {
				sd.cd.clocks[c.clockID] = c
			}
		}
		for _, c1 := range clocks {
			for _, c2 := range clocks {
				cd := sd.cd // Only put this in the built-in clock offset map if both clocks are built-in.
				if c1.isBuiltin() && c2.isBuiltin() {
					cd = ta.cd
				}
				cd.offsetGraph[clockPair{st: c1.clockID, en: c2.clockID}] = int64(c2.curNS) - int64(c1.curNS)
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
	clockID := ta.getClockID(p)
	var cd *clockData
	if _, ok := ta.cd.clocks[clockID]; ok {
		cd = ta.cd
	} else if _, ok := sd.cd.clocks[clockID]; ok {
		cd = sd.cd
	}

	if c, ok := cd.clocks[clockID]; ok {
		if c.incremental {
			c.curNS += c.multiplier * p.GetTimestamp()
		} else {
			c.curNS = c.multiplier * p.GetTimestamp()
		}
	} else {
		return nil, false, fmt.Errorf("unknown clock with ID %d", clockID)
	}

	return sd, false, nil
}

func (ta *traceAnalyzer) parseTrackEvents(visit func(sd *seqData, p *trace.TracePacket) error) error {
	fmt.Printf("len: %d\n", len(ta.tr.Packet))

	for _, p := range ta.tr.Packet {
		sd, skip, err := ta.parseState(p)
		if err != nil {
			return err
		}
		if skip {
			continue
		}

		if d := p.GetTrackEvent(); d != nil {
			visit(sd, p)
		}
	}

	return nil
}

type frame struct {
	ts uint64
	numExpected int
	numReceived int
}

type frameStats struct {
	ta *traceAnalyzer
	prevTS uint64
	frames []*frame
}

func newFrameStats(tr *trace.Trace) (*frameStats, error) {
	ta, err := newTraceAnalyzer(tr)
	if err != nil {
		return nil, err
	}
	return &frameStats {
		ta: ta,
	}, nil
}

func (fs *frameStats) parseTrackEvent(sd *seqData, p *trace.TracePacket) error {
	// clockID := fs.ta.getClockID(p)
	ts, err := fs.ta.getTimeNSInDomain(p, uint32(trace.BuiltinClock_BUILTIN_CLOCK_BOOTTIME))
	if err != nil {
		return err
	}

	uuid := sd.uuid
	d := p.GetTrackEvent()

	if d.TrackUuid != nil {
		uuid = *d.TrackUuid
	}

	trackName := sd.trackMap[uuid]

	if trackName != "VizCompositorThread" {
		return nil
	}

	name := fs.ta.getEventName(p, d)
	fmt.Println("event: ", name)
	// fmt.Println("timestamp: ", ts, clockID, p.GetTimestamp())

	// for _, da := range d.GetDebugAnnotations() {
	// 	fmt.Println("ann: ", fs.ta.getAnnotationName(p, da), ":", da.String())
	// }

	// fmt.Println()

	// DisplayScheduler::BeginFrame

	// IssueBeginFrame
	// DidNotProduceFrame
	// ReceiveCompositorFrame

	// IssueBeginFrame - DidNotProduceFrame - ReceiveCompositorFrame

	// DrmEventFlipComplete (DrmThread)
	// vblank tv_sec, tv_usec (present time)


	// if time specified in beginframe

	if fs.prevTS > ts {
		return errors.New("broken trace")  // track events on the same track should be in order
	}
	fs.prevTS = ts

	return nil
}

func (fs *frameStats) computePercentageDroppedFrames() (float32, error) {
	err := fs.ta.parseTrackEvents(func(sd *seqData, p *trace.TracePacket) error {
		return fs.parseTrackEvent(sd, p)
	})
	if err != nil {
		return 0.0, err
	}
	return 0.0, nil
}

func main() {
	buf, err := ioutil.ReadFile("/home/edcourtney/software/traces/new-trace.data")
	if err != nil {
		panic(err)
	}
	tr := &trace.Trace{}
	if err := proto.Unmarshal(buf, tr); err != nil {
		panic(err)
	}

	fs, err := newFrameStats(tr)
	if err != nil {
		panic(err)
	}
	p, err := fs.computePercentageDroppedFrames()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Percent dropped: %.2f\n", p)
}
