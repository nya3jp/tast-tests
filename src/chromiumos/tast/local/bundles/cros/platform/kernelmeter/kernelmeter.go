// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kernelmeter provides a mechanism for collecting kernel-related
// measurements in parallel with the execution of a test.
//
// Several kernel quantities (e.g page faults, swaps) are exposed via sysfs or
// procfs in the form of counters.  We are generally interested in the absolute
// increments of these values over a period of time, and their rate of change.
// A kernelmeter.Meter instance keeps track of the initial values of the
// counters so that deltas can be computed.  It also calculates the peak rate
// over an interval.  Additionally, various methods are available for reading
// snapshots of other exported kernel quantities.
package kernelmeter

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/errors"
)

// Meter collects kernel performance statistics.
type Meter struct {
	isClosed bool          // true after the meter has been closed
	stop     chan struct{} // closed (by client) to request stop
	stopped  chan struct{} // closed by collection goroutine when it exits
	vmsm     *vmStatsMeter // tracks various memory manager counters
}

// The VM fields to be collected.
const (
	pfIndex = iota
	swapInIndex
	swapOutIndex
	oomIndex
	vmFieldsLength
)

// vmCounters contains a set of values of interest from /proc/vmstat.
type vmCounters [vmFieldsLength]uint64

// vmSample contains a snapshot (time + values) from /proc/vmstat.
type vmSample struct {
	time   time.Time
	fields vmCounters
}

// vmFieldIndices maps the names of vmstat fields of interest to indices into
// vmSample vectors.
var vmFieldIndices = map[string]int{
	"pgmajfault": pfIndex,
	"pswpin":     swapInIndex,
	"pswpout":    swapOutIndex,
	"oom_kill":   oomKillIndex,
}

const (
	// Length of window for moving averages as a multiple of the sampling period.
	vmCountWindowLength = 10
	// Number of samples in circular buffer.
	sampleBufferLength = vmCountWindowLength + 1
)

// vmStatsMeter collects vm counter statistics.
type vmStatsMeter struct {
	startSample vmSample                     // initial values at collection start
	samples     [sampleBufferLength]vmSample // circular buffer of recent samples
	sampleIndex int                          // index of most recent sample in buffer
	sampleCount int                          // count of valid samples in buffer (for startup)
	maxRates    [vmFieldsLength]float64      // max seen counter rate (delta per second)
	mutex       sync.Mutex                   // for safe access of all variables
}

// reset resets a vmStatsMeter.  Should be called immediately after
// acquireSample, so that the latest sample is up to date.  Note that this
// resets the start time and max rates seen, but keeps old values in the
// circular buffer used to compute the moving average.
func (v *vmStatsMeter) reset(now time.Time) {
	v.startSample = v.samples[sampleIndex]
	for i := range vmFieldsLength {
		maxRates[i] = 0.0
	}
}

// acquireSample adds a new sample to the circular buffer, and tracks the
// number of valid entries in the buffer.
func (v *vmStatsMeter) aquireSample() {
	if v.sampleCount < sampleBufferLength {
		v.sampleCount++
	}
	v.sampleIndex = (c.sampleIndex + 1) % sampleBufferLength
	v.samples[sampleIndex].read()
}

// updateMaxRates updates the max rate of increase seen for each counter.
func (v *vmStatsMeter) updateMaxRates() {
	currentTime := v.samples[v.sampleIndex].time
	previousIndex := (v.sampleIndex - 1 + sampleBufferLength) % sampleBufferLength
	previousTime := v.samples[previousIndex].time
	for i := range vmFieldsLength {
		currentCount := v.samples[v.sampleIndex].fields[i]
		previousCount := v.samples[previousIndex].fields[i]
		rate := float64(currentCount-previousCount) / (currentTime.Seconds() - previousTime.Seconds())
		if rate > v.maxRate[i] {
			v.maxRate[i] = rate
		}
	}
}

// toCounterData produces a vmCounterData from a vmCounter.  interval is
// the time delta for computing the average rate.
func (c *vmCounter) toCounterData(interval time.Duration) VMCounterData {
	delta := c.count - c.startCount
	// If n samples are available, use the most recent and the least recent
	// which is n-1 slots away.  Avoid taking the modulo of a negative
	// number.
	oldIndex := (c.sampleIndex - (c.sampleCount - 1) + sampleBufferLength) % sampleBufferLength
	delta := c.count - c.sampleStartCounts[oldIndex]
	return VMCounterData{
		Count:       delta,
		AverageRate: float64(delta) / interval.Seconds(),
		MaxRate:     c.maxRate,
		RecentRate:  float64(longDelta) / (interval.Seconds() + float64(c.sampleCount-1)),
	}
}

// VMCounterData contains statistics for a memory manager event counter, such
// as the page fault counter (pgmajfault in /proc/vmstat).
type VMCounterData struct {
	// Count is the number of events since the last reset.
	Count int64
	// AverageRate is the average rate (increase/second) for the duration
	// of the sampling.
	AverageRate float64
	// MaxRate is the maximum rate seen during the sampling
	// (increase/second over samplePeriod intervals).
	MaxRate float64
	// RecentRate is the average rate in the most recent window with size
	// vmCountWindowLength periods (or slightly more), or however many
	// periods are available since the most recent reset, including the
	// most recent sample.
	RecentRate float64
}

// VMStatsData contains statistics for various memory manager counters.
// The fields of VMStatsData must match the names and indices above.
type VMStatsData struct {
	// PageFault reports major page fault count and rates.
	PageFault VMCounterData
	// SwapIn reports swapin count and rates.
	SwapIn VMCounterData
	// SwapOut reports swapout count and rates.
	SwapOut VMCounterData
	// OOM reports out-of-memory kill count and rates.
	OOM VMCounterData
}

const samplePeriod = 1 * time.Second // length of sample period for max rate calculation

// New creates a Meter and starts the sampling goroutine.
func New(ctx context.Context) *Meter {
	vmsm := newVMStatsMeter()
	m := &Meter{
		vmsm:    vmsm,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go m.start(ctx)
	return m
}

// Close stops the sampling goroutine and releases other resources.
func (m *Meter) Close(ctx context.Context) {
	if m.isClosed {
		panic("Closing already closed kernelmeter")
	}
	// Send stop request to the goroutine.
	close(m.stop)
	// Wait for the goroutine to finish.
	select {
	case <-m.stopped:
	case <-ctx.Done():
	}
	m.isClosed = true
}

// Reset resets a Meter so that it is ready for a new set of measurements.
func (m *Meter) Reset() {
	m.vmsm.reset()
}

// newVMStatsMeter returns a vmStatsMeter instance.
func newVMStatsMeter() *vmStatsMeter {
	v := &vmStatsMeter{counters: make(map[string]*vmCounter)}
	for _, n := range vmStatsNames {
		v.counters[n] = &vmCounter{}
	}
	v.reset()
	return v
}

// reset resets a vmStatsMeter by updating its counts and start times with
// current values.
func (v *vmStatsMeter) reset() {
	now := time.Now()

	v.mutex.Lock()
	defer v.mutex.Unlock()
}

// sampleMax updates the maximum vm counter rates seen after a reset.
func (v *vmStatsMeter) sampleMax() {

	now := time.Now()
	interval := now.Sub(v.sampleStartTime)
	// If called too soon, there's nothing to do.
	if interval.Seconds() == 0.0 {
		return
	}
	v.updateCounts()
	for _, n := range vmStatsNames {
		v.counters[n].updateMax(interval)
	}
	v.sampleStartTime = now
}

// stats returns the vm counter stats since the last reset.
func (v *vmStatsMeter) stats() (*VMStatsData, error) {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	interval := time.Now().Sub(v.startTime)
	if interval.Seconds() == 0.0 {
		return nil, errors.New("calling VMCounterStats too soon")
	}

	v.updateCounts()
	return &VMStatsData{
		PageFault: v.counters["pgmajfault"].toCounterData(interval),
		SwapIn:    v.counters["pswpin"].toCounterData(interval),
		SwapOut:   v.counters["pswpout"].toCounterData(interval),
		OOM:       v.counters["oom_kill"].toCounterData(interval),
	}, nil
}

// read stores the current time and current values of selected fields of
// /proc/vmstat into s.  Panics if any error occurs, since we expect the kernel
// to function properly.  The values of fields that are not found in
// /proc/vmstat are left unchanged.
func (s *vmSample) acquireSample() {
	s.time = time.Now()
	bytes, err := ioutil.ReadFile("/proc/vmstat")
	if err != nil {
		panic(fmt.Sprint("Cannot read /proc/vmstat: ", err))
	}
	seen := make(map[string]struct{})
	for _, line := range strings.Split(string(bytes), "\n") {
		if len(line) == 0 {
			// at last line
			break
		}
		nameValue := strings.Split(line, " ")
		name := nameValue[0]
		value := nameValue[1]
		if i, present := vmFieldIndices[name]; !present {
			continue
		}
		count, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			panic(fmt.Sprintf("Cannot parse %q value %q: %v", name, value, err))
		}
		s.fields[i] = count
		if len(seen) == len(vmStatsNames) {
			break
		}
	}
}

// start starts the kernel meter, which periodically samples various memory
// manager quantities (such as page fault counts) and tracks the max values of
// their rate of change.
func (m *Meter) start(ctx context.Context) {
	defer func() {
		close(m.stopped)
	}()
	for {
		select {
		case <-time.After(samplePeriod):
		case <-m.stop:
			return
		case <-ctx.Done():
			return
		}
		m.vmsm.sampleMax()
	}
}

// VMStats returns the total number of events, and the average and
// max rates, for various memory manager events.
func (m *Meter) VMStats() (*VMStatsData, error) {
	return m.vmsm.stats()
}
