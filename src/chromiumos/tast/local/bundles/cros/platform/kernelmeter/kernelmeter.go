// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kernelmeter provides a mechanism for collecting kernel-related
// measurements in parallel with the execution of a test.
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
	"chromiumos/tast/testing"
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
func (counter *vmCounter) reset() {
	counter.startCount = counter.count
	counter.sampleStartCount = counter.count
	counter.maxRate = 0.0
}

// updateMax updates the max rate of increase seen.
func (counter *vmCounter) updateMax(interval time.Duration) {
	rate := float64(counter.count-counter.sampleStartCount) / interval.Seconds()
	counter.sampleStartCount = counter.count
	if rate > counter.maxRate {
		counter.maxRate = rate
	}
}

// VMCounterData contains statistics for a memory manager event counter, such
// as the page fault counter (pgmajfault in /proc/vmstat).
type VMCounterData struct {
	Count       int64   // how many events have occurred
	AverageRate float64 // average rate (increase/second) for the duration of the sampling
	MaxRate     float64 // max rate seen during the sampling (increase/second over samplePeriod inttervals)
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
	PageFaultData, SwapInData, SwapOutData, OomData VMCounterData
}

const samplePeriod = 1 * time.Second // length of sample period for max rate calculation

// New creates a Meter and starts the sampling goroutine.
func New(ctx context.Context) *Meter {
	vmsm := newVMStatsMeter()
	vmsm.reset()
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

func newVMStatsMeter() *vmStatsMeter {
	vmsm := &vmStatsMeter{counters: make(map[string]*vmCounter)}
	for _, n := range vmStatsNames {
		vmsm.counters[n] = &vmCounter{}
	}
	return vmsm
}

// Reset initializes or resets a vmStatsMeter.
func (vmsm *vmStatsMeter) reset() {
	now := time.Now()

	vmsm.mutex.Lock()
	defer vmsm.mutex.Unlock()

	vmsm.updateCounts()
	vmsm.startTime = now
	vmsm.sampleStartTime = now

	for _, n := range vmStatsNames {
		vmsm.counters[n].reset()
	}
}

// sampleMax tracks the maximum vm counter rates seen after a reset.
func (vmsm *vmStatsMeter) sampleMax() {
	vmsm.mutex.Lock()
	defer vmsm.mutex.Unlock()

	now := time.Now()
	interval := now.Sub(vmsm.sampleStartTime)
	// If called too soon, there's nothing to do.
	if interval.Seconds() == 0.0 {
		return
	}
	vmsm.updateCounts()
	for _, n := range vmStatsNames {
		vmsm.counters[n].updateMax(interval)
	}
	vmsm.sampleStartTime = now
}

// counterToData converts a vmCounter to a vmCounterData.
func (vmsm *vmStatsMeter) counterToData(counterName string, interval time.Duration) VMCounterData {
	counter := vmsm.counters[counterName]
	delta := counter.count - counter.startCount
	return VMCounterData{
		Count:       counter.count,
		AverageRate: float64(delta) / interval.Seconds(),
		MaxRate:     counter.maxRate,
	}
}

// stats returns the vm counter stats since the last reset.
func (vmsm *vmStatsMeter) stats() (*VMStatsData, error) {
	vmsm.mutex.Lock()
	defer vmsm.mutex.Unlock()

	interval := time.Now().Sub(vmsm.startTime)
	if interval.Seconds() == 0.0 {
		return nil, errors.New("calling VMCounterStats too soon")
	}

	vmsm.updateCounts()
	return &VMStatsData{
		PageFaultData: vmsm.counterToData("pgmajfault", interval),
		SwapInData:    vmsm.counterToData("pswpin", interval),
		SwapOutData:   vmsm.counterToData("pswpout", interval),
		OomData:       vmsm.counterToData("oom_kill", interval),
	}, nil
}

// updateCounts updates the values of selected fields of /proc/vmstat.
// Panics if any error occurs, since we expect the kernel to function properly.
// The values of fields that are not found in /proc/vmstat are left unchanged.
func (vmsm *vmStatsMeter) updateCounts() {
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
		if _, present := vmsm.counters[name]; !present {
			continue
		}
		count, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			panic(fmt.Sprintf("Cannot parse %q value %q: %v", name, value, err))
		}
		vmsm.counters[name].count = count
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
	testing.ContextLog(ctx, "Kernel meter goroutine has started")
	defer func() {
		close(m.stopped)
		testing.ContextLog(ctx, "Kernel meter goroutine has stopped")
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
