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
	isClosed         bool            // true after the meter has been closed
	hasTimedOut      bool            // true if the meter has timed out
	stop             chan struct{}   // closed (by client) to request stop
	stopped          chan struct{}   // closed by collection goroutine when it exits
	pfm              *pageFaultMeter // page-fault tracking data
	sampleCycleCount int64           // Count of sampling cycles
}

// pageFaultMeter collects page fault statistics.
type pageFaultMeter struct {
	startTime        time.Time  // time of collection start
	startCount       int64      // page fault counter at start
	sampleStartTime  time.Time  // start time of sample period for sample rate
	sampleStartCount int64      // page fault counter at start of sample period
	maxRate          float64    // max seen page fault rate
	mutex            sync.Mutex // for safe access of all variables
}

// PageFaultData is used to return page fault statistics.
type PageFaultData struct {
	Count       int64   // how many page faults have occurred
	AverageRate float64 // average rate for the duration of the sampling
	MaxRate     float64 // max rate seen in a short interval during the sampling
}

const samplePeriod = 1 * time.Second // length of sample period for max rate calculation

// New creates a Meter and starts the sampling goroutine.
func New(ctx context.Context) *Meter {
	pfm := &pageFaultMeter{}
	pfm.reset()
	m := &Meter{
		pfm:     pfm,
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
		m.isClosed = true
	case <-ctx.Done():
		m.hasTimedOut = true
	}
}

// Reset resets a Meter so that it is ready for a new set of measurements.
func (m *Meter) Reset() {
	m.pfm.reset()
}

// Reset initializes or resets a pageFaultMeter.
func (pfm *pageFaultMeter) reset() {
	now := time.Now()
	count := totalFaults()
	pfm.mutex.Lock()
	pfm.startTime = now
	pfm.startCount = count
	pfm.sampleStartTime = now
	pfm.sampleStartCount = count
	pfm.maxRate = 0.0
	pfm.mutex.Unlock()
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

// start starts the kernel meter, which samples the page fault rate
// periodically according to samplePeriod to track its max value.
func (m *Meter) start(ctx context.Context) {
	testing.ContextLog(ctx, "Kernel meter goroutine has started")
	defer func() {
		close(m.stopped)
		testing.ContextLog(ctx, "Kernel meter goroutine has stopped")
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
		m.pfm.mutex.Lock()
		interval := now.Sub(m.pfm.startTime).Seconds()
		if interval > 0 {
			rate := float64(count-m.pfm.startCount) / interval
			if rate > m.pfm.maxRate {
				m.pfm.maxRate = rate
			}
		}
		m.pfm.sampleStartTime = now
		m.pfm.sampleStartCount = count
		m.pfm.mutex.Unlock()
	}
}

// PageFaultStats returns the total number of page faults, and the average and
// max page fault rate.  If called too soon, it may block for a short time
// until a value for the max rate can be computed.
func (m *Meter) PageFaultStats(ctx context.Context) (*PageFaultData, error) {
	if m.isClosed {
		panic("Kernelmeter is closed")
	}
	if m.hasTimedOut {
		return nil, errors.New("Page fault stats not available after timeout")
	}
	m.pfm.mutex.Lock()
	count := totalFaults() - m.pfm.startCount
	interval := time.Now().Sub(m.pfm.startTime).Seconds()
	maxRate := m.pfm.maxRate
	m.pfm.mutex.Unlock()
	return &PageFaultData{
		Count:       count,
		AverageRate: float64(count) / interval,
		MaxRate:     maxRate,
	}, nil
}
