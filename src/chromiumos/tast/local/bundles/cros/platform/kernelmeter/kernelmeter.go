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
	"time"

	"chromiumos/tast/testing"
)

// Meter collects kernel performance statistics.
type Meter struct {
	isStopped        bool            // true after the meter has been stopped
	stop             chan struct{}   // closed (by client) to request stop
	stopped          chan struct{}   // closed when collection has stopped
	pfm              *pageFaultMeter // page-fault tracking data
	sampleCycleCount int64           // Count of sampling cycles
}

// pageFaultMeter collects page fault statistics.
type pageFaultMeter struct {
	startTime        time.Time // time of collection start
	startCount       int64     // page fault counter at start
	sampleStartTime  time.Time // start time of sample period for sample rate
	sampleStartCount int64     // page fault counter at start of sample period
	maxRate          float64   // max seen page fault rate
}

const samplePeriod = 1 * time.Second // length of sample period for max rate calculation

// New creates a Meter and starts the sampling goroutine.
func New(ctx context.Context) *Meter {
	meter := &Meter{
		pfm: newPageFaultMeter(),
	}
	meter.start(ctx)
	return meter
}

// initialize initializes the page fault meter.
func newPageFaultMeter() *pageFaultMeter {
	now := time.Now()
	count := totalFaults()
	return &pageFaultMeter{
		startTime:        now,
		startCount:       count,
		sampleStartTime:  now,
		sampleStartCount: count,
		maxRate:          0.0,
	}
}

// totalFaults returns the total number of major page faults since boot.
// Panics if any error occurs, since we expect the kernel to function properly.
func totalFaults() int64 {
	bytes, err := ioutil.ReadFile("/proc/vmstat")
	if err != nil {
		panic(fmt.Sprint("Cannot read /proc/vmstat: ", err))
	}
	var value string
	for _, line := range strings.Split(string(bytes), "\n") {
		if strings.HasPrefix(line, "pgmajfault ") {
			value = strings.Split(line, " ")[1]
			break
		}
	}
	if len(value) == 0 {
		panic("Cannot find pgmajfault in /proc/vmstat")
	}
	count, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("Cannot parse pgmajfault value %q: %v", value, err))
	}
	return count
}

// Start starts the kernel meter, which samples the page fault rate
// periodically according to samplePeriod to track its max value.
func (m *Meter) start(ctx context.Context) {
	m.stop = make(chan struct{})
	m.stopped = make(chan struct{})
	go func() {
		testing.ContextLog(ctx, "Kernel meter thread has started")
		defer func() {
			close(m.stopped)
			testing.ContextLog(ctx, "Kernel meter thread has stopped")
		}()
		for {
			select {
			case <-m.stop:
				// Wait for at least one sample cycle before
				// stopping.  The m.stop channel is closed so
				// it will also fire next time around.
				if m.sampleCycleCount >= 1 {
					return
				}
			case <-time.After(samplePeriod):
			case <-ctx.Done():
				return
			}
			m.sampleCycleCount++
			count := totalFaults()
			now := time.Now()
			// Use milliseconds resolution for rate computation
			interval := now.Sub(m.pfm.startTime).Seconds()
			if interval > 0 {
				rate := float64(count-m.pfm.startCount) / interval
				if rate > m.pfm.maxRate {
					m.pfm.maxRate = rate
				}
			}
			m.pfm.sampleStartTime = now
			m.pfm.sampleStartCount = count
		}
	}()
}

// PageFaultStats stops the sampler and returns the total number of page
// faults, and the average and max page fault rate.  If called too soon, it may
// block for a short time until a value for the max rate can be computed.
func (m *Meter) PageFaultStats(ctx context.Context) (faultCount int64, averageRage, maxRate float64) {
	if m.isStopped {
		panic("PageFaultStats may only be called once per Meter")
	}
	// Send stop request to the goroutine.
	close(m.stop)
	// Wait for the goroutine to finish.
	select {
	case <-m.stopped:
	case <-ctx.Done():
	}
	m.isStopped = true
	count := totalFaults() - m.pfm.startCount
	interval := time.Now().Sub(m.pfm.startTime).Seconds()
	return count, float64(count) / interval, m.pfm.maxRate
}
