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
	startTime         time.Time              // time of collection start
	sampleStartTime   time.Time              // start time of sample period for sample rate
	mutex             sync.Mutex             // for safe access of all variables
	startCounts       [vmStatsLength]int64   // vm counter values at start
	sampleStartCounts [vmStatsLength]int64   // vm counter values at start of sample period
	maxRates          [vmStatsLength]float64 // max seen vm counter rate
	counts            map[string]int64       // names/values of fields of interest in /proc/vmstat
}

// VMCounterData contains statistics for a memory manager event counter, such
// as the page fault counter (pgmajfault in /proc/vmstat).
type VMCounterData struct {
	Count       int64   // how many events have occurred
	AverageRate float64 // average rate for the duration of the sampling
	MaxRate     float64 // max rate seen in a short interval during the sampling
}

// The VM fields to be collected.
const (
	pfIndex = iota
	swapInIndex
	swapOutIndex
	oomIndex
	vmStatsLength
)

// The names in vmStatsNames must match the indices above.
var vmStatsNames = [vmStatsLength]string{
	"pgmajfault",
	"pswpin",
	"pswpout",
	"oom_kill",
}

// VMStatsData contains statistics for various memory manager counters.
// The fields of VMStatsData must match the names and indices above.
type VMStatsData struct {
	PageFaultData,
	SwapInData,
	SwapOutData,
	OomData VMCounterData
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
	vmsm := &vmStatsMeter{}
	vmsm.counts = make(map[string]int64)
	for _, n := range vmStatsNames {
		vmsm.counts[n] = 0
	}
	return vmsm
}

// Reset initializes or resets a vmStatsMeter.
func (vmsm *vmStatsMeter) reset() {
	now := time.Now()
	vmsm.updateCounts()

	vmsm.mutex.Lock()
	defer vmsm.mutex.Unlock()

	vmsm.startTime = now
	vmsm.sampleStartTime = now

	for i, n := range vmStatsNames {
		count := vmsm.counts[n]
		vmsm.startCounts[i] = count
		vmsm.sampleStartCounts[i] = count
		vmsm.maxRates[i] = 0.0
	}
}

// sampleMax tracks the maximum vm counter rates seen after a reset.
func (vmsm *vmStatsMeter) sampleMax() {
	vmsm.updateCounts()
	now := time.Now()

	vmsm.mutex.Lock()
	defer vmsm.mutex.Unlock()

	interval := now.Sub(vmsm.sampleStartTime).Seconds()
	// If called to soon, there's nothing to do.
	if interval == 0 {
		return
	}
	for i, n := range vmStatsNames {
		count := vmsm.counts[n]
		rate := float64(count-vmsm.sampleStartCounts[i]) / interval
		vmsm.sampleStartCounts[i] = count
		if rate > vmsm.maxRates[i] {
			vmsm.maxRates[i] = rate
		}
	}
	vmsm.sampleStartTime = now
}

// stats returns the vm counter stats since the last reset.
func (vmsm *vmStatsMeter) stats() (*VMStatsData, error) {
	vmsm.updateCounts()
	vmsm.mutex.Lock()
	defer vmsm.mutex.Unlock()

	interval := time.Now().Sub(vmsm.startTime).Seconds()
	if interval == 0.0 {
		return nil, errors.New("calling VMCounterStats too soon")
	}
	vmcd := [vmStatsLength]VMCounterData{}
	for i, n := range vmStatsNames {
		count := vmsm.counts[n]
		vmcd[i].Count = count
		delta := count - vmsm.startCounts[i]
		vmcd[i].AverageRate = float64(delta) / interval
		vmcd[i].MaxRate = vmsm.maxRates[i]
	}
	return &VMStatsData{
		PageFaultData: vmcd[pfIndex],
		SwapInData:    vmcd[swapInIndex],
		SwapOutData:   vmcd[swapOutIndex],
		OomData:       vmcd[oomIndex],
	}, nil
}

// updateCounts updates the values of selected fields of /proc/vmstat.
// Panics if any error occurs, since we expect the kernel to function properly.
// The values of fields that are missing is left unchanged.
func (vmsm *vmStatsMeter) updateCounts() {
	bytes, err := ioutil.ReadFile("/proc/vmstat")
	if err != nil {
		panic(fmt.Sprint("Cannot read /proc/vmstat: ", err))
	}
	missing := vmStatsLength
	for _, line := range strings.Split(string(bytes), "\n") {
		if len(line) == 0 {
			// at last line
			break
		}
		nameValue := strings.Split(line, " ")
		name := nameValue[0]
		value := nameValue[1]
		_, present := vmsm.counts[name]
		if present {
			count, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				panic(fmt.Sprintf("Cannot parse %q value %q: %v", name, value, err))
			}
			vmsm.counts[name] = count
			missing--
			if missing == 0 {
				break
			}
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
