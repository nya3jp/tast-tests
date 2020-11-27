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
		tr: tr,
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

func (ta *traceAnalyzer) parseTrackEvents(visit func(ts uint64, trackName string, sd *seqData, p *trace.TracePacket) error) error {
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
			ts, err := ta.getTimeNSInDomain(p, uint32(trace.BuiltinClock_BUILTIN_CLOCK_BOOTTIME))
			if err != nil {
				return err
			}

			uuid := sd.uuid
			if d.TrackUuid != nil {
				uuid = *d.TrackUuid
			}

			visit(ts, sd.trackMap[uuid], sd, p)
		}
	}

	return nil
}

// N.B. values with the same clock domain are:
// 1: ts, receivedNS, drawNS
// 2: deadlineTVNS, presentTVNS
type frame struct {
	numExpected int

	// Clock domain 1:
	ts         uint64
	receivedNS []uint64
	drawNS     uint64

	// Clock domain 2:
	wantPresentTVNS uint64
	presentedTVNS   uint64
}

type vblank struct {
	ts            uint64
	presentedTVNS uint64
}

type frameStats struct {
	ta      *traceAnalyzer
	frames  []*frame
	vblanks []vblank
}

func newFrameStats(tr *trace.Trace) (*frameStats, error) {
	ta, err := newTraceAnalyzer(tr)
	if err != nil {
		return nil, err
	}
	return &frameStats{
		ta: ta,
	}, nil
}

func (fs *frameStats) getFrame(ts uint64) *frame {
	closest := int64(-1)
	var closestf *frame
	for _, f := range fs.frames {
		dist := int64(ts) - int64(f.ts)
		if dist >= 0 && (closest == -1 || dist < closest) {
			closest = dist
			closestf = f
		}
	}
	return closestf
}

func (fs *frameStats) parseVizThread(ts uint64, trackName string, sd *seqData, p *trace.TracePacket) error {
	if trackName != "VizCompositorThread" {
		return nil
	}

	// TODO: Add SLICE_BEGIN Stuff back

	d := p.GetTrackEvent()
	name := fs.ta.getEventName(p, d)
	if name == "DisplayScheduler::BeginFrame" {
		f := &frame{ts: ts}
		for _, da := range d.GetDebugAnnotations() {
			if fs.ta.getAnnotationName(p, da) == "args" {
				nv := da.GetNestedValue()
				if nv == nil || len(nv.DictKeys) != len(nv.DictValues) {
					return errors.New("broken trace")
				}

				for i, v := range nv.DictValues {
					if nv.DictKeys[i] == "frame_time_us" {
						f.wantPresentTVNS += 1000 * uint64(v.GetDoubleValue())
					}
					if nv.DictKeys[i] == "interval_us" {
						f.wantPresentTVNS += 1000 * uint64(v.GetDoubleValue())
					}
				}
			}
		}
		fs.frames = append(fs.frames, f)
	}

	if name == "Graphics.Pipeline" {
		f := fs.getFrame(ts)
		if f != nil {
			for _, da := range d.GetDebugAnnotations() {
				if fs.ta.getAnnotationName(p, da) == "step" {
					switch da.GetStringValue() {
					case "IssueBeginFrame":
						f.numExpected++
					case "ReceiveCompositorFrame", "DidNotProduceFrame":
						f.receivedNS = append(f.receivedNS, ts)
					}
				}
			}
		}
	}

	if name == "Display::DrawAndSwap" {
		f := fs.getFrame(ts)
		if f != nil {
			f.drawNS = ts
		}
	}

	return nil
}

func (fs *frameStats) parseDRMThread(ts uint64, trackName string, sd *seqData, p *trace.TracePacket) error {
	if trackName != "DrmThread" {
		return nil
	}

	d := p.GetTrackEvent()
	name := fs.ta.getEventName(p, d)
	if name == "DrmEventFlipComplete" {
		var vblankNS uint64
		for _, da := range d.GetDebugAnnotations() {
			if fs.ta.getAnnotationName(p, da) == "data" {
				nv := da.GetNestedValue()
				if nv == nil || len(nv.DictKeys) != len(nv.DictValues) {
					return errors.New("broken trace")
				}

				for i, v := range nv.DictValues {
					switch nv.DictKeys[i] {
					case "vblank.tv_sec":
						vblankNS += uint64(v.GetIntValue()) * 1000 * 1000 * 1000
					case "vblank.tv_usec":
						vblankNS += uint64(v.GetIntValue()) * 1000
					}
				}
			}
		}
		fs.vblanks = append(fs.vblanks, vblank{ts: ts, presentedTVNS: vblankNS})
	}

	return nil
}

const frameFudge = uint64(2 * 1000 * 1000) // 2ms of frame fudge.

func analyzeFrames(frames []*frame) (float32, error) {
	dropped := 0
	broken := 0
	for _, f := range frames {
		if f.numExpected == 0 {
			continue
		}

		if f.wantPresentTVNS == 0 || f.ts == 0 {
			return 0.0, errors.New("broken trace")
		}

		// Dropped because we missed vblank.
		if f.presentedTVNS > f.wantPresentTVNS+frameFudge {
			dropped++
			// fmt.Println("dropped vblank: ", f.presentedTVNS, f.wantPresentTVNS, float32(f.presentedTVNS - f.wantPresentTVNS) / 1000.0 / 1000.0)
		} else if f.presentedTVNS == 0 || f.drawNS == 0 {
			// fmt.Printf("missing flip or draw - frame %d %q\n", i, f)
			dropped++
			broken++
		} else {
			// Dropped because we didn't hear back from client compositors.
			onTimeReceived := 0
			for _, v := range f.receivedNS {
				if v <= f.drawNS {
					onTimeReceived++
				}
			}
			if onTimeReceived < f.numExpected {
				fmt.Println("dropped from client: ", f.numExpected, onTimeReceived)
				dropped++
			}
		}
	}

	fmt.Println("BROKEN: ", broken, " dropped: ", dropped, " total: ", len(frames))

	return float32(dropped) / float32(len(frames)), nil
}

// 16ms of cost
const missedMatchCost = 16 * 1000 * 1000

func absDiff(a uint64, b uint64) uint64 {
	v := int64(a) - int64(b)
	if v < 0 {
		v = -v
	}
	return uint64(v)
}

func min(a uint64, b uint64) uint64 {
	if a > b {
		return b
	}
	return a
}

func (fs *frameStats) alignVblanks() {
	type state struct {
		i int
		j int
	}
	// DP to match up vblanks with frames:
	dp := make([][]uint64, len(fs.vblanks)+1)
	back := make([][]state, len(fs.vblanks)+1)
	for i := range dp {
		dp[i] = make([]uint64, len(fs.frames)+1)
		back[i] = make([]state, len(fs.frames)+1)
		for j := range dp[i] {
			dp[i][j] = ^uint64(0)
		}
	}

	largestDiff := uint64(0)

	for i := 0; i < len(fs.vblanks)+1; i++ {
		for j := 0; j < len(fs.frames)+1; j++ {
			var vblank *vblank
			var frame *frame
			if i < len(fs.vblanks) {
				vblank = &fs.vblanks[i]
			}
			if j < len(fs.frames) {
				frame = fs.frames[j]
			}
			if vblank != nil && dp[i][j]+missedMatchCost < dp[i+1][j] { // Don't match this vblank.
				back[i+1][j] = state{i: i, j: j}
				dp[i+1][j] = dp[i][j] + missedMatchCost
			}
			if frame != nil && dp[i][j]+missedMatchCost < dp[i][j+1] { // Only consider vblanks that happen after the frame start time.
				back[i][j+1] = state{i: i, j: j}
				dp[i][j+1] = dp[i][j] + missedMatchCost
			}
			// Check this frame should have actually produced a frame:
			// 1. changed content, numExpected > 0
			// 2. actually swapped, drawNS != 0
			if frame != nil && vblank != nil && frame.numExpected > 0 && frame.drawNS != 0 {
				asdf := absDiff(vblank.ts, vblank.presentedTVNS)
				if asdf > largestDiff {
					largestDiff = asdf
				}
				cost := absDiff(vblank.presentedTVNS, frame.wantPresentTVNS) // Deadline should match up to present.
				if vblank.ts >= frame.ts && dp[i][j]+cost < dp[i+1][j+1] {   // Match this frame and vblank.
					back[i+1][j+1] = state{i: i, j: j}
					dp[i+1][j+1] = dp[i][j] + cost
				}
			}
		}
	}

	// Backtrack
	cur := state{
		i: len(fs.vblanks),
		j: len(fs.frames),
	}
	for cur.i > 0 && cur.j > 0 {
		next := back[cur.i][cur.j]
		if cur.i != next.i && cur.j != next.j { // Matched a frame and vblank.
			fs.frames[next.j].presentedTVNS = fs.vblanks[next.i].presentedTVNS
		}
		cur = next
	}
}

func (fs *frameStats) computeProportionDroppedFrames() (float32, error) {
	err := fs.ta.parseTrackEvents(func(ts uint64, trackName string, sd *seqData, p *trace.TracePacket) error {
		return fs.parseVizThread(ts, trackName, sd, p)
	})
	if err != nil {
		return 0.0, err
	}
	err = fs.ta.parseTrackEvents(func(ts uint64, trackName string, sd *seqData, p *trace.TracePacket) error {
		return fs.parseDRMThread(ts, trackName, sd, p)
	})
	if err != nil {
		return 0.0, err
	}

	fs.alignVblanks()

	// Ignore the last frame as it may not be complete.
	v, err := analyzeFrames(fs.frames[:len(fs.frames)-1])
	if err != nil {
		return 0.0, err
	}

	return v, nil
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
	p, err := fs.computeProportionDroppedFrames()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Percent dropped: %.2f%%\n", p*100)
}
