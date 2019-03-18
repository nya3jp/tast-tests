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

// vmStatsMeter collects vm counter statistics.
type vmStatsMeter struct {
	startTime       time.Time             // time of collection start
	sampleStartTime time.Time             // start time of sample period for sample rate
	mutex           sync.Mutex            // for safe access of all variables
	counters        map[string]*vmCounter // names/values of fields of interest in /proc/vmstat
}

// vmCounter is used in keeping track of average and max rates for various
// /proc/vmstat counters.
type vmCounter struct {
	count            int64   // current vm counter value
	startCount       int64   // vm counter value at start
	sampleStartCount int64   // vm counter value at start of sample period
	maxRate          float64 // max seen vm counter rate in increase per second
}

// reset resets a vmCounter for reuse.  Should be called shortly after
// updateCounts.
func (c *vmCounter) reset() {
	c.startCount = c.count
	c.sampleStartCount = c.count
	c.maxRate = 0.0
}

// updateMax updates the max rate of increase seen.  interval is the time
// elapsed since the last reset.
func (c *vmCounter) updateMax(interval time.Duration) {
	rate := float64(c.count-c.sampleStartCount) / interval.Seconds()
	c.sampleStartCount = c.count
	if rate > c.maxRate {
		c.maxRate = rate
	}
}

// toCounterData produces a vmCounterData from a vmCounter.  interval is
// the time delta for computing the average rate.
func (c *vmCounter) toCounterData(interval time.Duration) VMCounterData {
	delta := c.count - c.startCount
	return VMCounterData{
		Count:       delta,
		AverageRate: float64(delta) / interval.Seconds(),
		MaxRate:     c.maxRate,
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
}

// The names of stats if interest.  These match fields in /proc/vmstat.
var vmStatsNames = []string{
	"pgmajfault",
	"pswpin",
	"pswpout",
	"oom_kill",
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

	v.updateCounts()
	v.startTime = now
	v.sampleStartTime = now

	for _, n := range vmStatsNames {
		v.counters[n].reset()
	}
}

// sampleMax updates the maximum vm counter rates seen after a reset.
func (v *vmStatsMeter) sampleMax() {
	v.mutex.Lock()
	defer v.mutex.Unlock()

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

// updateCounts updates the values of selected fields of /proc/vmstat.
// Panics if any error occurs, since we expect the kernel to function properly.
// The values of fields that are not found in /proc/vmstat are left unchanged.
func (v *vmStatsMeter) updateCounts() {
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
		if _, present := v.counters[name]; !present {
			continue
		}
		count, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			panic(fmt.Sprintf("Cannot parse %q value %q: %v", name, value, err))
		}
		v.counters[name].count = count
		seen[name] = struct{}{}
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
